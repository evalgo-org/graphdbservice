package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestFormatJSONError tests the JSON error formatting helper function
func TestFormatJSONError(t *testing.T) {
	tests := []struct {
		name          string
		jsonInput     string
		expectedParts []string // Parts that should appear in the error message
	}{
		{
			name:      "Missing closing brace",
			jsonInput: `{"version": "v0.0.1", "tasks": [`,
			expectedParts: []string{
				"unexpected end",
				"Hint:",
			},
		},
		{
			name:      "Trailing comma",
			jsonInput: `{"version": "v0.0.1", "tasks": [],}`,
			expectedParts: []string{
				"invalid character",
				"Hint:",
			},
		},
		{
			name:      "Missing quotes on key",
			jsonInput: `{version: "v0.0.1"}`,
			expectedParts: []string{
				"invalid character",
			},
		},
		{
			name:      "Syntax error with position",
			jsonInput: `{"version": "v0.0.1" "tasks": []}`,
			expectedParts: []string{
				"line",
				"column",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req MigrationRequest
			err := json.Unmarshal([]byte(tt.jsonInput), &req)

			if err == nil {
				t.Fatal("Expected JSON parsing to fail, but it succeeded")
			}

			// Format the error
			formattedError := formatJSONError(err, tt.jsonInput)

			// Check that expected parts are in the formatted error
			for _, expectedPart := range tt.expectedParts {
				if !strings.Contains(strings.ToLower(formattedError), strings.ToLower(expectedPart)) {
					t.Errorf("Expected formatted error to contain %q, but got: %s", expectedPart, formattedError)
				}
			}

			// Ensure the message is not just the raw error
			if formattedError == err.Error() {
				t.Error("Formatted error should be more descriptive than raw error")
			}
		})
	}
}

// TestUIExecuteHandlerJSONErrors tests the UI handler with various invalid JSON inputs
func TestUIExecuteHandlerJSONErrors(t *testing.T) {
	tests := []struct {
		name             string
		jsonInput        string
		expectedStatus   int
		expectedContains []string
	}{
		{
			name:           "Empty JSON input",
			jsonInput:      "",
			expectedStatus: http.StatusOK, // Returns 200 for HTMX to display error
			expectedContains: []string{
				"Task JSON is required",
			},
		},
		{
			name:           "Invalid JSON - missing brace",
			jsonInput:      `{"version": "v0.0.1"`,
			expectedStatus: http.StatusOK, // Returns 200 for HTMX to display error
			expectedContains: []string{
				"Invalid JSON",
				"Hint:",
			},
		},
		{
			name:           "Invalid JSON - trailing comma",
			jsonInput:      `{"version": "v0.0.1", "tasks": [],}`,
			expectedStatus: http.StatusOK, // Returns 200 for HTMX to display error
			expectedContains: []string{
				"Invalid JSON",
				"Hint:",
			},
		},
		{
			name:           "Valid JSON but missing version",
			jsonInput:      `{"tasks": []}`,
			expectedStatus: http.StatusOK, // Returns 200 for HTMX to display error
			expectedContains: []string{
				"Version is required",
			},
		},
		{
			name:           "Valid JSON but missing tasks",
			jsonInput:      `{"version": "v0.0.1"}`,
			expectedStatus: http.StatusOK, // Returns 200 for HTMX to display error
			expectedContains: []string{
				"At least one task is required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Echo instance
			e := echo.New()

			// Create form data
			formData := fmt.Sprintf("task_json=%s", tt.jsonInput)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/ui/execute", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Call handler
			err := uiExecuteHandler(c)

			// For Echo handlers that return HTTPError, we need to handle it
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					rec.WriteHeader(he.Code)
					rec.WriteString(he.Message.(string))
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			// Check status code
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d but got %d", tt.expectedStatus, rec.Code)
			}

			// Check response body contains expected strings
			body := rec.Body.String()
			for _, expected := range tt.expectedContains {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected body to contain %q but got: %s", expected, body)
				}
			}
		})
	}
}

// TestGetErrorLocation tests the error location helper function
func TestGetErrorLocation(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		offset  int64
	}{
		{
			name:    "First line error",
			jsonStr: `{"version": "v0.0.1"`,
			offset:  20,
		},
		{
			name: "Multi-line error",
			jsonStr: `{
  "version": "v0.0.1",
  "tasks": [
}`,
			offset: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col, snippet := getErrorLocation(tt.jsonStr, tt.offset)

			// Line should be positive
			if line < 1 {
				t.Errorf("Expected line >= 1 but got %d", line)
			}

			// Column should be positive
			if col < 1 {
				t.Errorf("Expected column >= 1 but got %d", col)
			}

			// Snippet should not be empty
			if snippet == "" {
				t.Error("Expected non-empty snippet")
			}

			// Snippet should contain a pointer (^)
			if !strings.Contains(snippet, "^") {
				t.Errorf("Expected snippet to contain pointer '^', but got: %s", snippet)
			}

			// Snippet should contain newline (for multi-line format)
			if !strings.Contains(snippet, "\n") {
				t.Errorf("Expected snippet to contain newline for error pointer, but got: %s", snippet)
			}
		})
	}
}
