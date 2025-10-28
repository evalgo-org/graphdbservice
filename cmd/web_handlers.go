package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"evalgo.org/graphservice/auth"
	"evalgo.org/graphservice/web/templates"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// TaskSession represents an active task execution session
type TaskSession struct {
	ID        string
	Tasks     []templates.TaskStatus
	Clients   map[chan templates.TaskStatus]bool
	mutex     sync.RWMutex
	StartTime time.Time
	EndTime   *time.Time
	// User context for migration logging
	UserID    string
	Username  string
	IPAddress string
	UserAgent string
}

var (
	sessions      = make(map[string]*TaskSession)
	sessionsMutex sync.RWMutex
)

// formatJSONError converts a JSON parsing error into a user-friendly message
func formatJSONError(err error, jsonStr string) string {
	errMsg := err.Error()

	// Handle syntax errors with position information
	if syntaxErr, ok := err.(*json.SyntaxError); ok {
		line, col, snippet := getErrorLocation(jsonStr, syntaxErr.Offset)
		return fmt.Sprintf(
			"%s at line %d, column %d<br><br><code style=\"background: #f5f5f5; padding: 0.5rem; display: block; border-radius: 4px;\">%s</code><br><small>Hint: Check for missing commas, quotes, brackets, or trailing commas</small>",
			errMsg, line, col, snippet,
		)
	}

	// Handle type errors
	if typeErr, ok := err.(*json.UnmarshalTypeError); ok {
		return fmt.Sprintf(
			"Type mismatch in field '%s': expected %s but got %s<br><small>Hint: Check that field values match the expected data type (string, number, object, array)</small>",
			typeErr.Field, typeErr.Type, typeErr.Value,
		)
	}

	// Handle common JSON errors with helpful hints
	lowerErr := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lowerErr, "unexpected end of json"):
		return "Unexpected end of JSON input<br><small>Hint: The JSON is incomplete. Check for missing closing brackets '}' or ']'</small>"
	case strings.Contains(lowerErr, "invalid character"):
		if strings.Contains(lowerErr, "after object key") {
			return fmt.Sprintf("%s<br><small>Hint: Check for missing colon ':' after a property name, or missing comma ',' between properties</small>", errMsg)
		}
		if strings.Contains(lowerErr, "looking for beginning of value") {
			return fmt.Sprintf("%s<br><small>Hint: Check for trailing commas or missing values</small>", errMsg)
		}
		return fmt.Sprintf("%s<br><small>Hint: Look for special characters, unescaped quotes, or formatting issues</small>", errMsg)
	case strings.Contains(lowerErr, "expecting property name"):
		return "Expecting property name enclosed in double quotes<br><small>Hint: All object keys must be enclosed in double quotes, e.g., \"key\": \"value\"</small>"
	default:
		return fmt.Sprintf("%s<br><small>Hint: Validate your JSON using a JSON validator tool or check the example format</small>", errMsg)
	}
}

// getErrorLocation finds the line, column, and context snippet for a JSON error
func getErrorLocation(jsonStr string, offset int64) (line int, col int, snippet string) {
	line = 1
	col = 1
	lastLineStart := 0

	for i := 0; i < int(offset) && i < len(jsonStr); i++ {
		if jsonStr[i] == '\n' {
			line++
			col = 1
			lastLineStart = i + 1
		} else {
			col++
		}
	}

	// Extract the problematic line
	lineEnd := lastLineStart
	for lineEnd < len(jsonStr) && jsonStr[lineEnd] != '\n' {
		lineEnd++
	}

	snippet = strings.TrimSpace(jsonStr[lastLineStart:lineEnd])
	if len(snippet) > 80 {
		// Truncate long lines but show the error position
		start := col - 40
		if start < 0 {
			start = 0
		}
		end := start + 80
		if end > len(snippet) {
			end = len(snippet)
		}
		snippet = "..." + snippet[start:end] + "..."
	}

	// Add a pointer to the error position
	pointer := strings.Repeat(" ", col-1) + "^"
	snippet = snippet + "\n" + pointer

	return line, col, snippet
}

// uiIndexHandler serves the main UI page
func uiIndexHandler(c echo.Context) error {
	// Get user from context (set by auth middleware)
	var user *auth.User
	if val := c.Get("user"); val != nil {
		if u, ok := val.(*auth.User); ok {
			user = u
		}
	}

	component := templates.Index(user)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// uiExecuteHandler processes task execution requests from the UI
func uiExecuteHandler(c echo.Context) error {
	// Parse the JSON from form
	taskJSON := c.FormValue("task_json")
	if taskJSON == "" {
		// Return 200 for HTMX to swap the content (validation errors are expected)
		return c.HTML(http.StatusOK, `
			<div class="alert alert-error">
				<strong>Error:</strong> Task JSON is required
			</div>
		`)
	}

	// Parse the migration request
	var req MigrationRequest
	if err := json.Unmarshal([]byte(taskJSON), &req); err != nil {
		// Generate user-friendly error message
		friendlyMsg := formatJSONError(err, taskJSON)
		// Return 200 for HTMX to swap the content (validation errors are expected)
		return c.HTML(http.StatusOK, fmt.Sprintf(`
			<div class="alert alert-error">
				<strong>Invalid JSON:</strong> %s
			</div>
		`, friendlyMsg))
	}

	// Validate request
	if req.Version == "" {
		// Return 200 for HTMX to swap the content (validation errors are expected)
		return c.HTML(http.StatusOK, `
			<div class="alert alert-error">
				<strong>Error:</strong> Version is required
			</div>
		`)
	}

	if len(req.Tasks) == 0 {
		// Return 200 for HTMX to swap the content (validation errors are expected)
		return c.HTML(http.StatusOK, `
			<div class="alert alert-error">
				<strong>Error:</strong> At least one task is required
			</div>
		`)
	}

	// Get user context for migration logging
	userID, username, _, authenticated := GetCurrentUser(c)
	if !authenticated {
		userID = "anonymous"
		username = "anonymous"
	}

	// Create session
	sessionID := uuid.New().String()
	session := &TaskSession{
		ID:        sessionID,
		Tasks:     make([]templates.TaskStatus, len(req.Tasks)),
		Clients:   make(map[chan templates.TaskStatus]bool),
		StartTime: time.Now(),
		UserID:    userID,
		Username:  username,
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	}

	// Initialize task statuses
	for i, task := range req.Tasks {
		session.Tasks[i] = templates.TaskStatus{
			Index:   i,
			Action:  task.Action,
			Status:  "pending",
			Message: "Waiting to start...",
		}

		// Extract repo/graph info for display
		if task.Src != nil {
			session.Tasks[i].SrcRepo = task.Src.Repo
			session.Tasks[i].SrcGraph = task.Src.Graph
			session.Tasks[i].SrcURL = task.Src.URL
		}
		if task.Tgt != nil {
			session.Tasks[i].TgtRepo = task.Tgt.Repo
			session.Tasks[i].TgtGraph = task.Tgt.Graph
			session.Tasks[i].TgtURL = task.Tgt.URL
		}
	}

	// Store session
	sessionsMutex.Lock()
	sessions[sessionID] = session
	sessionsMutex.Unlock()

	// Start task execution in background
	go executeTasksWithUpdates(sessionID, req)

	// Render initial task list
	component := templates.TaskResults(sessionID, session.Tasks)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// uiStreamHandler handles SSE connections for real-time task updates
func uiStreamHandler(c echo.Context) error {
	sessionID := c.Param("sessionID")

	// Get session
	sessionsMutex.RLock()
	session, exists := sessions[sessionID]
	sessionsMutex.RUnlock()

	if !exists {
		return c.String(http.StatusNotFound, "Session not found")
	}

	// Set headers for SSE
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no")

	// Create client channel
	clientChan := make(chan templates.TaskStatus, 10)

	// Register client
	session.mutex.Lock()
	session.Clients[clientChan] = true
	session.mutex.Unlock()

	// Cleanup on disconnect
	defer func() {
		session.mutex.Lock()
		delete(session.Clients, clientChan)
		session.mutex.Unlock()
		close(clientChan)
	}()

	// Send current state immediately
	session.mutex.RLock()
	for _, task := range session.Tasks {
		select {
		case clientChan <- task:
		default:
		}
	}
	session.mutex.RUnlock()

	// Stream updates
	for {
		select {
		case task, ok := <-clientChan:
			if !ok {
				return nil
			}

			// Render task update
			component := templates.TaskUpdate(task)

			// Write SSE message
			if _, err := fmt.Fprintf(c.Response().Writer, "event: task-update\n"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(c.Response().Writer, "data: "); err != nil {
				return err
			}
			if err := component.Render(c.Request().Context(), c.Response().Writer); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(c.Response().Writer, "\n\n"); err != nil {
				return err
			}

			// Flush if the response writer supports it
			if flusher, ok := c.Response().Writer.(http.Flusher); ok {
				flusher.Flush()
			}

		case <-c.Request().Context().Done():
			return nil
		}
	}
}

// executeTasksWithUpdates executes tasks and broadcasts updates to connected clients
func executeTasksWithUpdates(sessionID string, req MigrationRequest) {
	sessionsMutex.RLock()
	session, exists := sessions[sessionID]
	sessionsMutex.RUnlock()

	if !exists {
		return
	}

	// Start migration session logging (if auth is enabled)
	var migrationSession *auth.MigrationSession
	if migrationLogger != nil {
		// Serialize the request to JSON for logging
		requestJSON, _ := json.Marshal(req)

		var err error
		migrationSession, err = migrationLogger.StartSession(
			session.UserID,
			session.Username,
			session.IPAddress,
			session.UserAgent,
			len(req.Tasks),
			string(requestJSON),
		)
		if err != nil {
			fmt.Printf("Warning: Failed to start migration session logging: %v\n", err)
		} else {
			fmt.Printf("Started migration logging session: %s for user %s\n", migrationSession.ID, session.Username)
		}
	}

	// Track if any errors occurred
	hasError := false

	// Execute each task
	for i, task := range req.Tasks {
		// Record start time
		taskStartTime := time.Now()

		// Update status to in-progress
		session.mutex.Lock()
		session.Tasks[i].Status = "in-progress"
		session.Tasks[i].Message = "Executing..."
		session.Tasks[i].StartTime = taskStartTime.Format("15:04:05")
		taskUpdate := session.Tasks[i]
		session.mutex.Unlock()

		// Broadcast update
		broadcastTaskUpdate(session, taskUpdate)

		// Log task start
		if migrationSession != nil {
			repoID := ""
			graphID := ""
			if task.Src != nil {
				repoID = task.Src.Repo
				if task.Src.Graph != "" {
					graphID = task.Src.Graph
				}
			}
			srcURL := ""
			if task.Src != nil {
				srcURL = task.Src.URL
			}
			tgtURL := ""
			if task.Tgt != nil {
				tgtURL = task.Tgt.URL
			}

			if err := migrationLogger.StartTask(migrationSession.ID, i, task.Action, srcURL, tgtURL, repoID, graphID); err != nil {
				fmt.Printf("Warning: Failed to log task start: %v\n", err)
			}
		}

		// Validate task
		if err := validateTask(task); err != nil {
			hasError = true
			taskEndTime := time.Now()
			duration := taskEndTime.Sub(taskStartTime)

			session.mutex.Lock()
			session.Tasks[i].Status = "error"
			session.Tasks[i].Message = fmt.Sprintf("Validation failed: %v", err)
			session.Tasks[i].EndTime = taskEndTime.Format("15:04:05")
			session.Tasks[i].Duration = formatDuration(duration)
			taskUpdate = session.Tasks[i]
			session.mutex.Unlock()

			// Log task failure
			if migrationSession != nil {
				if logErr := migrationLogger.FailTask(migrationSession.ID, i, "validation_error", err.Error(), 0); logErr != nil {
					fmt.Printf("Warning: Failed to log task failure: %v\n", logErr)
				}
			}

			broadcastTaskUpdate(session, taskUpdate)
			continue
		}

		// Execute task with timeout
		resultChan := make(chan struct {
			result map[string]interface{}
			err    error
		}, 1)

		go func() {
			result, err := processTask(task, nil, i)
			resultChan <- struct {
				result map[string]interface{}
				err    error
			}{result, err}
		}()

		// Wait for result or timeout
		select {
		case res := <-resultChan:
			taskEndTime := time.Now()
			duration := taskEndTime.Sub(taskStartTime)

			session.mutex.Lock()
			if res.err != nil {
				hasError = true
				session.Tasks[i].Status = "error"
				session.Tasks[i].Message = fmt.Sprintf("Error: %v", res.err)

				// Log task failure
				if migrationSession != nil {
					if logErr := migrationLogger.FailTask(migrationSession.ID, i, "execution_error", res.err.Error(), 0); logErr != nil {
						fmt.Printf("Warning: Failed to log task failure: %v\n", logErr)
					}
				}
			} else {
				session.Tasks[i].Status = "success"
				if msg, ok := res.result["message"].(string); ok {
					session.Tasks[i].Message = msg
				} else {
					session.Tasks[i].Message = "Task completed successfully"
				}

				// Log task success
				if migrationSession != nil {
					dataSize := int64(0)
					tripleCount := int64(0)
					// Extract metrics from result if available
					if size, ok := res.result["data_size"].(int64); ok {
						dataSize = size
					}
					if count, ok := res.result["triple_count"].(int64); ok {
						tripleCount = count
					}

					if err := migrationLogger.CompleteTask(migrationSession.ID, i, dataSize, tripleCount); err != nil {
						fmt.Printf("Warning: Failed to log task completion: %v\n", err)
					}
				}
			}
			session.Tasks[i].EndTime = taskEndTime.Format("15:04:05")
			session.Tasks[i].Duration = formatDuration(duration)
			taskUpdate = session.Tasks[i]
			session.mutex.Unlock()

		case <-time.After(10 * time.Minute): // 10 minute timeout per task
			hasError = true
			taskEndTime := time.Now()
			duration := taskEndTime.Sub(taskStartTime)

			session.mutex.Lock()
			session.Tasks[i].Status = "timeout"
			session.Tasks[i].Message = "Task timed out after 10 minutes"
			session.Tasks[i].EndTime = taskEndTime.Format("15:04:05")
			session.Tasks[i].Duration = formatDuration(duration)
			taskUpdate = session.Tasks[i]
			session.mutex.Unlock()

			// Log task timeout
			if migrationSession != nil {
				if logErr := migrationLogger.TimeoutTask(migrationSession.ID, i, duration); logErr != nil {
					fmt.Printf("Warning: Failed to log task timeout: %v\n", logErr)
				}
			}
		}

		// Broadcast final update
		broadcastTaskUpdate(session, taskUpdate)
	}

	// Mark session as complete
	now := time.Now()
	session.mutex.Lock()
	session.EndTime = &now
	session.mutex.Unlock()

	// Complete or fail the migration session
	if migrationSession != nil {
		if hasError {
			if err := migrationLogger.FailSession(migrationSession.ID, "One or more tasks failed"); err != nil {
				fmt.Printf("Warning: Failed to mark migration session as failed: %v\n", err)
			}
		} else {
			if err := migrationLogger.CompleteSession(migrationSession.ID); err != nil {
				fmt.Printf("Warning: Failed to complete migration session: %v\n", err)
			} else {
				fmt.Printf("Completed migration logging session: %s\n", migrationSession.ID)
			}
		}
	}

	// Cleanup session after 1 hour
	time.AfterFunc(1*time.Hour, func() {
		sessionsMutex.Lock()
		delete(sessions, sessionID)
		sessionsMutex.Unlock()
	})
}

// broadcastTaskUpdate sends task update to all connected clients
func broadcastTaskUpdate(session *TaskSession, task templates.TaskStatus) {
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	for client := range session.Clients {
		select {
		case client <- task:
		default:
			// Client buffer full, skip
		}
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}
