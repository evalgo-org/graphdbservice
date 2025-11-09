// Package operations implements GraphDB operation handlers.
package operations

import (
	"context"
	"fmt"
	"mime/multipart"
	"os"

	"eve.evalgo.org/db"
	"graphdbservice/internal/client"
	"graphdbservice/internal/domain"
	"graphdbservice/internal/helpers"
)

// RepoMigrationHandler handles repository migration operations
type RepoMigrationHandler struct {
	BaseHandler
}

// NewRepoMigrationHandler creates a new repository migration handler
func NewRepoMigrationHandler(clientMgr *client.Manager) Handler {
	return &RepoMigrationHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes repository migration
func (h *RepoMigrationHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Src == nil || task.Tgt == nil {
		return nil, domain.NewValidationError("task", "both src and tgt repositories required for migration")
	}

	result := Result()
	srcURL := helpers.NormalizeURL(task.Src.URL)
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	// Get HTTP clients
	srcClient, err := h.ClientManager.GetClient(srcURL)
	if err != nil {
		return nil, domain.NewOperationError("repo-migration", "failed to get source client", err)
	}

	tgtClient, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("repo-migration", "failed to get target client", err)
	}

	// Fetch source repository configuration and data
	db.HttpClient = srcClient
	srcGraphDB, err := db.GraphDBRepositories(srcURL, task.Src.Username, task.Src.Password)
	if err != nil {
		return nil, domain.NewOperationError("repo-migration", "failed to fetch source repositories", err)
	}

	confFile, dataFile, err := h.downloadRepositoryBackup(srcURL, task.Src, srcGraphDB)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(confFile) }()
	defer func() { _ = os.Remove(dataFile) }()

	// Restore to target
	db.HttpClient = tgtClient
	err = db.GraphDBRestoreConf(tgtURL, task.Tgt.Username, task.Tgt.Password, confFile)
	if err != nil {
		return nil, domain.NewOperationError("repo-migration", "failed to restore repository configuration", err)
	}

	err = db.GraphDBRestoreBrf(tgtURL, task.Tgt.Username, task.Tgt.Password, dataFile)
	if err != nil {
		return nil, domain.NewOperationError("repo-migration", "failed to restore repository data", err)
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Repository migrated successfully")
	SetResult(result, "src_repo", task.Src.Repo)
	SetResult(result, "tgt_repo", task.Tgt.Repo)
	SetResult(result, "data_size", helpers.GetFileSize(dataFile))

	return result, nil
}

func (h *RepoMigrationHandler) downloadRepositoryBackup(url string, repo *domain.Repository, graphDB *db.GraphDBSparqlResponse) (string, string, error) {
	foundRepo := false
	var confFile, dataFile string
	var err error

	for _, bind := range graphDB.Results.Bindings {
		if id, exists := bind["id"].(map[string]interface{}); exists {
			if value, ok := id["value"].(string); ok && value == repo.Repo {
				foundRepo = true
				confFile, err = db.GraphDBRepositoryConf(url, repo.Username, repo.Password, value)
				if err != nil {
					return "", "", domain.NewOperationError("repo-migration", "failed to download repository config", err)
				}

				dataFile, err = db.GraphDBRepositoryBrf(url, repo.Username, repo.Password, value)
				if err != nil {
					return "", "", domain.NewOperationError("repo-migration", "failed to download repository data", err)
				}
				break
			}
		}
	}

	if !foundRepo {
		return "", "", domain.NewNotFoundError("repository", repo.Repo)
	}

	return confFile, dataFile, nil
}

// GraphMigrationHandler handles graph migration operations
type GraphMigrationHandler struct {
	BaseHandler
}

// NewGraphMigrationHandler creates a new graph migration handler
func NewGraphMigrationHandler(clientMgr *client.Manager) Handler {
	return &GraphMigrationHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes graph migration
func (h *GraphMigrationHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Src == nil || task.Tgt == nil {
		return nil, domain.NewValidationError("task", "both src and tgt required for graph migration")
	}

	if task.Src.Graph == "" || task.Tgt.Graph == "" {
		return nil, domain.NewValidationError("task", "source and target graphs must be specified")
	}

	result := Result()
	srcURL := helpers.NormalizeURL(task.Src.URL)
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	// Get HTTP clients
	srcClient, err := h.ClientManager.GetClient(srcURL)
	if err != nil {
		return nil, domain.NewOperationError("graph-migration", "failed to get source client", err)
	}

	tgtClient, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("graph-migration", "failed to get target client", err)
	}

	cleanup := helpers.NewFileCleanup()
	defer func() { _ = cleanup.Cleanup() }()

	// Export source graph
	tempGraphFile := fmt.Sprintf("%s%s.brf", helpers.TempFileRepoRenamePrefix, helpers.MD5Hash(task.Src.Graph))
	db.HttpClient = srcClient
	err = db.GraphDBExportGraphRdf(srcURL, task.Src.Username, task.Src.Password, task.Src.Repo, task.Src.Graph, tempGraphFile)
	if err != nil {
		return nil, domain.NewOperationError("graph-migration", "failed to export source graph", err)
	}
	cleanup.Add(tempGraphFile)

	// Delete target graph if it exists
	db.HttpClient = tgtClient
	_ = db.GraphDBDeleteGraph(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph)

	// Import to target
	err = db.GraphDBImportGraphRdf(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph, tempGraphFile)
	if err != nil {
		return nil, domain.NewOperationError("graph-migration", "failed to import graph to target", err)
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Graph migrated successfully")
	SetResult(result, "src_graph", task.Src.Graph)
	SetResult(result, "tgt_graph", task.Tgt.Graph)
	SetResult(result, "data_size", helpers.GetFileSize(tempGraphFile))

	return result, nil
}
