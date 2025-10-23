package cmd

import (
	"errors"
	// "fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"

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

// Migration handler
func migrationHandler(c echo.Context) error {
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
			return echo.NewHTTPError(http.StatusBadRequest, "Task %d: %s", i, err.Error())
		}
	}

	// Process tasks (implement your business logic here)
	results := make([]map[string]interface{}, len(req.Tasks))
	for i, task := range req.Tasks {
		result, err := processTask(task)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Task %d failed: %s", i, err.Error())
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
		"delete-repo":     true,
		"delete-graph":    true,
		"create-repo":     true,
		"import-graph":    true,
		"rename-repo":     true,
		"rename-graph":    true,
	}

	if !validActions[task.Action] {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action: %s", task.Action)
	}

	switch task.Action {
	case "repo-migration", "graph-migration":
		if task.Src == nil || task.Tgt == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Both src and tgt are required for %s", task.Action)
		}
	case "delete-repo", "delete-graph", "create-repo", "import-graph":
		if task.Tgt == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "tgt is required for %s", task.Action)
		}
	case "rename-repo":
		if task.Tgt == nil || task.Tgt.RepoOld == "" || task.Tgt.RepoNew == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "tgt with repo_old and repo_new are required for rename-repo")
		}
	case "rename-graph":
		if task.Tgt == nil || task.Tgt.GraphOld == "" || task.Tgt.GraphNew == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "tgt with graph_old and graph_new are required for rename-graph")
		}
	}

	return nil
}

func processTask(task Task) (map[string]interface{}, error) {
	// Implement your actual business logic here
	// This is a placeholder that returns success for all tasks

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
		result["src_graph"] = task.Src.Graph
		result["tgt_graph"] = task.Tgt.Graph

	case "graph-migration":
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
		result["message"] = "Graph deleted successfully"
		result["graph"] = task.Tgt.Graph

	case "repo-create":
		result["message"] = "Repository created successfully"
		result["repo"] = task.Tgt.Repo

	case "graph-import":
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

// url, _ := cmd.Flags().GetString("url")
// user, _ := cmd.Flags().GetString("user")
// pass, _ := cmd.Flags().GetString("pass")
// bkp, _ := cmd.Flags().GetBool("bkp")
// restore, _ := cmd.Flags().GetBool("restore")
// importRdfXml, _ := cmd.Flags().GetBool("import")
// exportRdfXml, _ := cmd.Flags().GetBool("export")
// restorePath, _ := cmd.Flags().GetString("restore-path")
// importPath, _ := cmd.Flags().GetString("import-dir")
// importFile, _ := cmd.Flags().GetString("import-file")
// deleteGraph, _ := cmd.Flags().GetBool("delete-graph")
// repo, _ := cmd.Flags().GetString("repo")
// graph, _ := cmd.Flags().GetString("graph")
// listGraphs, _ := cmd.Flags().GetBool("list-graphs")
// if exportRdfXml {
// 	db.GraphDBExportGraphRdf(url, user, pass, repo, graph, eve.URLToFilePath(graph)+".rdf")
// 	return
// }
// if listGraphs {
// 	db.GraphDBListGraphs(url, user, pass, repo)
// 	return
// }
// if bkp {
// 	resp := db.GraphDBRepositories(url, user, pass)
// 	for _, bind := range resp.Results.Bindings {
// 		fmt.Println(db.GraphDBRepositoryConf(url, user, pass, bind.Id["value"]))
// 		fmt.Println(db.GraphDBRepositoryBrf(url, user, pass, bind.Id["value"]))
// 	}
// 	return
// }
// if restore {
// 	ttlFiles := listFiles(restorePath, "ttl")
// 	for _, ttlFile := range ttlFiles {
// 		fmt.Println("import repo config from", ttlFile)
// 		db.GraphDBRestoreConf(url, user, pass, ttlFile)
// 	}
// 	brfFiles := listFiles(restorePath, "brf")
// 	for _, brfFile := range brfFiles {
// 		fmt.Println("import repo data from", brfFile)
// 		db.GraphDBRestoreBrf(url, user, pass, brfFile)
// 	}
// 	return
// }
// if deleteGraph {
// 	fmt.Println("delete repo", repo, "graph", graph)
// 	db.GraphDBDeleteGraph(url, user, pass, repo, graph)
// }
// if importRdfXml {
// 	if importFile != "" {
// 		fmt.Println("import repo data from", importFile)
// 		db.GraphDBImportGraphRdf(url, user, pass, repo, graph, importFile)
// 		return
// 	} else {
// 		rdfFiles := listFiles(importPath, "rdf")
// 		for _, rdfFile := range rdfFiles {
// 			fmt.Println("import repo data from", rdfFile)
// 			db.GraphDBImportGraphRdf(url, user, pass, repo, graph, rdfFile)
// 		}
// 	}
// 	return
// }

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
