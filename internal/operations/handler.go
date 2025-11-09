// Package operations implements GraphDB operation handlers.
package operations

import (
	"context"
	"mime/multipart"

	"graphdbservice/internal/client"
	"graphdbservice/internal/domain"
)

// Handler defines the interface for GraphDB operation handlers
type Handler interface {
	// Handle executes the operation and returns the result
	Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error)
}

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	ClientManager *client.Manager
}

// Result is a helper for building operation results
func Result() map[string]interface{} {
	return make(map[string]interface{})
}

// SetResult sets a field in the result map
func SetResult(result map[string]interface{}, key string, value interface{}) {
	result[key] = value
}

// GetResult gets a field from the result map
func GetResult(result map[string]interface{}, key string) (interface{}, bool) {
	val, exists := result[key]
	return val, exists
}
