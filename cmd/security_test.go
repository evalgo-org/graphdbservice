package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"evalgo.org/graphservice/auth"
	"github.com/labstack/echo/v4"
)

// TestEndpointSecurity verifies that all sensitive endpoints require proper authentication
func TestEndpointSecurity(t *testing.T) {
	// Initialize authentication for testing
	if err := InitializeAuth(); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	// Set up test environment
	os.Setenv("AUTH_MODE", "rbac")
	os.Setenv("JWT_SECRET", "test-secret-key-12345")
	os.Setenv("API_KEY", "test-api-key-12345")

	e := echo.New()

	// Get auth mode
	authMode := getAuthMode()

	// Register all routes exactly as in main application
	e.POST("/v1/api/action", migrationHandler, apiKeyMiddleware)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})
	e.GET("/login", loginPageHandler)
	e.POST("/auth/login", loginHandler)

	ui := e.Group("", AuthMiddleware(authMode))
	ui.GET("/", uiIndexHandler)
	ui.GET("/ui", uiIndexHandler)
	ui.POST("/ui/execute", uiExecuteHandler)
	ui.GET("/api/users/me", getCurrentUserHandler)
	ui.POST("/api/users/me/password", changePasswordHandler)

	admin := ui.Group("/admin", AdminOnlyMiddleware())
	admin.GET("/users/api", listUsersAPIHandler)
	admin.POST("/users", createUserHandler)
	admin.GET("/users/:username", getUserHandler)
	admin.PUT("/users/:username", updateUserHandler)
	admin.DELETE("/users/:username", deleteUserHandler)
	admin.GET("/audit/api", getAuditLogsAPIHandler)
	admin.POST("/audit/rotate", rotateAuditLogsHandler)
	admin.GET("/migrations/stats", getMigrationStatsHandler)
	admin.GET("/migrations/active", getActiveSessionsHandler)
	admin.GET("/migrations/session/:id", getMigrationSessionHandler)
	admin.GET("/migrations/summary/:date", getDailySummaryHandler)
	admin.POST("/migrations/rotate", rotateOldMigrationLogsHandler)

	tests := []struct {
		name           string
		method         string
		path           string
		headers        map[string]string
		expectedStatus int
		description    string
	}{
		// Public endpoints - should be accessible
		{
			name:           "Health check is public",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			description:    "Health endpoint should be publicly accessible",
		},
		{
			name:           "Login page is public",
			method:         "GET",
			path:           "/login",
			expectedStatus: http.StatusOK,
			description:    "Login page should be publicly accessible",
		},

		// API endpoint - requires API key
		{
			name:           "API endpoint without API key is blocked",
			method:         "POST",
			path:           "/v1/api/action",
			expectedStatus: http.StatusUnauthorized,
			description:    "API endpoint should require API key",
		},
		{
			name:   "API endpoint with invalid API key is blocked",
			method: "POST",
			path:   "/v1/api/action",
			headers: map[string]string{
				"x-api-key": "invalid-key",
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "API endpoint should reject invalid API keys",
		},
		{
			name:   "API endpoint with valid API key is allowed",
			method: "POST",
			path:   "/v1/api/action",
			headers: map[string]string{
				"x-api-key":    "test-api-key-12345",
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusBadRequest, // Will fail validation but auth passed
			description:    "API endpoint should accept valid API keys",
		},

		// Protected UI endpoints - require authentication
		{
			name:           "Home page requires authentication",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Home page should require authentication",
		},
		{
			name:           "UI execute requires authentication",
			method:         "POST",
			path:           "/ui/execute",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "UI execute should require authentication",
		},
		{
			name:           "Get current user requires authentication",
			method:         "GET",
			path:           "/api/users/me",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "User profile endpoint should require authentication",
		},
		{
			name:           "Change password requires authentication",
			method:         "POST",
			path:           "/api/users/me/password",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Password change should require authentication",
		},

		// Admin endpoints - require admin role
		{
			name:           "List users API requires authentication",
			method:         "GET",
			path:           "/admin/users/api",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Admin endpoints should require authentication",
		},
		{
			name:           "Create user requires authentication",
			method:         "POST",
			path:           "/admin/users",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "User creation should require authentication",
		},
		{
			name:           "Get user requires authentication",
			method:         "GET",
			path:           "/admin/users/testuser",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Get user endpoint should require authentication",
		},
		{
			name:           "Update user requires authentication",
			method:         "PUT",
			path:           "/admin/users/testuser",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Update user should require authentication",
		},
		{
			name:           "Delete user requires authentication",
			method:         "DELETE",
			path:           "/admin/users/testuser",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Delete user should require authentication",
		},
		{
			name:           "Audit logs API requires authentication",
			method:         "GET",
			path:           "/admin/audit/api",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Audit logs should require authentication",
		},
		{
			name:           "Rotate audit logs requires authentication",
			method:         "POST",
			path:           "/admin/audit/rotate",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Audit rotation should require authentication",
		},
		{
			name:           "Migration stats requires authentication",
			method:         "GET",
			path:           "/admin/migrations/stats",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Migration stats should require authentication",
		},
		{
			name:           "Active sessions requires authentication",
			method:         "GET",
			path:           "/admin/migrations/active",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Active sessions should require authentication",
		},
		{
			name:           "Migration session details requires authentication",
			method:         "GET",
			path:           "/admin/migrations/session/test-id",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Session details should require authentication",
		},
		{
			name:           "Daily summary requires authentication",
			method:         "GET",
			path:           "/admin/migrations/summary/2025-10-28",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Daily summary should require authentication",
		},
		{
			name:           "Rotate migration logs requires authentication",
			method:         "POST",
			path:           "/admin/migrations/rotate",
			expectedStatus: http.StatusFound, // Redirect to login
			description:    "Migration log rotation should require authentication",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			var req *http.Request
			if tt.method == "POST" || tt.method == "PUT" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader("{}"))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Record response
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// Check status
			if rec.Code != tt.expectedStatus {
				t.Errorf("%s: expected status %d, got %d\nDescription: %s",
					tt.name, tt.expectedStatus, rec.Code, tt.description)
			}
		})
	}
}

// TestAdminOnlyEndpointsWithUserRole verifies that user role cannot access admin endpoints
func TestAdminOnlyEndpointsWithUserRole(t *testing.T) {
	// Initialize authentication
	if err := InitializeAuth(); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	os.Setenv("AUTH_MODE", "rbac")
	os.Setenv("JWT_SECRET", "test-secret-key-12345")

	// Create a test user in the store
	testUser := auth.User{
		ID:       "user-id",
		Username: "testuser",
		Role:     auth.RoleUser,
	}

	// Create the user in the store
	_, err := userStore.CreateUser("testuser", "TestPass123!", "test@example.com", auth.RoleUser)
	if err != nil {
		t.Logf("Note: User may already exist: %v", err)
	}

	// Generate token
	token, err := auth.GenerateToken(testUser, getJWTSecret(), 24)
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}

	e := echo.New()
	authMode := getAuthMode()

	ui := e.Group("", AuthMiddleware(authMode))
	admin := ui.Group("/admin", AdminOnlyMiddleware())
	admin.GET("/users/api", listUsersAPIHandler)
	admin.POST("/users", createUserHandler)
	admin.GET("/migrations/stats", getMigrationStatsHandler)

	adminEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/users/api"},
		{"POST", "/admin/users"},
		{"GET", "/admin/migrations/stats"},
	}

	for _, endpoint := range adminEndpoints {
		t.Run("User role blocked from "+endpoint.path, func(t *testing.T) {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			req.AddCookie(&http.Cookie{
				Name:  "session",
				Value: token,
			})

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// Should be forbidden (403) for user role
			if rec.Code != http.StatusForbidden {
				t.Errorf("Expected status 403 Forbidden for user role accessing %s, got %d",
					endpoint.path, rec.Code)
			}
		})
	}
}

// TestAuthMiddlewareBypassAttempts tests various authentication bypass attempts
func TestAuthMiddlewareBypassAttempts(t *testing.T) {
	if err := InitializeAuth(); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	os.Setenv("AUTH_MODE", "rbac")
	os.Setenv("JWT_SECRET", "test-secret-key-12345")

	e := echo.New()
	authMode := getAuthMode()

	ui := e.Group("", AuthMiddleware(authMode))
	ui.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"data": "sensitive"})
	})

	tests := []struct {
		name        string
		cookieValue string
		description string
	}{
		{
			name:        "No cookie",
			cookieValue: "",
			description: "Request without session cookie should be rejected",
		},
		{
			name:        "Invalid token format",
			cookieValue: "invalid-token",
			description: "Invalid JWT format should be rejected",
		},
		{
			name:        "Expired token",
			cookieValue: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MzAwMDAwMDAsInVzZXJfaWQiOiJ0ZXN0IiwidXNlcm5hbWUiOiJ0ZXN0Iiwicm9sZSI6InVzZXIifQ.invalid",
			description: "Expired token should be rejected",
		},
		{
			name:        "Malformed JWT",
			cookieValue: "not.a.jwt",
			description: "Malformed JWT should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.cookieValue != "" {
				req.AddCookie(&http.Cookie{
					Name:  "session",
					Value: tt.cookieValue,
				})
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// All invalid authentication attempts should redirect to login
			if rec.Code != http.StatusFound {
				t.Errorf("%s: expected redirect (302), got %d\nDescription: %s",
					tt.name, rec.Code, tt.description)
			}

			// Check redirect location
			location := rec.Header().Get("Location")
			if !strings.HasPrefix(location, "/login") {
				t.Errorf("%s: expected redirect to /login, got %s", tt.name, location)
			}
		})
	}
}

// TestAPIKeyMiddleware tests the API key authentication
func TestAPIKeyMiddleware(t *testing.T) {
	os.Setenv("API_KEY", "correct-api-key")

	e := echo.New()
	e.POST("/api/test", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "success"})
	}, apiKeyMiddleware)

	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
		description    string
	}{
		{
			name:           "Valid API key",
			apiKey:         "correct-api-key",
			expectedStatus: http.StatusOK,
			description:    "Valid API key should allow access",
		},
		{
			name:           "Invalid API key",
			apiKey:         "wrong-api-key",
			expectedStatus: http.StatusUnauthorized,
			description:    "Invalid API key should be rejected",
		},
		{
			name:           "Missing API key",
			apiKey:         "",
			expectedStatus: http.StatusUnauthorized,
			description:    "Missing API key should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/test", strings.NewReader("{}"))
			if tt.apiKey != "" {
				req.Header.Set("x-api-key", tt.apiKey)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("%s: expected status %d, got %d\nDescription: %s",
					tt.name, tt.expectedStatus, rec.Code, tt.description)
			}
		})
	}
}
