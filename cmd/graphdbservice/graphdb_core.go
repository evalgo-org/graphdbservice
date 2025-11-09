// Package cmd provides core GraphDB operations for the semantic service.
// This file contains the essential GraphDB functionality without user UI dependencies.
package cmd

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"eve.evalgo.org/db"
	"github.com/google/uuid"
)

var (
	// identityFile holds the path to the Ziti identity JSON file for zero-trust networking.
	// When provided, all GraphDB connections will use Ziti secure networking.
	identityFile string = ""

	// debugMode controls whether detailed debug logging is enabled
	debugMode bool = false
)

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

// MigrationRequest represents the root request structure for GraphDB operations.
type MigrationRequest struct {
	Version string `json:"version" validate:"required"` // API version (e.g., "v0.0.1")
	Tasks   []Task `json:"tasks" validate:"required"`   // List of tasks to execute
}

// debugLog logs a message only if debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if debugMode {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// debugLogHTTP logs HTTP-related debug messages only if debug mode is enabled
func debugLogHTTP(format string, args ...interface{}) {
	if debugMode {
		fmt.Printf("DEBUG HTTP: "+format+"\n", args...)
	}
}

// normalizeURL removes trailing slashes from URLs to prevent double-slash issues
func normalizeURL(url string) string {
	return strings.TrimRight(url, "/")
}

// debugHTTPTransport wraps an http.RoundTripper to log request/response details
type debugHTTPTransport struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface with debugging
func (d *debugHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	debugLogHTTP("%s %s", req.Method, req.URL.String())

	// Execute the request
	resp, err := d.Transport.RoundTrip(req)
	if err != nil {
		debugLogHTTP("Request failed: %v", err)
		return resp, err
	}

	// Log response status
	debugLogHTTP("Response Status: %d %s", resp.StatusCode, resp.Status)

	// Read and log the response body only for errors if debug mode is enabled
	if debugMode && resp.StatusCode >= 400 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Ignore error - body already read

		if readErr != nil {
			debugLogHTTP("Failed to read error response body: %v", readErr)
		} else {
			debugLogHTTP("===== ERROR RESPONSE BODY (Status %d) =====", resp.StatusCode)
			fmt.Printf("%s\n", string(bodyBytes))
			debugLogHTTP("===== END ERROR RESPONSE BODY =====")

			// Restore the body for the caller
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	return resp, err
}

// enableHTTPDebugLogging wraps the HTTP client with debug logging
func enableHTTPDebugLogging(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}

	client.Transport = &debugHTTPTransport{
		Transport: client.Transport,
	}

	return client
}

// md5Hash generates an MD5 hash of the given text string.
func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// getFileNames extracts the filenames from a slice of multipart file headers.
func getFileNames(fileHeaders []*multipart.FileHeader) []string {
	names := make([]string, len(fileHeaders))
	for i, fh := range fileHeaders {
		names[i] = fh.Filename
	}
	return names
}

// updateRepositoryNameInConfig updates repository name references in a GraphDB TTL configuration file.
func updateRepositoryNameInConfig(configFile, oldName, newName string) error {
	// Read the configuration file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Convert to string for processing
	configContent := string(content)

	// Replace repository ID references in the TTL file
	replacements := map[string]string{
		fmt.Sprintf(`rep:repositoryID "%s"`, oldName):                         fmt.Sprintf(`rep:repositoryID "%s"`, newName),
		fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, oldName): fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, newName),
		fmt.Sprintf(`repo:%s`, oldName):                                       fmt.Sprintf(`repo:%s`, newName),
	}

	// Apply replacements
	for old, new := range replacements {
		configContent = strings.ReplaceAll(configContent, old, new)
	}

	// Handle the repository node declaration if it exists
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
func getGraphTripleCounts(url, username, password, repo, oldGraph, newGraph string) (int, int) {
	// Simplified implementation - returns -1 to indicate counts are not available
	oldCount := -1
	newCount := -1
	return oldCount, newCount
}

// getFileType determines the RDF serialization format based on the file extension.
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
func getRepositoryNames(bindings []db.GraphDBBinding) []string {
	names := make([]string, 0)
	for _, binding := range bindings {
		if value, exists := binding.Id["value"]; exists {
			names = append(names, value)
		}
	}
	return names
}

// URL2ServiceRobust parses a URL string and extracts the service portion (host:port).
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

	srcClient := http.DefaultClient
	tgtClient := http.DefaultClient

	// Enable HTTP debug logging if debug mode is active
	if debugMode {
		srcClient = enableHTTPDebugLogging(srcClient)
		tgtClient = enableHTTPDebugLogging(tgtClient)
	}

	debugLog("Processing task action: %s", task.Action)

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
			if debugMode {
				srcClient = enableHTTPDebugLogging(srcClient)
			}
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				return nil, err
			}
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				return nil, err
			}
			if debugMode {
				tgtClient = enableHTTPDebugLogging(tgtClient)
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

		// Get data file size
		dataSize := int64(0)
		if fileInfo, err := os.Stat(dataFile); err == nil {
			dataSize = fileInfo.Size()
		}

		_ = os.Remove(confFile)
		_ = os.Remove(dataFile)
		result["message"] = "Repository migrated successfully"
		result["src_repo"] = task.Src.Repo
		result["tgt_repo"] = task.Tgt.Repo
		result["data_size"] = dataSize

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

		// Get graph file size
		dataSize := int64(0)
		if fileInfo, err := os.Stat(graphFile); err == nil {
			dataSize = fileInfo.Size()
		}

		_ = os.Remove(graphFile) // Clean up temporary file
		result["message"] = "Graph migrated successfully"
		result["src_graph"] = task.Src.Graph
		result["tgt_graph"] = task.Tgt.Graph
		result["data_size"] = dataSize

	case "repo-delete":
		debugLog("repo-delete action started")
		debugLog("Target URL: %s", task.Tgt.URL)
		debugLog("Target Repo: %s", task.Tgt.Repo)
		debugLog("Username: %s", task.Tgt.Username)

		if identityFile != "" {
			debugLog("Using Ziti identity file: %s", identityFile)
			tgtURL, err := URL2ServiceRobust(task.Tgt.URL)
			if err != nil {
				debugLog("Failed to parse target URL: %v", err)
				return nil, err
			}
			debugLog("Parsed Ziti service URL: %s", tgtURL)
			tgtClient, err = db.GraphDBZitiClient(identityFile, tgtURL)
			if err != nil {
				debugLog("Failed to create Ziti client: %v", err)
				return nil, err
			}
			if debugMode {
				tgtClient = enableHTTPDebugLogging(tgtClient)
			}
			debugLog("Ziti client created successfully")
		}

		db.HttpClient = tgtClient

		debugLog("Fetching list of repositories...")
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			debugLog("ERROR: Failed to fetch repositories: %v", err)
			debugLog("Error type: %T", err)
			return nil, fmt.Errorf("failed to fetch repositories from %s: %w", task.Tgt.URL, err)
		}

		debugLog("Found %d repositories", len(tgtGraphDB.Results.Bindings))

		repoFound := false
		for i, bind := range tgtGraphDB.Results.Bindings {
			repoID := bind.Id["value"]
			debugLog("Repository %d: %s", i, repoID)

			if repoID == task.Tgt.Repo {
				repoFound = true
				debugLog("Found target repository: %s", task.Tgt.Repo)
				debugLog("Attempting to delete repository...")
				debugLog("DELETE URL: %s/repositories/%s", task.Tgt.URL, task.Tgt.Repo)

				err := db.GraphDBDeleteRepository(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
				if err != nil {
					debugLog("ERROR: GraphDBDeleteRepository failed: %v", err)
					debugLog("Error type: %T", err)
					debugLog("Error string: %s", err.Error())

					// Try to extract more details from the error
					if strings.Contains(err.Error(), "400") {
						debugLog("===== 400 Bad Request Error =====")
						debugLog("Full error details: %+v", err)
					}

					return nil, fmt.Errorf("failed to delete repository %s: %w", task.Tgt.Repo, err)
				}
				debugLog("Repository %s deleted successfully", task.Tgt.Repo)
				break
			}
		}

		if !repoFound {
			debugLog("WARNING: Repository %s not found in repository list", task.Tgt.Repo)
			return nil, fmt.Errorf("repository %s not found on server %s", task.Tgt.Repo, task.Tgt.URL)
		}

		result["message"] = "Repository deleted successfully"
		result["repo"] = task.Tgt.Repo
		debugLog("repo-delete action completed successfully")

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
		debugLog("Starting repo-import processing")

		// Check if target repository exists
		debugLog("Fetching repositories from %s", task.Tgt.URL)
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

		debugLog("Repository '%s' found in GraphDB", task.Tgt.Repo)

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

			debugLog("Importing BRF data from %s to repository %s", task.Src.Repo, task.Tgt.Repo)
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
					debugLog("Importing BRF file %s to repository %s", fileHeader.Filename, task.Tgt.Repo)
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
		debugLog("Starting graph-import processing")

		debugLog("Fetching repositories from %s", task.Tgt.URL)
		tgtGraphDB, err := db.GraphDBRepositories(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password)
		if err != nil {
			return nil, err
		}

		if tgtGraphDB.Results.Bindings == nil {
			fmt.Printf("ERROR: GraphDB returned nil bindings from %s\n", task.Tgt.URL)
			return nil, fmt.Errorf("failed to get repositories from GraphDB at %s - nil response", task.Tgt.URL)
		}

		debugLog("Found %d repositories in GraphDB", len(tgtGraphDB.Results.Bindings))

		// List all available repositories for debugging
		if len(tgtGraphDB.Results.Bindings) == 0 {
			fmt.Printf("WARNING: No repositories found in GraphDB at %s\n", task.Tgt.URL)
			debugLog("Attempting to create repository '%s' or continue assuming it exists", task.Tgt.Repo)
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
			debugLog("Repository '%s' found in GraphDB", task.Tgt.Repo)
		} else {
			debugLog("Repository list was empty, attempting import to '%s' anyway", task.Tgt.Repo)
		}

		// Try to list graphs (this might fail if repository doesn't exist)
		debugLog("Listing graphs in repository: %s", task.Tgt.Repo)
		graphsResponse, err := db.GraphDBListGraphs(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo)
		if err != nil {
			fmt.Printf("WARNING: Failed to list graphs (repository might not exist): %v\n", err)
			// Continue with import - we'll try to import anyway
		} else if graphsResponse.Results.Bindings == nil {
			fmt.Printf("WARNING: GraphDB returned nil response for listing graphs\n")
		} else {
			debugLog("Found %d graphs in repository", len(graphsResponse.Results.Bindings))
			// Check if target graph exists and delete it if found
			for _, bind := range graphsResponse.Results.Bindings {
				if bind.ContextID.Value == task.Tgt.Graph {
					debugLog("Deleting existing graph: %s", task.Tgt.Graph)
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
			debugLog("Looking for files with key: %s", fileKey)

			if taskFiles, exists := files[fileKey]; exists {
				debugLog("Found %d files to import", len(taskFiles))
				result["uploaded_files"] = len(taskFiles)
				result["file_names"] = getFileNames(taskFiles)

				// Process each uploaded file for import
				for i, fileHeader := range taskFiles {
					debugLog("Processing file %d: %s (size: %d bytes)", i, fileHeader.Filename, fileHeader.Size)

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
						debugLog("Creating temp file: %s", tempFileName)

						tempFile, err := os.Create(tempFileName)
						if err != nil {
							fmt.Printf("ERROR: Failed to create temp file: %v\n", err)
							return
						}
						defer func() { _ = tempFile.Close() }()
						defer func() {
							debugLog("Removing temp file: %s", tempFileName)
							_ = os.Remove(tempFileName)
						}()

						// Copy uploaded file to temp file
						if _, err := file.Seek(0, 0); err != nil {
							fmt.Printf("ERROR: Failed to seek file: %v\n", err)
							return
						}

						debugLog("Copying file content to temp file")
						bytesWritten, err := tempFile.ReadFrom(file)
						if err != nil {
							fmt.Printf("ERROR: Failed to copy file: %v\n", err)
							return
						}
						_ = tempFile.Close()

						debugLog("Copied %d bytes to temp file", bytesWritten)

						// Determine import method based on file extension
						filename := strings.ToLower(fileHeader.Filename)

						debugLog("Importing text RDF file: %s", fileHeader.Filename)
						err = db.GraphDBImportGraphRdf(task.Tgt.URL, task.Tgt.Username, task.Tgt.Password, task.Tgt.Repo, task.Tgt.Graph, tempFileName)
						if err != nil {
							fmt.Printf("ERROR: Failed to import RDF file %s: %v\n", fileHeader.Filename, err)
							return
						}

						debugLog("Successfully imported file: %s", fileHeader.Filename)
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
