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

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid target credentials: %v", err))
	}

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

	tgtURL, tgtUser, tgtPass, tgtRepoName, err := semantic.ExtractRepositoryCredentials(tgtRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid target credentials: %v", err))
	}

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

// executeSemanticItemList handles ItemList (workflow with multiple actions)
func executeSemanticItemList(c echo.Context, jsonData []byte) error {
	// TODO: Implement workflow execution
	// This would execute multiple actions in sequence or parallel
	return echo.NewHTTPError(http.StatusNotImplemented, "ItemList workflows not yet implemented")
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
