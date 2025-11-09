// Package domain defines error types for GraphDB operations.
package domain

import "fmt"

// OperationError is a custom error type for operation failures
type OperationError struct {
	Operation string // The operation that failed (e.g., "repo-migration")
	Message   string // Human-readable error message
	Cause     error  // Underlying error
}

func (e *OperationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s failed: %s (%v)", e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s failed: %s", e.Operation, e.Message)
}

func (e *OperationError) Unwrap() error {
	return e.Cause
}

// NotFoundError indicates a repository or graph was not found
type NotFoundError struct {
	Type     string // "repository" or "graph"
	Identifier string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Type, e.Identifier)
}

// ValidationError indicates input validation failed
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ConflictError indicates a resource already exists
type ConflictError struct {
	Type       string // "repository" or "graph"
	Identifier string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s already exists: %s", e.Type, e.Identifier)
}

// NewOperationError creates a new OperationError
func NewOperationError(operation, message string, cause error) *OperationError {
	return &OperationError{
		Operation: operation,
		Message:   message,
		Cause:     cause,
	}
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(typ, identifier string) *NotFoundError {
	return &NotFoundError{
		Type:       typ,
		Identifier: identifier,
	}
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewConflictError creates a new ConflictError
func NewConflictError(typ, identifier string) *ConflictError {
	return &ConflictError{
		Type:       typ,
		Identifier: identifier,
	}
}
