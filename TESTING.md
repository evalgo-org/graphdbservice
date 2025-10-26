# Testing Guide for GraphDB Service

This document describes the testing strategy and current test coverage for the GraphDB Service application.

## Test Structure

The project follows Go's standard testing conventions with tests located in `*_test.go` files alongside the code they test.

### Test Files
- `cmd/graphdb_test.go`: Main test file with 1,241 lines of comprehensive tests

## Current Test Coverage

As of the latest test run: **10.4% overall coverage**

### Function Coverage Breakdown

| Function | Coverage | Notes |
|----------|----------|-------|
| `md5Hash` | 100.0% | ✅ Fully tested |
| `apiKeyMiddleware` | 0.0% | ⚠️ Requires echo integration test |
| `getFileNames` | 100.0% | ✅ Fully tested |
| `updateRepositoryNameInConfig` | 85.7% | ✅ Well tested |
| `getGraphTripleCounts` | 100.0% | ✅ Fully tested (stub function) |
| `getFileType` | 100.0% | ✅ Fully tested |
| `getRepositoryNames` | 100.0% | ✅ Fully tested |
| `migrationHandler` | 0.0% | ⚠️ Requires integration test |
| `migrationHandlerJSON` | 58.8% | ⚠️ Partially tested |
| `migrationHandlerMultipart` | 0.0% | ⚠️ Requires integration test |
| `validateTask` | 84.6% | ✅ Well tested |
| `URL2ServiceRobust` | 83.3% | ✅ Well tested |
| `processTask` | 0.0% | ⚠️ Requires integration test (920 lines) |
| `init` | 100.0% | ✅ Automatically tested |

## Test Categories

### 1. Unit Tests (Current)

Unit tests verify individual functions in isolation with mocked dependencies.

**Existing Tests:**

#### Helper Function Tests
- ✅ `TestMd5Hash`: Tests MD5 hash generation with 3 test cases
- ✅ `TestGetFileType`: Tests RDF format detection with 11 test cases
- ✅ `TestURL2ServiceRobust`: Tests URL parsing with 4 test cases
- ✅ `TestGetFileNames`: Tests filename extraction from multipart headers
- ✅ `TestGetRepositoryNames`: Tests repository name extraction from bindings
- ✅ `TestUpdateRepositoryNameInConfig`: Tests TTL configuration file updates
- ✅ `TestGetGraphTripleCounts`: Tests stub triple counting function

#### Validation Tests
- ✅ `TestValidateTask`: Tests task validation with 9 test cases
  - Valid actions: repo-migration, graph-migration, repo-delete, graph-delete, repo-rename, graph-rename
  - Invalid actions and missing required fields

#### Middleware Tests
- ✅ `TestApiKeyMiddleware`: Tests API key authentication with 3 test cases
  - Valid API key
  - Invalid API key
  - Missing API key

#### Handler Tests
- ✅ `TestMigrationHandlerJSON`: Tests JSON request handler with 4 test cases
  - Invalid JSON format
  - Missing version
  - Missing tasks
  - Invalid action

#### GraphDB Operation Tests
- ✅ `TestGraphDBRepositories`: Tests repository listing
- ✅ `TestGraphDBRepositoryConf`: Tests repository configuration download
- ✅ `TestGraphDBRepositoryBrf`: Tests repository backup download
- ✅ `TestGraphDBListGraphs`: Tests graph listing
- ✅ `TestGraphDBExportGraphRdf`: Tests graph export
- ✅ `TestGraphDBImportGraphRdf`: Tests graph import
- ✅ `TestGraphDBDeleteRepository`: Tests repository deletion
- ✅ `TestGraphDBDeleteGraph`: Tests graph deletion
- ✅ `TestGraphDBRestoreConf`: Tests repository restoration from config
- ✅ `TestGraphDBRestoreBrf`: Tests repository restoration from backup

### 2. Integration Tests (Planned)

Integration tests will verify the complete system with real GraphDB instances or more comprehensive mocks.

**Planned Tests:**

#### End-to-End Handler Tests
- ⚠️ `TestMigrationHandlerE2E`: Test complete migration handler flow
- ⚠️ `TestMigrationHandlerMultipartE2E`: Test multipart form handling
- ⚠️ `TestApiKeyMiddlewareWithEcho`: Test middleware with full Echo setup

#### ProcessTask Integration Tests
The `processTask` function (920 lines) handles 9 different actions and requires extensive integration testing:

1. ⚠️ `TestProcessTaskRepoMigration`: Test complete repository migration
   - Download source config and data
   - Create target repository
   - Restore data
   - Verify success

2. ⚠️ `TestProcessTaskGraphMigration`: Test graph migration
   - Export source graph
   - Import to target graph
   - Verify data integrity

3. ⚠️ `TestProcessTaskRepoDelete`: Test repository deletion

4. ⚠️ `TestProcessTaskGraphDelete`: Test graph deletion

5. ⚠️ `TestProcessTaskRepoCreate`: Test repository creation from config file

6. ⚠️ `TestProcessTaskGraphImport`: Test graph import from RDF files
   - Multiple file formats
   - Large files
   - Error handling

7. ⚠️ `TestProcessTaskRepoImport`: Test repository import from BRF backup

8. ⚠️ `TestProcessTaskRepoRename`: Test repository renaming
   - Backup old repository
   - Create new repository
   - Restore data
   - Delete old repository

9. ⚠️ `TestProcessTaskGraphRename`: Test graph renaming
   - Export old graph
   - Import to new graph
   - Delete old graph

#### GraphDB Version Compatibility Tests
- ⚠️ `TestGraphDB10_8_Compatibility`: Test with GraphDB 10.8.x
- ⚠️ `TestGraphDB10_7_Compatibility`: Test with GraphDB 10.7.x
- ⚠️ `TestGraphDB10_6_Compatibility`: Test with GraphDB 10.6.x
- ⚠️ `TestGraphDBUpgradeScenario`: Test migration between versions

#### Ziti Networking Tests
- ⚠️ `TestZitiConnectivity`: Test Ziti zero-trust networking
- ⚠️ `TestZitiFailover`: Test Ziti connection failures

## Running Tests

### Run All Tests
```bash
go test ./...
```

### Run Tests with Coverage
```bash
go test ./cmd -cover
```

### Generate Coverage Report
```bash
go test ./cmd -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Run Specific Test
```bash
go test ./cmd -run TestMd5Hash
```

### Run Tests with Verbose Output
```bash
go test ./cmd -v
```

### Run Tests with Race Detection
```bash
go test ./cmd -race
```

## Test Infrastructure

### Mock GraphDB Server

The test suite includes a comprehensive mock GraphDB server (`setupMockGraphDBServer`) that simulates:

- Repository listing (`/repositories`)
- Repository operations (`/rest/repositories/*`)
- Graph operations (`/repositories/{repo}/rdf-graphs/*`)
- Config download (`/rest/repositories/{repo}/download-ttl`)
- BRF backup/restore (`/repositories/{repo}/statements`)
- SPARQL updates

### Test Fixtures

Test files and configurations are created as needed using temporary files:
- TTL configuration files
- RDF data files
- BRF backup files

## Test Guidelines

### Writing Unit Tests

1. **Use Table-Driven Tests**: Define test cases in a slice for easy addition
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"case 1", "input1", "output1"},
    {"case 2", "input2", "output2"},
}
```

2. **Test Edge Cases**: Always test empty inputs, nil values, and boundary conditions

3. **Use Subtests**: Use `t.Run()` for better test organization and parallel execution

4. **Clean Up Resources**: Use `defer` to ensure cleanup happens
```go
defer os.Remove(tmpFile.Name())
defer cleanup()
```

### Writing Integration Tests

1. **Use Docker Compose**: Spin up real GraphDB instances for testing
2. **Test Data Isolation**: Each test should use separate repositories/graphs
3. **Test Timeouts**: Set appropriate timeouts for long-running operations
4. **Test Failure Scenarios**: Test network failures, auth failures, etc.

## CI/CD Integration

Tests are automatically run in the GitLab CI/CD pipeline:

```yaml
test:unit:
  stage: test
  script:
    - go test ./cmd -v -cover -race
```

### Coverage Thresholds

Current target: 10%+ (baseline)
Future target: 60%+ (with integration tests)

## Future Improvements

1. **Increase Unit Test Coverage**
   - Add more handler tests with proper Echo setup
   - Test error paths more thoroughly

2. **Add Integration Tests**
   - Set up Docker-based GraphDB test environment
   - Test all processTask operations end-to-end
   - Test GraphDB version compatibility

3. **Add Performance Tests**
   - Benchmark critical operations
   - Test with large datasets
   - Memory leak detection

4. **Add Property-Based Tests**
   - Use fuzzing for input validation
   - Generate random valid/invalid requests

## Dependencies

Test dependencies are managed in `go.mod`:
- `testing` (standard library)
- `net/http/httptest` (standard library)
- `github.com/labstack/echo/v4` (for handler tests)
- `eve.evalgo.org/db` (GraphDB client library)

## Troubleshooting

### Tests Fail Due to API Key
Ensure `API_KEY` environment variable is not set when running tests, or set it to the test value:
```bash
unset API_KEY
go test ./cmd
```

### Mock Server Port Conflicts
The mock server uses dynamic ports from `httptest.NewServer()`, so port conflicts should not occur.

### Coverage Report Not Generated
Ensure you use the `-coverprofile` flag:
```bash
go test ./cmd -coverprofile=coverage.out
```

## Contact

For questions about testing, contact:
- Francisc Simon <francisc.simon@pantopix.com>

---

**Last Updated**: 2025-10-26
**Test Suite Version**: v0.0.3
**Total Tests**: 20+ test functions with 80+ sub-tests
**Current Coverage**: 10.4%
