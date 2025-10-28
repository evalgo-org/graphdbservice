package cmd

import (
	"net/http"

	"evalgo.org/graphservice/auth"
	"github.com/labstack/echo/v4"
)

// AuthMiddleware validates JWT tokens and enforces authentication
func AuthMiddleware(mode auth.AuthMode) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip authentication if mode is "none"
			if mode == auth.AuthModeNone {
				return next(c)
			}

			// Check for JWT token in cookie
			cookie, err := c.Cookie("session")
			if err != nil || cookie.Value == "" {
				// No session cookie, redirect to login
				return c.Redirect(http.StatusFound, "/login")
			}

			// Validate token
			claims, err := auth.ValidateToken(cookie.Value, getJWTSecret())
			if err != nil {
				// Invalid or expired token, redirect to login
				return c.Redirect(http.StatusFound, "/login?error=Session expired. Please login again")
			}

			// Store user info in context for handlers to use
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
			c.Set("authenticated", true)

			return next(c)
		}
	}
}

// AdminOnlyMiddleware ensures only admin users can access
func AdminOnlyMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := c.Get("role")
			if role == nil || role.(string) != auth.RoleAdmin {
				return echo.NewHTTPError(http.StatusForbidden, "Admin access required")
			}
			return next(c)
		}
	}
}

// GetCurrentUser returns the current authenticated user from context
func GetCurrentUser(c echo.Context) (userID, username, role string, authenticated bool) {
	if val := c.Get("user_id"); val != nil {
		userID = val.(string)
	}
	if val := c.Get("username"); val != nil {
		username = val.(string)
	}
	if val := c.Get("role"); val != nil {
		role = val.(string)
	}
	if val := c.Get("authenticated"); val != nil {
		authenticated = val.(bool)
	}
	return
}
