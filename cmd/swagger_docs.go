package cmd

// This file contains Swagger/OpenAPI documentation annotations for all API endpoints.
// The actual handler implementations are in other files.

// Health endpoint
// @Summary Health check
// @Description Returns the health status of the service
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "status: healthy"
// @Router /health [get]
func swaggerHealthCheck() {}

// Login page
// @Summary Login page
// @Description Renders the HTML login page
// @Tags Authentication
// @Produce html
// @Success 200 {string} string "HTML login page"
// @Router /login [get]
func swaggerLoginPage() {}

// Login handler
// @Summary User login
// @Description Authenticate user with username and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param credentials body object{username=string,password=string} true "Login credentials"
// @Success 200 {object} object{token=string,user=object{username=string,role=string}} "JWT token and user info"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Invalid credentials"
// @Router /auth/login [post]
func swaggerLogin() {}

// Logout handler
// @Summary User logout
// @Description Clear authentication session and redirect to login
// @Tags Authentication
// @Security BearerAuth
// @Success 302 {string} string "Redirect to login page"
// @Router /logout [get]
func swaggerLogout() {}

// Migration handler (main API endpoint)
// @Summary Execute GraphDB migration tasks
// @Description Execute one or more GraphDB migration tasks (repository migration, graph migration, etc.)
// @Description
// @Description Supported actions:
// @Description - repo-migration: Full repository migration (backup and restore)
// @Description - graph-migration: Named graph migration
// @Description - repo-create: Create a new repository
// @Description - repo-delete: Delete a repository
// @Description - repo-rename: Rename a repository
// @Description - graph-import: Import RDF data into a graph
// @Description - graph-export: Export RDF data from a graph
// @Description - graph-delete: Delete a named graph
// @Description - graph-rename: Rename a named graph
// @Tags Migration
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body MigrationRequest true "Migration request with tasks"
// @Success 200 {object} object{version=string,results=[]object} "Migration results"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /v1/api/action [post]
func swaggerMigrationHandler() {}

// UI execute handler
// @Summary Execute tasks via Web UI
// @Description Execute GraphDB tasks through the web interface with real-time updates
// @Tags Migration
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body object{version=string,tasks=[]object} true "Task execution request"
// @Success 200 {object} object{session_id=string} "Session ID for streaming results"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /ui/execute [post]
func swaggerUIExecute() {}

// UI stream handler
// @Summary Stream task execution results
// @Description Server-Sent Events stream for real-time task execution updates
// @Tags Migration
// @Produce text/event-stream
// @Security BearerAuth
// @Param sessionID path string true "Session ID"
// @Success 200 {string} string "SSE stream"
// @Router /ui/stream/{sessionID} [get]
func swaggerUIStream() {}

// Get current user
// @Summary Get current user info
// @Description Returns information about the currently authenticated user
// @Tags Profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{id=string,username=string,email=string,role=string} "User information"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/users/me [get]
func swaggerGetCurrentUser() {}

// Change password
// @Summary Change user password
// @Description Change the password for the currently authenticated user
// @Tags Profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body object{old_password=string,new_password=string} true "Password change request"
// @Success 200 {object} map[string]string "Password changed successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/users/me/password [post]
func swaggerChangePassword() {}

// List users (API)
// @Summary List all users
// @Description Returns a list of all users (admin only)
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {array} object{id=string,username=string,email=string,role=string,locked=boolean} "List of users"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Router /admin/users/api [get]
func swaggerListUsersAPI() {}

// Create user
// @Summary Create a new user
// @Description Create a new user account (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body object{username=string,password=string,email=string,role=string,must_change_password=boolean} true "User data"
// @Success 201 {object} object{id=string,username=string} "User created"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 409 {object} map[string]string "User already exists"
// @Router /admin/users [post]
func swaggerCreateUser() {}

// Get user
// @Summary Get user details
// @Description Get details of a specific user (admin only)
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} object{id=string,username=string,email=string,role=string,locked=boolean,failed_logins=int,must_change_password=boolean} "User details"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 404 {object} map[string]string "User not found"
// @Router /admin/users/{username} [get]
func swaggerGetUser() {}

// Update user
// @Summary Update user
// @Description Update user details (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Param user body object{email=string,password=string,role=string,locked=boolean,must_change_password=boolean,reset_failed_logins=boolean} true "User update data"
// @Success 200 {object} map[string]string "User updated successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 404 {object} map[string]string "User not found"
// @Router /admin/users/{username} [put]
func swaggerUpdateUser() {}

// Delete user
// @Summary Delete user
// @Description Delete a user account (admin only)
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} map[string]string "User deleted successfully"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 404 {object} map[string]string "User not found"
// @Router /admin/users/{username} [delete]
func swaggerDeleteUser() {}

// Get audit logs (API)
// @Summary Get audit logs
// @Description Retrieve audit logs with optional filtering (admin only)
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Param date query string false "Filter by date (YYYY-MM-DD)"
// @Param username query string false "Filter by username"
// @Param action query string false "Filter by action"
// @Param limit query int false "Limit number of results" default(100)
// @Success 200 {object} object{logs=[]object,total=int} "Audit logs"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Router /admin/audit/api [get]
func swaggerGetAuditLogs() {}

// Rotate audit logs
// @Summary Rotate old audit logs
// @Description Archive audit logs older than the retention period (admin only)
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string "Logs rotated successfully"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /admin/audit/rotate [post]
func swaggerRotateAuditLogs() {}

// Get migration stats
// @Summary Get migration statistics
// @Description Retrieve migration session statistics (admin only)
// @Tags Logs
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{active_sessions=int,completed_sessions=int,failed_sessions=int,total_tasks=int,completed_tasks=int,failed_tasks=int,timeout_tasks=int,total_data_size=int} "Migration statistics"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Router /admin/migrations/stats [get]
func swaggerGetMigrationStats() {}

// Get active sessions
// @Summary Get active migration sessions
// @Description Retrieve all currently running migration sessions (admin only)
// @Tags Logs
// @Produce json
// @Security BearerAuth
// @Success 200 {array} object{id=string,username=string,status=string,start_time=string,total_tasks=int} "Active sessions"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Router /admin/migrations/active [get]
func swaggerGetActiveSessions() {}

// Get migration session
// @Summary Get migration session details
// @Description Retrieve detailed information about a specific migration session (admin only)
// @Tags Logs
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} object{id=string,username=string,status=string,start_time=string,duration_ms=int,total_tasks=int,completed_tasks=int,failed_tasks=int,timeout_tasks=int,total_data_size_bytes=int,metadata=object} "Session details"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 404 {object} map[string]string "Session not found"
// @Router /admin/migrations/session/{id} [get]
func swaggerGetMigrationSession() {}

// Get daily summary
// @Summary Get daily migration summary
// @Description Retrieve migration summary for a specific date (admin only)
// @Tags Logs
// @Produce json
// @Security BearerAuth
// @Param date path string true "Date (YYYY-MM-DD)"
// @Success 200 {object} object{date=string,total_sessions=int,completed_sessions=int,failed_sessions=int,total_tasks=int,completed_tasks=int,failed_tasks=int,total_data_size=int,sessions=[]object} "Daily summary"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Router /admin/migrations/summary/{date} [get]
func swaggerGetDailySummary() {}

// Rotate migration logs
// @Summary Rotate old migration logs
// @Description Archive migration logs older than the retention period (admin only)
// @Tags Logs
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string "Logs rotated successfully"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - admin only"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /admin/migrations/rotate [post]
func swaggerRotateMigrationLogs() {}
