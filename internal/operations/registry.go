// Package operations implements GraphDB operation handlers.
package operations

import (
	"context"
	"fmt"
	"mime/multipart"

	"graphdbservice/internal/client"
	"graphdbservice/internal/domain"
)

// Registry manages operation handlers
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates a new operation registry
func NewRegistry(clientMgr *client.Manager) *Registry {
	reg := &Registry{
		handlers: make(map[string]Handler),
	}

	// Register all handlers
	reg.Register("repo-migration", NewRepoMigrationHandler(clientMgr))
	reg.Register("graph-migration", NewGraphMigrationHandler(clientMgr))
	reg.Register("repo-delete", NewRepoDeleteHandler(clientMgr))
	reg.Register("graph-delete", NewGraphDeleteHandler(clientMgr))
	reg.Register("repo-create", NewRepoCreateHandler(clientMgr))
	reg.Register("graph-import", NewGraphImportHandler(clientMgr))
	reg.Register("repo-import", NewRepoImportHandler(clientMgr))
	reg.Register("repo-rename", NewRepoRenameHandler(clientMgr))
	reg.Register("graph-rename", NewGraphRenameHandler(clientMgr))

	return reg
}

// Register registers a handler for an action
func (r *Registry) Register(action string, handler Handler) {
	r.handlers[action] = handler
}

// Handle executes an operation by routing to the appropriate handler
func (r *Registry) Handle(ctx context.Context, task domain.Task, files map[string][]*multipart.FileHeader) (map[string]interface{}, error) {
	handler, exists := r.handlers[task.Action]
	if !exists {
		return nil, fmt.Errorf("unknown action: %s", task.Action)
	}

	return handler.Handle(ctx, task, files)
}

// GetHandler returns the handler for an action (useful for testing)
func (r *Registry) GetHandler(action string) (Handler, bool) {
	handler, exists := r.handlers[action]
	return handler, exists
}
