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
- **User Authentication & RBAC**: Web UI authentication with role-based access control
- **User Management**: Complete admin interface for managing users, roles, and permissions
- **Web UI**: Interactive web interface with HTMX for real-time updates
- **Multipart Form Uploads**: Support for configuration and data file uploads
- **Multiple RDF Formats**: Support for RDF/XML, Turtle, N-Triples, JSON-LD, and Binary RDF
- **Error Handling**: Comprehensive error reporting with detailed messages
- **Audit Logging**: Track all user actions and system events
- **Migration Logging**: Complete traceability of all migration operations with detailed task tracking
- **Logging**: Structured logging for debugging and monitoring

## Architecture

The service is built using:
- **Echo Framework**: High-performance HTTP server and routing
- **Templ**: Type-safe Go HTML templates for the web UI
- **HTMX**: Modern interactive UI with server-sent events
- **Cobra**: Powerful CLI framework for command-line interface
- **Viper**: Flexible configuration management
- **Eve Library**: Shared utilities and GraphDB client from eve.evalgo.org

### Component Structure

```
pxgraphservice/
├── main.go                  # Application entry point
├── cmd/                     # Command-line interface & handlers
│   ├── root.go             # Root command and configuration
│   ├── version.go          # Version command
│   ├── graphdb.go          # GraphDB service command and API handlers
│   ├── graphdb_test.go     # Comprehensive unit tests
│   ├── web_handlers.go     # Web UI handlers
│   ├── auth_init.go        # Authentication initialization
│   ├── auth_handlers.go    # Login/logout handlers
│   ├── auth_middleware.go  # Authentication & authorization middleware
│   └── user_handlers.go    # User management API handlers
├── auth/                    # Authentication package
│   ├── models.go           # User, Claims, and database models
│   ├── password.go         # Password hashing and validation
│   ├── jwt.go              # JWT token generation and validation
│   ├── storage.go          # File-based user storage with locking
│   ├── audit.go            # Audit logging for user actions
│   ├── migration_logger.go # Migration operation logging and tracking
│   ├── *_test.go           # Comprehensive unit tests (60+ tests)
├── web/                     # Web UI templates
│   └── templates/          # Templ templates
│       ├── layout.templ    # Base layout with navbar
│       ├── index.templ     # Main task execution page
│       ├── login.templ     # Login page
│       ├── users.templ     # User management page
│       ├── change_password.templ  # Password change page
│       ├── audit.templ     # Audit logs page
│       └── migration_logs.templ # Migration logs page
├── data/                    # Runtime data (not in version control)
│   ├── users/              # User database and backups
│   ├── audit/              # Audit log files
│   └── migrations/         # Migration log files and sessions
├── go.mod                   # Go module dependencies
├── taskfile.yml             # Task automation
└── README.md                # This file
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
| `API_KEY` | API key for REST API authentication | - | Yes (for API) |
| `AUTH_MODE` | Authentication mode: `none`, `simple`, `rbac` | `none` | No |
| `JWT_SECRET` | Secret key for JWT token signing | - | Yes (if AUTH_MODE ≠ none) |
| `SESSION_TIMEOUT` | Session timeout in seconds | 3600 | No |
| `DATA_DIR` | Directory for user data storage | `./data` | No |
| `PORT` | HTTP server port | 8080 | No |

### Configuration File

Create a `.cobra.yaml` in your home directory:

```yaml
author: "Your Name <your.email@example.com>"
license: "apache"
```

## Usage

### Starting the Service

#### Without Authentication (Default)

```bash
# Set required API key for REST API
export API_KEY=your-secret-api-key

# Start the service (default port 8080)
graphservice graphdb

# Start on custom port
export PORT=9000
graphservice graphdb

# With Ziti identity for secure networking
graphservice graphdb --identity /path/to/ziti-identity.json
```

#### With User Authentication (RBAC)

```bash
# Enable RBAC authentication mode
export AUTH_MODE=rbac
export JWT_SECRET=your-long-random-secret-key-min-32-chars
export API_KEY=your-api-key

# Start the service
graphservice graphdb

# On first startup, an initial admin user will be created
# Credentials will be displayed in the console - save them!
# Example output:
#   Username: admin
#   Password: +SEZpz)22l2p[cIY
```

#### Accessing the Web UI

```bash
# Without authentication (AUTH_MODE=none)
# Navigate to: http://localhost:8080/

# With authentication (AUTH_MODE=rbac or simple)
# Navigate to: http://localhost:8080/login
# Login with the admin credentials shown on first startup
```

### Web UI Features

#### Main Task Execution Page

The web UI provides an interactive interface for executing GraphDB operations:

1. **Task JSON Input**: Paste or type your task JSON definition
2. **Load Example**: Click to load a sample migration task
3. **Execute Tasks**: Submit tasks and watch real-time progress
4. **Server-Sent Events**: Live updates of task status using SSE
5. **Enhanced Error Messages**: Clear, descriptive error messages with JSON validation

#### User Management (Admin Only)

Access the user management page at `/admin/users` (requires admin role):

**Features:**
- **List Users**: View all users with their roles, status, and activity
- **Create User**: Add new users with username, password, email, and role
- **Edit User**: Update user details, roles, and status
- **Delete User**: Remove users (with self-deletion prevention)
- **Lock/Unlock Accounts**: Control user access
- **Reset Failed Logins**: Unlock accounts after failed login attempts
- **Set Password Policy**: Force password change on first login

**User Management Operations:**
```
1. Login as admin
2. Click "Manage Users" in the navbar
3. View user list with detailed information
4. Click "Create New User" to add users
5. Click "Edit" to modify user details
6. Click "Delete" to remove users
```

#### Password Management

All authenticated users can change their password at `/profile/change-password`:

**Features:**
- Current password verification
- Strong password requirements:
  - Minimum 8 characters
  - At least one uppercase letter
  - At least one lowercase letter
  - At least one number
  - At least one special character
- Client-side and server-side validation
- Clear error messages and hints
- Automatic redirect after successful change

**Password Change Process:**
```
1. Login with your credentials
2. Click "Change Password" in the navbar
3. Enter current password
4. Enter new password (must meet requirements)
5. Confirm new password
6. Click "Change Password"
```

#### Migration Logs (Admin Only)

Access the migration logs page at `/admin/migrations` (requires admin role):

**Features:**
- **Real-time Statistics Dashboard**: View active sessions, completion rates, and success metrics
- **Session Tracking**: Complete history of all migration operations
- **Task-Level Details**: Individual task status, duration, and data size
- **Search and Filter**: Filter by date, username, or status
- **Date Range Analytics**: View migration statistics across custom date ranges
- **Error Tracking**: Detailed error messages and failure analysis
- **Active Session Monitoring**: Real-time updates of running migrations
- **Data Size Metrics**: Track total data transferred in migrations

**Migration Log Information:**
Each migration session includes:
- Session ID and user information
- Start/end timestamps and duration
- Status (running, completed, failed)
- Task breakdown (total, completed, failed, timeout)
- Data size transferred
- Source and target URLs
- Individual task details with errors

**Migration Log Storage:**
- Logs stored in `${DATA_DIR}/migrations/`
- Daily summary files: `migration_YYYY-MM-DD.json` (uncompressed)
- Detailed session files: `sessions/[session-id].json`
- **Automated Log Rotation Strategy:**
  - Keep daily logs for **7 days** (uncompressed for fast access)
  - Compress logs older than 7 days into **weekly archives** (tar.gz)
  - Keep weekly archives for **4 weeks**
  - Delete archives older than 4 weeks
  - Weekly archives named: `migration_YYYY-WWW.tar.gz` (ISO week format)

**Benefits:**
- Complete audit trail of all data movements
- Performance analysis and optimization insights
- Debugging failed migrations with full context
- Compliance reporting for data governance
- Capacity planning based on historical data

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

# Run only auth tests
go test -v ./auth/...

# Using task
task test
```

The test suite includes:

**GraphDB Operations (cmd/):**
- Mock HTTP server for GraphDB API
- Tests for all GraphDB operations
- Helper function tests
- Validation tests

**Authentication (auth/):**
- Password hashing and validation (8 tests)
- JWT token generation and validation (6 tests)
- User storage operations (12 tests)
- File locking and concurrency
- Persistence and backup tests
- **Total: 60+ authentication tests, all passing**

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

### Authentication Modes

The service supports three authentication modes:

#### 1. None Mode (Default - `AUTH_MODE=none`)
- No web UI authentication required
- API key still required for REST API endpoints
- Suitable for trusted internal networks
- Backward compatible with existing deployments

#### 2. Simple Mode (`AUTH_MODE=simple`)
- Basic username/password authentication
- JWT-based session management
- Single role: all authenticated users have same permissions
- Good for simple deployments

#### 3. RBAC Mode (`AUTH_MODE=rbac`)
- Role-Based Access Control
- Two roles: `admin` and `user`
- Admins can manage users and access all features
- Regular users can execute tasks and change passwords
- Recommended for production environments

### Web UI Authentication

When authentication is enabled (`AUTH_MODE=simple` or `AUTH_MODE=rbac`):

**Features:**
- JWT-based sessions with configurable timeout
- HTTP-only, Secure, SameSite=Strict cookies
- Automatic session expiration
- Failed login tracking (locks account after 5 attempts)
- Password strength requirements enforced
- Account locking/unlocking by admins

**Security Measures:**
```bash
# Generate a strong JWT secret (minimum 32 characters)
export JWT_SECRET=$(openssl rand -base64 32)

# Set session timeout (in seconds, default 3600 = 1 hour)
export SESSION_TIMEOUT=3600

# Enable RBAC mode
export AUTH_MODE=rbac
```

### User Data Storage

User data is stored in the filesystem with strong security:

- **Location**: `${DATA_DIR}/users/` (default: `./data/users/`)
- **Format**: JSON with bcrypt password hashes
- **Permissions**: Files are created with 0600 (owner read/write only)
- **Concurrency**: File locking prevents race conditions
- **Backups**: Automatic backup before each modification
- **Schema Version**: Versioned format for future migrations

**User Database Structure:**
```json
{
  "version": "1.0.0",
  "users": {
    "admin": {
      "id": "uuid",
      "username": "admin",
      "password_hash": "bcrypt-hash",
      "role": "admin",
      "created_at": "timestamp",
      "updated_at": "timestamp",
      "last_login_at": "timestamp",
      "failed_logins": 0,
      "locked": false,
      "must_change_password": false
    }
  },
  "updated_at": "timestamp"
}
```

### API Key Authentication

All REST API endpoints require a valid API key in the `x-api-key` header:

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

1. **Use RBAC Mode**: Enable `AUTH_MODE=rbac` for production
2. **Strong JWT Secret**: Use a minimum 32-character random secret
3. **Rotate Secrets**: Change JWT_SECRET and API_KEY regularly
4. **Use HTTPS**: Always use TLS in production
5. **Secure Data Directory**: Ensure proper file system permissions on `DATA_DIR`
6. **Change Default Password**: Force users to change password on first login
7. **Monitor Failed Logins**: Review audit logs for suspicious activity
8. **Backup User Data**: Regular backups of `./data/users/`
9. **Limit Network Access**: Use firewalls or Ziti to restrict access
10. **Session Timeout**: Configure appropriate SESSION_TIMEOUT for your security needs

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
