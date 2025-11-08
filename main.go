// Package main provides the entry point for the GraphDB Service application.
//
// GraphDB Service is a comprehensive API service for managing GraphDB repositories and RDF graphs.
// It provides RESTful endpoints for repository migration, graph management, data import/export,
// and various administrative operations on GraphDB instances.
//
// The service supports:
//   - Repository migration between GraphDB instances
//   - Named graph operations (import, export, delete, rename)
//   - Repository management (create, delete, rename)
//   - Secure connectivity via Ziti zero-trust networking
//   - API key authentication for all operations
//   - Multipart form uploads for configuration and data files
//
// Usage:
//
//	graphservice graphdb [flags]
//
// Environment Variables:
//   - API_KEY: Required API key for authentication
//   - PORT: HTTP server port (default: 8080)
//
// Example:
//
//	export API_KEY=your-secret-key
//	export PORT=8080
//	graphservice graphdb --identity /path/to/ziti/identity.json
package main

import cmd "evalgo.org/graphservice/cmd/graphdbservice"

// main is the application entry point that delegates to the cobra command structure.
func main() {
	_ = cmd.Execute()
}
