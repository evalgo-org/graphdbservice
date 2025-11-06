package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"eve.evalgo.org/semantic"
	"github.com/labstack/echo/v4"
)

// handleSemanticAction is the main handler for semantic JSON-LD GraphDB operations
// It accepts Schema.org compliant JSON-LD actions and executes them
// Endpoint: POST /v1/api/semantic/action
//
// @Summary Execute semantic GraphDB operations
// @Description Accept Schema.org JSON-LD actions for GraphDB operations (TransferAction, CreateAction, etc.)
// @Tags Migration
// @Accept json,multipart/form-data
// @Produce json
// @Param x-api-key header string true "API Key"
// @Param action body object true "Schema.org JSON-LD Action"
// @Success 200 {object} map[string]interface{} "Action executed successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/api/semantic/action [post]
func handleSemanticAction(c echo.Context) error {
	contentType := c.Request().Header.Get("Content-Type")

	// Check if this is a multipart/form-data request
	if contentType != "" && strings.HasPrefix(contentType, "multipart/form-data") {
		return handleSemanticActionMultipart(c)
	}

	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to read request body: %v", err))
	}

	// Parse as SemanticAction
	action, err := semantic.ParseSemanticAction(body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse action: %v", err))
	}

	// Route to appropriate handler based on @type
	switch action.Type {
	case "TransferAction":
		return executeSemanticTransferAction(c, action)
	case "CreateAction":
		return executeSemanticCreateAction(c, action)
	case "DeleteAction":
		return executeSemanticDeleteAction(c, action)
	case "UpdateAction":
		return executeSemanticUpdateAction(c, action)
	case "UploadAction":
		return executeSemanticUploadAction(c, action)
	case "ItemList":
		return executeSemanticItemList(c, action)
	case "ScheduledAction":
		return handleScheduledAction(c, action)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported action type: %s", action.Type))
	}
}

// handleSemanticActionMultipart handles multipart/form-data requests with file uploads
// This is used for operations like CreateAction with config files or UploadAction with data files
func handleSemanticActionMultipart(c echo.Context) error {
	// Parse the multipart request using EVE semantic library
	semanticReq, err := semantic.ParseMultipartSemanticRequest(c.Request())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart request: %v", err))
	}

	// Convert the action interface{} to *SemanticAction
	action, ok := semanticReq.Action.(*semantic.SemanticAction)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action type in multipart request")
	}

	// Convert EVE multipart files to the format expected by processTask
	files := make(map[string][]*multipart.FileHeader)
	for key, fileHeaders := range semanticReq.Files {
		files[key] = fileHeaders
	}

	// Route based on action type and execute with files
	switch action.Type {
	case "CreateAction":
		return executeSemanticCreateActionWithFiles(c, action, files)

	case "UploadAction":
		return executeSemanticUploadActionWithFiles(c, action, files)

	default:
		// For other action types, execute without files
		return executeSemanticActionByType(c, action)
	}
}

// executeSemanticCreateActionWithFiles handles CreateAction with file uploads
func executeSemanticCreateActionWithFiles(c echo.Context, action *semantic.SemanticAction, files map[string][]*multipart.FileHeader) error {
	// Extract repository from result using helper
	repo, err := semantic.GetGraphDBRepositoryFromAction(action, "result")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid result", err)
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid credentials", err)
	}
	tgtURL = normalizeURL(tgtURL)

	// Create task with repository info
	task := Task{
		Action: "repo-create",
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	// The files are passed with key "config" for repository creation
	// Convert to the format expected by processTask: task_0_config
	taskFiles := make(map[string][]*multipart.FileHeader)
	if configFiles, exists := files["config"]; exists && len(configFiles) > 0 {
		taskFiles["task_0_config"] = configFiles
	}

	// Execute the task with files
	result, err := processTask(task, taskFiles, 0)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Creation failed", err)
	}

	// Set result and success status
	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)
	return c.JSON(http.StatusOK, action)
}

// executeSemanticUploadActionWithFiles handles UploadAction with file uploads
func executeSemanticUploadActionWithFiles(c echo.Context, action *semantic.SemanticAction, files map[string][]*multipart.FileHeader) error {
	// Try to extract target repository using helper
	targetRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "target")
	if err != nil {
		// Try extracting from DataCatalog
		catalog, catalogErr := semantic.GetDataCatalogFromAction(action, "target")
		if catalogErr != nil {
			return semantic.ReturnActionError(c, action, "Invalid target", err)
		}

		props := catalog.Properties
		if props == nil {
			return semantic.ReturnActionError(c, action, "Missing credentials in target", nil)
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)
		serverURL, _ := props["serverUrl"].(string)

		targetRepo = &semantic.GraphDBRepository{
			Identifier: catalog.Identifier,
			Properties: map[string]interface{}{
				"serverUrl": serverURL,
				"username":  username,
				"password":  password,
			},
		}
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(targetRepo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid target credentials", err)
	}
	tgtURL = normalizeURL(tgtURL)

	// Check if it's a graph import or repository import
	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		graphURI := semantic.ExtractGraphIdentifier(graph)

		task := Task{
			Action: "graph-import",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				Repo:     tgtRepoName,
				Graph:    graphURI,
			},
		}

		// The files are passed with key "data" for graph import
		// Convert to the format expected by processTask: task_0_files
		taskFiles := make(map[string][]*multipart.FileHeader)
		if dataFiles, exists := files["data"]; exists && len(dataFiles) > 0 {
			taskFiles["task_0_files"] = dataFiles
		}

		result, err := processTask(task, taskFiles, 0)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Graph import failed", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)
		return c.JSON(http.StatusOK, action)
	}

	// Repository import (BRF file)
	task := Task{
		Action: "repo-import",
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	// The files are passed with key "data" for repo import
	// Convert to the format expected by processTask: task_0_files
	taskFiles := make(map[string][]*multipart.FileHeader)
	if dataFiles, exists := files["data"]; exists && len(dataFiles) > 0 {
		taskFiles["task_0_files"] = dataFiles
	}

	result, err := processTask(task, taskFiles, 0)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Repository import failed", err)
	}

	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)
	return c.JSON(http.StatusOK, action)
}

// executeSemanticActionByType routes a SemanticAction to the appropriate handler
func executeSemanticActionByType(c echo.Context, action *semantic.SemanticAction) error {
	switch action.Type {
	case "TransferAction":
		return executeSemanticTransferAction(c, action)
	case "CreateAction":
		return executeSemanticCreateAction(c, action)
	case "DeleteAction":
		return executeSemanticDeleteAction(c, action)
	case "UpdateAction":
		return executeSemanticUpdateAction(c, action)
	case "UploadAction":
		return executeSemanticUploadAction(c, action)
	case "ItemList":
		return executeSemanticItemList(c, action)
	case "ScheduledAction":
		return handleScheduledAction(c, action)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported action type: %s", action.Type))
	}
}

// executeSemanticTransferAction handles TransferAction (repo-migration, graph-migration)
func executeSemanticTransferAction(c echo.Context, action *semantic.SemanticAction) error {
	// Determine if it's repo migration or graph migration by checking for object property
	if _, hasObject := action.Properties["object"]; hasObject {
		// Graph migration: transfer specific graph
		return executeGraphMigration(c, action)
	}

	// Repository migration: transfer entire repository
	return executeRepositoryMigration(c, action)
}

// executeRepositoryMigration performs a full repository migration
func executeRepositoryMigration(c echo.Context, action *semantic.SemanticAction) error {
	// Extract source repository using helper
	srcRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "fromLocation")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid fromLocation", err)
	}

	// Extract target repository using helper
	tgtRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "toLocation")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid toLocation", err)
	}

	// Get credentials
	srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid source credentials", err)
	}
	srcURL = normalizeURL(srcURL)

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid target credentials", err)
	}
	tgtURL = normalizeURL(tgtURL)

	// Create legacy Task for execution
	task := Task{
		Action: "repo-migration",
		Src: &Repository{
			URL:      srcURL,
			Username: srcUser,
			Password: srcPass,
			Repo:     srcRepoName,
		},
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	// Execute the task
	result, err := processTask(task, nil, 0)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Migration failed", err)
	}

	// Set result and success status
	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)
	return c.JSON(http.StatusOK, action)
}

// executeGraphMigration performs a graph migration
func executeGraphMigration(c echo.Context, action *semantic.SemanticAction) error {
	// Extract source repository using helper
	srcRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "fromLocation")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid fromLocation", err)
	}

	// Extract target repository using helper
	tgtRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "toLocation")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid toLocation", err)
	}

	// Extract graph using helper
	graph, err := semantic.GetGraphDBGraphFromAction(action, "object")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid object (graph)", err)
	}

	// Get credentials
	srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid source credentials", err)
	}
	srcURL = normalizeURL(srcURL)

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid target credentials", err)
	}
	tgtURL = normalizeURL(tgtURL)

	graphURI := semantic.ExtractGraphIdentifier(graph)

	// Create legacy Task for execution
	task := Task{
		Action: "graph-migration",
		Src: &Repository{
			URL:      srcURL,
			Username: srcUser,
			Password: srcPass,
			Repo:     srcRepoName,
			Graph:    graphURI,
		},
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
			Graph:    graphURI,
		},
	}

	// Execute the task
	result, err := processTask(task, nil, 0)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Graph migration failed", err)
	}

	// Set result and success status
	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)
	return c.JSON(http.StatusOK, action)
}

// executeSemanticCreateAction handles CreateAction (repo-create)
func executeSemanticCreateAction(c echo.Context, action *semantic.SemanticAction) error {
	// Extract repository from result using helper
	repo, err := semantic.GetGraphDBRepositoryFromAction(action, "result")
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid result", err)
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid credentials", err)
	}
	tgtURL = normalizeURL(tgtURL)

	// Create legacy Task for execution
	task := Task{
		Action: "repo-create",
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	// Execute the task (will handle config file from multipart if present)
	result, err := processTask(task, nil, 0)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Creation failed", err)
	}

	// Set result and success status
	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)
	return c.JSON(http.StatusOK, action)
}

// executeSemanticDeleteAction handles DeleteAction (repo-delete, graph-delete)
func executeSemanticDeleteAction(c echo.Context, action *semantic.SemanticAction) error {
	// Try to parse as repository first using helper
	repo, repoErr := semantic.GetGraphDBRepositoryFromAction(action, "object")
	if repoErr == nil {
		// Repository deletion
		tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Invalid credentials", err)
		}
		tgtURL = normalizeURL(tgtURL)

		task := Task{
			Action: "repo-delete",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				Repo:     tgtRepoName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Deletion failed", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)
		return c.JSON(http.StatusOK, action)
	}

	// Try to parse as graph using helper
	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		// Graph deletion - need repository info from IncludedInDataCatalog
		if graph.IncludedInDataCatalog == nil {
			return semantic.ReturnActionError(c, action, "Graph must include includedInDataCatalog", nil)
		}

		graphURI := semantic.ExtractGraphIdentifier(graph)
		repoURL := graph.IncludedInDataCatalog.URL
		repoName := graph.IncludedInDataCatalog.Identifier

		// Extract credentials from properties
		props := graph.IncludedInDataCatalog.Properties
		if props == nil {
			return semantic.ReturnActionError(c, action, "Missing credentials in includedInDataCatalog", nil)
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)

		task := Task{
			Action: "graph-delete",
			Tgt: &Repository{
				URL:      repoURL,
				Username: username,
				Password: password,
				Repo:     repoName,
				Graph:    graphURI,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Graph deletion failed", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)
		return c.JSON(http.StatusOK, action)
	}

	return semantic.ReturnActionError(c, action, fmt.Sprintf("Invalid object: must be repository or graph. Repo error: %v, Graph error: %v", repoErr, graphErr), nil)
}

// executeSemanticUpdateAction handles UpdateAction (repo-rename, graph-rename)
func executeSemanticUpdateAction(c echo.Context, action *semantic.SemanticAction) error {
	// Get target name using helper
	targetName := semantic.GetTargetNameFromAction(action)

	// Try to parse as repository rename using helper
	repo, repoErr := semantic.GetGraphDBRepositoryFromAction(action, "object")
	if repoErr == nil {
		tgtURL, tgtUser, tgtPass, oldRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Invalid credentials", err)
		}
		tgtURL = normalizeURL(tgtURL)

		task := Task{
			Action: "repo-rename",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				RepoOld:  oldRepoName,
				RepoNew:  targetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Rename failed", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)
		return c.JSON(http.StatusOK, action)
	}

	// Try to parse as graph rename using helper
	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		if graph.IncludedInDataCatalog == nil {
			return semantic.ReturnActionError(c, action, "Graph must include includedInDataCatalog", nil)
		}

		oldGraphURI := semantic.ExtractGraphIdentifier(graph)
		repoURL := graph.IncludedInDataCatalog.URL
		repoName := graph.IncludedInDataCatalog.Identifier

		props := graph.IncludedInDataCatalog.Properties
		if props == nil {
			return semantic.ReturnActionError(c, action, "Missing credentials in includedInDataCatalog", nil)
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)

		task := Task{
			Action: "graph-rename",
			Tgt: &Repository{
				URL:      repoURL,
				Username: username,
				Password: password,
				Repo:     repoName,
				GraphOld: oldGraphURI,
				GraphNew: targetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Graph rename failed", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)
		return c.JSON(http.StatusOK, action)
	}

	return semantic.ReturnActionError(c, action, fmt.Sprintf("Invalid object: must be repository or graph. Repo error: %v, Graph error: %v", repoErr, graphErr), nil)
}

// executeSemanticUploadAction handles UploadAction (graph-import, repo-import)
func executeSemanticUploadAction(c echo.Context, action *semantic.SemanticAction) error {
	// Extract target repository using helper
	targetRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "target")
	if err != nil {
		// Try extracting from DataCatalog using helper
		catalog, catalogErr := semantic.GetDataCatalogFromAction(action, "target")
		if catalogErr != nil {
			return semantic.ReturnActionError(c, action, "Invalid target", err)
		}

		props := catalog.Properties
		if props == nil {
			return semantic.ReturnActionError(c, action, "Missing credentials in target", nil)
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)
		serverURL, _ := props["serverUrl"].(string)

		targetRepo = &semantic.GraphDBRepository{
			Identifier: catalog.Identifier,
			Properties: map[string]interface{}{
				"serverUrl": serverURL,
				"username":  username,
				"password":  password,
			},
		}
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(targetRepo)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Invalid target credentials", err)
	}
	tgtURL = normalizeURL(tgtURL)

	// Check if it's graph import or repo import using helper
	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		// Graph import
		graphURI := semantic.ExtractGraphIdentifier(graph)

		task := Task{
			Action: "graph-import",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				Repo:     tgtRepoName,
				Graph:    graphURI,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return semantic.ReturnActionError(c, action, "Graph import failed", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)
		return c.JSON(http.StatusOK, action)
	}

	// Repository import (BRF file)
	task := Task{
		Action: "repo-import",
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	result, err := processTask(task, nil, 0)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Repository import failed", err)
	}

	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)
	return c.JSON(http.StatusOK, action)
}

// ItemListWorkflow represents a Schema.org ItemList for workflow execution
type ItemListWorkflow struct {
	Context         interface{}    `json:"@context"`
	Type            string         `json:"@type"`
	Identifier      string         `json:"identifier"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Parallel        bool           `json:"parallel"`
	Concurrency     int            `json:"concurrency"`
	ItemListElement []ListItemNode `json:"itemListElement"`
}

// ListItemNode represents a ListItem in the ItemList
type ListItemNode struct {
	Type     string                 `json:"@type"`
	Position int                    `json:"position"`
	Item     map[string]interface{} `json:"item"`
}

// executeSemanticItemList handles ItemList (workflow with multiple actions)
func executeSemanticItemList(c echo.Context, action *semantic.SemanticAction) error {
	debugLog("executeSemanticItemList called")

	// Convert action to ItemListWorkflow
	actionJSON, err := json.Marshal(action)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Failed to marshal action", err)
	}

	var workflow ItemListWorkflow
	if err := json.Unmarshal(actionJSON, &workflow); err != nil {
		debugLog("Failed to unmarshal ItemList: %v\n", err)
		return semantic.ReturnActionError(c, action, "Failed to parse ItemList", err)
	}

	debugLog("Parsed ItemList: %s with %d items\n", workflow.Identifier, len(workflow.ItemListElement))
	debugLog("Parallel=%v, Concurrency=%d\n", workflow.Parallel, workflow.Concurrency)

	if len(workflow.ItemListElement) == 0 {
		debugLog("ItemList has no items")
		return echo.NewHTTPError(http.StatusBadRequest, "ItemList must contain at least one item")
	}

	// Set default concurrency if not specified
	if workflow.Concurrency <= 0 {
		workflow.Concurrency = 1
		debugLog("Set default concurrency to 1")
	}

	var results []map[string]interface{}
	var errors []string

	if workflow.Parallel {
		// Execute actions in parallel with concurrency limit
		debugLog("Executing %d actions in PARALLEL (concurrency: %d)\n", len(workflow.ItemListElement), workflow.Concurrency)
		results, errors = executeActionsParallel(c, workflow.ItemListElement, workflow.Concurrency)
	} else {
		// Execute actions sequentially
		debugLog("Executing %d actions SEQUENTIALLY\n", len(workflow.ItemListElement))
		results, errors = executeActionsSequential(c, workflow.ItemListElement)
	}

	debugLog("Execution complete. Success: %d, Failed: %d\n", len(results)-len(errors), len(errors))

	// Build response
	response := map[string]interface{}{
		"@context":       "https://schema.org",
		"@type":          "ItemList",
		"identifier":     workflow.Identifier,
		"actionStatus":   "CompletedActionStatus",
		"totalItems":     len(workflow.ItemListElement),
		"successfulItems": len(results) - len(errors),
		"failedItems":    len(errors),
		"results":        results,
	}

	if len(errors) > 0 {
		response["errors"] = errors
		response["actionStatus"] = "FailedActionStatus"
	}

	statusCode := http.StatusOK
	if len(errors) == len(workflow.ItemListElement) {
		// All actions failed
		statusCode = http.StatusInternalServerError
	} else if len(errors) > 0 {
		// Some actions failed
		statusCode = http.StatusMultiStatus
	}

	return c.JSON(statusCode, response)
}

// executeActionsSequential executes actions one by one in order
func executeActionsSequential(c echo.Context, items []ListItemNode) ([]map[string]interface{}, []string) {
	results := make([]map[string]interface{}, len(items))
	var errors []string

	for i, listItem := range items {
		result, err := executeWorkflowItem(c, listItem, i)
		if err != nil {
			errorMsg := fmt.Sprintf("Item %d (position %d): %v", i, listItem.Position, err)
			errors = append(errors, errorMsg)
			results[i] = map[string]interface{}{
				"position": listItem.Position,
				"status":   "failed",
				"error":    err.Error(),
			}
		} else {
			results[i] = result
		}
	}

	return results, errors
}

// executeActionsParallel executes actions in parallel with concurrency control
func executeActionsParallel(c echo.Context, items []ListItemNode, concurrency int) ([]map[string]interface{}, []string) {
	results := make([]map[string]interface{}, len(items))
	var errors []string

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, concurrency)

	// Channel to collect results
	type resultPair struct {
		index  int
		result map[string]interface{}
		err    error
	}
	resultChan := make(chan resultPair, len(items))

	// Launch goroutines for each item
	for i, listItem := range items {
		go func(idx int, item ListItemNode) {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := executeWorkflowItem(c, item, idx)
			resultChan <- resultPair{
				index:  idx,
				result: result,
				err:    err,
			}
		}(i, listItem)
	}

	// Collect results
	for i := 0; i < len(items); i++ {
		pair := <-resultChan
		if pair.err != nil {
			errorMsg := fmt.Sprintf("Item %d (position %d): %v", pair.index, items[pair.index].Position, pair.err)
			errors = append(errors, errorMsg)
			results[pair.index] = map[string]interface{}{
				"position": items[pair.index].Position,
				"status":   "failed",
				"error":    pair.err.Error(),
			}
		} else {
			results[pair.index] = pair.result
		}
	}

	return results, errors
}

// executeWorkflowItem executes a single workflow item (typically a ScheduledAction)
func executeWorkflowItem(c echo.Context, listItem ListItemNode, index int) (map[string]interface{}, error) {
	debugLog("executeWorkflowItem called for index %d, position %d\n", index, listItem.Position)

	// Extract the item (which should be a ScheduledAction or direct action)
	itemJSON, err := json.Marshal(listItem.Item)
	if err != nil {
		debugLog("Failed to marshal item: %v\n", err)
		return nil, fmt.Errorf("failed to marshal item: %w", err)
	}

	// Check the @type of the item
	itemType, ok := listItem.Item["@type"].(string)
	if !ok {
		debugLog("Item missing @type field")
		return nil, fmt.Errorf("item missing @type field")
	}

	debugLog("Item @type: %s\n", itemType)

	// Create a temporary response recorder to capture the action result
	// since handleJSONLDAction writes directly to the response
	var actionResult map[string]interface{}

	// For ScheduledAction, extract the body and execute it
	if itemType == "ScheduledAction" {
		debugLog("Handling ScheduledAction - extracting body")

		// Extract additionalProperty.body
		props, ok := listItem.Item["additionalProperty"].(map[string]interface{})
		if !ok {
			debugLog("ScheduledAction missing additionalProperty")
			return nil, fmt.Errorf("ScheduledAction missing additionalProperty")
		}

		body, ok := props["body"]
		if !ok {
			debugLog("ScheduledAction missing body in additionalProperty")
			return nil, fmt.Errorf("ScheduledAction missing body in additionalProperty")
		}

		// Execute the inner action
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}

		var innerAction map[string]interface{}
		if err := json.Unmarshal(bodyJSON, &innerAction); err != nil {
			return nil, fmt.Errorf("failed to unmarshal inner action: %w", err)
		}

		// Execute based on action type
		actionType, ok := innerAction["@type"].(string)
		if !ok {
			debugLog("Inner action missing @type")
			return nil, fmt.Errorf("inner action missing @type")
		}

		debugLog("Executing inner action: %s\n", actionType)

		// Execute the action and capture result
		actionResult, err = executeActionDirect(c, actionType, bodyJSON)
		if err != nil {
			debugLog("Action execution failed: %v\n", err)
			return nil, err
		}

		debugLog("Action executed successfully: %s\n", actionType)

		// Add item metadata to result
		actionResult["position"] = listItem.Position
		actionResult["itemIndex"] = index
		if name, ok := listItem.Item["name"].(string); ok {
			actionResult["itemName"] = name
		}
		if id, ok := listItem.Item["identifier"].(string); ok {
			actionResult["itemIdentifier"] = id
		}

		return actionResult, nil
	}

	// Direct action (not wrapped in ScheduledAction)
	actionResult, err = executeActionDirect(c, itemType, itemJSON)
	if err != nil {
		return nil, err
	}

	actionResult["position"] = listItem.Position
	actionResult["itemIndex"] = index

	return actionResult, nil
}

// executeActionDirect executes an action directly and returns the result
func executeActionDirect(c echo.Context, actionType string, jsonData []byte) (map[string]interface{}, error) {
	// Parse as SemanticAction
	action, err := semantic.ParseSemanticAction(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse action: %w", err)
	}

	switch actionType {
	case "TransferAction":
		return executeTransferActionDirect(action)
	case "DeleteAction":
		return executeDeleteActionDirect(action)
	case "CreateAction":
		return executeCreateActionDirect(action)
	case "UpdateAction":
		return executeUpdateActionDirect(action)
	case "UploadAction":
		return executeUploadActionDirect(action)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}

// executeDeleteActionDirect executes a DeleteAction and returns the result directly
func executeDeleteActionDirect(action *semantic.SemanticAction) (map[string]interface{}, error) {
	debugLog("executeDeleteActionDirect called")
	debugLog("DeleteAction identifier: %s\n", action.Identifier)

	// Try to parse as repository first using helper
	repo, repoErr := semantic.GetGraphDBRepositoryFromAction(action, "object")
	if repoErr == nil {
		// Repository deletion
		debugLog("Detected repository deletion")
		tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			debugLog("Failed to extract credentials: %v\n", err)
			return nil, fmt.Errorf("invalid credentials: %w", err)
		}
		tgtURL = normalizeURL(tgtURL)

		debugLog("Deleting repository: %s at %s\n", tgtRepoName, tgtURL)

		task := Task{
			Action: "repo-delete",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				Repo:     tgtRepoName,
			},
		}

		debugLog("Calling processTask for repo-delete")
		result, err := processTask(task, nil, 0)
		if err != nil {
			debugLog("processTask failed: %v\n", err)
			return nil, fmt.Errorf("deletion failed: %w", err)
		}

		debugLog("Repository deletion successful: %v\n", result)

		// Return as map for workflow compatibility
		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)

		actionMap := make(map[string]interface{})
		actionJSON, _ := json.Marshal(action)
		_ = json.Unmarshal(actionJSON, &actionMap)
		return actionMap, nil
	}

	// Try to parse as graph using helper
	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		// Graph deletion
		if graph.IncludedInDataCatalog == nil {
			return nil, fmt.Errorf("graph must include includedInDataCatalog")
		}

		graphURI := semantic.ExtractGraphIdentifier(graph)
		repoURL := graph.IncludedInDataCatalog.URL
		repoName := graph.IncludedInDataCatalog.Identifier

		props := graph.IncludedInDataCatalog.Properties
		if props == nil {
			return nil, fmt.Errorf("missing credentials in includedInDataCatalog")
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)

		task := Task{
			Action: "graph-delete",
			Tgt: &Repository{
				URL:      repoURL,
				Username: username,
				Password: password,
				Repo:     repoName,
				Graph:    graphURI,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("graph deletion failed: %w", err)
		}

		// Return as map for workflow compatibility
		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)

		actionMap := make(map[string]interface{})
		actionJSON, _ := json.Marshal(action)
		_ = json.Unmarshal(actionJSON, &actionMap)
		return actionMap, nil
	}

	return nil, fmt.Errorf("invalid object: must be repository or graph")
}

// executeTransferActionDirect executes a TransferAction and returns the result directly
func executeTransferActionDirect(action *semantic.SemanticAction) (map[string]interface{}, error) {
	if _, hasObject := action.Properties["object"]; hasObject {
		// Graph migration
		srcRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "fromLocation")
		if err != nil {
			return nil, fmt.Errorf("invalid fromLocation: %w", err)
		}

		tgtRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "toLocation")
		if err != nil {
			return nil, fmt.Errorf("invalid toLocation: %w", err)
		}

		graph, err := semantic.GetGraphDBGraphFromAction(action, "object")
		if err != nil {
			return nil, fmt.Errorf("invalid object (graph): %w", err)
		}

		srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
		if err != nil {
			return nil, fmt.Errorf("invalid source credentials: %w", err)
		}
		srcURL = normalizeURL(srcURL)

		tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
		if err != nil {
			return nil, fmt.Errorf("invalid target credentials: %w", err)
		}
		tgtURL = normalizeURL(tgtURL)

		graphURI := semantic.ExtractGraphIdentifier(graph)

		task := Task{
			Action: "graph-migration",
			Src: &Repository{
				URL:      srcURL,
				Username: srcUser,
				Password: srcPass,
				Repo:     srcRepoName,
				Graph:    graphURI,
			},
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				Repo:     tgtRepoName,
				Graph:    graphURI,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("graph migration failed: %w", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)

		actionMap := make(map[string]interface{})
		actionJSON, _ := json.Marshal(action)
		_ = json.Unmarshal(actionJSON, &actionMap)
		return actionMap, nil
	}

	// Repository migration
	srcRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "fromLocation")
	if err != nil {
		return nil, fmt.Errorf("invalid fromLocation: %w", err)
	}

	tgtRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "toLocation")
	if err != nil {
		return nil, fmt.Errorf("invalid toLocation: %w", err)
	}

	srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid source credentials: %w", err)
	}
	srcURL = normalizeURL(srcURL)

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid target credentials: %w", err)
	}
	tgtURL = normalizeURL(tgtURL)

	task := Task{
		Action: "repo-migration",
		Src: &Repository{
			URL:      srcURL,
			Username: srcUser,
			Password: srcPass,
			Repo:     srcRepoName,
		},
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	result, err := processTask(task, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)

	actionMap := make(map[string]interface{})
	actionJSON, _ := json.Marshal(action)
	_ = json.Unmarshal(actionJSON, &actionMap)
	return actionMap, nil
}

// executeCreateActionDirect executes a CreateAction and returns the result directly
func executeCreateActionDirect(action *semantic.SemanticAction) (map[string]interface{}, error) {
	repo, err := semantic.GetGraphDBRepositoryFromAction(action, "result")
	if err != nil {
		return nil, fmt.Errorf("invalid result: %w", err)
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}
	tgtURL = normalizeURL(tgtURL)

	task := Task{
		Action: "repo-create",
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	result, err := processTask(task, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("creation failed: %w", err)
	}

	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)

	actionMap := make(map[string]interface{})
	actionJSON, _ := json.Marshal(action)
	_ = json.Unmarshal(actionJSON, &actionMap)
	return actionMap, nil
}

// executeUpdateActionDirect executes an UpdateAction and returns the result directly
func executeUpdateActionDirect(action *semantic.SemanticAction) (map[string]interface{}, error) {
	targetName := semantic.GetTargetNameFromAction(action)

	repo, repoErr := semantic.GetGraphDBRepositoryFromAction(action, "object")
	if repoErr == nil {
		tgtURL, tgtUser, tgtPass, oldRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			return nil, fmt.Errorf("invalid credentials: %w", err)
		}
		tgtURL = normalizeURL(tgtURL)

		task := Task{
			Action: "repo-rename",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				RepoOld:  oldRepoName,
				RepoNew:  targetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("rename failed: %w", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)

		actionMap := make(map[string]interface{})
		actionJSON, _ := json.Marshal(action)
		_ = json.Unmarshal(actionJSON, &actionMap)
		return actionMap, nil
	}

	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		if graph.IncludedInDataCatalog == nil {
			return nil, fmt.Errorf("graph must include includedInDataCatalog")
		}

		oldGraphURI := semantic.ExtractGraphIdentifier(graph)
		repoURL := graph.IncludedInDataCatalog.URL
		repoName := graph.IncludedInDataCatalog.Identifier

		props := graph.IncludedInDataCatalog.Properties
		if props == nil {
			return nil, fmt.Errorf("missing credentials in includedInDataCatalog")
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)

		task := Task{
			Action: "graph-rename",
			Tgt: &Repository{
				URL:      repoURL,
				Username: username,
				Password: password,
				Repo:     repoName,
				GraphOld: oldGraphURI,
				GraphNew: targetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("graph rename failed: %w", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)

		actionMap := make(map[string]interface{})
		actionJSON, _ := json.Marshal(action)
		_ = json.Unmarshal(actionJSON, &actionMap)
		return actionMap, nil
	}

	return nil, fmt.Errorf("invalid object: must be repository or graph")
}

// executeUploadActionDirect executes an UploadAction and returns the result directly
func executeUploadActionDirect(action *semantic.SemanticAction) (map[string]interface{}, error) {
	targetRepo, err := semantic.GetGraphDBRepositoryFromAction(action, "target")
	if err != nil {
		catalog, catalogErr := semantic.GetDataCatalogFromAction(action, "target")
		if catalogErr != nil {
			return nil, fmt.Errorf("invalid target: %w", err)
		}

		props := catalog.Properties
		if props == nil {
			return nil, fmt.Errorf("missing credentials in target")
		}

		username, _ := props["username"].(string)
		password, _ := props["password"].(string)
		serverURL, _ := props["serverUrl"].(string)

		targetRepo = &semantic.GraphDBRepository{
			Identifier: catalog.Identifier,
			Properties: map[string]interface{}{
				"serverUrl": serverURL,
				"username":  username,
				"password":  password,
			},
		}
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(targetRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid target credentials: %w", err)
	}
	tgtURL = normalizeURL(tgtURL)

	graph, graphErr := semantic.GetGraphDBGraphFromAction(action, "object")
	if graphErr == nil {
		graphURI := semantic.ExtractGraphIdentifier(graph)

		task := Task{
			Action: "graph-import",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				Repo:     tgtRepoName,
				Graph:    graphURI,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("graph import failed: %w", err)
		}

		action.Properties["result"] = result
		semantic.SetSuccessOnAction(action)

		actionMap := make(map[string]interface{})
		actionJSON, _ := json.Marshal(action)
		_ = json.Unmarshal(actionJSON, &actionMap)
		return actionMap, nil
	}

	task := Task{
		Action: "repo-import",
		Tgt: &Repository{
			URL:      tgtURL,
			Username: tgtUser,
			Password: tgtPass,
			Repo:     tgtRepoName,
		},
	}

	result, err := processTask(task, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("repository import failed: %w", err)
	}

	action.Properties["result"] = result
	semantic.SetSuccessOnAction(action)

	actionMap := make(map[string]interface{})
	actionJSON, _ := json.Marshal(action)
	_ = json.Unmarshal(actionJSON, &actionMap)
	return actionMap, nil
}

// handleScheduledAction extracts the inner action from a ScheduledAction wrapper
func handleScheduledAction(c echo.Context, action *semantic.SemanticAction) error {
	// Extract the actual action from additionalProperty
	if action.Properties == nil {
		return semantic.ReturnActionError(c, action, "ScheduledAction missing additionalProperty", nil)
	}

	// The HTTP body should be in additionalProperty.body
	body, ok := action.Properties["body"]
	if !ok {
		return semantic.ReturnActionError(c, action, "ScheduledAction missing body in additionalProperty", nil)
	}

	// Re-marshal and parse as SemanticAction
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Failed to marshal body", err)
	}

	innerAction, err := semantic.ParseSemanticAction(bodyJSON)
	if err != nil {
		return semantic.ReturnActionError(c, action, "Failed to parse inner action", err)
	}

	return executeSemanticActionByType(c, innerAction)
}

// ============================================================================
// Helper Functions
// ============================================================================
