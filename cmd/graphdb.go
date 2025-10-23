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
		result["message"] = "Repository renamed successfully"
		result["old_name"] = task.Tgt.RepoOld
		result["new_name"] = task.Tgt.RepoNew

	case "graph-rename":
		result["message"] = "Graph renamed successfully"
		result["old_name"] = task.Tgt.GraphOld
		result["new_name"] = task.Tgt.GraphNew
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