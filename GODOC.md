# GraphDB Service - API Documentation

This document provides comprehensive godoc documentation for the GraphDB Service application.

## Viewing Documentation

### Option 1: Command Line
```bash
# View documentation for the entire project
go doc -all evalgo.org/graphservice

# View documentation for the cmd package
go doc -all evalgo.org/graphservice/cmd

# View documentation for a specific type
go doc evalgo.org/graphservice/cmd.MigrationRequest

# View documentation for a specific function
go doc evalgo.org/graphservice/cmd.processTask
```

### Option 2: Local Web Server (Recommended)
```bash
# Start the pkgsite documentation server
pkgsite -http=:6060

# Then open http://localhost:6060/evalgo.org/graphservice in your browser
```

### Option 3: Go Package Documentation Site
Once published, the documentation will be available at:
https://pkg.go.dev/evalgo.org/graphservice

---

## Package: main

**Import Path:** `evalgo.org/graphservice`

### Overview
Package main provides the entry point for the GraphDB Service application.

GraphDB Service is a comprehensive API service for managing GraphDB repositories and RDF graphs. It provides RESTful endpoints for repository migration, graph management, data import/export, and various administrative operations on GraphDB instances.

### Features
- Repository migration between GraphDB instances
- Named graph operations (import, export, delete, rename)
- Repository management (create, delete, rename)
- Secure connectivity via Ziti zero-trust networking
- API key authentication for all operations
- Multipart form uploads for configuration and data files

### Usage
```bash
graphservice graphdb [flags]
```

### Environment Variables
- `API_KEY`: Required API key for authentication
- `PORT`: HTTP server port (default: 8080)

### Example
```bash
export API_KEY=your-secret-key
export PORT=8080
graphservice graphdb --identity /path/to/ziti/identity.json
```

---

## Package: cmd

**Import Path:** `evalgo.org/graphservice/cmd`

### Overview
Package cmd provides the command-line interface for the GraphDB Service application.

This package implements a cobra-based CLI with commands for:
- `graphdb`: Start the GraphDB service API server
- `version`: Display version and build information

### Configuration
The CLI supports configuration via:
- Command-line flags
- Configuration files (YAML format)
- Environment variables

Configuration File Locations:
- Specified via `--config` flag
- `$HOME/.cobra.yaml` (default)

### Author
Francisc Simon <francisc.simon@pantopix.com>

### License
Apache License 2.0

---

## Types

### MigrationRequest

```go
type MigrationRequest struct {
    Version string `json:"version" validate:"required"` // API version (e.g., "v0.0.1")
    Tasks   []Task `json:"tasks" validate:"required"`   // List of tasks to execute
}
```

**Description:**
MigrationRequest represents the root request structure for GraphDB operations. It contains the API version and a list of tasks to be executed sequentially.

**Example JSON:**
```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "repo-migration",
      "src": {
        "url": "http://source:7200",
        "username": "admin",
        "password": "pass",
        "repo": "source-repo"
      },
      "tgt": {
        "url": "http://target:7200",
        "username": "admin",
        "password": "pass",
        "repo": "target-repo"
      }
    }
  ]
}
```

---

### Task

```go
type Task struct {
    Action string      `json:"action" validate:"required"` // The action to perform
    Src    *Repository `json:"src,omitempty"`              // Source repository/graph (for migration operations)
    Tgt    *Repository `json:"tgt,omitempty"`              // Target repository/graph (for all operations)
}
```

**Description:**
Task represents a single operation to be performed on GraphDB repositories or graphs.

**Supported Actions:**
- `repo-migration`: Migrate entire repository (config + data)
- `graph-migration`: Migrate a named graph between repositories
- `repo-delete`: Delete a repository
- `graph-delete`: Delete a named graph
- `repo-create`: Create a new repository from TTL configuration
- `graph-import`: Import RDF data into a graph
- `repo-import`: Import repository from BRF backup file
- `repo-rename`: Rename a repository (backup, recreate, restore)
- `graph-rename`: Rename a graph (export, import, delete)

---

### Repository

```go
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
```

**Description:**
Repository represents the connection details and identifiers for a GraphDB repository or graph. Different fields are required depending on the operation being performed.

**Field Usage by Action:**

| Action | Required Fields in Src | Required Fields in Tgt |
|--------|------------------------|------------------------|
| `repo-migration` | URL, Username, Password, Repo | URL, Username, Password, Repo |
| `graph-migration` | URL, Username, Password, Repo, Graph | URL, Username, Password, Repo, Graph |
| `repo-delete` | N/A | URL, Username, Password, Repo |
| `graph-delete` | N/A | URL, Username, Password, Repo, Graph |
| `repo-create` | N/A | URL, Username, Password, Repo (+ config file) |
| `graph-import` | N/A | URL, Username, Password, Repo, Graph (+ data files) |
| `repo-import` | N/A | URL, Username, Password, Repo (+ BRF file) |
| `repo-rename` | N/A | URL, Username, Password, RepoOld, RepoNew |
| `graph-rename` | N/A | URL, Username, Password, Repo, GraphOld, GraphNew |

---

## Functions

### Execute

```go
func Execute() error
```

**Description:**
Execute executes the root command and returns any error that occurs. This is the main entry point for the CLI application.

**Returns:**
- `error`: Any error that occurs during command execution

---

### URL2ServiceRobust

```go
func URL2ServiceRobust(urlStr string) (string, error)
```

**Description:**
URL2ServiceRobust parses a URL string and extracts the service portion (host:port). This function is more forgiving than standard URL parsing - it automatically adds the http:// scheme if missing, which helps with user-provided URLs.

The function is primarily used when working with Ziti networking to extract the service identifier from a full GraphDB URL.

**Examples:**
- `"http://localhost:7200"` → `"localhost:7200"`
- `"https://graphdb.example.com:7200"` → `"graphdb.example.com:7200"`
- `"graphdb:7200"` → `"graphdb:7200"` (scheme added automatically)
- `"localhost"` → `"localhost"` (port omitted if not specified)

**Parameters:**
- `urlStr`: The URL string to parse (with or without scheme)

**Returns:**
- `service`: The host:port portion of the URL, or just host if port not specified
- `error`: An error if the URL cannot be parsed

---

### apiKeyMiddleware

```go
func apiKeyMiddleware(next echo.HandlerFunc) echo.HandlerFunc
```

**Description:**
apiKeyMiddleware validates the API key in the request header. It checks the "x-api-key" header against the API_KEY environment variable. If the key is missing or invalid, it returns a 401 Unauthorized error.

**Environment Variables:**
- `API_KEY`: The expected API key value

**HTTP Headers:**
- `x-api-key`: The API key provided by the client

**Returns:**
- 401 Unauthorized if the API key is missing or invalid
- Otherwise, passes control to the next handler

---

### getFileType

```go
func getFileType(filename string) string
```

**Description:**
getFileType determines the RDF serialization format based on the file extension. This is used to specify the correct content type when importing data into GraphDB.

**Supported Formats:**
- `.brf`: Binary RDF (GraphDB's backup format)
- `.rdf`, `.xml`: RDF/XML
- `.ttl`: Turtle
- `.nt`: N-Triples
- `.n3`: Notation3
- `.jsonld`, `.json`: JSON-LD
- `.trig`: TriG (for named graphs)
- `.nq`: N-Quads
- default: `unknown`

**Parameters:**
- `filename`: The name of the file (extension is extracted and compared)

**Returns:**
- The RDF format identifier string (e.g., "turtle", "rdf-xml", "binary-rdf")

---

### validateTask

```go
func validateTask(task Task) error
```

**Description:**
validateTask validates that a task has the correct structure and required fields based on its action type. This prevents invalid requests from being processed.

**Validation Rules:**
- `repo-migration`, `graph-migration`: Requires both Src and Tgt repositories
- `repo-delete`, `graph-delete`: Requires Tgt repository
- `repo-create`: Requires Tgt repository (config file validated separately)
- `graph-import`: Requires Tgt repository (data files validated separately)
- `repo-import`: Requires Tgt repository (BRF file validated separately)
- `repo-rename`: Requires Tgt with RepoOld and RepoNew fields
- `graph-rename`: Requires Tgt with GraphOld and GraphNew fields

**Parameters:**
- `task`: The Task structure to validate

**Returns:**
- `error`: 400 Bad Request if validation fails, describing the missing/invalid field
- `nil`: If the task structure is valid for its action type

---

### processTask

```go
func processTask(task Task, files map[string][]*multipart.FileHeader, taskIndex int) (map[string]interface{}, error)
```

**Description:**
processTask executes a single GraphDB operation task based on the specified action. This is the core processing function that handles all supported operations.

**Supported Operations:**

1. **repo-migration**: Migrates an entire repository from source to target GraphDB
   - Downloads source repository configuration (TTL) and data (BRF)
   - Creates target repository with the configuration
   - Restores data from BRF backup
   - Cleans up temporary files

2. **graph-migration**: Migrates a named graph between repositories
   - Exports source graph as RDF
   - Imports RDF into target graph
   - Cleans up temporary files

3. **repo-delete**: Deletes a repository from GraphDB

4. **graph-delete**: Deletes a named graph from a repository

5. **repo-create**: Creates a new repository from TTL configuration
   - Reads configuration from multipart file upload
   - Creates repository via GraphDB API

6. **graph-import**: Imports RDF data files into a named graph
   - Reads RDF files from multipart upload
   - Detects RDF format from file extensions
   - Imports each file into the target graph

7. **repo-import**: Restores a repository from BRF backup file

8. **repo-rename**: Renames a repository (backup, recreate, restore flow)
   - Downloads old repository configuration and data
   - Updates repository name in configuration
   - Creates new repository with updated name
   - Restores data from backup
   - Deletes old repository

9. **graph-rename**: Renames a named graph (export, import, delete flow)
   - Exports old graph as RDF
   - Imports RDF into new graph
   - Deletes old graph

**Ziti Support:**
If the global `identityFile` variable is set, the function creates Ziti-enabled HTTP clients for secure, zero-trust networking between the service and GraphDB instances.

**Error Handling:**
The function includes panic recovery to handle unexpected errors gracefully. All temporary files are cleaned up, even when errors occur.

**Parameters:**
- `task`: The Task structure containing action type and repository details
- `files`: Map of multipart file headers keyed by form field name (e.g., "task_0_config")
- `taskIndex`: The index of this task in the request (used to look up files)

**Returns:**
- `result`: A map containing task results (action, status, message, etc.)
- `error`: An error if the task fails, or nil on success

---

### migrationHandler

```go
func migrationHandler(c echo.Context) error
```

**Description:**
migrationHandler is the main HTTP handler for the `/v1/api/action` endpoint. It routes requests to either the JSON or multipart handler based on the Content-Type header.

**Supported Formats:**
- `application/json`: Routes to migrationHandlerJSON
- `multipart/form-data`: Routes to migrationHandlerMultipart

**HTTP Details:**
- **Method**: POST
- **Endpoint**: `/v1/api/action`
- **Authentication**: Requires `x-api-key` header (enforced by middleware)

---

### migrationHandlerJSON

```go
func migrationHandlerJSON(c echo.Context) error
```

**Description:**
migrationHandlerJSON processes GraphDB operations submitted as JSON requests. This handler is used for operations that don't require file uploads (e.g., repository migration, graph migration, delete operations).

**Request Format:**
```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "repo-migration",
      "src": {...},
      "tgt": {...}
    }
  ]
}
```

**Response Format:**
```json
{
  "status": "success",
  "version": "v0.0.1",
  "results": [...]
}
```

**HTTP Details:**
- **Method**: POST
- **Content-Type**: application/json

---

### migrationHandlerMultipart

```go
func migrationHandlerMultipart(c echo.Context) error
```

**Description:**
migrationHandlerMultipart processes GraphDB operations submitted as multipart form data. This handler is required for operations that involve file uploads.

**Required for Actions:**
- `repo-create` (requires TTL configuration file)
- `graph-import` (requires RDF data files)
- `repo-import` (requires BRF backup file)

**Form Fields:**
- `"request"`: JSON string containing the MigrationRequest object
- `"task_{index}_config"`: Configuration file for task at index (for repo-create)
- `"task_{index}_files"`: RDF data files for task at index (for graph-import/repo-import)

**HTTP Details:**
- **Method**: POST
- **Content-Type**: multipart/form-data
- **Memory Limit**: 32MB per request

---

## Helper Functions

### md5Hash

```go
func md5Hash(text string) string
```

Generates an MD5 hash of the given text string, used for creating unique temporary file names.

---

### getFileNames

```go
func getFileNames(fileHeaders []*multipart.FileHeader) []string
```

Extracts the filenames from a slice of multipart file headers.

---

### updateRepositoryNameInConfig

```go
func updateRepositoryNameInConfig(configFile, oldName, newName string) error
```

Updates repository name references in a GraphDB TTL configuration file. Used during repository rename operations.

---

### getGraphTripleCounts

```go
func getGraphTripleCounts(url, username, password, repo, oldGraph, newGraph string) (int, int)
```

Retrieves the triple counts for two graphs in a GraphDB repository (stub implementation - returns -1).

---

### getRepositoryNames

```go
func getRepositoryNames(bindings []db.GraphDBBinding) []string
```

Extracts repository names from GraphDB API response bindings.

---

## API Endpoints

### POST /v1/api/action

**Description**: Main endpoint for executing GraphDB operations

**Authentication**: Required (`x-api-key` header)

**Content-Type**:
- `application/json` - For operations without file uploads
- `multipart/form-data` - For operations requiring file uploads

**Request Body** (JSON):
```json
{
  "version": "v0.0.1",
  "tasks": [...]
}
```

**Response** (JSON):
```json
{
  "status": "success",
  "version": "v0.0.1",
  "results": [...]
}
```

**Status Codes**:
- `200 OK`: Operation completed successfully
- `400 Bad Request`: Invalid request format or validation error
- `401 Unauthorized`: Missing or invalid API key
- `500 Internal Server Error`: Task processing failed

---

### GET /health

**Description**: Health check endpoint

**Authentication**: Not required

**Response** (JSON):
```json
{
  "status": "healthy"
}
```

**Status Codes**:
- `200 OK`: Service is healthy

---

## Development

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...
```

### Building
```bash
# Standard build
go build -o graphdb-service .

# Build with version information
go build -ldflags "-X evalgo.org/graphservice/cmd.version=v1.0.0 \
  -X evalgo.org/graphservice/cmd.commit=$(git rev-parse HEAD) \
  -X evalgo.org/graphservice/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o graphdb-service .
```

### Running Locally
```bash
# Set environment variables
export API_KEY=your-secret-key
export PORT=8080

# Run the service
./graphdb-service graphdb

# With Ziti identity
./graphdb-service graphdb --identity /path/to/ziti/identity.json
```

---

## Documentation Generated

This documentation was generated on 2025-10-26 using Go's built-in godoc tool.

To regenerate:
```bash
go doc -all evalgo.org/graphservice > GODOC.md
go doc -all evalgo.org/graphservice/cmd >> GODOC.md
```

To view interactively:
```bash
pkgsite -http=:6060
# Open http://localhost:6060/evalgo.org/graphservice
```
