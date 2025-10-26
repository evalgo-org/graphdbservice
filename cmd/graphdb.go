package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"crypto/md5"
	"net/url"

	// eve "eve.evalgo.org/common"
	"eve.evalgo.org/db"
	"github.com/spf13/cobra"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var(
	identityFile string = ""
)

// Request structures
type MigrationRequest struct {
	Version string `json:"version" validate:"required"`
	Tasks   []Task `json:"tasks" validate:"required"`
}

type Task struct {
	Action string      `json:"action" validate:"required"`
	Src    *Repository `json:"src,omitempty"`
	Tgt    *Repository `json:"tgt,omitempty"`
}

type Repository struct {
	URL      string `json:"url,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Graph    string `json:"graph,omitempty"`
	RepoOld  string `json:"repo_old,omitempty"`
	RepoNew  string `json:"repo_new,omitempty"`
	GraphOld string `json:"graph_old,omitempty"`
	GraphNew string `json:"graph_new,omitempty"`
}

func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// API Key validation middleware
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

// Helper function to extract file names from file headers
func getFileNames(fileHeaders []*multipart.FileHeader) []string {
	names := make([]string, len(fileHeaders))
	for i, fh := range fileHeaders {
		names[i] = fh.Filename
	}
	return names
}

// Helper function to update repository name in TTL configuration file
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
		fmt.Sprintf(`rep:repositoryID "%s"`, oldName): fmt.Sprintf(`rep:repositoryID "%s"`, newName),
		fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, oldName): fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, newName),
		fmt.Sprintf(`repo:%s`, oldName): fmt.Sprintf(`repo:%s`, newName),
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

// Helper function to get triple counts for graphs (for verification purposes)
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

// Helper function to determine RDF file type based on extension
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

// Helper function to extract repository names from GraphDB bindings
func getRepositoryNames(bindings []db.GraphDBBinding) []string {
	names := make([]string, 0)
	for _, binding := range bindings {
		if value, exists := binding.Id["value"]; exists {
			names = append(names, value)
		}
	}
	return names
}

// Migration handler with multipart form support
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

// Original JSON handler for backward compatibility
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

// New multipart form handler
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
			form.RemoveAll()
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

func processTask(task Task, files map[string][]*multipart.FileHeader, taskIndex int) (map[string]interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC RECOVERED in processTask for action %s: %v\n", task.Action, r)
		}
	}()
	
	var srcClient *http.Client = http.DefaultClient
	var tgtClient *http.Client = http.DefaultClient

	fmt.Printf("DEBUG: Processing task action: %s\n", task.Action)
	
	result := map[string]interface{}{
		"action": task.Action,
		"status": "completed",
	}

	switch task.Action {
	case "repo-migration":
		if identityFile != "" {
			srcURL,err := URL2ServiceRobust(task.Src.URL)
			if err != nil {
				return nil, err
			}
			srcClient, err = db.GraphDBZitiClient(identityFile, srcURL)
			if err != nil {
				return nil, err
			}
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = srcClient
		srcGraphDB,err := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
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
		tgtGraphDB,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		os.Remove(confFile)
		os.Remove(dataFile)
		result["message"] = "Repository migrated successfully"
		result["src_repo"] = task.Src.Repo
		result["tgt_repo"] = task.Tgt.Repo

	case "graph-migration":
		if identityFile != "" {
			srcURL,err := URL2ServiceRobust(task.Src.URL)
			if err != nil {
				return nil, err
			}
			srcClient, err = db.GraphDBZitiClient(identityFile, srcURL)
			if err != nil {
				return nil, err
			}
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = srcClient
		srcGraphDB,err := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
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
		tgtGraphDB,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		os.Remove(graphFile) // Clean up temporary file
		result["message"] = "Graph migrated successfully"
		result["src_graph"] = task.Src.Graph
		result["tgt_graph"] = task.Tgt.Graph

	case "repo-delete":
		if identityFile != "" {
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
		}
		db.HttpClient = tgtClient
		tgtGraphDB,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
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
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
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
			os.Remove(dataFile)
			
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
					defer file.Close()
					
					// Save file temporarily
					tempFileName := fmt.Sprintf("/tmp/%s", fileHeader.Filename)
					defer os.Remove(tempFileName)
					
					tempFile, err := os.Create(tempFileName)
					if err != nil {
						return nil, fmt.Errorf("failed to create temp file: %w", err)
					}
					defer tempFile.Close()
					
					_, err = tempFile.ReadFrom(file)
					if err != nil {
						return nil, fmt.Errorf("failed to copy file: %w", err)
					}
					tempFile.Close()
					
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
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
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
		existingRepos,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		defer file.Close()
		
		// Save uploaded config to temporary file
		configFile := fmt.Sprintf("/tmp/repo_create_%s_%s", md5Hash(repoName), fileHeader.Filename)
		defer os.Remove(configFile)
		
		tempFile, err := os.Create(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp config file: %w", err)
		}
		defer tempFile.Close()
		
		// Copy uploaded file to temp file
		if _, err := file.Seek(0, 0); err != nil {
			return nil, fmt.Errorf("failed to seek config file: %w", err)
		}
		if _, err := tempFile.ReadFrom(file); err != nil {
			return nil, fmt.Errorf("failed to copy config file: %w", err)
		}
		tempFile.Close()
		
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
		verifyRepos,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
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
		tgtGraphDB,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		fmt.Println("232","fooooo", task.Tgt)
		graphsResponse, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
		fmt.Println("233",err)
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
						defer file.Close()
						
						// Save file temporarily and import it
						tempFileName := fmt.Sprintf("/tmp/%s", fileHeader.Filename)
						fmt.Printf("DEBUG: Creating temp file: %s\n", tempFileName)
						
						tempFile, err := os.Create(tempFileName)
						if err != nil {
							fmt.Printf("ERROR: Failed to create temp file: %v\n", err)
							return
						}
						defer tempFile.Close()
						defer func() {
							fmt.Printf("DEBUG: Removing temp file: %s\n", tempFileName)
							os.Remove(tempFileName)
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
						tempFile.Close()
						
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
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
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
		srcGraphDB,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		defer os.Remove(confFile) // Clean up config file
		
		// Step 5: Export each graph individually
		graphBackups := make(map[string]string) // map[graphURI]fileName
		var graphExportErrors []string
		
		for _, bind := range graphsList.Results.Bindings {
			graphURI := bind.ContextID.Value
			if graphURI == "" {
				continue // Skip empty graph URIs
			}
			
			// Create a unique filename for each graph
			graphFileName := fmt.Sprintf("/tmp/repo_rename_%s_%s.rdf", md5Hash(oldRepoName), md5Hash(graphURI))
			
			err := db.GraphDBExportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, oldRepoName, graphURI, graphFileName)
			if err != nil {
				graphExportErrors = append(graphExportErrors, fmt.Sprintf("failed to export graph '%s': %v", graphURI, err))
				continue
			}
			
			// Verify the export file was created and has content
			if fileInfo, err := os.Stat(graphFileName); err != nil || fileInfo.Size() == 0 {
				graphExportErrors = append(graphExportErrors, fmt.Sprintf("graph '%s' export file is empty or missing", graphURI))
				os.Remove(graphFileName) // Clean up empty file
				continue
			}
			
			graphBackups[graphURI] = graphFileName
		}
		
		// Clean up graph backup files when done
		defer func() {
			for _, fileName := range graphBackups {
				os.Remove(fileName)
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
			db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, newRepoName)
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
			tgtURL,err := URL2ServiceRobust(task.Tgt.URL)
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
		tgtGraphDB,err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		
		// Step 3: Export the old graph to a temporary file
		tempFileName := fmt.Sprintf("/tmp/graph_rename_%s.rdf", md5Hash(oldGraphName))
		defer os.Remove(tempFileName) // Clean up temporary file
		
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

func listFiles(dir string, ext string) []string {
	root := os.DirFS(dir)
	mdFiles, err := fs.Glob(root, "*."+ext)
	if err != nil {
		log.Fatal(err)
	}
	var files []string
	for _, v := range mdFiles {
		files = append(files, path.Join(dir, v))
	}
	return files
}

var graphdbCmd = &cobra.Command{
	Use:   "graphdb",
	Short: "service to integrate with graphdb",
	Long:  `service to integrate with graphdb`,
	Run: func(cmd *cobra.Command, args []string) {
		identityFile, _ = cmd.Flags().GetString("identity")

		e := echo.New()
		
		// Add request size limit (100MB)
		e.Use(middleware.BodyLimit("100M"))
		
		// Add timeout middleware
		e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
			Timeout: 300 * time.Second, // 5 minutes
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
		// Apply API key middleware and register the migration handler
		e.POST("/v1/api/action", migrationHandler, apiKeyMiddleware)
		// Health check endpoint (without API key requirement)
		e.GET("/health", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
		})
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		// Start server
		e.Logger.Fatal(e.Start(":" + port))
	},
}