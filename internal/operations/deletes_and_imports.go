// Package operations implements GraphDB operation handlers.
package operations

import (
	"context"
	"fmt"
	"mime/multipart"

	"eve.evalgo.org/db"
	"graphdbservice/internal/client"
	"graphdbservice/internal/domain"
	"graphdbservice/internal/helpers"
)

// RepoDeleteHandler handles repository deletion operations
type RepoDeleteHandler struct {
	BaseHandler
}

// NewRepoDeleteHandler creates a new repository deletion handler
func NewRepoDeleteHandler(clientMgr *client.Manager) Handler {
	return &RepoDeleteHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes repository deletion
func (h *RepoDeleteHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.Repo == "" {
		return nil, domain.NewValidationError("task", "tgt repository name required")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("repo-delete", "failed to get client", err)
	}

	db.HttpClient = client
	err = db.GraphDBDeleteRepository(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
	if err != nil {
		return nil, domain.NewOperationError("repo-delete", fmt.Sprintf("failed to delete repository %s", task.Tgt.Repo), err)
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Repository deleted successfully")
	SetResult(result, "repo", task.Tgt.Repo)

	return result, nil
}

// GraphDeleteHandler handles graph deletion operations
type GraphDeleteHandler struct {
	BaseHandler
}

// NewGraphDeleteHandler creates a new graph deletion handler
func NewGraphDeleteHandler(clientMgr *client.Manager) Handler {
	return &GraphDeleteHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes graph deletion
func (h *GraphDeleteHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.Graph == "" {
		return nil, domain.NewValidationError("task", "tgt graph name required")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("graph-delete", "failed to get client", err)
	}

	db.HttpClient = client
	err = db.GraphDBDeleteGraph(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph)
	if err != nil {
		return nil, domain.NewOperationError("graph-delete", fmt.Sprintf("failed to delete graph %s", task.Tgt.Graph), err)
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Graph deleted successfully")
	SetResult(result, "graph", task.Tgt.Graph)

	return result, nil
}

// RepoImportHandler handles repository import operations
type RepoImportHandler struct {
	BaseHandler
}

// NewRepoImportHandler creates a new repository import handler
func NewRepoImportHandler(clientMgr *client.Manager) Handler {
	return &RepoImportHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes repository import
func (h *RepoImportHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.Repo == "" {
		return nil, domain.NewValidationError("task", "tgt repository name required")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("repo-import", "failed to get client", err)
	}

	db.HttpClient = client

	cleanup := helpers.NewFileCleanup()
	defer func() { _ = cleanup.Cleanup() }()

	// Handle file uploads
	if files != nil {
		fileKey := fmt.Sprintf(helpers.MultipartFilesKeyFormat, 0)
		if taskFiles, exists := files[fileKey]; exists && len(taskFiles) > 0 {
			brfFile, err := helpers.SaveMultipartFileWithCleanup(taskFiles[0], helpers.TempFileRepoImportPrefix, cleanup)
			if err != nil {
				return nil, domain.NewOperationError("repo-import", "failed to save uploaded file", err)
			}

			err = db.GraphDBRestoreBrf(tgtURL, task.Tgt.Username, task.Tgt.Password, brfFile)
			if err != nil {
				return nil, domain.NewOperationError("repo-import", "failed to import BRF file", err)
			}

			SetResult(result, "imported_file", taskFiles[0].Filename)
		}
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Repository import completed successfully")
	SetResult(result, "target_repository", task.Tgt.Repo)

	return result, nil
}

// GraphImportHandler handles graph import operations
type GraphImportHandler struct {
	BaseHandler
}

// NewGraphImportHandler creates a new graph import handler
func NewGraphImportHandler(clientMgr *client.Manager) Handler {
	return &GraphImportHandler{
		BaseHandler: BaseHandler{ClientManager: clientMgr},
	}
}

// Handle executes graph import
func (h *GraphImportHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	if task.Tgt == nil || task.Tgt.Graph == "" {
		return nil, domain.NewValidationError("task", "tgt graph name required")
	}

	if files == nil {
		return nil, domain.NewValidationError("files", "graph-import requires uploaded files")
	}

	result := Result()
	tgtURL := helpers.NormalizeURL(task.Tgt.URL)

	client, err := h.ClientManager.GetClient(tgtURL)
	if err != nil {
		return nil, domain.NewOperationError("graph-import", "failed to get client", err)
	}

	db.HttpClient = client

	cleanup := helpers.NewFileCleanup()
	defer func() { _ = cleanup.Cleanup() }()

	// Process uploaded files
	fileKey := fmt.Sprintf(helpers.MultipartFilesKeyFormat, 0)
	taskFiles, exists := files[fileKey]
	if !exists || len(taskFiles) == 0 {
		return nil, fmt.Errorf("no files found for key %s", fileKey)
	}

	processedCount := 0
	fileNames := make([]string, 0)

	for i, fileHeader := range taskFiles {
		rdfFile, err := helpers.SaveMultipartFileWithCleanup(fileHeader, helpers.TempFileGraphImportPrefix, cleanup)
		if err != nil {
			helpers.DebugLog("Failed to save file %s: %v", fileHeader.Filename, err)
			continue
		}

		err = db.GraphDBImportGraphRdf(tgtURL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph, rdfFile)
		if err != nil {
			helpers.DebugLog("Failed to import file %s: %v", fileHeader.Filename, err)
			continue
		}

		processedCount++
		fileNames = append(fileNames, fileHeader.Filename)
		SetResult(result, fmt.Sprintf("file_%d_processed", i), fileHeader.Filename)
		SetResult(result, fmt.Sprintf("file_%d_type", i), helpers.GetFileType(fileHeader.Filename))
	}

	if processedCount == 0 {
		return nil, domain.NewOperationError("graph-import", "failed to import any files", nil)
	}

	SetResult(result, "status", "completed")
	SetResult(result, "message", "Graph imported successfully")
	SetResult(result, "graph", task.Tgt.Graph)
	SetResult(result, "uploaded_files", processedCount)
	SetResult(result, "file_names", fileNames)

	return result, nil
}
