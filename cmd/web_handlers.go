package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

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
}

var (
	sessions      = make(map[string]*TaskSession)
	sessionsMutex sync.RWMutex
)

// uiIndexHandler serves the main UI page
func uiIndexHandler(c echo.Context) error {
	component := templates.Index()
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// uiExecuteHandler processes task execution requests from the UI
func uiExecuteHandler(c echo.Context) error {
	// Parse the JSON from form
	taskJSON := c.FormValue("task_json")
	if taskJSON == "" {
		return c.HTML(http.StatusBadRequest, `
			<div class="alert alert-error">
				Task JSON is required
			</div>
		`)
	}

	// Parse the migration request
	var req MigrationRequest
	if err := json.Unmarshal([]byte(taskJSON), &req); err != nil {
		return c.HTML(http.StatusBadRequest, fmt.Sprintf(`
			<div class="alert alert-error">
				Invalid JSON format: %s
			</div>
		`, err.Error()))
	}

	// Validate request
	if req.Version == "" {
		return c.HTML(http.StatusBadRequest, `
			<div class="alert alert-error">
				Version is required
			</div>
		`)
	}

	if len(req.Tasks) == 0 {
		return c.HTML(http.StatusBadRequest, `
			<div class="alert alert-error">
				At least one task is required
			</div>
		`)
	}

	// Create session
	sessionID := uuid.New().String()
	session := &TaskSession{
		ID:        sessionID,
		Tasks:     make([]templates.TaskStatus, len(req.Tasks)),
		Clients:   make(map[chan templates.TaskStatus]bool),
		StartTime: time.Now(),
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
			fmt.Fprintf(c.Response().Writer, "event: task-update\n")
			fmt.Fprintf(c.Response().Writer, "data: ")
			if err := component.Render(c.Request().Context(), c.Response().Writer); err != nil {
				return err
			}
			fmt.Fprintf(c.Response().Writer, "\n\n")

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

		// Validate task
		if err := validateTask(task); err != nil {
			taskEndTime := time.Now()
			duration := taskEndTime.Sub(taskStartTime)

			session.mutex.Lock()
			session.Tasks[i].Status = "error"
			session.Tasks[i].Message = fmt.Sprintf("Validation failed: %v", err)
			session.Tasks[i].EndTime = taskEndTime.Format("15:04:05")
			session.Tasks[i].Duration = formatDuration(duration)
			taskUpdate = session.Tasks[i]
			session.mutex.Unlock()

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
				session.Tasks[i].Status = "error"
				session.Tasks[i].Message = fmt.Sprintf("Error: %v", res.err)
			} else {
				session.Tasks[i].Status = "success"
				if msg, ok := res.result["message"].(string); ok {
					session.Tasks[i].Message = msg
				} else {
					session.Tasks[i].Message = "Task completed successfully"
				}
			}
			session.Tasks[i].EndTime = taskEndTime.Format("15:04:05")
			session.Tasks[i].Duration = formatDuration(duration)
			taskUpdate = session.Tasks[i]
			session.mutex.Unlock()

		case <-time.After(10 * time.Minute): // 10 minute timeout per task
			taskEndTime := time.Now()
			duration := taskEndTime.Sub(taskStartTime)

			session.mutex.Lock()
			session.Tasks[i].Status = "timeout"
			session.Tasks[i].Message = "Task timed out after 10 minutes"
			session.Tasks[i].EndTime = taskEndTime.Format("15:04:05")
			session.Tasks[i].Duration = formatDuration(duration)
			taskUpdate = session.Tasks[i]
			session.mutex.Unlock()
		}

		// Broadcast final update
		broadcastTaskUpdate(session, taskUpdate)
	}

	// Mark session as complete
	now := time.Now()
	session.mutex.Lock()
	session.EndTime = &now
	session.mutex.Unlock()

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
