package cmd

import (
	"fmt"
	"net/http"

	"evalgo.org/graphservice/auth"
	"evalgo.org/graphservice/web/templates"
	"github.com/labstack/echo/v4"
)

// loginPageHandler serves the login page
func loginPageHandler(c echo.Context) error {
	// Check if already logged in
	cookie, err := c.Cookie("session")
	if err == nil && cookie.Value != "" {
		// Validate existing session
		if _, err := auth.ValidateToken(cookie.Value, getJWTSecret()); err == nil {
			// Already logged in, redirect to home
			return c.Redirect(http.StatusFound, "/")
		}
	}

	// Show login page
	errorMsg := c.QueryParam("error")
	component := templates.Login(errorMsg)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// loginHandler processes login requests
func loginHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Validate input
	if username == "" || password == "" {
		return c.Redirect(http.StatusFound, "/login?error="+encodeURIComponent("Username and password are required"))
	}

	// Get user from store
	user, err := userStore.GetUser(username)
	if err != nil {
		// User not found - use generic error message to prevent username enumeration
		return c.Redirect(http.StatusFound, "/login?error="+encodeURIComponent("Invalid username or password"))
	}

	// Check if account is locked
	if user.Locked {
		return c.Redirect(http.StatusFound, "/login?error="+encodeURIComponent("Account is locked. Please contact an administrator"))
	}

	// Verify password
	if !auth.CheckPasswordHash(password, user.PasswordHash) {
		// Record failed login attempt
		_ = userStore.RecordLoginAttempt(username, false)

		// Get updated user to check if account is now locked
		user, _ = userStore.GetUser(username)
		if user != nil && user.Locked {
			return c.Redirect(http.StatusFound, "/login?error="+encodeURIComponent("Too many failed attempts. Account is now locked"))
		}

		return c.Redirect(http.StatusFound, "/login?error="+encodeURIComponent("Invalid username or password"))
	}

	// Record successful login
	_ = userStore.RecordLoginAttempt(username, true)

	// Generate JWT token
	token, err := auth.GenerateToken(*user, getJWTSecret(), getSessionTimeoutHours())
	if err != nil {
		fmt.Printf("ERROR: Failed to generate token: %v\n", err)
		return c.Redirect(http.StatusFound, "/login?error="+encodeURIComponent("Internal error. Please try again"))
	}

	// Set session cookie
	cookie := &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(c), // Set Secure flag if HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   getSessionTimeoutHours() * 3600,
	}
	c.SetCookie(cookie)

	// Log audit entry
	logAudit(c, user.ID, username, "login", "auth", true, "", nil)

	// Redirect to home page
	return c.Redirect(http.StatusFound, "/")
}

// logoutHandler processes logout requests
func logoutHandler(c echo.Context) error {
	// Get user info from context (set by middleware)
	username := ""
	userID := ""
	if val := c.Get("username"); val != nil {
		username = val.(string)
	}
	if val := c.Get("user_id"); val != nil {
		userID = val.(string)
	}

	// Clear session cookie
	cookie := &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(c),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1, // Delete cookie
	}
	c.SetCookie(cookie)

	// Log audit entry
	if username != "" {
		logAudit(c, userID, username, "logout", "auth", true, "", nil)
	}

	// Redirect to login page
	return c.Redirect(http.StatusFound, "/login")
}

// Helper functions

func encodeURIComponent(s string) string {
	// Simple URL encoding for error messages
	return s // For now, Echo handles this
}

func isHTTPS(c echo.Context) bool {
	// Check if request is HTTPS
	return c.Scheme() == "https" || c.Request().Header.Get("X-Forwarded-Proto") == "https"
}

func getJWTSecret() string {
	// Get JWT secret from environment variable
	secret := getEnv("JWT_SECRET", "")
	if secret == "" {
		fmt.Println("WARNING: JWT_SECRET not set. Using default (INSECURE for production)")
		secret = "default-jwt-secret-change-in-production"
	}
	return secret
}

func getSessionTimeoutHours() int {
	// Get session timeout from environment variable (default 1 hour)
	timeout := getEnvInt("SESSION_TIMEOUT", 3600) // in seconds
	return timeout / 3600                          // convert to hours
}

func getAuthMode() auth.AuthMode {
	// Get authentication mode from environment variable
	mode := getEnv("AUTH_MODE", string(auth.AuthModeNone))
	return auth.AuthMode(mode)
}

func logAudit(c echo.Context, userID, username, action, resource string, success bool, errorMsg string, details map[string]interface{}) {
	// TODO: Implement audit logging (Phase 3)
	// For now, just log to console
	fmt.Printf("[AUDIT] User=%s Action=%s Resource=%s Success=%v IP=%s\n",
		username, action, resource, success, c.RealIP())
}
