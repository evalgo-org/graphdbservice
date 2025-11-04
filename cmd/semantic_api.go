package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

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
// @Accept json
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
	// Parse raw JSON to determine format
	var rawJSON map[string]interface{}
	if err := c.Bind(&rawJSON); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse JSON: %v", err))
	}

	// Check if it's JSON-LD by looking for @context or @type
	if _, hasContext := rawJSON["@context"]; hasContext {
		return handleJSONLDAction(c, rawJSON)
	}

	if actionType, hasType := rawJSON["@type"]; hasType {
		if typeStr, ok := actionType.(string); ok && isSemanticActionType(typeStr) {
			return handleJSONLDAction(c, rawJSON)
		}
	}

	// Fallback to legacy format
	return echo.NewHTTPError(http.StatusBadRequest, "Request must be Schema.org JSON-LD with @type field")
}

// isSemanticActionType checks if a type string is a semantic action
func isSemanticActionType(t string) bool {
	switch t {
	case "TransferAction", "CreateAction", "DeleteAction", "UpdateAction", "UploadAction", "ItemList", "ScheduledAction":
		return true
	default:
		return false
	}
}

// handleJSONLDAction processes a JSON-LD action
func handleJSONLDAction(c echo.Context, data map[string]interface{}) error {
	// Get @type to determine action kind
	actionType, ok := data["@type"].(string)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Missing or invalid @type in JSON-LD")
	}

	// Re-marshal to bytes for parsing
	jsonData, err := json.Marshal(data)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to marshal JSON-LD: %v", err))
	}

	// Route based on action type
	switch actionType {
	case "TransferAction":
		return executeSemanticTransferAction(c, jsonData)

	case "CreateAction":
		return executeSemanticCreateAction(c, jsonData)

	case "DeleteAction":
		return executeSemanticDeleteAction(c, jsonData)

	case "UpdateAction":
		return executeSemanticUpdateAction(c, jsonData)

	case "UploadAction":
		return executeSemanticUploadAction(c, jsonData)

	case "ItemList":
		return executeSemanticItemList(c, jsonData)

	case "ScheduledAction":
		// ScheduledAction wraps another action - extract the inner action
		return handleScheduledAction(c, jsonData)

	default:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported action type: %s", actionType))
	}
}

// executeSemanticTransferAction handles TransferAction (repo-migration, graph-migration)
func executeSemanticTransferAction(c echo.Context, jsonData []byte) error {
	var action semantic.TransferAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse TransferAction: %v", err))
	}

	// Determine if it's repo migration or graph migration
	// Check if fromLocation/toLocation are repositories or if object is a graph
	if action.Object != nil {
		// Graph migration: transfer specific graph
		return executeGraphMigration(c, &action)
	}

	// Repository migration: transfer entire repository
	return executeRepositoryMigration(c, &action)
}

// executeRepositoryMigration performs a full repository migration
func executeRepositoryMigration(c echo.Context, action *semantic.TransferAction) error {
	// Extract source repository
	srcRepo, err := extractRepository(action.FromLocation)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid fromLocation: %v", err))
	}

	// Extract target repository
	tgtRepo, err := extractRepository(action.ToLocation)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid toLocation: %v", err))
	}

	// Get credentials
	srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid source credentials: %v", err))
	}
	srcURL = normalizeURL(srcURL) // Remove trailing slash

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid target credentials: %v", err))
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Migration failed: %v", err))
	}

	// Return semantic response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "TransferAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	})
}

// executeGraphMigration performs a graph migration
func executeGraphMigration(c echo.Context, action *semantic.TransferAction) error {
	// Extract source repository
	srcRepo, err := extractRepository(action.FromLocation)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid fromLocation: %v", err))
	}

	// Extract target repository
	tgtRepo, err := extractRepository(action.ToLocation)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid toLocation: %v", err))
	}

	// Extract graph
	graph, err := extractGraph(action.Object)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid object (graph): %v", err))
	}

	// Get credentials
	srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid source credentials: %v", err))
	}
	srcURL = normalizeURL(srcURL) // Remove trailing slash

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid target credentials: %v", err))
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Graph migration failed: %v", err))
	}

	// Return semantic response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "TransferAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	})
}

// executeSemanticCreateAction handles CreateAction (repo-create)
func executeSemanticCreateAction(c echo.Context, jsonData []byte) error {
	var action semantic.CreateAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse CreateAction: %v", err))
	}

	// Extract repository from result
	repo, err := extractRepository(action.Result)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid result: %v", err))
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid credentials: %v", err))
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Creation failed: %v", err))
	}

	// Return semantic response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "CreateAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	})
}

// executeSemanticDeleteAction handles DeleteAction (repo-delete, graph-delete)
func executeSemanticDeleteAction(c echo.Context, jsonData []byte) error {
	var action semantic.DeleteAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse DeleteAction: %v", err))
	}

	// Try to parse as repository first
	repo, repoErr := extractRepository(action.Object)
	if repoErr == nil {
		// Repository deletion
		tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid credentials: %v", err))
		}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Deletion failed: %v", err))
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "DeleteAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		})
	}

	// Try to parse as graph
	graph, graphErr := extractGraph(action.Object)
	if graphErr == nil {
		// Graph deletion - need repository info from IncludedInDataCatalog
		if graph.IncludedInDataCatalog == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Graph must include includedInDataCatalog")
		}

		graphURI := semantic.ExtractGraphIdentifier(graph)
		repoURL := graph.IncludedInDataCatalog.URL
		repoName := graph.IncludedInDataCatalog.Identifier

		// Extract credentials from properties
		props := graph.IncludedInDataCatalog.Properties
		if props == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Missing credentials in includedInDataCatalog")
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
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Graph deletion failed: %v", err))
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "DeleteAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		})
	}

	return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid object: must be repository or graph. Repo error: %v, Graph error: %v", repoErr, graphErr))
}

// executeSemanticUpdateAction handles UpdateAction (repo-rename, graph-rename)
func executeSemanticUpdateAction(c echo.Context, jsonData []byte) error {
	var action semantic.UpdateAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse UpdateAction: %v", err))
	}

	// Try to parse as repository rename
	repo, repoErr := extractRepository(action.Object)
	if repoErr == nil {
		tgtURL, tgtUser, tgtPass, oldRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid credentials: %v", err))
		}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

		task := Task{
			Action: "repo-rename",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				RepoOld:  oldRepoName,
				RepoNew:  action.TargetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Rename failed: %v", err))
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "UpdateAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		})
	}

	// Try to parse as graph rename
	graph, graphErr := extractGraph(action.Object)
	if graphErr == nil {
		if graph.IncludedInDataCatalog == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Graph must include includedInDataCatalog")
		}

		oldGraphURI := semantic.ExtractGraphIdentifier(graph)
		repoURL := graph.IncludedInDataCatalog.URL
		repoName := graph.IncludedInDataCatalog.Identifier

		props := graph.IncludedInDataCatalog.Properties
		if props == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Missing credentials in includedInDataCatalog")
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
				GraphNew: action.TargetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Graph rename failed: %v", err))
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "UpdateAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		})
	}

	return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid object: must be repository or graph. Repo error: %v, Graph error: %v", repoErr, graphErr))
}

// executeSemanticUploadAction handles UploadAction (graph-import, repo-import)
func executeSemanticUploadAction(c echo.Context, jsonData []byte) error {
	var action semantic.UploadAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse UploadAction: %v", err))
	}

	// Extract target repository
	targetRepo, err := extractRepository(action.Target)
	if err != nil {
		// Try extracting from DataCatalog
		if catalog, ok := action.Target.(*semantic.DataCatalog); ok {
			props := catalog.Properties
			if props == nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Missing credentials in target")
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
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid target: %v", err))
		}
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(targetRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid target credentials: %v", err))
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

	// Check if it's graph import or repo import
	graph, graphErr := extractGraph(action.Object)
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
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Graph import failed: %v", err))
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "UploadAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		})
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
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Repository import failed: %v", err))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "UploadAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	})
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
func executeSemanticItemList(c echo.Context, jsonData []byte) error {
	debugLog("executeSemanticItemList called")

	var workflow ItemListWorkflow
	if err := json.Unmarshal(jsonData, &workflow); err != nil {
		debugLog("Failed to unmarshal ItemList: %v\n", err)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse ItemList: %v", err))
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
	switch actionType {
	case "TransferAction":
		return executeTransferActionDirect(jsonData)
	case "DeleteAction":
		return executeDeleteActionDirect(jsonData)
	case "CreateAction":
		return executeCreateActionDirect(jsonData)
	case "UpdateAction":
		return executeUpdateActionDirect(jsonData)
	case "UploadAction":
		return executeUploadActionDirect(jsonData)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}

// executeDeleteActionDirect executes a DeleteAction and returns the result directly
func executeDeleteActionDirect(jsonData []byte) (map[string]interface{}, error) {
	debugLog("executeDeleteActionDirect called")

	var action semantic.DeleteAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		debugLog("Failed to unmarshal DeleteAction: %v\n", err)
		return nil, fmt.Errorf("failed to parse DeleteAction: %w", err)
	}

	debugLog("DeleteAction identifier: %s\n", action.Identifier)

	// Try to parse as repository first
	repo, repoErr := extractRepository(action.Object)
	if repoErr == nil {
		// Repository deletion
		debugLog("Detected repository deletion")
		tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			debugLog("Failed to extract credentials: %v\n", err)
			return nil, fmt.Errorf("invalid credentials: %w", err)
		}
		tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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

		return map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "DeleteAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		}, nil
	}

	// Try to parse as graph
	graph, graphErr := extractGraph(action.Object)
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

		return map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "DeleteAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		}, nil
	}

	return nil, fmt.Errorf("invalid object: must be repository or graph")
}

// executeTransferActionDirect executes a TransferAction and returns the result directly
func executeTransferActionDirect(jsonData []byte) (map[string]interface{}, error) {
	var action semantic.TransferAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return nil, fmt.Errorf("failed to parse TransferAction: %w", err)
	}

	if action.Object != nil {
		// Graph migration
		srcRepo, err := extractRepository(action.FromLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid fromLocation: %w", err)
		}

		tgtRepo, err := extractRepository(action.ToLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid toLocation: %w", err)
		}

		graph, err := extractGraph(action.Object)
		if err != nil {
			return nil, fmt.Errorf("invalid object (graph): %w", err)
		}

		srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
		if err != nil {
			return nil, fmt.Errorf("invalid source credentials: %w", err)
		}
	srcURL = normalizeURL(srcURL) // Remove trailing slash

		tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
		if err != nil {
			return nil, fmt.Errorf("invalid target credentials: %w", err)
		}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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

		return map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "TransferAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		}, nil
	}

	// Repository migration
	srcRepo, err := extractRepository(action.FromLocation)
	if err != nil {
		return nil, fmt.Errorf("invalid fromLocation: %w", err)
	}

	tgtRepo, err := extractRepository(action.ToLocation)
	if err != nil {
		return nil, fmt.Errorf("invalid toLocation: %w", err)
	}

	srcURL, srcUser, srcPass, srcRepoName, err := semantic.ExtractRepositoryCredentials(srcRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid source credentials: %w", err)
	}
	srcURL = normalizeURL(srcURL) // Remove trailing slash

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid target credentials: %w", err)
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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

	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "TransferAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	}, nil
}

// executeCreateActionDirect executes a CreateAction and returns the result directly
func executeCreateActionDirect(jsonData []byte) (map[string]interface{}, error) {
	var action semantic.CreateAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return nil, fmt.Errorf("failed to parse CreateAction: %w", err)
	}

	repo, err := extractRepository(action.Result)
	if err != nil {
		return nil, fmt.Errorf("invalid result: %w", err)
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(repo)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

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

	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "CreateAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	}, nil
}

// executeUpdateActionDirect executes an UpdateAction and returns the result directly
func executeUpdateActionDirect(jsonData []byte) (map[string]interface{}, error) {
	var action semantic.UpdateAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return nil, fmt.Errorf("failed to parse UpdateAction: %w", err)
	}

	repo, repoErr := extractRepository(action.Object)
	if repoErr == nil {
		tgtURL, tgtUser, tgtPass, oldRepoName, err := semantic.ExtractRepositoryCredentials(repo)
		if err != nil {
			return nil, fmt.Errorf("invalid credentials: %w", err)
		}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

		task := Task{
			Action: "repo-rename",
			Tgt: &Repository{
				URL:      tgtURL,
				Username: tgtUser,
				Password: tgtPass,
				RepoOld:  oldRepoName,
				RepoNew:  action.TargetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("rename failed: %w", err)
		}

		return map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "UpdateAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		}, nil
	}

	graph, graphErr := extractGraph(action.Object)
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
				GraphNew: action.TargetName,
			},
		}

		result, err := processTask(task, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("graph rename failed: %w", err)
		}

		return map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "UpdateAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		}, nil
	}

	return nil, fmt.Errorf("invalid object: must be repository or graph")
}

// executeUploadActionDirect executes an UploadAction and returns the result directly
func executeUploadActionDirect(jsonData []byte) (map[string]interface{}, error) {
	var action semantic.UploadAction
	if err := json.Unmarshal(jsonData, &action); err != nil {
		return nil, fmt.Errorf("failed to parse UploadAction: %w", err)
	}

	targetRepo, err := extractRepository(action.Target)
	if err != nil {
		if catalog, ok := action.Target.(*semantic.DataCatalog); ok {
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
		} else {
			return nil, fmt.Errorf("invalid target: %w", err)
		}
	}

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(targetRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid target credentials: %w", err)
	}
	tgtURL = normalizeURL(tgtURL) // Remove trailing slash

	graph, graphErr := extractGraph(action.Object)
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

		return map[string]interface{}{
			"@context":     "https://schema.org",
			"@type":        "UploadAction",
			"identifier":   action.Identifier,
			"actionStatus": "CompletedActionStatus",
			"result":       result,
		}, nil
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

	return map[string]interface{}{
		"@context":     "https://schema.org",
		"@type":        "UploadAction",
		"identifier":   action.Identifier,
		"actionStatus": "CompletedActionStatus",
		"result":       result,
	}, nil
}

// handleScheduledAction extracts the inner action from a ScheduledAction wrapper
func handleScheduledAction(c echo.Context, jsonData []byte) error {
	var scheduled semantic.SemanticScheduledAction
	if err := json.Unmarshal(jsonData, &scheduled); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse ScheduledAction: %v", err))
	}

	// Extract the actual action from additionalProperty
	if scheduled.Properties == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ScheduledAction missing additionalProperty")
	}

	// The HTTP body should be in additionalProperty.body
	body, ok := scheduled.Properties["body"]
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "ScheduledAction missing body in additionalProperty")
	}

	// Re-marshal and handle the inner action
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to marshal body: %v", err))
	}

	var innerAction map[string]interface{}
	if err := json.Unmarshal(bodyJSON, &innerAction); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse inner action: %v", err))
	}

	return handleJSONLDAction(c, innerAction)
}

// ============================================================================
// Helper Functions
// ============================================================================

// extractRepository extracts a GraphDBRepository from an interface{}
func extractRepository(v interface{}) (*semantic.GraphDBRepository, error) {
	// Re-marshal and unmarshal to get the correct type
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	var repo semantic.GraphDBRepository
	if err := json.Unmarshal(data, &repo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal as repository: %w", err)
	}

	return &repo, nil
}

// extractGraph extracts a GraphDBGraph from an interface{}
func extractGraph(v interface{}) (*semantic.GraphDBGraph, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	var graph semantic.GraphDBGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to unmarshal as graph: %w", err)
	}

	return &graph, nil
}
