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
	"crypto/md5"

	// eve "eve.evalgo.org/common"
	"eve.evalgo.org/db"
	"github.com/spf13/cobra"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

// Migration handler with multipart form support
func migrationHandler(c echo.Context) error {
	// Check if this is a multipart form request
	contentType := c.Request().Header.Get("Content-Type")
	
	if contentType != "" && strings.HasPrefix(contentType, "multipart/form-data") {
		return migrationHandlerMultipart(c)
	} else {
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
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to parse multipart form")
	}
	defer form.RemoveAll() // Clean up temporary files
	
	// Get JSON request from form field
	jsonFields, exists := form.Value["request"]
	if !exists || len(jsonFields) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Missing 'request' field in form data")
	}
	
	// Parse JSON request
	var req MigrationRequest
	if err := json.Unmarshal([]byte(jsonFields[0]), &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON in request field: "+err.Error())
	}
	
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
		result, err := processTask(task, files, i)
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

func validateTask(task Task) error {
	validActions := map[string]bool{
		"repo-migration":  true,
		"graph-migration": true,
		"repo-delete":     true,
		"graph-delete":    true,
		"repo-create":     true,
		"graph-import":    true,
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
	case "repo-delete", "graph-delete", "repo-create", "graph-import":
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

func processTask(task Task, files map[string][]*multipart.FileHeader, taskIndex int) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"action": task.Action,
		"status": "completed",
	}

	switch task.Action {
	case "repo-migration":
		srcGraphDB := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
		foundRepo := false
		confFile := ""
		dataFile := ""
		for _, bind := range srcGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Src.Repo {
				foundRepo = true
				confFile = db.GraphDBRepositoryConf(task.Src.URL, task.Src.Username, task.Src.Password, bind.Id["value"])
				dataFile = db.GraphDBRepositoryBrf(task.Src.URL, task.Src.Username, task.Src.Password, bind.Id["value"])
			}
		}
		if !foundRepo {
			return nil, errors.New("could not find required src repository " + task.Src.Repo)
		}
		tgtGraphDB := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		for _, bind := range tgtGraphDB.Results.Bindings {
			if bind.Id["value"] == task.Src.Repo {
				err := db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Src.Repo)
				if err != nil {
					return nil, err
				}
			}
		}
		err := db.GraphDBRestoreConf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, confFile)
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
		srcGraphDB := db.GraphDBRepositories(task.Src.URL, task.Src.Username, task.Src.Password)
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

		tgtGraphDB := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		tgtGraphDB := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		fmt.Println(task.Tgt)
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

	case "repo-create":
		// Handle optional repository configuration files
		if files != nil {
			fileKey := fmt.Sprintf("task_%d_config", taskIndex)
			if taskFiles, exists := files[fileKey]; exists {
				result["config_files"] = getFileNames(taskFiles)
				// Process config files if needed
				for _, fileHeader := range taskFiles {
					file, err := fileHeader.Open()
					if err != nil {
						return nil, fmt.Errorf("failed to open config file %s: %w", fileHeader.Filename, err)
					}
					defer file.Close()
					// TODO: Process repository configuration file
				}
			}
		}
		result["message"] = "Repository created successfully"
		result["repo"] = task.Tgt.Repo

	case "graph-import":
		tgtGraphDB := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		foundRepo := false
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

				// Handle uploaded files for import
				if files != nil {
					fileKey := fmt.Sprintf("task_%d_files", taskIndex)
					if taskFiles, exists := files[fileKey]; exists {
						result["uploaded_files"] = len(taskFiles)
						result["file_names"] = getFileNames(taskFiles)
						
						// Process each uploaded file for import
						for i, fileHeader := range taskFiles {
							file, err := fileHeader.Open()
							if err != nil {
								return nil, fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err)
							}
							defer file.Close()
							
							// Save file temporarily and import it
							tempFileName := fmt.Sprintf("/tmp/%s", fileHeader.Filename)
							tempFile, err := os.Create(tempFileName)
							if err != nil {
								return nil, fmt.Errorf("failed to create temp file: %w", err)
							}
							defer tempFile.Close()
							defer os.Remove(tempFileName)
							
							// Copy uploaded file to temp file
							if _, err := file.Seek(0, 0); err != nil {
								return nil, fmt.Errorf("failed to seek file: %w", err)
							}
							if _, err := tempFile.ReadFrom(file); err != nil {
								return nil, fmt.Errorf("failed to copy file: %w", err)
							}
							tempFile.Close()
							
							// Import the file using your existing function
							err = db.GraphDBImportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph, tempFileName)
							if err != nil {
								return nil, fmt.Errorf("failed to import file %s: %w", fileHeader.Filename, err)
							}
							
							result[fmt.Sprintf("file_%d_processed", i)] = fileHeader.Filename
						}
					} else {
						return nil, fmt.Errorf("graph-import action requires files to be uploaded with key 'task_%d_files'", taskIndex)
					}
				} else {
					return nil, fmt.Errorf("graph-import action requires files to be uploaded")
				}
			}
		}
		if !foundRepo {
			return nil, errors.New("could not find required tgt repository " + task.Tgt.Repo)
		}
		result["message"] = "Graph imported successfully"
		result["graph"] = task.Tgt.Graph

	case "repo-rename":
		// GraphDB doesn't have a direct rename API, so we need to:
		// 1. Create backup of old repository (config + individual graphs)
		// 2. Create new repository with new name
		// 3. Restore individual graphs to new repository
		// 4. Delete old repository
		
		oldRepoName := task.Tgt.RepoOld
		newRepoName := task.Tgt.RepoNew
		
		// Step 1: Check if source repository exists
		srcGraphDB := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
		confFile := db.GraphDBRepositoryConf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, oldRepoName)
		if confFile == "" {
			return nil, fmt.Errorf("failed to backup configuration for repository '%s'", oldRepoName)
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
		
		oldGraphName := task.Tgt.GraphOld
		newGraphName := task.Tgt.GraphNew
		repoName := task.Tgt.Repo
		
		// Step 1: Check if repository exists
		tgtGraphDB := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
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
	graphdbCmd.Flags().String("url", "http://build-001.graphdb.px:7200", "graphdb instance to connect to")
	graphdbCmd.Flags().String("repo", "", "repository to be used for importing rdf data")
	graphdbCmd.Flags().String("user", "", "user to authenticate with")
	graphdbCmd.Flags().String("pass", "", "password to authenticate with")
	graphdbCmd.Flags().String("graph", "https://schema.org/Person", "graph name")
	graphdbCmd.Flags().Bool("delete-graph", false, "delete the grapth either before import of just delete it")
	graphdbCmd.Flags().Bool("bkp", false, "create an backup")
	graphdbCmd.Flags().Bool("restore", false, "restore from backup files")
	graphdbCmd.Flags().Bool("import", false, "import rdf+xml file")
	graphdbCmd.Flags().Bool("export", false, "export rdf+xml file")
	graphdbCmd.Flags().Bool("list-graphs", false, "lists all graphs of a given repository")
	graphdbCmd.Flags().String("import-dir", ".", "import directory from where to pick up the .rdf files")
	graphdbCmd.Flags().String("import-file", "", "import .rdf file path")
	graphdbCmd.Flags().String("restore-path", ".", "restore path from where to pick up the .ttl and .brf files")
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
		e := echo.New()
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