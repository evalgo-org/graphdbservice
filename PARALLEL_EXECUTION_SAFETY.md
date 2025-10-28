# Parallel Execution Safety

## Overview
This document describes how the service handles parallel execution of migration tasks safely without file naming conflicts.

## Problem Statement

When multiple migration tasks run in parallel (either through multiple API requests or concurrent UI sessions), temporary file operations could cause conflicts if they use the same filenames. This could lead to:

- **Data corruption**: One task overwrites another's temporary files
- **Race conditions**: Two tasks trying to create/delete the same file
- **Failed operations**: File not found errors due to premature deletion by another task
- **Unpredictable results**: Mixed data from different operations

### Original Vulnerable Code

The original implementation used predictable filenames based on repository names and MD5 hashes:

```go
// VULNERABLE: Two parallel operations on same repo would conflict
tempFileName := fmt.Sprintf("/tmp/%s", fileHeader.Filename)
configFile := fmt.Sprintf("/tmp/repo_create_%s_%s", md5Hash(repoName), fileHeader.Filename)
graphFileName := fmt.Sprintf("/tmp/repo_rename_%s_%s.rdf", md5Hash(oldRepoName), md5Hash(graphURI))
```

**Risk Scenarios:**
1. Two users migrate the same repository simultaneously → file overwrite
2. Same repository renamed twice in parallel → graph backup collision
3. Multiple graph imports with same filename → data mixing

## Solution: UUID-Based Unique Filenames

All temporary file operations now use UUID v4 to guarantee unique filenames:

```go
// SAFE: Globally unique identifier prevents any collision
tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("repo_import_%s%s", uuid.New().String(), fileExt))
```

### Benefits

1. **Collision-Free**: UUID v4 provides 122 bits of randomness (~5.3×10³⁶ possible values)
2. **Cross-Platform**: `os.TempDir()` respects OS conventions (/tmp on Linux, %TEMP% on Windows)
3. **Preserves Extension**: Original file extensions maintained for proper MIME type detection
4. **Self-Documenting**: Prefix indicates operation type (repo_import, graph_rename, etc.)
5. **Automatic Cleanup**: Defer statements ensure files deleted even on errors

## Changes Made

### 1. Import Additions
```go
import (
    "path/filepath"                    // For cross-platform path operations
    "github.com/google/uuid"          // For UUID generation
)
```

### 2. Repository Import (repo-import)
**Location**: `cmd/graphdb.go:1041`

**Before:**
```go
tempFileName := fmt.Sprintf("/tmp/%s", fileHeader.Filename)
```

**After:**
```go
fileExt := filepath.Ext(fileHeader.Filename)
tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("repo_import_%s%s", uuid.New().String(), fileExt))
```

**Example**: `/tmp/repo_import_a3bb189e-2b8f-4d91-9c4a-f8e7d6c5b4a3.brf`

---

### 3. Repository Creation (repo-create)
**Location**: `cmd/graphdb.go:1121`

**Before:**
```go
configFile := fmt.Sprintf("/tmp/repo_create_%s_%s", md5Hash(repoName), fileHeader.Filename)
```

**After:**
```go
fileExt := filepath.Ext(fileHeader.Filename)
configFile := filepath.Join(os.TempDir(), fmt.Sprintf("repo_create_%s%s", uuid.New().String(), fileExt))
```

**Example**: `/tmp/repo_create_7c4d3e2f-1a5b-4c8d-9e6f-0b1a2c3d4e5f.ttl`

---

### 4. Graph Import (graph-import)
**Location**: `cmd/graphdb.go:1293`

**Before:**
```go
tempFileName := fmt.Sprintf("/tmp/%s", fileHeader.Filename)
```

**After:**
```go
fileExt := filepath.Ext(fileHeader.Filename)
tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("graph_import_%s%s", uuid.New().String(), fileExt))
```

**Example**: `/tmp/graph_import_9f8e7d6c-5b4a-3c2d-1e0f-9a8b7c6d5e4f.ttl`

---

### 5. Repository Rename (repo-rename)
**Location**: `cmd/graphdb.go:1416`

**Before:**
```go
graphFileName := fmt.Sprintf("/tmp/repo_rename_%s_%s.rdf", md5Hash(oldRepoName), md5Hash(graphURI))
```

**After:**
```go
graphFileName := filepath.Join(os.TempDir(), fmt.Sprintf("repo_rename_%s.rdf", uuid.New().String()))
```

**Example**: `/tmp/repo_rename_2e3f4a5b-6c7d-8e9f-0a1b-2c3d4e5f6a7b.rdf`

**Note**: Each graph gets its own UUID, allowing parallel rename of multiple repositories with overlapping graph names.

---

### 6. Graph Rename (graph-rename)
**Location**: `cmd/graphdb.go:1573`

**Before:**
```go
tempFileName := fmt.Sprintf("/tmp/graph_rename_%s.rdf", md5Hash(oldGraphName))
```

**After:**
```go
tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("graph_rename_%s.rdf", uuid.New().String()))
```

**Example**: `/tmp/graph_rename_5d4c3b2a-1e0f-9a8b-7c6d-5e4f3a2b1c0d.rdf`

---

## Parallel Execution Scenarios

### Scenario 1: Simultaneous Repository Migrations
**Setup**: Two users migrate "prod-repo" from Server A to Server B

**Before**: ❌ CONFLICT
```
User 1: /tmp/repo_rename_a1b2c3d4_graph1.rdf  ← Created
User 2: /tmp/repo_rename_a1b2c3d4_graph1.rdf  ← OVERWRITES User 1's file!
```

**After**: ✅ SAFE
```
User 1: /tmp/repo_rename_f4e3d2c1-1234-5678-abcd-ef0123456789.rdf
User 2: /tmp/repo_rename_a9b8c7d6-9876-5432-fedc-ba9876543210.rdf
```

---

### Scenario 2: Multiple Graph Imports
**Setup**: Three parallel imports of "data.ttl" to different graphs

**Before**: ❌ CONFLICT
```
Task 1: /tmp/data.ttl  ← Created
Task 2: /tmp/data.ttl  ← OVERWRITES Task 1!
Task 3: /tmp/data.ttl  ← OVERWRITES Task 2!
```

**After**: ✅ SAFE
```
Task 1: /tmp/graph_import_123e4567-e89b-12d3-a456-426614174000.ttl
Task 2: /tmp/graph_import_987f6543-e21b-45d3-a987-654321098765.ttl
Task 3: /tmp/graph_import_456a7890-c12d-34e5-b678-901234567890.ttl
```

---

### Scenario 3: Concurrent Repository Creations
**Setup**: Two repos created with same config filename "repo-config.ttl"

**Before**: ❌ CONFLICT (if repo names hash the same or have collision)
```
Repo A: /tmp/repo_create_abc123_repo-config.ttl  ← Created
Repo B: /tmp/repo_create_abc123_repo-config.ttl  ← POTENTIAL CONFLICT
```

**After**: ✅ SAFE
```
Repo A: /tmp/repo_create_a1b2c3d4-e5f6-7890-abcd-ef0123456789.ttl
Repo B: /tmp/repo_create_f9e8d7c6-b5a4-3210-fedc-ba9876543210.ttl
```

---

## Performance Considerations

### UUID Generation Performance
- **Speed**: ~3 microseconds per UUID on modern hardware
- **Impact**: Negligible compared to GraphDB network operations (milliseconds to seconds)
- **Scalability**: Can generate millions of UUIDs per second

### Disk Space
- **Temporary files**: Automatically cleaned up via `defer` statements
- **Cleanup timing**: Immediate after operation completes or on error
- **Leak prevention**: Even if process crashes, OS cleans /tmp on reboot

### Memory Usage
- **UUID size**: 36 bytes (string representation) or 16 bytes (binary)
- **Per-operation overhead**: < 100 bytes per temporary file
- **Total impact**: Insignificant even for 1000+ concurrent operations

## Testing

### Unit Tests
All existing tests pass without modification, confirming:
- Backward compatibility maintained
- File operations work correctly
- Error handling unchanged
- Cleanup logic functions properly

### Manual Testing Scenarios

#### Test 1: Parallel Repository Creation
```bash
# Terminal 1
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: test" \
  -F 'request={"version": "v0.0.1", "tasks": [{"action": "repo-create", "tgt": {...}}]}' \
  -F 'task_0_config=@repo-config.ttl'

# Terminal 2 (run simultaneously)
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: test" \
  -F 'request={"version": "v0.0.1", "tasks": [{"action": "repo-create", "tgt": {...}}]}' \
  -F 'task_0_config=@repo-config.ttl'
```

**Expected**: Both succeed without conflicts

#### Test 2: Concurrent Graph Imports
```bash
# Start 5 parallel graph imports
for i in {1..5}; do
  curl -X POST http://localhost:8080/v1/api/action \
    -H "x-api-key: test" \
    -F 'request={"version": "v0.0.1", "tasks": [{"action": "graph-import", "tgt": {...}}]}' \
    -F 'task_0_files=@data.ttl' &
done
wait
```

**Expected**: All 5 imports succeed independently

#### Test 3: Repository Rename While Importing
```bash
# Terminal 1: Start rename (slow operation)
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: test" \
  -d '{"version": "v0.0.1", "tasks": [{"action": "repo-rename", "tgt": {...}}]}'

# Terminal 2: Import to same repo (immediate)
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: test" \
  -F 'request={"version": "v0.0.1", "tasks": [{"action": "graph-import", "tgt": {...}}]}' \
  -F 'task_0_files=@data.ttl'
```

**Expected**: Both operations complete successfully without interfering

## Monitoring and Debugging

### Checking Temporary Files
```bash
# List all service temporary files
ls -lh /tmp/*_import_* /tmp/*_create_* /tmp/*_rename_*

# Monitor file creation in real-time
watch -n 0.5 'ls -lh /tmp/ | grep -E "import|create|rename"'

# Check for leaked files (should be empty after operations complete)
find /tmp -name "*_import_*" -o -name "*_create_*" -o -name "*_rename_*" -mmin +10
```

### Logging UUID Filenames
All operations log the temporary filename:
```
DEBUG: Creating temp file: /tmp/graph_import_a3bb189e-2b8f-4d91-9c4a-f8e7d6c5b4a3.ttl
DEBUG: Removing temp file: /tmp/graph_import_a3bb189e-2b8f-4d91-9c4a-f8e7d6c5b4a3.ttl
```

This enables:
- Tracking file lifecycle
- Debugging cleanup issues
- Auditing parallel operations
- Troubleshooting file permission errors

## Security Considerations

### File Permissions
Temporary files are created with permissions `0644` (owner read/write, others read):
```go
tempFile, err := os.Create(tempFileName)  // Creates with default umask
```

**Recommendation for production**:
```go
// Create with restricted permissions
tempFile, err := os.OpenFile(tempFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
```

### Temporary Directory Security
- Uses `os.TempDir()` which respects OS security settings
- On Linux: `/tmp` (world-writable but sticky bit prevents deletion by others)
- Consider using dedicated directory with restricted permissions in production

### UUID Predictability
- UUIDs are **not secret** - they appear in logs
- UUIDs provide **uniqueness**, not **security**
- Do not rely on UUID secrecy for access control
- File permissions and API authentication are the security layers

## Future Improvements

### 1. Configurable Temp Directory
Allow users to specify custom temp directory:
```go
var tempDir = flag.String("temp-dir", os.TempDir(), "Directory for temporary files")
```

### 2. Disk Space Monitoring
Check available space before creating large temporary files:
```go
func checkDiskSpace(path string, requiredBytes int64) error {
    var stat syscall.Statfs_t
    syscall.Statfs(path, &stat)
    availableBytes := stat.Bavail * uint64(stat.Bsize)
    if availableBytes < uint64(requiredBytes) {
        return fmt.Errorf("insufficient disk space")
    }
    return nil
}
```

### 3. Temporary File Metrics
Track temporary file operations:
- Count of active temp files
- Total disk space used
- Average cleanup time
- Leaked file detection

### 4. Graceful Shutdown Cleanup
Register cleanup handler for SIGTERM/SIGINT:
```go
func cleanupTempFiles() {
    pattern := filepath.Join(os.TempDir(), "*_import_*")
    files, _ := filepath.Glob(pattern)
    for _, f := range files {
        os.Remove(f)
    }
}
```

## Conclusion

The UUID-based filename approach provides:
- ✅ **Thread-safe parallel execution**
- ✅ **Zero collision probability** (for practical purposes)
- ✅ **Cross-platform compatibility**
- ✅ **Easy debugging and monitoring**
- ✅ **Negligible performance overhead**
- ✅ **Automatic cleanup guarantees**

This change enables the service to safely handle high-concurrency workloads without any risk of file operation conflicts.
