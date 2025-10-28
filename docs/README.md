# Pantopix GraphDB Service API Documentation

Complete OpenAPI/Swagger documentation for the Pantopix GraphDB Service REST API.

## Accessing the Documentation

Once the service is running, you can access the interactive Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

Replace `localhost:8080` with your actual host and port configuration.

## API Overview

The Pantopix GraphDB Service provides a comprehensive REST API for managing GraphDB repositories, performing migrations, and administering users.

### Base URL

```
http://localhost:8080/
```

### Authentication

The API supports two authentication methods:

1. **API Key Authentication** (`X-API-Key` header)
   - Used for external integrations
   - Required for `/v1/api/*` endpoints
   - Example: `X-API-Key: your-api-key-here`

2. **JWT Bearer Authentication** (`Authorization` header)
   - Used for web UI and user-facing APIs
   - Format: `Authorization: Bearer <jwt-token>`
   - Obtain token via `/auth/login` endpoint

## API Endpoints

### Health & Status

- `GET /health` - Health check endpoint (no auth required)

### Authentication

- `GET /login` - Login page (HTML)
- `POST /auth/login` - User authentication (returns JWT token)
- `GET /logout` - User logout

### Migration Operations

- `POST /v1/api/action` - Execute GraphDB migration tasks (API Key required)
- `POST /ui/execute` - Execute tasks via web UI (JWT required)
- `GET /ui/stream/{sessionID}` - Stream task execution results via SSE (JWT required)

### User Management (Admin Only)

- `GET /admin/users/api` - List all users
- `POST /admin/users` - Create new user
- `GET /admin/users/{username}` - Get user details
- `PUT /admin/users/{username}` - Update user
- `DELETE /admin/users/{username}` - Delete user

### User Profile

- `GET /api/users/me` - Get current user info
- `POST /api/users/me/password` - Change password

### Audit Logs (Admin Only)

- `GET /admin/audit/api` - Get audit logs
- `POST /admin/audit/rotate` - Rotate old audit logs

### Migration Logs (Admin Only)

- `GET /admin/migrations/stats` - Get migration statistics
- `GET /admin/migrations/active` - Get active migration sessions
- `GET /admin/migrations/session/{id}` - Get session details
- `GET /admin/migrations/summary/{date}` - Get daily summary
- `POST /admin/migrations/rotate` - Rotate old migration logs

## Supported Migration Actions

The `/v1/api/action` endpoint supports the following task actions:

### Repository Operations

- **repo-migration**: Full repository migration (backup and restore)
  - Backs up source repository configuration and data
  - Restores to target GraphDB instance
  - Preserves all repository settings and RDF data

- **repo-create**: Create a new repository
  - Uploads repository configuration (TTL format)
  - Creates repository on target GraphDB

- **repo-delete**: Delete a repository
  - Removes repository and all its data

- **repo-rename**: Rename a repository
  - Creates new repository with new name
  - Migrates all data from old to new
  - Deletes old repository

### Graph Operations

- **graph-migration**: Named graph migration
  - Exports named graph from source repository
  - Imports to target repository
  - Supports graph renaming during migration

- **graph-import**: Import RDF data into a graph
  - Uploads RDF file (Turtle, RDF/XML, N-Triples, etc.)
  - Imports into specified named graph

- **graph-export**: Export RDF data from a graph
  - Downloads named graph as RDF file
  - Supports multiple RDF formats

- **graph-delete**: Delete a named graph
  - Removes graph and all its triples

- **graph-rename**: Rename a named graph
  - Exports graph data
  - Imports with new name
  - Deletes old graph

## Request Examples

### Execute Migration Task (API Key)

```bash
curl -X POST http://localhost:8080/v1/api/action \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "version": "v0.0.1",
    "tasks": [
      {
        "action": "repo-migration",
        "src": {
          "url": "http://source-graphdb:7200",
          "username": "admin",
          "password": "admin",
          "repo": "source-repo"
        },
        "tgt": {
          "url": "http://target-graphdb:7200",
          "username": "admin",
          "password": "admin",
          "repo": "target-repo"
        }
      }
    ]
  }'
```

### User Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "your-password"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "username": "admin",
    "role": "admin"
  }
}
```

### Create User (Admin, JWT)

```bash
curl -X POST http://localhost:8080/admin/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -d '{
    "username": "newuser",
    "password": "SecurePass123!",
    "email": "user@example.com",
    "role": "user",
    "must_change_password": true
  }'
```

### Get Migration Statistics (Admin, JWT)

```bash
curl -X GET http://localhost:8080/admin/migrations/stats \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

Response:
```json
{
  "active_sessions": 2,
  "completed_sessions": 45,
  "failed_sessions": 3,
  "total_tasks": 150,
  "completed_tasks": 142,
  "failed_tasks": 5,
  "timeout_tasks": 3,
  "total_data_size": 1073741824
}
```

## Response Codes

- `200 OK` - Successful request
- `201 Created` - Resource created successfully
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication required or failed
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource already exists
- `500 Internal Server Error` - Server error

## Data Models

### MigrationRequest

```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "repo-migration|graph-migration|repo-create|...",
      "src": {
        "url": "string",
        "username": "string",
        "password": "string",
        "repo": "string",
        "graph": "string (optional)"
      },
      "tgt": {
        "url": "string",
        "username": "string",
        "password": "string",
        "repo": "string",
        "graph": "string (optional)"
      }
    }
  ]
}
```

### User

```json
{
  "id": "uuid",
  "username": "string",
  "email": "string",
  "role": "admin|user",
  "locked": false,
  "failed_logins": 0,
  "must_change_password": false,
  "last_login_at": "2025-10-28T12:00:00Z"
}
```

### MigrationSession

```json
{
  "id": "uuid",
  "username": "string",
  "status": "running|completed|failed",
  "start_time": "2025-10-28T12:00:00Z",
  "duration_ms": 45000,
  "total_tasks": 5,
  "completed_tasks": 4,
  "failed_tasks": 1,
  "timeout_tasks": 0,
  "total_data_size_bytes": 104857600,
  "metadata": {
    "request_json": "{...}"
  }
}
```

## Rate Limiting

Currently, no rate limiting is enforced. Consider implementing rate limiting for production deployments.

## Updating Documentation

When adding new endpoints or modifying existing ones:

1. Add Swagger annotations to `cmd/swagger_docs.go`
2. Regenerate documentation:
   ```bash
   ~/go/bin/swag init -g cmd/graphdb.go --output docs
   ```
3. Rebuild the application:
   ```bash
   go build -o graphservice
   ```

## Files

- `docs/swagger.json` - OpenAPI specification (JSON format)
- `docs/swagger.yaml` - OpenAPI specification (YAML format)
- `docs/docs.go` - Go embedded documentation
- `cmd/swagger_docs.go` - Swagger annotation definitions
- `cmd/graphdb.go` - Main API info and route definitions

## Support

For questions or issues with the API, contact:
- Email: support@pantopix.com
- Website: https://pantopix.com

## License

Proprietary - Copyright Pantopix
