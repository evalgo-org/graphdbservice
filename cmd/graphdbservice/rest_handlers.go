package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// REST endpoint request types

type GraphQueryRequest struct {
	Query string `json:"query"`
}

type CreateNodeRequest struct {
	Labels     []string               `json:"labels"`
	Properties map[string]interface{} `json:"properties"`
}

type UpdateNodeRequest struct {
	Properties map[string]interface{} `json:"properties"`
}

type CreateRelationshipRequest struct {
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// registerRESTEndpoints adds REST endpoints that convert to semantic actions
func registerRESTEndpoints(apiGroup *echo.Group, apiKeyMiddleware echo.MiddlewareFunc) {
	// POST /v1/api/queries - Execute graph query
	apiGroup.POST("/queries", executeQueryREST, apiKeyMiddleware)

	// POST /v1/api/nodes - Create node
	apiGroup.POST("/nodes", createNodeREST, apiKeyMiddleware)

	// PUT /v1/api/nodes/:id - Update node
	apiGroup.PUT("/nodes/:id", updateNodeREST, apiKeyMiddleware)

	// DELETE /v1/api/nodes/:id - Delete node
	apiGroup.DELETE("/nodes/:id", deleteNodeREST, apiKeyMiddleware)

	// POST /v1/api/relationships - Create relationship
	apiGroup.POST("/relationships", createRelationshipREST, apiKeyMiddleware)
}

// executeQueryREST handles REST POST /v1/api/queries
func executeQueryREST(c echo.Context) error {
	var req GraphQueryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid request: %v", err)})
	}

	if req.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	// Convert to JSON-LD SearchAction
	action := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "SearchAction",
		"query":    req.Query,
		"object": map[string]interface{}{
			"@type": "Dataset",
		},
	}

	return callSemanticHandler(c, action)
}

// createNodeREST handles REST POST /v1/api/nodes
func createNodeREST(c echo.Context) error {
	var req CreateNodeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid request: %v", err)})
	}

	if len(req.Labels) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "labels is required"})
	}

	// Convert to JSON-LD CreateAction
	action := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "CreateAction",
		"object": map[string]interface{}{
			"@type":              "Thing",
			"additionalType":     req.Labels,
			"additionalProperty": req.Properties,
		},
	}

	return callSemanticHandler(c, action)
}

// updateNodeREST handles REST PUT /v1/api/nodes/:id
func updateNodeREST(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	var req UpdateNodeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid request: %v", err)})
	}

	if req.Properties == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "properties is required"})
	}

	// Convert to JSON-LD UpdateAction
	action := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "UpdateAction",
		"object": map[string]interface{}{
			"@type":              "Thing",
			"identifier":         id,
			"additionalProperty": req.Properties,
		},
	}

	return callSemanticHandler(c, action)
}

// deleteNodeREST handles REST DELETE /v1/api/nodes/:id
func deleteNodeREST(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	// Convert to JSON-LD DeleteAction
	action := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "DeleteAction",
		"object": map[string]interface{}{
			"@type":      "Thing",
			"identifier": id,
		},
	}

	return callSemanticHandler(c, action)
}

// createRelationshipREST handles REST POST /v1/api/relationships
func createRelationshipREST(c echo.Context) error {
	var req CreateRelationshipRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid request: %v", err)})
	}

	if req.From == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "from is required"})
	}
	if req.To == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "to is required"})
	}
	if req.Type == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type is required"})
	}

	// Convert to JSON-LD CreateAction for relationship
	action := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "CreateAction",
		"object": map[string]interface{}{
			"@type":     "Role",
			"name":      req.Type,
			"startDate": req.From,
			"endDate":   req.To,
		},
	}

	if req.Properties != nil {
		action["object"].(map[string]interface{})["additionalProperty"] = req.Properties
	}

	return callSemanticHandler(c, action)
}

// callSemanticHandler converts action to JSON and calls the semantic action handler
func callSemanticHandler(c echo.Context, action map[string]interface{}) error {
	// Marshal action to JSON
	actionJSON, err := json.Marshal(action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to marshal action: %v", err)})
	}

	// Create new request with JSON-LD body
	newReq := c.Request().Clone(c.Request().Context())
	newReq.Body = io.NopCloser(bytes.NewReader(actionJSON))
	newReq.Header.Set("Content-Type", "application/json")

	// Create new context with modified request
	newCtx := c.Echo().NewContext(newReq, c.Response())
	newCtx.SetPath(c.Path())
	newCtx.SetParamNames(c.ParamNames()...)
	newCtx.SetParamValues(c.ParamValues()...)

	// Call the existing semantic action handler
	return handleSemanticAction(newCtx)
}
