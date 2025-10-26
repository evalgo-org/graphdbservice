# GraphDB Service (pxgraphservice)

A comprehensive RESTful API service for managing GraphDB repositories and RDF graphs. This service provides high-level operations for repository migration, graph management, data import/export, and various administrative tasks across GraphDB instances.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Starting the Service](#starting-the-service)
  - [API Operations](#api-operations)
- [API Reference](#api-reference)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Building](#building)
  - [Testing](#testing)
- [CI/CD](#cicd)
- [Dependencies](#dependencies)
- [License](#license)

## Features

### Repository Operations
- **Repository Migration**: Transfer complete repositories between GraphDB instances
- **Repository Creation**: Create new repositories with custom configurations
- **Repository Deletion**: Remove repositories and all associated data
- **Repository Rename**: Rename repositories while preserving all data and graphs
- **Repository Import**: Import data into existing repositories from BRF files

### Graph Operations
- **Graph Migration**: Move named graphs between repositories
- **Graph Import**: Import RDF data into named graphs
- **Graph Export**: Export named graphs in various RDF formats
- **Graph Deletion**: Remove specific named graphs
- **Graph Rename**: Rename named graphs while preserving data

### Advanced Features
- **Ziti Zero-Trust Networking**: Secure connectivity via OpenZiti overlay networks
- **API Key Authentication**: Protect all endpoints with API key authentication
- **Multipart Form Uploads**: Support for configuration and data file uploads
- **Multiple RDF Formats**: Support for RDF/XML, Turtle, N-Triples, JSON-LD, and Binary RDF
- **Error Handling**: Comprehensive error reporting with detailed messages
- **Logging**: Structured logging for debugging and monitoring

## Architecture

The service is built using:
- **Echo Framework**: High-performance HTTP server and routing
- **Cobra**: Powerful CLI framework for command-line interface
- **Viper**: Flexible configuration management
- **Eve Library**: Shared utilities and GraphDB client from eve.evalgo.org

### Component Structure

```
pxgraphservice/
├── main.go              # Application entry point
├── cmd/                 # Command-line interface
│   ├── root.go         # Root command and configuration
│   ├── version.go      # Version command
│   ├── graphdb.go      # GraphDB service command and API handlers
│   └── graphdb_test.go # Comprehensive unit tests
├── go.mod              # Go module dependencies
├── taskfile.yml        # Task automation
└── README.md           # This file
```

## Installation

### From Source

```bash
# Clone the repository
git clone https://gitlab.com/your-org/pxgraphservice.git
cd pxgraphservice

# Download dependencies
go mod download

# Build the binary
go build -o graphservice main.go

# Install to $GOPATH/bin
go install
```

### Using Docker

```bash
# Using nixpacks
nixpacks build . --name pxgraphservice:latest

# Run the container
docker run -d \
  -p 8080:8080 \
  -e API_KEY=your-secret-key \
  pxgraphservice:latest
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `API_KEY` | API key for authentication | - | Yes |
| `PORT` | HTTP server port | 8080 | No |

### Configuration File

Create a `.cobra.yaml` in your home directory:

```yaml
author: "Your Name <your.email@example.com>"
license: "apache"
```

## Usage

### Starting the Service

Basic usage:

```bash
# Set required environment variables
export API_KEY=your-secret-api-key

# Start the service (default port 8080)
graphservice graphdb

# Start on custom port
export PORT=9000
graphservice graphdb

# With Ziti identity for secure networking
graphservice graphdb --identity /path/to/ziti-identity.json
```

### API Operations

The service exposes a RESTful API at `/v1/api/action` with the following operations:

#### Repository Migration

Copy an entire repository from source to target GraphDB instance:

```bash
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "v0.0.1",
    "tasks": [{
      "action": "repo-migration",
      "src": {
        "url": "http://source-graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "source-repo"
      },
      "tgt": {
        "url": "http://target-graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "target-repo"
      }
    }]
  }'
```

#### Graph Import

Import RDF data into a named graph:

```bash
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: your-secret-key" \
  -F "request={\"version\":\"v0.0.1\",\"tasks\":[{\"action\":\"graph-import\",\"tgt\":{\"url\":\"http://graphdb:7200\",\"username\":\"admin\",\"password\":\"password\",\"repo\":\"my-repo\",\"graph\":\"http://example.org/graph/data\"}}]}" \
  -F "task_0_files=@data.rdf"
```

#### Repository Creation

Create a new repository with custom configuration:

```bash
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: your-secret-key" \
  -F "request={\"version\":\"v0.0.1\",\"tasks\":[{\"action\":\"repo-create\",\"tgt\":{\"url\":\"http://graphdb:7200\",\"username\":\"admin\",\"password\":\"password\",\"repo\":\"new-repo\"}}]}" \
  -F "task_0_config=@repo-config.ttl"
```

## API Reference

### Request Format

All requests use the following JSON structure:

```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "action-name",
      "src": { /* source configuration */ },
      "tgt": { /* target configuration */ }
    }
  ]
}
```

### Supported Actions

| Action | Description | Required Fields |
|--------|-------------|-----------------|
| `repo-migration` | Migrate repository between instances | src, tgt |
| `graph-migration` | Migrate named graph between repositories | src, tgt |
| `repo-delete` | Delete a repository | tgt |
| `graph-delete` | Delete a named graph | tgt |
| `repo-create` | Create new repository | tgt + config file |
| `graph-import` | Import RDF data into graph | tgt + data files |
| `repo-import` | Import data into repository | tgt + BRF file |
| `repo-rename` | Rename a repository | tgt (repo_old, repo_new) |
| `graph-rename` | Rename a named graph | tgt (graph_old, graph_new) |

### Response Format

Success response:

```json
{
  "status": "success",
  "version": "v0.0.1",
  "results": [
    {
      "action": "repo-migration",
      "status": "completed",
      "message": "Repository migrated successfully",
      "src_repo": "source-repo",
      "tgt_repo": "target-repo"
    }
  ]
}
```

Error response:

```json
{
  "status": "error",
  "message": "Task 0: tgt is required for repo-delete"
}
```

## Development

### Prerequisites

- Go 1.25.1 or higher
- Docker (for local GraphDB instances)
- Task (for automation)

### Building

```bash
# Build the application
go build -o graphservice main.go

# Build with version information
go build -ldflags="-X 'cmd.version=v1.0.0' -X 'cmd.commit=$(git rev-parse HEAD)' -X 'cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o graphservice main.go
```

### Testing

#### Unit Tests

Run the comprehensive unit test suite:

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Using task
task test
```

The test suite includes:
- Mock HTTP server for GraphDB API
- Tests for all GraphDB operations
- Helper function tests
- Validation tests
- Authentication tests

#### Integration Tests

Set up local GraphDB instances for testing:

```bash
# Start test GraphDB instances
task local-setup

# This starts two GraphDB containers:
# - test-src-graphdb on port 7201
# - test-tgt-graphdb on port 7202

# Test against real GraphDB instances
export TEST_GRAPHDB_URL=http://localhost:7201
go test -v ./... -tags=integration
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Vet code
go vet ./...
```

## CI/CD

The project uses GitLab CI for continuous integration and deployment.

### Pipeline Stages

1. **Test**: Run unit tests
2. **Build**: Build the application
3. **Deploy**: Deploy to target environment (if configured)

### Running Tests in CI

The `.gitlab-ci.yml` file is configured to:
- Run unit tests on every commit
- Generate code coverage reports
- Fail the build if tests fail

## Dependencies

### Direct Dependencies

- **eve.evalgo.org v0.0.6**: GraphDB client library and shared utilities
- **github.com/labstack/echo/v4**: HTTP web framework
- **github.com/spf13/cobra**: CLI framework
- **github.com/spf13/viper**: Configuration management
- **go.hein.dev/go-version**: Version information handling

### Key Transitive Dependencies

- OpenZiti SDK for zero-trust networking
- PostgreSQL driver (via eve)
- CouchDB client (via eve)
- AWS SDK (via eve)
- Various other utilities

For a complete list, see `go.mod`.

## Supported RDF Formats

The service supports the following RDF serialization formats:

| Format | Extension | MIME Type | Usage |
|--------|-----------|-----------|-------|
| Binary RDF | `.brf` | `application/x-binary-rdf` | Repository backups |
| RDF/XML | `.rdf`, `.xml` | `application/rdf+xml` | Graph import/export |
| Turtle | `.ttl` | `text/turtle` | Configuration files |
| N-Triples | `.nt` | `application/n-triples` | Data exchange |
| N3 | `.n3` | `text/n3` | Data exchange |
| JSON-LD | `.jsonld`, `.json` | `application/ld+json` | Web integration |
| TriG | `.trig` | `application/trig` | Named graphs |
| N-Quads | `.nq` | `application/n-quads` | Quad data |

## Security Considerations

### API Key Authentication

All API endpoints require a valid API key in the `x-api-key` header:

```bash
curl -H "x-api-key: your-secret-key" http://localhost:8080/v1/api/action
```

### Ziti Zero-Trust Networking

For production deployments, use Ziti for secure, identity-based connectivity:

```bash
graphservice graphdb --identity /path/to/ziti-identity.json
```

This enables:
- Network invisibility (no exposed ports)
- End-to-end encryption
- Identity-based access control
- Dynamic policy enforcement

### Best Practices

1. **Rotate API Keys**: Change API keys regularly
2. **Use HTTPS**: Always use TLS in production
3. **Limit Network Access**: Use firewalls or Ziti to restrict access
4. **Monitor Logs**: Review logs for suspicious activity
5. **Backup Data**: Regular backups of GraphDB repositories

## Troubleshooting

### Common Issues

**Issue**: `Missing x-api-key header`
- **Solution**: Include the API key in your request headers

**Issue**: `Failed to connect to GraphDB`
- **Solution**: Verify GraphDB URL and network connectivity

**Issue**: `Repository not found`
- **Solution**: Check repository name and ensure it exists

**Issue**: `Invalid Turtle syntax`
- **Solution**: Validate your repository configuration file

### Debugging

Enable debug logging:

```bash
# Set log level
export LOG_LEVEL=debug
graphservice graphdb
```

View detailed request/response logs in the console output.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Merge Request

### Code Standards

- Follow Go best practices and idioms
- Add godoc comments for all exported functions
- Write unit tests for new functionality
- Ensure all tests pass before submitting
- Run `go fmt` and `go vet` on your code

## License

Apache License 2.0

## Author

**Francisc Simon**
Email: francisc.simon@pantopix.com
Organization: Pantopix

## Acknowledgments

- GraphDB by Ontotext for the RDF database platform
- OpenZiti for zero-trust networking capabilities
- Echo framework for the HTTP server
- The Go community for excellent tooling and libraries

## Version

Current version: v0.0.3
Built with eve.evalgo.org v0.0.6

For version information, run:

```bash
graphservice version
```
