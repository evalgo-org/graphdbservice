package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"evalgo.org/graphservice/auth"
)

var (
	// userStore is the global user storage instance
	userStore *auth.UserStore
)

// InitializeAuth initializes the authentication system
func InitializeAuth() error {
	mode := getAuthMode()

	// If auth mode is "none", skip initialization
	if mode == auth.AuthModeNone {
		fmt.Println("Authentication: DISABLED (AUTH_MODE=none)")
		return nil
	}

	fmt.Printf("Authentication: ENABLED (AUTH_MODE=%s)\n", mode)

	// Get data directory
	dataDir := getEnv("DATA_DIR", "./data")

	// Initialize user store
	var err error
	userStore, err = auth.NewUserStore(dataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize user store: %w", err)
	}

	// Check if any users exist
	count, err := userStore.CountUsers()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	// If no users exist, create initial admin user
	if count == 0 {
		if err := createInitialAdmin(); err != nil {
			return fmt.Errorf("failed to create initial admin: %w", err)
		}
	}

	return nil
}

// createInitialAdmin creates the initial admin user with a random password
func createInitialAdmin() error {
	fmt.Println("\n" + strings.Repeat("━", 60))
	fmt.Println("  CREATING INITIAL ADMIN USER")
	fmt.Println(strings.Repeat("━", 60))

	// Generate random password
	password := generateRandomPassword(16)

	// Create admin user
	user, err := userStore.CreateUser("admin", password, "", auth.RoleAdmin)
	if err != nil {
		return err
	}

	// Mark that password must be changed
	err = userStore.UpdateUser("admin", map[string]interface{}{
		"must_change_password": true,
	})
	if err != nil {
		return err
	}

	fmt.Println("\n  Initial admin credentials created successfully!")
	fmt.Println()
	fmt.Println("  Username: admin")
	fmt.Printf("  Password: %s\n", password)
	fmt.Println()
	fmt.Println("  ⚠️  IMPORTANT:")
	fmt.Println("  - Save these credentials NOW")
	fmt.Println("  - Change the password after first login")
	fmt.Println("  - These credentials will not be shown again")
	fmt.Println()
	fmt.Printf("  User ID: %s\n", user.ID)
	fmt.Println(strings.Repeat("━", 60) + "\n")

	return nil
}

// generateRandomPassword generates a secure random password
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"

	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to less secure method
		return "ChangeMe123!"
	}

	password := make([]byte, length)
	for i := range password {
		password[i] = charset[int(b[i])%len(charset)]
	}

	return string(password)
}

// Helper functions for environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// generateJWTSecret generates a random JWT secret
func generateJWTSecret() string {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate JWT secret")
	}
	return base64.StdEncoding.EncodeToString(b)
}
