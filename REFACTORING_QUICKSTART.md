# Refactoring Quick Start Guide

## How to Use the New Refactored Code

### 1. Create a Client Manager

```go
import (
    "graphdbservice/internal/client"
    "graphdbservice/internal/helpers"
)

// Create once at service startup
clientMgr := client.NewManager(identityFile, debugMode)
```

### 2. Create an Operations Registry

```go
import "graphdbservice/internal/operations"

// Create once at service startup
registry := operations.NewRegistry(clientMgr)
```

### 3. Execute a Task

```go
import (
    "context"
    "graphdbservice/internal/domain"
)

// Create a task
task := domain.Task{
    Action: "repo-migration",
    Src: &domain.Repository{
        URL:      "http://source-graphdb:7200",
        Username: "admin",
        Password: "password",
        Repo:     "source-repo",
    },
    Tgt: &domain.Repository{
        URL:      "http://target-graphdb:7200",
        Username: "admin",
        Password: "password",
        Repo:     "target-repo",
    },
}

// Execute the task
result, err := registry.Handle(context.Background(), task, nil)
if err != nil {
    // Handle error appropriately
    log.Error(err)
    return
}

// Process result
fmt.Printf("Operation result: %v\n", result)
```

### 4. Handle File Uploads

```go
import "mime/multipart"

// For operations that accept files (repo-create, graph-import, etc.)
files := map[string][]*multipart.FileHeader{
    "task_0_config": configHeaders,  // or task_0_files for imports
}

result, err := registry.Handle(context.Background(), task, files)
```

## Error Handling

### Old Way (Generic Errors)
```go
if err != nil {
    log.Error("operation failed:", err)
    // Can't determine error type
}
```

### New Way (Typed Errors)
```go
import (
    "graphdbservice/internal/domain"
    "errors"
)

result, err := registry.Handle(ctx, task, files)
if err != nil {
    var notFoundErr *domain.NotFoundError
    if errors.As(err, &notFoundErr) {
        // Handle not found - maybe suggest creating the resource
    }
    
    var conflictErr *domain.ConflictError
    if errors.As(err, &conflictErr) {
        // Handle conflict - resource already exists
    }
    
    var opErr *domain.OperationError
    if errors.As(err, &opErr) {
        // Handle operation error - see underlying cause
        log.Error("operation failed:", opErr.Message, opErr.Cause)
    }
}
```

## File Handling Pattern

### Old Way (Error-Prone)
```go
confFile, _ := saveFile(header)
defer os.Remove(confFile)

// If we return early or panic, file might not be cleaned up
if err := process(confFile); err != nil {
    return nil, err  // ← confFile may leak!
}
```

### New Way (Safe)
```go
cleanup := helpers.NewFileCleanup()
defer cleanup.Cleanup()  // Guaranteed cleanup

confFile, err := helpers.SaveMultipartFileWithCleanup(header, helpers.TempFileRepoCreatePrefix, cleanup)
if err != nil {
    return nil, err  // Files are auto-cleaned
}

// confFile is registered for cleanup
// If we return early or panic, cleanup still happens
```

## Adding a New Operation

To add a new GraphDB operation:

### 1. Create a Handler

```go
// File: internal/operations/my_new_op.go

type MyNewOpHandler struct {
    BaseHandler
}

func NewMyNewOpHandler(clientMgr *client.Manager) Handler {
    return &MyNewOpHandler{
        BaseHandler: BaseHandler{ClientManager: clientMgr},
    }
}

func (h *MyNewOpHandler) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
    // Implementation
    result := Result()
    
    // ... do work ...
    
    SetResult(result, "status", "completed")
    SetResult(result, "message", "Operation completed")
    return result, nil
}
```

### 2. Register the Handler

```go
// In internal/operations/registry.go, add to NewRegistry():
reg.Register("my-new-op", NewMyNewOpHandler(clientMgr))
```

### 3. Test It

```go
// File: internal/operations/my_new_op_test.go

import "testing"

func TestMyNewOp(t *testing.T) {
    clientMgr := client.NewManager("", false)
    handler := NewMyNewOpHandler(clientMgr)
    
    task := domain.Task{
        Action: "my-new-op",
        // ... setup task ...
    }
    
    result, err := handler.Handle(context.Background(), task, nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Verify result
    if status, ok := result["status"].(string); !ok || status != "completed" {
        t.Error("expected status to be completed")
    }
}
```

## Common Patterns

### Checking if Repository Exists
```go
exists, err := helpers.ValidateRepositoryExists(client, url, username, password, repoName)
if err != nil {
    return nil, domain.NewOperationError("my-op", "failed to check repository", err)
}

if !exists {
    return nil, domain.NewNotFoundError("repository", repoName)
}
```

### Getting File Type
```go
fileType := helpers.GetFileType(filename)
// Returns: "turtle", "rdf-xml", "n-triples", "json-ld", etc.
```

### Normalizing URLs
```go
url := helpers.NormalizeURL(taskUrl)  // Removes trailing slashes
```

### Debug Logging
```go
helpers.DebugLog("Processing repository: %s", repoName)
helpers.DebugLogHTTP("GET %s", endpoint)
```

## Environment Configuration

The code maintains backward compatibility with environment variables:

```bash
export GRAPHDB_DEBUG=true          # Enables debug logging
export GRAPHDB_IDENTITY=/path/ziti  # Ziti identity file
export API_KEY=secret-key           # API authentication
```

Debug mode:
- Sets `helpers.DebugMode = true`
- Enables detailed HTTP logging via `EnableHTTPDebugLogging`
- Logs to stdout with "DEBUG:" prefix

## Migration Checklist

When migrating existing code to the new handlers:

- [ ] Replace `processTask()` calls with `registry.Handle()`
- [ ] Replace direct error creation with typed errors (`domain.New*Error()`)
- [ ] Replace `md5Hash()` with `helpers.MD5Hash()`
- [ ] Replace `normalizeURL()` with `helpers.NormalizeURL()`
- [ ] Replace manual file handling with `FileCleanup`
- [ ] Update imports to use `graphdbservice/internal/*`
- [ ] Test with `go test -v ./...`
- [ ] Run with race detector: `go test -race ./...`

## Next Steps

1. Update `cmd/graphdbservice/service.go` to create and wire the registry
2. Update `cmd/graphdbservice/semantic_api.go` to use `registry.Handle()`
3. Update `cmd/graphdbservice/rest_handlers.go` similarly
4. Run full test suite
5. Test with actual GraphDB instance
