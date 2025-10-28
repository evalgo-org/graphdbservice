package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User represents a user in the system
type User struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email,omitempty"`
	PasswordHash       string     `json:"password_hash"`
	Role               string     `json:"role"` // "admin" or "user"
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	FailedLogins       int        `json:"failed_logins"`
	Locked             bool       `json:"locked"`
	MustChangePassword bool       `json:"must_change_password"`
}

// UserDatabase represents the user storage structure
type UserDatabase struct {
	Version   string          `json:"version"`
	Users     map[string]User `json:"users"` // Key: username
	UpdatedAt time.Time       `json:"updated_at"`
}

// Claims represents JWT token claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Session represents an active user session (optional tracking)
type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastActivity time.Time `json:"last_activity"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
}

// SessionStore represents the session storage
type SessionStore struct {
	Sessions  map[string]Session `json:"sessions"` // Key: session ID
	UpdatedAt time.Time          `json:"updated_at"`
}

// AuthMode represents the authentication mode
type AuthMode string

const (
	AuthModeNone   AuthMode = "none"   // No authentication
	AuthModeSimple AuthMode = "simple" // Simple authentication (all users equal)
	AuthModeRBAC   AuthMode = "rbac"   // Role-based access control
)

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)
