package cmd

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"evalgo.org/graphservice/auth"
	"evalgo.org/graphservice/web/templates"
	"github.com/labstack/echo/v4"
)

// migrationLogsPageHandler serves the migration logs UI page
func migrationLogsPageHandler(c echo.Context) error {
	userID := c.Get("user_id")
	var user *auth.User
	if userID != nil {
		user, _ = getUserByID(userID.(string))
	}
	return renderMigrationLogsPage(c, user)
}

// listMigrationLogsHandler returns the list of migration sessions with filtering
func listMigrationLogsHandler(c echo.Context) error {
	if migrationLogger == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"sessions": []auth.MigrationSession{},
		})
	}

	// Parse query parameters
	date := c.QueryParam("date")
	username := c.QueryParam("username")
	status := c.QueryParam("status")
	limit := 50 // Default limit

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// If date is specified, get summary for that date
	if date != "" {
		summary, err := migrationLogger.GetDailySummary(date)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": fmt.Sprintf("Failed to fetch migration logs: %v", err),
			})
		}

		// Filter sessions by username and status if specified
		filteredSessions := filterSessions(summary.Sessions, username, status)

		// Limit results
		if len(filteredSessions) > limit {
			filteredSessions = filteredSessions[:limit]
		}

		return renderMigrationLogsList(c, filteredSessions)
	}

	// If no date specified, get recent sessions from today
	today := time.Now().Format("2006-01-02")
	summary, err := migrationLogger.GetDailySummary(today)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": fmt.Sprintf("Failed to fetch migration logs: %v", err),
		})
	}

	// Also get active sessions
	activeSessions := migrationLogger.GetActiveSessions()

	// Combine and filter - convert pointers to values
	allSessions := make([]auth.MigrationSession, 0, len(activeSessions)+len(summary.Sessions))
	for _, session := range activeSessions {
		if session != nil {
			allSessions = append(allSessions, *session)
		}
	}
	allSessions = append(allSessions, summary.Sessions...)

	filteredSessions := filterSessions(allSessions, username, status)

	// Limit results
	if len(filteredSessions) > limit {
		filteredSessions = filteredSessions[:limit]
	}

	return renderMigrationLogsList(c, filteredSessions)
}

// getMigrationSessionHandler returns detailed information about a specific session
func getMigrationSessionHandler(c echo.Context) error {
	if migrationLogger == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Migration logging not enabled")
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Session ID is required")
	}

	session, err := migrationLogger.GetSession(sessionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Session not found: %v", err))
	}

	return c.JSON(http.StatusOK, session)
}

// getMigrationStatsHandler returns statistics about migrations
func getMigrationStatsHandler(c echo.Context) error {
	if migrationLogger == nil {
		stats := map[string]interface{}{
			"total_sessions":      0,
			"active_sessions":     0,
			"completed_sessions":  0,
			"failed_sessions":     0,
			"total_tasks":         0,
			"completed_tasks":     0,
			"failed_tasks":        0,
			"timeout_tasks":       0,
			"total_data_size":     0,
		}
		c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
		c.Response().WriteHeader(http.StatusOK)
		return templates.MigrationStatsDashboard(stats).Render(c.Request().Context(), c.Response().Writer)
	}

	// Get active sessions
	activeSessions := migrationLogger.GetActiveSessions()

	// Get today's summary
	today := time.Now().Format("2006-01-02")
	summary, err := migrationLogger.GetDailySummary(today)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": fmt.Sprintf("Failed to fetch stats: %v", err),
		})
	}

	stats := map[string]interface{}{
		"active_sessions":     len(activeSessions),
		"total_sessions":      summary.TotalSessions,
		"completed_sessions":  summary.CompletedSessions,
		"failed_sessions":     summary.FailedSessions,
		"total_tasks":         summary.TotalTasks,
		"completed_tasks":     summary.CompletedTasks,
		"failed_tasks":        summary.FailedTasks,
		"timeout_tasks":       summary.TimeoutTasks,
		"total_data_size":     summary.TotalDataSize,
		"avg_duration_ms":     summary.AvgDuration,
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	return templates.MigrationStatsDashboard(stats).Render(c.Request().Context(), c.Response().Writer)
}

// getActiveSessionsHandler returns currently running migration sessions
func getActiveSessionsHandler(c echo.Context) error {
	if migrationLogger == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"sessions": []*auth.MigrationSession{},
		})
	}

	sessions := migrationLogger.GetActiveSessions()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

// getDailySummaryHandler returns the summary for a specific date
func getDailySummaryHandler(c echo.Context) error {
	if migrationLogger == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Migration logging not enabled")
	}

	date := c.Param("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	summary, err := migrationLogger.GetDailySummary(date)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch summary: %v", err))
	}

	return c.JSON(http.StatusOK, summary)
}

// rotateOldMigrationLogsHandler triggers log rotation
func rotateOldMigrationLogsHandler(c echo.Context) error {
	if migrationLogger == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Migration logging not enabled")
	}

	if err := migrationLogger.RotateOldLogs(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to rotate logs: %v", err))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Old logs rotated successfully",
	})
}

// Helper functions

// filterSessions filters sessions by username and status
func filterSessions(sessions []auth.MigrationSession, username, status string) []auth.MigrationSession {
	if username == "" && status == "" {
		return sessions
	}

	filtered := make([]auth.MigrationSession, 0)
	for _, session := range sessions {
		if username != "" && session.Username != username {
			continue
		}
		if status != "" && session.Status != status {
			continue
		}
		filtered = append(filtered, session)
	}

	return filtered
}

// renderMigrationLogsPage renders the migration logs page using templ
func renderMigrationLogsPage(c echo.Context, user *auth.User) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	return templates.MigrationLogs(user).Render(c.Request().Context(), c.Response().Writer)
}

// renderMigrationLogsList renders the sessions list using templ
func renderMigrationLogsList(c echo.Context, sessions []auth.MigrationSession) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	return templates.MigrationSessionsList(sessions).Render(c.Request().Context(), c.Response().Writer)
}

// getUserByID retrieves a user by ID (helper function)
func getUserByID(userID string) (*auth.User, error) {
	if userStore == nil {
		return nil, fmt.Errorf("user store not initialized")
	}

	// Get all users and find the one with matching ID
	db, err := userStore.Load()
	if err != nil {
		return nil, err
	}

	for _, user := range db.Users {
		if user.ID == userID {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}
