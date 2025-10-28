package cmd

import (
	"net/http"
	"time"

	"evalgo.org/graphservice/auth"
	"evalgo.org/graphservice/web/templates"
	"github.com/labstack/echo/v4"
)

// auditLogsPageHandler serves the audit logs page (admin only)
func auditLogsPageHandler(c echo.Context) error {
	// Get user from context
	var user *auth.User
	if val := c.Get("user"); val != nil {
		if u, ok := val.(*auth.User); ok {
			user = u
		}
	}

	component := templates.AuditLogs(user)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// listAuditLogsHandler returns audit logs for HTMX (admin only)
func listAuditLogsHandler(c echo.Context) error {
	if auditLogger == nil {
		return c.HTML(http.StatusOK, "<p>Audit logging is not enabled. Set AUTH_MODE to enable.</p>")
	}

	// Parse query parameters
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	username := c.QueryParam("username")
	action := c.QueryParam("action")

	// Set defaults if not provided
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	// Build search criteria
	criteria := auth.AuditSearchCriteria{
		StartDate: startDate,
		EndDate:   endDate,
		Username:  username,
		Action:    action,
	}

	// Search audit logs
	entries, err := auditLogger.SearchEntries(criteria)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "<p>Failed to load audit logs</p>")
	}

	// Reverse entries to show most recent first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	component := templates.AuditLogList(entries)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// getAuditLogsAPIHandler returns audit logs as JSON (admin only)
func getAuditLogsAPIHandler(c echo.Context) error {
	if auditLogger == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Audit logging is not enabled",
		})
	}

	// Parse query parameters
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	username := c.QueryParam("username")
	action := c.QueryParam("action")

	// Set defaults if not provided
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	// Build search criteria
	criteria := auth.AuditSearchCriteria{
		StartDate: startDate,
		EndDate:   endDate,
		Username:  username,
		Action:    action,
	}

	// Search audit logs
	entries, err := auditLogger.SearchEntries(criteria)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to load audit logs",
		})
	}

	// Reverse entries to show most recent first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
		"count":      len(entries),
		"entries":    entries,
	})
}

// rotateAuditLogsHandler triggers log rotation (admin only)
func rotateAuditLogsHandler(c echo.Context) error {
	if auditLogger == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Audit logging is not enabled",
		})
	}

	// Default: keep logs for 90 days
	daysToKeep := 90

	if err := auditLogger.RotateOldLogs(daysToKeep); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to rotate logs",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Logs rotated successfully",
	})
}
