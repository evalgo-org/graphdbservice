package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
)

const (
	SchemaVersion = "1.0.0"
)

// UserStore manages user persistence to filesystem
type UserStore struct {
	dataDir  string
	lockFile *flock.Flock
}

// NewUserStore creates a new user store
func NewUserStore(dataDir string) (*UserStore, error) {
	// Create data directory if it doesn't exist
	usersDir := filepath.Join(dataDir, "users")
	if err := os.MkdirAll(usersDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create users directory: %w", err)
	}

	store := &UserStore{
		dataDir:  dataDir,
		lockFile: flock.New(filepath.Join(usersDir, ".users.lock")),
	}

	return store, nil
}

// getUsersFilePath returns the path to the users.json file
func (s *UserStore) getUsersFilePath() string {
	return filepath.Join(s.dataDir, "users", "users.json")
}

// Load loads the user database from disk
func (s *UserStore) Load() (*UserDatabase, error) {
	filePath := s.getUsersFilePath()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Return empty database
		return &UserDatabase{
			Version:   SchemaVersion,
			Users:     make(map[string]User),
			UpdatedAt: time.Now(),
		}, nil
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read users file: %w", err)
	}

	// Parse JSON
	var db UserDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("failed to parse users file: %w", err)
	}

	return &db, nil
}

// Save saves the user database to disk with file locking
func (s *UserStore) Save(db *UserDatabase) error {
	filePath := s.getUsersFilePath()

	// Acquire file lock
	locked, err := s.lockFile.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return errors.New("unable to acquire lock - another process is writing")
	}
	defer s.lockFile.Unlock()

	// Create backup of existing file
	if _, err := os.Stat(filePath); err == nil {
		backupPath := filePath + ".backup"
		data, err := os.ReadFile(filePath)
		if err == nil {
			_ = os.WriteFile(backupPath, data, 0600)
		}
	}

	// Update timestamp
	db.UpdatedAt = time.Now()

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal database: %w", err)
	}

	// Write atomically (write to temp, then rename)
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, filePath); err != nil {
		_ = os.Remove(tempFile) // Cleanup temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// GetUser retrieves a user by username
func (s *UserStore) GetUser(username string) (*User, error) {
	db, err := s.Load()
	if err != nil {
		return nil, err
	}

	user, exists := db.Users[username]
	if !exists {
		return nil, errors.New("user not found")
	}

	return &user, nil
}

// CreateUser creates a new user
func (s *UserStore) CreateUser(username, password, email, role string) (*User, error) {
	db, err := s.Load()
	if err != nil {
		return nil, err
	}

	// Check if user already exists
	if _, exists := db.Users[username]; exists {
		return nil, errors.New("user already exists")
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := User{
		ID:                 uuid.New().String(),
		Username:           username,
		Email:              email,
		PasswordHash:       passwordHash,
		Role:               role,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		FailedLogins:       0,
		Locked:             false,
		MustChangePassword: false,
	}

	// Add to database
	db.Users[username] = user

	// Save
	if err := s.Save(db); err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (s *UserStore) UpdateUser(username string, updates map[string]interface{}) error {
	db, err := s.Load()
	if err != nil {
		return err
	}

	user, exists := db.Users[username]
	if !exists {
		return errors.New("user not found")
	}

	// Apply updates
	if email, ok := updates["email"].(string); ok {
		user.Email = email
	}
	if role, ok := updates["role"].(string); ok {
		user.Role = role
	}
	if locked, ok := updates["locked"].(bool); ok {
		user.Locked = locked
	}
	if mustChange, ok := updates["must_change_password"].(bool); ok {
		user.MustChangePassword = mustChange
	}
	if password, ok := updates["password"].(string); ok {
		hash, err := HashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = hash
	}

	user.UpdatedAt = time.Now()
	db.Users[username] = user

	return s.Save(db)
}

// DeleteUser deletes a user
func (s *UserStore) DeleteUser(username string) error {
	db, err := s.Load()
	if err != nil {
		return err
	}

	if _, exists := db.Users[username]; !exists {
		return errors.New("user not found")
	}

	delete(db.Users, username)
	return s.Save(db)
}

// RecordLoginAttempt records a failed login attempt
func (s *UserStore) RecordLoginAttempt(username string, success bool) error {
	db, err := s.Load()
	if err != nil {
		return err
	}

	user, exists := db.Users[username]
	if !exists {
		return errors.New("user not found")
	}

	if success {
		// Reset failed logins on successful login
		user.FailedLogins = 0
		now := time.Now()
		user.LastLoginAt = &now
	} else {
		// Increment failed logins
		user.FailedLogins++
		// Lock account after 5 failed attempts
		if user.FailedLogins >= 5 {
			user.Locked = true
		}
	}

	user.UpdatedAt = time.Now()
	db.Users[username] = user

	return s.Save(db)
}

// ListUsers returns all users
func (s *UserStore) ListUsers() ([]User, error) {
	db, err := s.Load()
	if err != nil {
		return nil, err
	}

	users := make([]User, 0, len(db.Users))
	for _, user := range db.Users {
		users = append(users, user)
	}

	return users, nil
}

// CountUsers returns the number of users
func (s *UserStore) CountUsers() (int, error) {
	db, err := s.Load()
	if err != nil {
		return 0, err
	}

	return len(db.Users), nil
}
