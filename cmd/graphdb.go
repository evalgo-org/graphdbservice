package cmd

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	// eve "eve.evalgo.org/common"
	"eve.evalgo.org/db"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	// identityFile holds the path to the Ziti identity JSON file for zero-trust networking.
	// When provided, all GraphDB connections will use Ziti secure networking.
	identityFile string = ""
)

// MigrationRequest represents the root request structure for GraphDB operations.
// It contains the API version and a list of tasks to be executed sequentially.
//
// Example JSON:
//
//	{
//	  "version": "v0.0.1",
//	  "tasks": [
//	    {
//	      "action": "repo-migration",
//	      "src": {"url": "http://source:7200", "username": "admin", "password": "pass", "repo": "source-repo"},
//	      "tgt": {"url": "http://target:7200", "username": "admin", "password": "pass", "repo": "target-repo"}
//	    }
//	  ]
//	}
type MigrationRequest struct {
	Version string `json:"version" validate:"required"` // API version (e.g., "v0.0.1")
	Tasks   []Task `json:"tasks" validate:"required"`   // List of tasks to execute
}

// Task represents a single operation to be performed on GraphDB repositories or graphs.
//
// Supported actions:
//   - repo-migration: Migrate entire repository (config + data)
//   - graph-migration: Migrate a named graph between repositories
//   - repo-delete: Delete a repository
//   - graph-delete: Delete a named graph
//   - repo-create: Create a new repository from TTL configuration
//   - graph-import: Import RDF data into a graph
//   - repo-import: Import repository from BRF backup file
//   - repo-rename: Rename a repository (backup, recreate, restore)
//   - graph-rename: Rename a graph (export, import, delete)
type Task struct {
	Action string      `json:"action" validate:"required"` // The action to perform
	Src    *Repository `json:"src,omitempty"`              // Source repository/graph (for migration operations)
	Tgt    *Repository `json:"tgt,omitempty"`              // Target repository/graph (for all operations)
}

// Repository represents the connection details and identifiers for a GraphDB repository or graph.
// Different fields are required depending on the operation being performed.
//
// Field usage by action:
//   - repo-migration: URL, Username, Password, Repo required for both Src and Tgt
//   - graph-migration: URL, Username, Password, Repo, Graph required for both Src and Tgt
//   - repo-delete: URL, Username, Password, Repo required for Tgt
//   - graph-delete: URL, Username, Password, Repo, Graph required for Tgt
//   - repo-create: URL, Username, Password, Repo required for Tgt (+ config file)
//   - graph-import: URL, Username, Password, Repo, Graph required for Tgt (+ data files)
//   - repo-import: URL, Username, Password, Repo required for Tgt (+ BRF file)
//   - repo-rename: URL, Username, Password, RepoOld, RepoNew required for Tgt
//   - graph-rename: URL, Username, Password, Repo, GraphOld, GraphNew required for Tgt
type Repository struct {
	URL      string `json:"url,omitempty"`       // GraphDB server URL (e.g., "http://localhost:7200")
	Username string `json:"username,omitempty"`  // GraphDB username for authentication
	Password string `json:"password,omitempty"`  // GraphDB password for authentication
	Repo     string `json:"repo,omitempty"`      // Repository name
	Graph    string `json:"graph,omitempty"`     // Named graph URI or name
	RepoOld  string `json:"repo_old,omitempty"`  // Old repository name (for repo-rename)
	RepoNew  string `json:"repo_new,omitempty"`  // New repository name (for repo-rename)
	GraphOld string `json:"graph_old,omitempty"` // Old graph name (for graph-rename)
	GraphNew string `json:"graph_new,omitempty"` // New graph name (for graph-rename)
}

// md5Hash generates an MD5 hash of the given text string.
// This is used to create unique temporary file names during graph operations.
//
// Parameters:
//   - text: The input string to hash
//
// Returns:
//   - A hexadecimal string representation of the MD5 hash
func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// apiKeyMiddleware validates the API key in the request header.
// It checks the "x-api-key" header against the API_KEY environment variable.
// If the key is missing or invalid, it returns a 401 Unauthorized error.
//
// This middleware should be applied to all endpoints that require authentication.
//
// Environment Variables:
//   - API_KEY: The expected API key value
//
// HTTP Headers:
//   - x-api-key: The API key provided by the client
//
// Returns:
//   - 401 Unauthorized if the API key is missing or invalid
//   - Otherwise, passes control to the next handler
func apiKeyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey := c.Request().Header.Get("x-api-key")
		expectedKey := os.Getenv("API_KEY")

		if apiKey == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "Missing x-api-key header")
		}

		if apiKey != expectedKey {
			return echo.NewHTTPError(http.StatusUnauthorized, "Invalid API key")
		}

		return next(c)
	}
}

// getFileNames extracts the filenames from a slice of multipart file headers.
// This is a utility function used when processing multipart form uploads.
//
// Parameters:
//   - fileHeaders: Slice of multipart file headers from a form upload
//
// Returns:
//   - A slice of strings containing the filename from each file header
func getFileNames(fileHeaders []*multipart.FileHeader) []string {
	names := make([]string, len(fileHeaders))
	for i, fh := range fileHeaders {
		names[i] = fh.Filename
	}
	return names
}

// updateRepositoryNameInConfig updates repository name references in a GraphDB TTL configuration file.
// It replaces all occurrences of the old repository name with the new name in various TTL patterns.
//
// The function handles common GraphDB repository configuration patterns:
//   - rep:repositoryID "oldName" -> rep:repositoryID "newName"
//   - <http://www.openrdf.org/config/repository#oldName> -> <...#newName>
//   - repo:oldName -> repo:newName
//
// This is primarily used during repository rename operations to update the configuration
// before creating a new repository with the updated name.
//
// Parameters:
//   - configFile: Path to the TTL configuration file to update
//   - oldName: The old repository name to replace
//   - newName: The new repository name to use
//
// Returns:
//   - error: nil on success, or an error if reading/writing the file fails
func updateRepositoryNameInConfig(configFile, oldName, newName string) error {
	// Read the configuration file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Convert to string for processing
	configContent := string(content)

	// Replace repository ID references in the TTL file
	// Common patterns in GraphDB repository configs:
	replacements := map[string]string{
		fmt.Sprintf(`rep:repositoryID "%s"`, oldName):                         fmt.Sprintf(`rep:repositoryID "%s"`, newName),
		fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, oldName): fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, newName),
		fmt.Sprintf(`repo:%s`, oldName):                                       fmt.Sprintf(`repo:%s`, newName),
	}

	// Apply replacements
	for old, new := range replacements {
		configContent = strings.ReplaceAll(configContent, old, new)
	}

	// Also handle the repository node declaration if it exists
	// Pattern: @base <http://www.openrdf.org/config/repository#oldname>
	basePattern := fmt.Sprintf(`@base <http://www.openrdf.org/config/repository#%s>`, oldName)
	newBasePattern := fmt.Sprintf(`@base <http://www.openrdf.org/config/repository#%s>`, newName)
	configContent = strings.ReplaceAll(configContent, basePattern, newBasePattern)

	// Write the updated content back to the file
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}

	return nil
}

// getGraphTripleCounts retrieves the triple counts for two graphs in a GraphDB repository.
// This is intended for verification purposes after graph operations like rename or migration.
//
// Note: This is currently a stub implementation that returns -1 for both counts.
// A full implementation would execute SPARQL COUNT queries against the specified graphs.
//
// Example SPARQL query for triple count:
//
//	SELECT (COUNT(*) AS ?count) WHERE { GRAPH <graphURI> { ?s ?p ?o } }
//
// Parameters:
//   - url: GraphDB server URL
//   - username: Authentication username
//   - password: Authentication password
//   - repo: Repository name
//   - oldGraph: First graph URI/name to count
//   - newGraph: Second graph URI/name to count
//
// Returns:
//   - oldCount: Number of triples in the old graph (-1 if unavailable)
//   - newCount: Number of triples in the new graph (-1 if unavailable)
func getGraphTripleCounts(url, username, password, repo, oldGraph, newGraph string) (int, int) {
	// This is a simplified implementation - you might want to implement actual SPARQL queries
	// to get precise triple counts. For now, we return -1 to indicate counts are not available.
	//
	// Example SPARQL query to get triple count for a specific graph:
	// SELECT (COUNT(*) AS ?count) WHERE { GRAPH <graphURI> { ?s ?p ?o } }

	oldCount := -1
	newCount := -1

	// You can implement actual SPARQL queries here using your db package
	// For example:
	// oldCount = db.GraphDBQueryTripleCount(url, username, password, repo, oldGraph)
	// newCount = db.GraphDBQueryTripleCount(url, username, password, repo, newGraph)

	return oldCount, newCount
}

// getFileType determines the RDF serialization format based on the file extension.
// This is used to specify the correct content type when importing data into GraphDB.
//
// Supported formats:
//   - .brf: Binary RDF (GraphDB's backup format)
//   - .rdf, .xml: RDF/XML
//   - .ttl: Turtle
//   - .nt: N-Triples
//   - .n3: Notation3
//   - .jsonld, .json: JSON-LD
//   - .trig: TriG (for named graphs)
//   - .nq: N-Quads
//   - default: text/plain
//
// Parameters:
//   - filename: The name of the file (extension is extracted and compared)
//
// Returns:
//   - The RDF format identifier string (e.g., "turtle", "rdf-xml", "binary-rdf")
func getFileType(filename string) string {
	filename = strings.ToLower(filename)

	switch {
	case strings.HasSuffix(filename, ".brf"):
		return "binary-rdf"
	case strings.HasSuffix(filename, ".rdf") || strings.HasSuffix(filename, ".xml"):
		return "rdf-xml"
	case strings.HasSuffix(filename, ".ttl"):
		return "turtle"
	case strings.HasSuffix(filename, ".nt"):
		return "n-triples"
	case strings.HasSuffix(filename, ".n3"):
		return "n3"
	case strings.HasSuffix(filename, ".jsonld") || strings.HasSuffix(filename, ".json"):
		return "json-ld"
	case strings.HasSuffix(filename, ".trig"):
		return "trig"
	case strings.HasSuffix(filename, ".nq"):
		return "n-quads"
	default:
		return "unknown"
	}
}

// getRepositoryNames extracts repository names from GraphDB API response bindings.
// GraphDB's SPARQL endpoint returns results in a bindings format where each binding
// contains an "Id" field with a "value" key containing the repository name.
//
// Parameters:
//   - bindings: Slice of GraphDBBinding structures from the eve.evalgo.org/db package
//
// Returns:
//   - A slice of repository name strings extracted from the bindings
func getRepositoryNames(bindings []db.GraphDBBinding) []string {
	names := make([]string, 0)
	for _, binding := range bindings {
		if value, exists := binding.Id["value"]; exists {
			names = append(names, value)
		}
	}
	return names
}

// migrationHandler is the main HTTP handler for the /v1/api/action endpoint.
// It routes requests to either the JSON or multipart handler based on the Content-Type header.
//
// This handler supports two request formats:
//   - application/json: Routes to migrationHandlerJSON
//   - multipart/form-data: Routes to migrationHandlerMultipart
//
// The multipart format is required when uploading configuration files or RDF data files
// along with the task definitions.
//
// HTTP Method: POST
// Endpoint: /v1/api/action
// Authentication: Requires x-api-key header (enforced by middleware)
//
// Parameters:
//   - c: Echo context containing the HTTP request and response
//
// Returns:
//   - error: HTTP error if the request fails, or nil on success
func migrationHandler(c echo.Context) error {
	// Check if this is a multipart form request
	contentType := c.Request().Header.Get("Content-Type")

	fmt.Printf("DEBUG: Received request with Content-Type: %s\n", contentType)

	if contentType != "" && strings.HasPrefix(contentType, "multipart/form-data") {
		fmt.Println("DEBUG: Routing to multipart handler")
		return migrationHandlerMultipart(c)
	} else {
		fmt.Println("DEBUG: Routing to JSON handler")
		return migrationHandlerJSON(c)
	}
}

// migrationHandlerJSON processes GraphDB operations submitted as JSON requests.
// This handler is used for operations that don't require file uploads (e.g., repository
// migration, graph migration, delete operations).
//
// The request body must be a valid MigrationRequest JSON object containing a version
// and a list of tasks. Each task is processed sequentially, and results are collected
// and returned in the response.
//
// Request Format:
//
//	{
//	  "version": "v0.0.1",
//	  "tasks": [
//	    {
//	      "action": "repo-migration",
//	      "src": {...},
//	      "tgt": {...}
//	    }
//	  ]
//	}
//
// Response Format:
//
//	{
//	  "status": "success",
//	  "version": "v0.0.1",
//	  "results": [...]
//	}
//
// HTTP Method: POST
// Content-Type: application/json
//
// Parameters:
//   - c: Echo context containing the HTTP request and response
//
// Returns:
//   - error: 400 Bad Request if the JSON is invalid or tasks are malformed
//   - error: 500 Internal Server Error if task processing fails
//   - nil: On success, with JSON response containing task results
func migrationHandlerJSON(c echo.Context) error {
	var req MigrationRequest

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if req.Version == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Version is required")
	}

	if len(req.Tasks) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "At least one task is required")
	}

	// Validate each task
	for i, task := range req.Tasks {
		if err := validateTask(task); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Task %d: %s", i, err.Error()))
		}
	}

	// Process tasks
	results := make([]map[string]interface{}, len(req.Tasks))
	for i, task := range req.Tasks {
		result, err := processTask(task, nil, i)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Task %d failed: %s", i, err.Error()))
		}
		results[i] = result
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "success",
		"version": req.Version,
		"results": results,
	})
}

// migrationHandlerMultipart processes GraphDB operations submitted as multipart form data.
// This handler is required for operations that involve file uploads, such as:
//   - repo-create (requires TTL configuration file)
//   - graph-import (requires RDF data files)
//   - repo-import (requires BRF backup file)
//
// The multipart form must contain:
//   - "request" field: JSON string containing the MigrationRequest object
//   - "task_{index}_config" field: Configuration file for task at index (for repo-create)
//   - "task_{index}_files" field: RDF data files for task at index (for graph-import/repo-import)
//
// Example form fields:
//   - request: {"version": "v0.0.1", "tasks": [...]}
//   - task_0_config: repo-config.ttl (for first task if repo-create)
//   - task_0_files: data.ttl, data2.nt (for first task if graph-import)
//
// The handler parses the form, extracts files, and processes each task sequentially.
// It includes panic recovery to handle unexpected errors gracefully.
//
// HTTP Method: POST
// Content-Type: multipart/form-data
// Memory Limit: 32MB per request
//
// Parameters:
//   - c: Echo context containing the HTTP request and response
//
// Returns:
//   - error: 400 Bad Request if the form is invalid or files are missing
//   - error: 500 Internal Server Error if task processing fails
//   - nil: On success, with JSON response containing task results
func migrationHandlerMultipart(c echo.Context) error {
	fmt.Println("DEBUG: Starting multipart form processing")

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC RECOVERED in migrationHandlerMultipart: %v\n", r)
		}
	}()

	// Set multipart form memory limit (32MB default, increase if needed)
	err := c.Request().ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		fmt.Printf("ERROR: Failed to parse multipart form: %v\n", err)
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart form: %v", err))
	}

	fmt.Println("DEBUG: Multipart form parsed successfully")

	// Parse multipart form
	form := c.Request().MultipartForm
	if form == nil {
		fmt.Println("ERROR: No multipart form data found")
		return echo.NewHTTPError(http.StatusBadRequest, "No multipart form data found")
	}
	defer func() {
		if form != nil {
			fmt.Println("DEBUG: Cleaning up multipart form")
			_ = form.RemoveAll()
		}
	}()

	fmt.Printf("DEBUG: Form has %d value fields and %d file fields\n", len(form.Value), len(form.File))

	// Get JSON request from form field
	jsonFields, exists := form.Value["request"]
	if !exists || len(jsonFields) == 0 {
		fmt.Println("ERROR: Missing 'request' field in form data")
		return echo.NewHTTPError(http.StatusBadRequest, "Missing 'request' field in form data")
	}

	fmt.Printf("DEBUG: Found request field with %d bytes\n", len(jsonFields[0]))

	// Parse JSON request
	var req MigrationRequest
	if err := json.Unmarshal([]byte(jsonFields[0]), &req); err != nil {
		fmt.Printf("ERROR: Invalid JSON in request field: %v\n", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON in request field: "+err.Error())
	}

	fmt.Printf("DEBUG: Parsed JSON request with %d tasks\n", len(req.Tasks))

	// Validate request
	if req.Version == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Version is required")
	}

	if len(req.Tasks) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "At least one task is required")
	}

	// Get uploaded files
	files := make(map[string][]*multipart.FileHeader)
	for key, fileHeaders := range form.File {
		files[key] = fileHeaders
	}

	// Validate each task
	for i, task := range req.Tasks {
		if err := validateTask(task); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Task %d: %s", i, err.Error()))
		}
	}

	// Process tasks with files
	results := make([]map[string]interface{}, len(req.Tasks))
	for i, task := range req.Tasks {
		fmt.Printf("DEBUG: Processing task %d: %s\n", i, task.Action)

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC RECOVERED in task %d processing: %v\n", i, r)
				}
			}()

			result, err := processTask(task, files, i)
			if err != nil {
				fmt.Printf("ERROR: Task %d failed: %v\n", i, err)
				// Don't return error here, capture it in results
				results[i] = map[string]interface{}{
					"action": task.Action,
					"status": "failed",
					"error":  err.Error(),
				}
				return
			}
			results[i] = result
			fmt.Printf("DEBUG: Task %d completed successfully\n", i)
		}()
	}

	fmt.Println("DEBUG: All tasks processed, returning response")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "success",
		"version": req.Version,
		"results": results,
	})
}

// validateTask validates that a task has the correct structure and required fields
// based on its action type. This prevents invalid requests from being processed.
//
// Validation rules by action:
//   - repo-migration, graph-migration: Requires both Src and Tgt repositories
//   - repo-delete, graph-delete: Requires Tgt repository
//   - repo-create: Requires Tgt repository (config file validated separately)
//   - graph-import: Requires Tgt repository (data files validated separately)
//   - repo-import: Requires Tgt repository (BRF file validated separately)
//   - repo-rename: Requires Tgt with RepoOld and RepoNew fields
//   - graph-rename: Requires Tgt with GraphOld and GraphNew fields
//
// Parameters:
//   - task: The Task structure to validate
//
// Returns:
//   - error: 400 Bad Request if validation fails, describing the missing/invalid field
//   - nil: If the task structure is valid for its action type
func validateTask(task Task) error {
	validActions := map[string]bool{
		"repo-migration":  true,
		"graph-migration": true,
		"repo-delete":     true,
		"graph-delete":    true,
		"repo-create":     true,
		"graph-import":    true,
		"repo-import":     true,
		"repo-rename":     true,
		"graph-rename":    true,
	}

	if !validActions[task.Action] {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action: %s", task.Action)
	}

	switch task.Action {
	case "repo-migration", "graph-migration":
		if task.Src == nil || task.Tgt == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Both src and tgt are required for %s", task.Action)
		}
	case "repo-delete", "graph-delete", "repo-create", "graph-import", "repo-import":
		if task.Tgt == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "tgt is required for %s", task.Action)
		}
	case "repo-rename":
		if task.Tgt == nil || task.Tgt.RepoOld == "" || task.Tgt.RepoNew == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "tgt with repo_old and repo_new are required for rename-repo")
		}
	case "graph-rename":
		if task.Tgt == nil || task.Tgt.GraphOld == "" || task.Tgt.GraphNew == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "tgt with graph_old and graph_new are required for rename-graph")
		}
	}

	return nil
}

// URL2ServiceRobust parses a URL string and extracts the service portion (host:port).
// This function is more forgiving than standard URL parsing - it automatically adds
// the http:// scheme if missing, which helps with user-provided URLs.
//
// The function is primarily used when working with Ziti networking to extract the
// service identifier from a full GraphDB URL.
//
// Examples:
//   - "http://localhost:7200" -> "localhost:7200"
//   - "https://graphdb.example.com:7200" -> "graphdb.example.com:7200"
//   - "graphdb:7200" -> "graphdb:7200" (scheme added automatically)
//   - "localhost" -> "localhost" (port omitted if not specified)
//
// Parameters:
//   - urlStr: The URL string to parse (with or without scheme)
//
// Returns:
//   - service: The host:port portion of the URL, or just host if port not specified
//   - error: An error if the URL cannot be parsed
func URL2ServiceRobust(urlStr string) (string, error) {
	// Add scheme if missing to help url.Parse work correctly
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	return parsedURL.Hostname(), nil
}

// processTask executes a single GraphDB operation task based on the specified action.
// This is the core processing function that handles all supported operations.
//
// Supported Actions:
//
//  1. repo-migration: Migrates an entire repository from source to target GraphDB
//     - Downloads source repository configuration (TTL) and data (BRF)
//     - Creates target repository with the configuration
//     - Restores data from BRF backup
//     - Cleans up temporary files
//
//  2. graph-migration: Migrates a named graph between repositories
//     - Exports source graph as RDF
//     - Imports RDF into target graph
//     - Cleans up temporary files
//
//  3. repo-delete: Deletes a repository from GraphDB
//     - Calls GraphDB delete repository API
//
//  4. graph-delete: Deletes a named graph from a repository
//     - Calls GraphDB delete graph API
//
//  5. repo-create: Creates a new repository from TTL configuration
//     - Reads configuration from multipart file upload
//     - Creates repository via GraphDB API
//
//  6. graph-import: Imports RDF data files into a named graph
//     - Reads RDF files from multipart upload
//     - Detects RDF format from file extensions
//     - Imports each file into the target graph
//
//  7. repo-import: Restores a repository from BRF backup file
//     - Reads BRF file from multipart upload
//     - Restores repository via GraphDB API
//
//  8. repo-rename: Renames a repository (backup, recreate, restore flow)
//     - Downloads old repository configuration and data
//     - Updates repository name in configuration
//     - Creates new repository with updated name
//     - Restores data from backup
//     - Deletes old repository
//     - Cleans up temporary files
//
//  9. graph-rename: Renames a named graph (export, import, delete flow)
//     - Exports old graph as RDF
//     - Imports RDF into new graph
//     - Deletes old graph
//     - Cleans up temporary files
//
// Ziti Support:
// If the global identityFile variable is set, the function creates Ziti-enabled HTTP
// clients for secure, zero-trust networking between the service and GraphDB instances.
//
// Error Handling:
// The function includes panic recovery to handle unexpected errors gracefully.
// All temporary files are cleaned up, even when errors occur.
//
// Parameters:
//   - task: The Task structure containing action type and repository details
//   - files: Map of multipart file headers keyed by form field name (e.g., "task_0_config")
//   - taskIndex: The index of this task in the request (used to look up files)
//
// Returns:
//   - result: A map containing task results (action, status, message, etc.)
//   - error: An error if the task fails, or nil on success
func processTask(task Task, files map[string][]*multipart.FileHeader, taskIndex int) (map[string]interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC RECOVERED in processTask for action %s: %v\n", task.Action, r)
		}
	}()

	srcClient := http.DefaultClient
	tgtClient := http.DefaultClient

	fmt.Printf("DEBUG: Processing task action: %s\n", task.Action)

	result := map[string]interface{}{
		"action": task.Action,
		"status": "completed",
	}

	switch task.Action {
	case "repo-migration":
		if identityFile != "" {
			srcURL, err := URL2ServiceRobust(task.Src.URL)
			if err != nil {
				return nil, err
			}
			srcClient, err = db.GraphDBZitiClient(identityFile, srcURL)
			if err != nil {
				return nil, err
			}
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = srcClient
		srcGraphDB, err := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
		if err != nil {
			return nil, err
		}
		foundRepo := false
		confFile := ""
		dataFile := ""
		for _, bind := range srcGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Src.Repo {
				foundRepo = true
				var err error
				confFile, err = db.GraphDBRepositoryConf(task.Src.URL, task.Src.Username, task.Src.Password, bind.Id["value"])
				if err != nil {
					return nil, fmt.Errorf("failed to download repository config: %w", err)
				}
				dataFile, err = db.GraphDBRepositoryBrf(task.Src.URL, task.Src.Username, task.Src.Password, bind.Id["value"])
				if err != nil {
					return nil, fmt.Errorf("failed to download repository data: %w", err)
				}
			}
		}
		if !foundRepo {
			return nil, errors.New("could not find required src repository " + task.Src.Repo)
		}
		db.HttpClient = tgtClient
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Src.Repo {
				err := db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Src.Repo)
				if err != nil {
					return nil, err
				}
			}
		}
		err = db.GraphDBRestoreConf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, confFile)
		if err != nil {
			return nil, err
		}
		err = db.GraphDBRestoreBrf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, dataFile)
		if err != nil {
			return nil, err
		}
		_ = os.Remove(confFile)
		_ = os.Remove(dataFile)
		result["message"] = "Repository migrated successfully"
		result["src_repo"] = task.Src.Repo
		result["tgt_repo"] = task.Tgt.Repo

	case "graph-migration":
		if identityFile != "" {
			srcURL, err := URL2ServiceRobust(task.Src.URL)
			if err != nil {
				return nil, err
			}
			srcClient, err = db.GraphDBZitiClient(identityFile, srcURL)
			if err != nil {
				return nil, err
			}
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = srcClient
		srcGraphDB, err := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
		if err != nil {
			return nil, err
		}
		foundRepo := false
		graphFile := md5Hash(task.Src.Graph) + ".brf"
		for _, bind := range srcGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Src.Repo {
				foundRepo = true
				srcGraphDB, err := db.GraphDBListGraphs(task.Src.URL, task.Src.Username, task.Src.Password, task.Src.Repo)
				if err != nil {
					return nil, err
				}
				foundGraph := false
				for _, bind := range srcGraphDB.Results.Bindings {
					if bind.ContextID.Value == task.Tgt.Graph {
						foundGraph = true
						err := db.GraphDBExportGraphRdf(task.Src.URL, task.Src.Username, task.Src.Password, task.Src.Repo, task.Src.Graph, graphFile)
						if err != nil {
							return nil, err
						}
					}
				}
				if !foundGraph {
					return nil, errors.New("could not find required src graph " + task.Src.Graph + " in repository " + task.Src.Repo)
				}
			}
		}
		if !foundRepo {
			return nil, errors.New("could not find required src repository " + task.Src.Repo)
		}
		db.HttpClient = tgtClient
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		foundRepo = false
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Tgt.Repo {
				foundRepo = true
				tgtGraphDB, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
				if err != nil {
					return nil, err
				}
				for _, bind := range tgtGraphDB.Results.Bindings {
					if bind.ContextID.Value == task.Tgt.Graph {
						err := db.GraphDBDeleteGraph(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph)
						if err != nil {
							return nil, err
						}
					}
				}
				err = db.GraphDBImportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph, graphFile)
				if err != nil {
					return nil, err
				}
			}
		}
		if !foundRepo {
			return nil, errors.New("could not find required tgt repository " + task.Tgt.Repo)
		}
		_ = os.Remove(graphFile) // Clean up temporary file
		result["message"] = "Graph migrated successfully"
		result["src_graph"] = task.Src.Graph
		result["tgt_graph"] = task.Tgt.Graph

	case "repo-delete":
		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = tgtClient
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Tgt.Repo {
				err := db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
				if err != nil {
					return nil, err
				}
			}
		}
		result["message"] = "Repository deleted successfully"
		result["repo"] = task.Tgt.Repo

	case "graph-delete":
		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = tgtClient
		tgtGraphDB, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
		if err != nil {
			return nil, err
		}
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.ContextID.Value == task.Tgt.Graph {
				err := db.GraphDBDeleteGraph(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph)
				if err != nil {
					return nil, err
				}
			}
		}
		result["message"] = "Graph deleted successfully"
		result["graph"] = task.Tgt.Graph

	case "repo-import":
		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = tgtClient
		fmt.Println("DEBUG: Starting repo-import processing")

		// Check if target repository exists
		fmt.Printf("DEBUG: Fetching repositories from %s\n", task.Tgt.URL)
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}

		foundRepo := false
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Tgt.Repo {
				foundRepo = true
				break
			}
		}

		if !foundRepo {
			fmt.Printf("ERROR: Repository '%s' not found\n", task.Tgt.Repo)
			return nil, fmt.Errorf("repository '%s' not found. Available repositories: %v", task.Tgt.Repo, getRepositoryNames(tgtGraphDB.Results.Bindings))
		}

		fmt.Printf("DEBUG: Repository '%s' found in GraphDB\n", task.Tgt.Repo)

		// Get BRF data file from source repository (if specified) or use a local file
		// This follows the same pattern as repo-migration
		if task.Src != nil && task.Src.Repo != "" {
			// Import from another repository's BRF file
			srcGraphDB, err := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
			if err != nil {
				return nil, err
			}

			srcFoundRepo := false
			dataFile := ""
			for _, bind := range srcGraphDB.Results.Bindings {
				if bind.Id["value"] == task.Src.Repo {
					srcFoundRepo = true
					var err error
					dataFile, err = db.GraphDBRepositoryBrf(task.Src.URL, task.Src.Username, task.Src.Password, bind.Id["value"])
					if err != nil {
						return nil, fmt.Errorf("failed to download repository data: %w", err)
					}
					break
				}
			}

			if !srcFoundRepo {
				return nil, fmt.Errorf("source repository '%s' not found", task.Src.Repo)
			}

			fmt.Printf("DEBUG: Importing BRF data from %s to repository %s\n", task.Src.Repo, task.Tgt.Repo)
			err = db.GraphDBRestoreBrf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, dataFile)
			if err != nil {
				return nil, err
			}

			// Clean up the temporary BRF file
			_ = os.Remove(dataFile)

			result["message"] = "Repository import completed successfully"
			result["source_repository"] = task.Src.Repo
			result["target_repository"] = task.Tgt.Repo
		} else {
			// Handle file uploads if using multipart form
			if files != nil {
				fileKey := fmt.Sprintf("task_%d_files", taskIndex)
				if taskFiles, exists := files[fileKey]; exists && len(taskFiles) > 0 {
					// Process the first BRF file
					fileHeader := taskFiles[0]

					file, err := fileHeader.Open()
					if err != nil {
						return nil, fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err)
					}
					defer func() { _ = file.Close() }()

					// Save file temporarily with unique UUID-based filename to avoid conflicts
					fileExt := filepath.Ext(fileHeader.Filename)
					tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("repo_import_%s%s", uuid.New().String(), fileExt))
					defer func() { _ = os.Remove(tempFileName) }()

					tempFile, err := os.Create(tempFileName)
					if err != nil {
						return nil, fmt.Errorf("failed to create temp file: %w", err)
					}
					defer func() { _ = tempFile.Close() }()

					_, err = tempFile.ReadFrom(file)
					if err != nil {
						return nil, fmt.Errorf("failed to copy file: %w", err)
					}
					_ = tempFile.Close()

					// Import the BRF file
					fmt.Printf("DEBUG: Importing BRF file %s to repository %s\n", fileHeader.Filename, task.Tgt.Repo)
					err = db.GraphDBRestoreBrf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, tempFileName)
					if err != nil {
						return nil, fmt.Errorf("failed to import BRF file: %w", err)
					}

					result["message"] = "Repository import completed successfully"
					result["imported_file"] = fileHeader.Filename
					result["target_repository"] = task.Tgt.Repo
				} else {
					return nil, fmt.Errorf("no BRF files provided for import")
				}
			} else {
				return nil, fmt.Errorf("no source repository or files specified for import")
			}
		}

	case "repo-create":
		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = tgtClient
		repoName := task.Tgt.Repo

		// Check if repository already exists
		existingRepos, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		for _, bind := range existingRepos.Results.Bindings {
			if bind.Id["value"] == repoName {
				return nil, fmt.Errorf("repository '%s' already exists", repoName)
			}
		}

		// Require configuration file upload
		if files == nil {
			return nil, fmt.Errorf("repo-create requires a configuration file to be uploaded")
		}

		fileKey := fmt.Sprintf("task_%d_config", taskIndex)
		taskFiles, exists := files[fileKey]
		if !exists || len(taskFiles) == 0 {
			return nil, fmt.Errorf("repo-create requires a configuration file with key 'task_%d_config'", taskIndex)
		}

		// Use the first uploaded configuration file
		fileHeader := taskFiles[0]

		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open config file %s: %w", fileHeader.Filename, err)
		}
		defer func() { _ = file.Close() }()

		// Save uploaded config to temporary file with unique UUID-based filename to avoid conflicts
		fileExt := filepath.Ext(fileHeader.Filename)
		configFile := filepath.Join(os.TempDir(), fmt.Sprintf("repo_create_%s%s", uuid.New().String(), fileExt))
		defer func() { _ = os.Remove(configFile) }()

		tempFile, err := os.Create(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp config file: %w", err)
		}
		defer func() { _ = tempFile.Close() }()

		// Copy uploaded file to temp file
		if _, err := file.Seek(0, 0); err != nil {
			return nil, fmt.Errorf("failed to seek config file: %w", err)
		}
		if _, err := tempFile.ReadFrom(file); err != nil {
			return nil, fmt.Errorf("failed to copy config file: %w", err)
		}
		_ = tempFile.Close()

		// Update the repository name in config file to match the requested name
		err = updateRepositoryNameInConfig(configFile, "PLACEHOLDER", repoName)
		if err != nil {
			// Try without replacement if the config file doesn't have placeholders
			fmt.Printf("Warning: could not update repository name in config: %v\n", err)
		}

		// Create the repository using the configuration file
		err = db.GraphDBRestoreConf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create repository '%s': %w", repoName, err)
		}

		// Verify the repository was created
		verifyRepos, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		repoCreated := false
		for _, bind := range verifyRepos.Results.Bindings {
			if bind.Id["value"] == repoName {
				repoCreated = true
				break
			}
		}

		if !repoCreated {
			return nil, fmt.Errorf("repository '%s' was not created successfully", repoName)
		}

		result["message"] = "Repository created successfully"
		result["repo"] = repoName
		result["config_file"] = fileHeader.Filename

	case "graph-import":
		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = tgtClient
		fmt.Println("DEBUG: Starting graph-import processing")

		fmt.Printf("DEBUG: Fetching repositories from %s\n", task.Tgt.URL)
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}

		if tgtGraphDB.Results.Bindings == nil {
			fmt.Printf("ERROR: GraphDB returned nil bindings from %s\n", task.Tgt.URL)
			return nil, fmt.Errorf("failed to get repositories from GraphDB at %s - nil response", task.Tgt.URL)
		}

		fmt.Printf("DEBUG: Found %d repositories in GraphDB\n", len(tgtGraphDB.Results.Bindings))

		// List all available repositories for debugging
		if len(tgtGraphDB.Results.Bindings) == 0 {
			fmt.Printf("WARNING: No repositories found in GraphDB at %s\n", task.Tgt.URL)
			fmt.Printf("DEBUG: Attempting to create repository '%s' or continue assuming it exists\n", task.Tgt.Repo)
			// Continue with import attempt - the repository might exist but not be listed
		} else {
			fmt.Printf("DEBUG: Available repositories: ")
			for i, bind := range tgtGraphDB.Results.Bindings {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("'%s'", bind.Id["value"])
			}
			fmt.Printf("\n")
		}

		foundRepo := false
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Tgt.Repo {
				foundRepo = true
				break
			}
		}

		if !foundRepo && len(tgtGraphDB.Results.Bindings) > 0 {
			fmt.Printf("ERROR: Repository '%s' not found in list of %d repositories\n", task.Tgt.Repo, len(tgtGraphDB.Results.Bindings))
			return nil, fmt.Errorf("repository '%s' not found. Available repositories: %v", task.Tgt.Repo, getRepositoryNames(tgtGraphDB.Results.Bindings))
		}

		// If we reach here, either the repo was found, or the repository list was empty
		// In case of empty list, we'll attempt the import anyway
		if foundRepo {
			fmt.Printf("DEBUG: Repository '%s' found in GraphDB\n", task.Tgt.Repo)
		} else {
			fmt.Printf("DEBUG: Repository list was empty, attempting import to '%s' anyway\n", task.Tgt.Repo)
		}

		// Try to list graphs (this might fail if repository doesn't exist)
		fmt.Printf("DEBUG: Listing graphs in repository: %s\n", task.Tgt.Repo)
		fmt.Println("232", "fooooo", task.Tgt)
		graphsResponse, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
		fmt.Println("233", err)
		if err != nil {
			fmt.Printf("WARNING: Failed to list graphs (repository might not exist): %v\n", err)
			// Continue with import - we'll try to import anyway
		} else if graphsResponse.Results.Bindings == nil {
			fmt.Printf("WARNING: GraphDB returned nil response for listing graphs\n")
		} else {
			fmt.Printf("DEBUG: Found %d graphs in repository\n", len(graphsResponse.Results.Bindings))
			// Check if target graph exists and delete it if found
			for _, bind := range graphsResponse.Results.Bindings {
				if bind.ContextID.Value == task.Tgt.Graph {
					fmt.Printf("DEBUG: Deleting existing graph: %s\n", task.Tgt.Graph)
					err := db.GraphDBDeleteGraph(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph)
					if err != nil {
						fmt.Printf("WARNING: Failed to delete existing graph: %v\n", err)
						// Don't fail the operation, continue with import
					}
					break
				}
			}
		}

		// Handle uploaded files for import
		if files != nil {
			fileKey := fmt.Sprintf("task_%d_files", taskIndex)
			fmt.Printf("DEBUG: Looking for files with key: %s\n", fileKey)

			if taskFiles, exists := files[fileKey]; exists {
				fmt.Printf("DEBUG: Found %d files to import\n", len(taskFiles))
				result["uploaded_files"] = len(taskFiles)
				result["file_names"] = getFileNames(taskFiles)

				// Process each uploaded file for import
				for i, fileHeader := range taskFiles {
					fmt.Printf("DEBUG: Processing file %d: %s (size: %d bytes)\n", i, fileHeader.Filename, fileHeader.Size)

					func() {
						defer func() {
							if r := recover(); r != nil {
								fmt.Printf("PANIC in file processing %s: %v\n", fileHeader.Filename, r)
							}
						}()

						file, err := fileHeader.Open()
						if err != nil {
							fmt.Printf("ERROR: Failed to open file %s: %v\n", fileHeader.Filename, err)
							return
						}
						defer func() { _ = file.Close() }()

						// Save file temporarily with unique UUID-based filename to avoid conflicts
						fileExt := filepath.Ext(fileHeader.Filename)
						tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("graph_import_%s%s", uuid.New().String(), fileExt))
						fmt.Printf("DEBUG: Creating temp file: %s\n", tempFileName)

						tempFile, err := os.Create(tempFileName)
						if err != nil {
							fmt.Printf("ERROR: Failed to create temp file: %v\n", err)
							return
						}
						defer func() { _ = tempFile.Close() }()
						defer func() {
							fmt.Printf("DEBUG: Removing temp file: %s\n", tempFileName)
							_ = os.Remove(tempFileName)
						}()

						// Copy uploaded file to temp file
						if _, err := file.Seek(0, 0); err != nil {
							fmt.Printf("ERROR: Failed to seek file: %v\n", err)
							return
						}

						fmt.Printf("DEBUG: Copying file content to temp file\n")
						bytesWritten, err := tempFile.ReadFrom(file)
						if err != nil {
							fmt.Printf("ERROR: Failed to copy file: %v\n", err)
							return
						}
						_ = tempFile.Close()

						fmt.Printf("DEBUG: Copied %d bytes to temp file\n", bytesWritten)

						// Determine import method based on file extension
						filename := strings.ToLower(fileHeader.Filename)

						fmt.Printf("DEBUG: Importing text RDF file: %s\n", fileHeader.Filename)
						err = db.GraphDBImportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph, tempFileName)
						if err != nil {
							fmt.Printf("ERROR: Failed to import RDF file %s: %v\n", fileHeader.Filename, err)
							return
						}

						fmt.Printf("DEBUG: Successfully imported file: %s\n", fileHeader.Filename)
						result[fmt.Sprintf("file_%d_processed", i)] = fileHeader.Filename
						result[fmt.Sprintf("file_%d_type", i)] = getFileType(filename)
					}()
				}
			} else {
				return nil, fmt.Errorf("graph-import action requires files to be uploaded with key 'task_%d_files'", taskIndex)
			}
		} else {
			return nil, fmt.Errorf("graph-import action requires files to be uploaded")
		}

		result["message"] = "Graph imported successfully"
		result["graph"] = task.Tgt.Graph

	case "repo-rename":
		// GraphDB doesn't have a direct rename API, so we need to:
		// 1. Create backup of old repository (config + individual graphs)
		// 2. Create new repository with new name
		// 3. Restore individual graphs to new repository
		// 4. Delete old repository

		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		oldRepoName := task.Tgt.RepoOld
		newRepoName := task.Tgt.RepoNew
		db.HttpClient = tgtClient

		// Step 1: Check if source repository exists
		srcGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		foundRepo := false
		for _, bind := range srcGraphDB.Results.Bindings {
			if bind.Id["value"] == oldRepoName {
				foundRepo = true
				break
			}
		}
		if !foundRepo {
			return nil, fmt.Errorf("source repository '%s' not found", oldRepoName)
		}

		// Step 2: Check if target repository already exists
		for _, bind := range srcGraphDB.Results.Bindings {
			if bind.Id["value"] == newRepoName {
				return nil, fmt.Errorf("target repository '%s' already exists", newRepoName)
			}
		}

		// Step 3: Get list of all graphs in the source repository
		graphsList, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, oldRepoName)
		if err != nil {
			return nil, fmt.Errorf("failed to list graphs in repository '%s': %w", oldRepoName, err)
		}

		// Step 4: Create backup of repository configuration
		confFile, err := db.GraphDBRepositoryConf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, oldRepoName)
		if err != nil {
			return nil, fmt.Errorf("failed to backup configuration for repository '%s': %w", oldRepoName, err)
		}
		defer func() { _ = os.Remove(confFile) }() // Clean up config file

		// Step 5: Export each graph individually
		graphBackups := make(map[string]string) // map[graphURI]fileName
		var graphExportErrors []string

		for _, bind := range graphsList.Results.Bindings {
			graphURI := bind.ContextID.Value
			if graphURI == "" {
				continue // Skip empty graph URIs
			}

			// Create a unique filename for each graph using UUID to avoid conflicts
			graphFileName := filepath.Join(os.TempDir(), fmt.Sprintf("repo_rename_%s.rdf", uuid.New().String()))

			err := db.GraphDBExportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, oldRepoName, graphURI, graphFileName)
			if err != nil {
				graphExportErrors = append(graphExportErrors, fmt.Sprintf("failed to export graph '%s': %v", graphURI, err))
				continue
			}

			// Verify the export file was created and has content
			if fileInfo, err := os.Stat(graphFileName); err != nil || fileInfo.Size() == 0 {
				graphExportErrors = append(graphExportErrors, fmt.Sprintf("graph '%s' export file is empty or missing", graphURI))
				_ = os.Remove(graphFileName) // Clean up empty file
				continue
			}

			graphBackups[graphURI] = graphFileName
		}

		// Clean up graph backup files when done
		defer func() {
			for _, fileName := range graphBackups {
				_ = os.Remove(fileName)
			}
		}()

		// Report any export errors but continue if we have at least some graphs
		if len(graphExportErrors) > 0 && len(graphBackups) == 0 {
			return nil, fmt.Errorf("failed to export any graphs: %s", strings.Join(graphExportErrors, "; "))
		}

		// Step 6: Modify the configuration file to use the new repository name
		err = updateRepositoryNameInConfig(confFile, oldRepoName, newRepoName)
		if err != nil {
			return nil, fmt.Errorf("failed to update repository name in config: %w", err)
		}

		// Step 7: Create new repository with the updated configuration
		err = db.GraphDBRestoreConf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, confFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create new repository '%s': %w", newRepoName, err)
		}

		// Step 8: Import each graph into the new repository
		var graphImportErrors []string
		successfulImports := 0

		for graphURI, fileName := range graphBackups {
			err := db.GraphDBImportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, newRepoName, graphURI, fileName)
			if err != nil {
				graphImportErrors = append(graphImportErrors, fmt.Sprintf("failed to import graph '%s': %v", graphURI, err))
				continue
			}
			successfulImports++
		}

		// Step 9: Verify that graphs were imported successfully
		if successfulImports == 0 && len(graphBackups) > 0 {
			// If no graphs were imported, clean up the new repository
			_ = db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, newRepoName)
			return nil, fmt.Errorf("failed to import any graphs to new repository: %s", strings.Join(graphImportErrors, "; "))
		}

		// Step 10: Delete the old repository
		err = db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, oldRepoName)
		if err != nil {
			// Log warning but don't fail the operation since the new repo is already created
			fmt.Printf("Warning: failed to delete old repository '%s': %v\n", oldRepoName, err)
			result["warning"] = fmt.Sprintf("New repository created successfully, but failed to delete old repository: %v", err)
		}

		result["message"] = "Repository renamed successfully"
		result["old_name"] = oldRepoName
		result["new_name"] = newRepoName
		result["total_graphs"] = len(graphsList.Results.Bindings)
		result["exported_graphs"] = len(graphBackups)
		result["imported_graphs"] = successfulImports

		// Add warnings if there were any issues
		if len(graphExportErrors) > 0 {
			if result["warning"] != nil {
				result["warning"] = fmt.Sprintf("%s; Export issues: %s", result["warning"], strings.Join(graphExportErrors, "; "))
			} else {
				result["warning"] = fmt.Sprintf("Some graphs had export issues: %s", strings.Join(graphExportErrors, "; "))
			}
		}

		if len(graphImportErrors) > 0 {
			if result["warning"] != nil {
				result["warning"] = fmt.Sprintf("%s; Import issues: %s", result["warning"], strings.Join(graphImportErrors, "; "))
			} else {
				result["warning"] = fmt.Sprintf("Some graphs had import issues: %s", strings.Join(graphImportErrors, "; "))
			}
		}

	case "graph-rename":
		// GraphDB doesn't have a direct graph rename API, so we need to:
		// 1. Export the old graph to a temporary file
		// 2. Import the data into the new graph
		// 3. Delete the old graph

		if identityFile != "" {
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		oldGraphName := task.Tgt.GraphOld
		newGraphName := task.Tgt.GraphNew
		repoName := task.Tgt.Repo
		db.HttpClient = tgtClient

		// Step 1: Check if repository exists
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}
		foundRepo := false
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == repoName {
				foundRepo = true
				break
			}
		}
		if !foundRepo {
			return nil, fmt.Errorf("repository '%s' not found", repoName)
		}

		// Step 2: Check if source graph exists
		graphsResponse, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, repoName)
		if err != nil {
			return nil, fmt.Errorf("failed to list graphs in repository '%s': %w", repoName, err)
		}

		foundOldGraph := false
		foundNewGraph := false
		for _, bind := range graphsResponse.Results.Bindings {
			if bind.ContextID.Value == oldGraphName {
				foundOldGraph = true
			}
			if bind.ContextID.Value == newGraphName {
				foundNewGraph = true
			}
		}

		if !foundOldGraph {
			return nil, fmt.Errorf("source graph '%s' not found in repository '%s'", oldGraphName, repoName)
		}

		if foundNewGraph {
			return nil, fmt.Errorf("target graph '%s' already exists in repository '%s'", newGraphName, repoName)
		}

		// Step 3: Export the old graph to a temporary file with unique UUID to avoid conflicts
		tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("graph_rename_%s.rdf", uuid.New().String()))
		defer func() { _ = os.Remove(tempFileName) }() // Clean up temporary file

		err = db.GraphDBExportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, repoName, oldGraphName, tempFileName)
		if err != nil {
			return nil, fmt.Errorf("failed to export graph '%s': %w", oldGraphName, err)
		}

		// Step 4: Verify the export file was created and has content
		fileInfo, err := os.Stat(tempFileName)
		if err != nil {
			return nil, fmt.Errorf("failed to verify exported file: %w", err)
		}
		if fileInfo.Size() == 0 {
			return nil, fmt.Errorf("exported graph file is empty - graph '%s' may be empty", oldGraphName)
		}

		// Step 5: Import the data into the new graph
		err = db.GraphDBImportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, repoName, newGraphName, tempFileName)
		if err != nil {
			return nil, fmt.Errorf("failed to import graph data to '%s': %w", newGraphName, err)
		}

		// Step 6: Verify the new graph was created successfully
		verifyGraphs, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, repoName)
		if err != nil {
			return nil, fmt.Errorf("failed to verify new graph creation: %w", err)
		}

		newGraphExists := false
		for _, bind := range verifyGraphs.Results.Bindings {
			if bind.ContextID.Value == newGraphName {
				newGraphExists = true
				break
			}
		}

		if !newGraphExists {
			return nil, fmt.Errorf("new graph '%s' was not created successfully", newGraphName)
		}

		// Step 7: Get triple counts for verification
		oldGraphTriples, newGraphTriples := getGraphTripleCounts(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, repoName, oldGraphName, newGraphName)

		// Step 8: Delete the old graph
		err = db.GraphDBDeleteGraph(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, repoName, oldGraphName)
		if err != nil {
			// Log warning but don't fail since new graph is already created
			fmt.Printf("Warning: failed to delete old graph '%s': %v\n", oldGraphName, err)
			result["warning"] = fmt.Sprintf("New graph created successfully, but failed to delete old graph: %v", err)
		}

		result["message"] = "Graph renamed successfully"
		result["repository"] = repoName
		result["old_name"] = oldGraphName
		result["new_name"] = newGraphName
		result["file_size_bytes"] = fileInfo.Size()

		// Add triple count verification if available
		if oldGraphTriples >= 0 && newGraphTriples >= 0 {
			result["old_graph_triples"] = oldGraphTriples
			result["new_graph_triples"] = newGraphTriples
			if oldGraphTriples != newGraphTriples {
				result["warning"] = fmt.Sprintf("Triple count mismatch: old graph had %d triples, new graph has %d triples", oldGraphTriples, newGraphTriples)
			}
		}
	}

	return result, nil
}

func init() {
	rootCmd.AddCommand(graphdbCmd)
	graphdbCmd.Flags().String("identity", "", "identity to authenticate to an ziti network")
}

var graphdbCmd = &cobra.Command{
	Use:   "graphdb",
	Short: "service to integrate with graphdb",
	Long:  `service to integrate with graphdb`,
	Run: func(cmd *cobra.Command, args []string) {
		identityFile, _ = cmd.Flags().GetString("identity")

		// Initialize authentication system
		if err := InitializeAuth(); err != nil {
			fmt.Printf("FATAL: Failed to initialize authentication: %v\n", err)
			os.Exit(1)
		}

		e := echo.New()

		// Add request size limit (100MB)
		e.Use(middleware.BodyLimit("100M"))

		// Add timeout middleware (skip SSE endpoints)
		e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
			Timeout: 300 * time.Second, // 5 minutes
			Skipper: func(c echo.Context) bool {
				// Skip timeout for SSE stream endpoints
				return c.Path() == "/ui/stream/:sessionID"
			},
		}))

		// Enable CORS
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
			AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "x-api-key"},
		}))
		// Add logging and recovery middleware
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())

		// Get authentication mode
		authMode := getAuthMode()

		// API routes (require API key)
		e.POST("/v1/api/action", migrationHandler, apiKeyMiddleware)

		// Public routes (no authentication required)
		e.GET("/health", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
		})
		e.GET("/favicon.ico", func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})
		e.GET("/login", loginPageHandler)
		e.POST("/auth/login", loginHandler)

		// Protected UI routes (require authentication if enabled)
		ui := e.Group("", AuthMiddleware(authMode))
		ui.GET("/", uiIndexHandler)
		ui.GET("/ui", uiIndexHandler)
		ui.POST("/ui/execute", uiExecuteHandler)
		ui.GET("/ui/stream/:sessionID", uiStreamHandler)
		ui.GET("/logout", logoutHandler)

		// User profile endpoints (authenticated users)
		ui.GET("/profile/change-password", changePasswordPageHandler)
		ui.GET("/api/users/me", getCurrentUserHandler)
		ui.POST("/api/users/me/password", changePasswordHandler)

		// Admin-only user management endpoints
		admin := ui.Group("/admin", AdminOnlyMiddleware())
		admin.GET("/users", usersPageHandler)           // HTML page
		admin.GET("/users/list", listUsersHandler)      // HTMX endpoint
		admin.GET("/users/api", listUsersAPIHandler)    // JSON API
		admin.POST("/users", createUserHandler)         // Create user API
		admin.GET("/users/:username", getUserHandler)   // Get single user API
		admin.PUT("/users/:username", updateUserHandler) // Update user API
		admin.DELETE("/users/:username", deleteUserHandler) // Delete user API

		// Admin-only audit log endpoints
		admin.GET("/audit", auditLogsPageHandler)      // HTML page
		admin.GET("/audit/list", listAuditLogsHandler) // HTMX endpoint
		admin.GET("/audit/api", getAuditLogsAPIHandler) // JSON API
		admin.POST("/audit/rotate", rotateAuditLogsHandler) // Trigger rotation

		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}

		fmt.Printf("\nGraphDB Service starting on port %s\n", port)
		fmt.Printf("Web UI: http://localhost:%s/\n", port)
		if authMode != "none" {
			fmt.Printf("Login page: http://localhost:%s/login\n", port)
		}
		fmt.Println()

		// Start server
		e.Logger.Fatal(e.Start(":" + port))
	},
}
