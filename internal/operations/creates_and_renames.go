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

// RepoCreateHandler handles repository creation operations
type RepoCreateHandler struct {
	BaseHandler
}

// NewRepoCreateHandler creates a new repository creation handler
func NewRepoCreateHandler(clientMgr *client.Manager) Handler {
	return &RepoCreateHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes repository creation
func (h *RepoCreateHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.Repo == "" {
		return nil, domain.NewValidationError("task", "tgt repository name required")
	}

	if files == nil {
		return nil, domain.NewValidationError("files", "repo-create requires a configuration file")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("repo-create", "failed to get client", err)
	}

	db.HttpClient = client

	cleanup := helpers.NewFileCleanup()
	defer func() { _ = cleanup.Cleanup() }()

	// Get configuration file
	fileKey := fmt.Sprintf(helpers.MultipartConfigKeyFormat, 0)
	taskFiles, exists := files[fileKey]
	if !exists || len(taskFiles) == 0 {
		return nil, fmt.Errorf("no config file found for key %s", fileKey)
	}

	configFile, err := helpers.SaveMultipartFileWithCleanup(taskFiles[0], helpers.TempFileRepoCreatePrefix, cleanup)
	if err != nil {
		return nil, domain.NewOperationError("repo-create", "failed to save config file", err)
	}

	// Create the repository
	err = db.GraphDBRestoreConf(tgtURL, task.Tgt.Username, task.Tgt.Password, configFile)
	if err != nil {
		return nil, domain.NewOperationError("repo-create", fmt.Sprintf("failed to create repository %s", task.Tgt.Repo), err)
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Repository created successfully")
	SetResult(result, "repo", task.Tgt.Repo)
	SetResult(result, "config_file", taskFiles[0].Filename)

	return result, nil
}

// RepoRenameHandler handles repository rename operations
type RepoRenameHandler struct {
	BaseHandler
}

// NewRepoRenameHandler creates a new repository rename handler
func NewRepoRenameHandler(clientMgr *client.Manager) Handler {
	return &RepoRenameHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes repository rename
func (h *RepoRenameHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.RepoOld == "" || task.Tgt.RepoNew == "" {
		return nil, domain.NewValidationError("task", "repo_old and repo_new names required")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("repo-rename", "failed to get client", err)
	}

	db.HttpClient = client

	cleanup := helpers.NewFileCleanup()
	defer func() { _ = cleanup.Cleanup() }()

	// Get backup of old repository
	confFile, err := db.GraphDBRepositoryConf(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.RepoOld)
	if err != nil {
		return nil, domain.NewOperationError("repo-rename", fmt.Sprintf("failed to backup repository %s", task.Tgt.RepoOld), err)
	}
	cleanup.Add(confFile)

	// Update repository name in config
	err = helpers.UpdateRepositoryNameInConfig(confFile, task.Tgt.RepoOld, task.Tgt.RepoNew)
	if err != nil {
		return nil, domain.NewOperationError("repo-rename", "failed to update repository name in config", err)
	}

	// Create new repository with updated config
	err = db.GraphDBRestoreConf(tgtURL, task.Tgt.Username, task.Tgt.Password, confFile)
	if err != nil {
		return nil, domain.NewOperationError("repo-rename", fmt.Sprintf("failed to create renamed repository %s", task.Tgt.RepoNew), err)
	}

	// Delete old repository
	_ = db.GraphDBDeleteRepository(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.RepoOld)

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Repository renamed successfully")
	SetResult(result, "old_name", task.Tgt.RepoOld)
	SetResult(result, "new_name", task.Tgt.RepoNew)

	return result, nil
}

// GraphRenameHandler handles graph rename operations
type GraphRenameHandler struct {
	BaseHandler
}

// NewGraphRenameHandler creates a new graph rename handler
func NewGraphRenameHandler(clientMgr *client.Manager) Handler {
	return &GraphRenameHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes graph rename
func (h *GraphRenameHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.GraphOld == "" || task.Tgt.GraphNew == "" {
		return nil, domain.NewValidationError("task", "graph_old and graph_new names required")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("graph-rename", "failed to get client", err)
	}

	db.HttpClient = client

	cleanup := helpers.NewFileCleanup()
	defer func() { _ = cleanup.Cleanup() }()

	// Export old graph
	tempFile := fmt.Sprintf("%s%s.rdf", helpers.TempFileGraphRenamePrefix, helpers.MD5Hash(task.Tgt.GraphOld))
	err = db.GraphDBExportGraphRdf(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.GraphOld, tempFile)
	if err != nil {
		return nil, domain.NewOperationError("graph-rename", fmt.Sprintf("failed to export graph %s", task.Tgt.GraphOld), err)
	}
	cleanup.Add(tempFile)

	// Verify export file is not empty
	err = helpers.VerifyFileNotEmpty(tempFile)
	if err != nil {
		return nil, domain.NewOperationError("graph-rename", fmt.Sprintf("exported graph %s is empty", task.Tgt.GraphOld), err)
	}

	// Import to new graph
	err = db.GraphDBImportGraphRdf(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.GraphNew, tempFile)
	if err != nil {
		return nil, domain.NewOperationError("graph-rename", fmt.Sprintf("failed to import to graph %s", task.Tgt.GraphNew), err)
	}

	// Delete old graph
	_ = db.GraphDBDeleteGraph(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.GraphOld)

	fileSize := helpers.GetFileSize(tempFile)

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Graph renamed successfully")
	SetResult(result, "repository", task.Tgt.Repo)
	SetResult(result, "old_name", task.Tgt.GraphOld)
	SetResult(result, "new_name", task.Tgt.GraphNew)
	SetResult(result, "file_size_bytes", fileSize)

	return result, nil
}
