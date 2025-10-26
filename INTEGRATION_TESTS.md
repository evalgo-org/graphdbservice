# Integration Testing Guide

This document describes how to run integration tests for the GraphDB Service application.

## Overview

Integration tests verify the complete functionality of the GraphDB Service by testing against real GraphDB instances running in Docker containers. These tests cover:

- Repository migration between GraphDB instances
- Graph migration within and across repositories
- Repository creation and deletion
- GraphDB version compatibility (10.8.5 ↔ 10.7.4)
- End-to-end API workflows with multipart uploads
- Real data operations with RDF triples

## Requirements

### System Requirements
- Docker installed and running
- Go 1.25.1 or later
- Task (taskfile) installed
- At least 8GB available RAM
- Ports 7201, 7202, 7203 available

### Network Requirements
- Internet access to pull GraphDB Docker images
- Ports must not be in use by other services

## Quick Start

### Run Complete Test Suite

The easiest way to run all integration tests:

```bash
task test:integration:full
```

This command will:
1. Pull required GraphDB Docker images
2. Start three GraphDB instances
3. Wait for instances to be ready
4. Run all integration tests
5. Stop and remove test instances

### Manual Setup and Testing

If you prefer more control:

```bash
# 1. Setup GraphDB instances
task test:integration:setup

# 2. Run integration tests
task test:integration

# 3. Cleanup when done
task test:integration:teardown
```

## Test Infrastructure

### GraphDB Instances

The test suite starts three GraphDB instances:

| Instance | Version | Port | Container Name | Purpose |
|----------|---------|------|----------------|---------|
| Source | 10.8.5 | 7201 | graphdb-source-test | Source for migrations |
| Target | 10.8.5 | 7202 | graphdb-target-test | Target for migrations |
| Version 10.7 | 10.7.4 | 7203 | graphdb-10-7-test | Version compatibility testing |

### Default Credentials

All test instances use default GraphDB credentials:
- **Username**: `admin`
- **Password**: `root`

### Container Configuration

Each container is configured with:
- 2GB heap size (`GDB_HEAP_SIZE=2g`)
- Mapped ports for HTTP API access
- Automatic removal on teardown

## Integration Tests

### Test Cases

#### 1. TestIntegrationSetup
**Purpose**: Verify test environment is properly configured

**What it tests**:
- All three GraphDB instances are accessible
- Can authenticate and list repositories
- Instances are responding correctly

**Duration**: ~5 seconds

---

#### 2. TestIntegrationRepoMigration
**Purpose**: Test complete repository migration workflow

**What it tests**:
- Create source repository with configuration
- Insert 100 test triples into source
- Migrate repository from source to target
- Verify target repository exists and contains data

**Workflow**:
1. Create `test-migration-source` repository on source GraphDB
2. Insert 100 RDF triples (entities with labels and values)
3. Execute `repo-migration` action via API
4. Verify `test-migration-target` repository exists on target GraphDB
5. Cleanup both repositories

**Duration**: ~30-60 seconds

---

#### 3. TestIntegrationGraphMigration
**Purpose**: Test named graph migration between repositories

**What it tests**:
- Create source and target repositories
- Insert data into named graph in source
- Migrate specific graph from source to target
- Verify graph exists in target repository

**Workflow**:
1. Create `test-graph-migration-source` and `test-graph-migration-target` repositories
2. Insert 50 RDF triples into `http://example.org/test-graph` in source
3. Execute `graph-migration` action via API
4. Verify named graph exists in target repository
5. Cleanup both repositories

**Duration**: ~30-60 seconds

---

#### 4. TestIntegrationRepoCreateAndDelete
**Purpose**: Test repository lifecycle operations

**What it tests**:
- Create repository from TTL configuration file
- Verify repository exists after creation
- Delete repository
- Verify repository no longer exists

**Workflow**:
1. Create multipart form request with TTL config file
2. Execute `repo-create` action via API
3. Verify `test-create-delete` repository exists
4. Execute `repo-delete` action via API
5. Verify repository no longer exists

**Duration**: ~20-30 seconds

---

#### 5. TestIntegrationGraphDBVersionCompatibility
**Purpose**: Test cross-version compatibility

**What it tests**:
- Create repositories on GraphDB 10.8.5 and 10.7.4
- Migrate data from 10.8.5 to 10.7.4
- Verify migration works across versions

**Workflow**:
1. Create repository on GraphDB 10.8.5 with 50 triples
2. Create repository on GraphDB 10.7.4
3. Execute `repo-migration` from 10.8.5 to 10.7.4
4. Verify migration succeeds
5. Cleanup repositories on both versions

**Duration**: ~30-60 seconds

---

### Total Test Duration

**Expected Time**: 2-5 minutes (depending on system performance)

## Running Tests

### Using Task Commands

```bash
# Full test suite (recommended)
task test:integration:full

# Setup only (leaves instances running)
task test:integration:setup

# Run tests (requires running instances)
task test:integration

# Teardown only
task test:integration:teardown
```

### Using Go Commands

```bash
# Run integration tests directly (requires running instances)
go test -v -tags=integration -timeout=10m ./cmd

# Run specific integration test
go test -v -tags=integration -run TestIntegrationRepoMigration ./cmd
```

### With Coverage

```bash
# Setup instances first
task test:integration:setup

# Run with coverage
go test -v -tags=integration -cover -coverprofile=integration-coverage.out ./cmd

# Generate coverage report
go tool cover -html=integration-coverage.out -o integration-coverage.html

# Cleanup
task test:integration:teardown
```

## Test Build Tags

Integration tests use the `integration` build tag to separate them from unit tests:

```go
//go:build integration
// +build integration
```

This allows:
- Unit tests to run quickly without Docker dependencies
- Integration tests to run only when explicitly requested
- CI/CD pipelines to run tests selectively

## Troubleshooting

### Ports Already in Use

**Problem**: `docker: Error response from daemon: port is already allocated`

**Solution**:
```bash
# Check what's using the ports
lsof -i :7201
lsof -i :7202
lsof -i :7203

# Stop conflicting containers
docker ps
docker stop <container_name>

# Or use different ports by modifying taskfile.yml
```

### GraphDB Not Ready

**Problem**: Tests fail with connection refused errors

**Solution**:
```bash
# Increase wait time in taskfile.yml (line 62)
- sleep 90  # Instead of sleep 60

# Or manually wait and check
docker logs graphdb-source-test
curl http://localhost:7201/rest/repositories
```

### Out of Memory

**Problem**: Docker containers crash or become unresponsive

**Solution**:
```bash
# Increase Docker memory allocation in Docker Desktop settings
# Or reduce heap size in taskfile.yml:
- docker run -d --name graphdb-source-test -p 7201:7200 -e GDB_HEAP_SIZE=1g ...
```

### Tests Timeout

**Problem**: `panic: test timed out after 10m0s`

**Solution**:
```bash
# Increase test timeout
go test -v -tags=integration -timeout=20m ./cmd

# Or in taskfile.yml:
- go test -v -tags=integration -timeout=20m ./cmd
```

### Docker Pull Fails

**Problem**: `Error response from daemon: pull access denied`

**Solution**:
```bash
# Login to Docker Hub
docker login

# Or use a different GraphDB image source
# Modify taskfile.yml to use your registry
```

### Cleanup Stuck Containers

**Problem**: Containers won't stop or remove

**Solution**:
```bash
# Force remove all test containers
docker rm -f graphdb-source-test graphdb-target-test graphdb-10-7-test

# If still stuck, restart Docker
sudo systemctl restart docker  # Linux
# Or restart Docker Desktop on Mac/Windows
```

## CI/CD Integration

### GitLab CI

Add to `.gitlab-ci.yml`:

```yaml
test:integration:
  stage: test
  services:
    - docker:dind
  variables:
    DOCKER_HOST: tcp://docker:2375
    DOCKER_TLS_CERTDIR: ""
  before_script:
    - apk add --no-cache go git
  script:
    - task test:integration:full
  only:
    - main
    - merge_requests
  when: manual  # Run manually to avoid resource usage
```

### GitHub Actions

Add to `.github/workflows/integration-tests.yml`:

```yaml
name: Integration Tests
on:
  workflow_dispatch:
  push:
    branches: [ main ]
jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      - name: Install Task
        run: |
          sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
      - name: Run Integration Tests
        run: task test:integration:full
```

## Test Data

### Sample RDF Data Format

Tests generate RDF data in Turtle format:

```turtle
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:entity0 a ex:TestEntity ;
    rdfs:label "Test Entity 0" ;
    ex:value 0 .

ex:entity1 a ex:TestEntity ;
    rdfs:label "Test Entity 1" ;
    ex:value 1 .
```

### Repository Configuration

Test repositories use minimal GraphDB configuration:

```turtle
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rep: <http://www.openrdf.org/config/repository#> .
@prefix sr: <http://www.openrdf.org/config/repository/sail#> .
@prefix graphdb: <http://www.ontotext.com/config/graphdb#> .

[] a rep:Repository ;
   rep:repositoryID "test-repo" ;
   rdfs:label "Test Repository" ;
   rep:repositoryImpl [
      rep:repositoryType "graphdb:SailRepository" ;
      sr:sailImpl [
         sail:sailType "graphdb:Sail" ;
         graphdb:read-only "false" ;
         graphdb:ruleset "empty" ;
      ]
   ] .
```

## Best Practices

### 1. Always Cleanup

Even if tests fail, ensure cleanup runs:
```bash
# Use the full task which handles cleanup
task test:integration:full

# Or use trap in shell scripts
trap "task test:integration:teardown" EXIT
```

### 2. Isolate Test Data

Each test creates uniquely named repositories:
- `test-migration-source`
- `test-graph-migration-target`
- `test-create-delete`

This prevents conflicts between tests.

### 3. Check Docker Resources

Before running tests:
```bash
# Check available disk space
docker system df

# Clean up if needed
docker system prune -af
```

### 4. Monitor During Tests

Watch logs in real-time:
```bash
# In separate terminals
docker logs -f graphdb-source-test
docker logs -f graphdb-target-test
docker logs -f graphdb-10-7-test
```

### 5. Verify Health Before Tests

```bash
# Setup instances
task test:integration:setup

# Manually verify health
curl http://localhost:7201/rest/repositories
curl http://localhost:7202/rest/repositories
curl http://localhost:7203/rest/repositories

# Then run tests
task test:integration
```

## Performance Considerations

### Resource Usage

Each GraphDB instance requires:
- **RAM**: ~2GB per instance (6GB total)
- **CPU**: 1-2 cores per instance
- **Disk**: ~500MB per instance
- **Network**: Moderate bandwidth for API calls

### Optimization Tips

1. **Reduce test data size** if tests are slow:
   ```go
   insertTestData(t, url, username, password, repo, 25)  // Instead of 100
   ```

2. **Reduce wait time** if GraphDB starts quickly:
   ```yaml
   - sleep 30  # Instead of 60 (in taskfile.yml)
   ```

3. **Run tests in parallel** with separate port ranges:
   ```yaml
   # Different test suite on ports 7211-7213
   - docker run -d --name graphdb-test2 -p 7211:7200 ...
   ```

## Maintenance

### Changing GraphDB Version

The GraphDB version is configured as a global variable in `taskfile.yml` at line 4:

```yaml
vars:
  GRAPHDB_VERS: 10.8.5
```

**To change the version:**

1. Edit `taskfile.yml` line 4:
   ```yaml
   vars:
     GRAPHDB_VERS: 10.9.0  # Or any other 10.x.x version
   ```

2. Run the tests:
   ```bash
   task test:integration:full
   ```

**Important Notes:**
- Only use 10.x.x versions to avoid license issues
- All tasks automatically use `{{.GRAPHDB_VERS}}` variable
- No need to update individual task commands
- The same version is used for both source and target instances

**Testing Multiple Versions:**

To test migrations between different versions:

1. Add additional test instances in `test:integration:setup`:
   ```yaml
   - docker pull ontotext/graphdb:10.7.4
   - docker run -d --name graphdb-10-7-test -p 7203:7200 -e GDB_HEAP_SIZE=2g ontotext/graphdb:10.7.4
   ```

2. Update test constants in `graphdb_integration_test.go`:
   ```go
   const graphDB107URL = "http://localhost:7203"
   ```

3. Add cross-version migration tests

### Adding New Test Cases

1. Add test function to `graphdb_integration_test.go`:
   ```go
   func TestIntegrationNewFeature(t *testing.T) {
       // Test implementation
   }
   ```

2. Follow existing patterns:
   - Use `waitForGraphDB()` to ensure availability
   - Use `setupTestRepository()` for repo creation
   - Use `cleanupTestRepository()` for cleanup with defer
   - Use unique repository names to avoid conflicts

3. Document the new test in this file

---

**Last Updated**: 2025-10-26
**Test Suite Version**: v0.0.3
**Supported GraphDB Versions**: 10.7.4, 10.8.5
**Total Test Cases**: 5 integration tests
