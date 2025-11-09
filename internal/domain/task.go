// Package domain defines the core domain types for GraphDB operations.
package domain

// Task represents a single operation to be performed on GraphDB repositories or graphs.
//
// Supported actions:
//   - repo-migration: Migrate entire repository (config + data)
//   - graph-migration: Migrate a named graph between repositories
//   - repo-delete: Delete a repository
//   - graph-delete: Delete a named graph
//   - repo-create: Create a new repository from TTL configuration
//   - graph-import: Import RDF data into a graph
//   - repo-import: Import data into repository from BRF backup file
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
