package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewUserStore(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	if store == nil {
		t.Fatal("NewUserStore() returned nil store")
	}

	// Check that users directory was created
	usersDir := filepath.Join(tempDir, "users")
	if _, err := os.Stat(usersDir); os.IsNotExist(err) {
		t.Error("NewUserStore() did not create users directory")
	}
}

func TestUserStore_LoadEmpty(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	db, err := store.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if db == nil {
		t.Fatal("Load() returned nil database")
	}

	if db.Version != SchemaVersion {
		t.Errorf("Load() version = %v, want %v", db.Version, SchemaVersion)
	}

	if len(db.Users) != 0 {
		t.Errorf("Load() users count = %v, want 0", len(db.Users))
	}
}

func TestUserStore_CreateUser(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	tests := []struct {
		name     string
		username string
		password string
		email    string
		role     string
		wantErr  bool
	}{
		{
			name:     "Valid admin user",
			username: "admin",
			password: "AdminPass123!",
			email:    "admin@example.com",
			role:     RoleAdmin,
			wantErr:  false,
		},
		{
			name:     "Valid regular user",
			username: "user1",
			password: "UserPass123!",
			email:    "user1@example.com",
			role:     RoleUser,
			wantErr:  false,
		},
		{
			name:     "Duplicate username",
			username: "admin",
			password: "AnotherPass123!",
			email:    "another@example.com",
			role:     RoleAdmin,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := store.CreateUser(tt.username, tt.password, tt.email, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if user == nil {
					t.Fatal("CreateUser() returned nil user")
				}
				if user.Username != tt.username {
					t.Errorf("CreateUser() username = %v, want %v", user.Username, tt.username)
				}
				if user.Email != tt.email {
					t.Errorf("CreateUser() email = %v, want %v", user.Email, tt.email)
				}
				if user.Role != tt.role {
					t.Errorf("CreateUser() role = %v, want %v", user.Role, tt.role)
				}
				if user.ID == "" {
					t.Error("CreateUser() did not generate ID")
				}
				if user.PasswordHash == "" || user.PasswordHash == tt.password {
					t.Error("CreateUser() did not hash password")
				}
			}
		})
	}
}

func TestUserStore_GetUser(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	// Create a test user
	username := "testuser"
	password := "TestPass123!"
	_, err = store.CreateUser(username, password, "test@example.com", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "Existing user",
			username: username,
			wantErr:  false,
		},
		{
			name:     "Non-existent user",
			username: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := store.GetUser(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && user == nil {
				t.Error("GetUser() returned nil user")
			}
			if !tt.wantErr && user.Username != tt.username {
				t.Errorf("GetUser() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}

func TestUserStore_UpdateUser(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	// Create a test user
	username := "updatetest"
	_, err = store.CreateUser(username, "OldPass123!", "old@example.com", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	tests := []struct {
		name     string
		username string
		updates  map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "Update email",
			username: username,
			updates: map[string]interface{}{
				"email": "new@example.com",
			},
			wantErr: false,
		},
		{
			name:     "Update role",
			username: username,
			updates: map[string]interface{}{
				"role": RoleAdmin,
			},
			wantErr: false,
		},
		{
			name:     "Lock account",
			username: username,
			updates: map[string]interface{}{
				"locked": true,
			},
			wantErr: false,
		},
		{
			name:     "Update password",
			username: username,
			updates: map[string]interface{}{
				"password": "NewPass123!",
			},
			wantErr: false,
		},
		{
			name:     "Non-existent user",
			username: "nonexistent",
			updates: map[string]interface{}{
				"email": "test@example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.UpdateUser(tt.username, tt.updates)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the update
				user, err := store.GetUser(tt.username)
				if err != nil {
					t.Fatalf("GetUser() failed: %v", err)
				}

				if email, ok := tt.updates["email"].(string); ok && user.Email != email {
					t.Errorf("Email not updated: got %v, want %v", user.Email, email)
				}
				if role, ok := tt.updates["role"].(string); ok && user.Role != role {
					t.Errorf("Role not updated: got %v, want %v", user.Role, role)
				}
				if locked, ok := tt.updates["locked"].(bool); ok && user.Locked != locked {
					t.Errorf("Locked not updated: got %v, want %v", user.Locked, locked)
				}
				if password, ok := tt.updates["password"].(string); ok {
					if !CheckPasswordHash(password, user.PasswordHash) {
						t.Error("Password not updated correctly")
					}
				}
			}
		})
	}
}

func TestUserStore_DeleteUser(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	// Create a test user
	username := "deletetest"
	_, err = store.CreateUser(username, "TestPass123!", "delete@example.com", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	// Delete the user
	err = store.DeleteUser(username)
	if err != nil {
		t.Fatalf("DeleteUser() failed: %v", err)
	}

	// Verify user is deleted
	_, err = store.GetUser(username)
	if err == nil {
		t.Error("GetUser() should fail after DeleteUser()")
	}

	// Try to delete non-existent user
	err = store.DeleteUser("nonexistent")
	if err == nil {
		t.Error("DeleteUser() should fail for non-existent user")
	}
}

func TestUserStore_RecordLoginAttempt(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	username := "logintest"
	_, err = store.CreateUser(username, "TestPass123!", "login@example.com", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	// Test successful login
	err = store.RecordLoginAttempt(username, true)
	if err != nil {
		t.Fatalf("RecordLoginAttempt(true) failed: %v", err)
	}

	user, _ := store.GetUser(username)
	if user.FailedLogins != 0 {
		t.Errorf("FailedLogins = %v after successful login, want 0", user.FailedLogins)
	}
	if user.LastLoginAt == nil {
		t.Error("LastLoginAt not set after successful login")
	}

	// Test failed login attempts
	for i := 1; i <= 5; i++ {
		err = store.RecordLoginAttempt(username, false)
		if err != nil {
			t.Fatalf("RecordLoginAttempt(false) failed: %v", err)
		}

		user, _ = store.GetUser(username)
		if user.FailedLogins != i {
			t.Errorf("FailedLogins = %v after %d failed attempts, want %d", user.FailedLogins, i, i)
		}
	}

	// After 5 failed attempts, account should be locked
	user, _ = store.GetUser(username)
	if !user.Locked {
		t.Error("Account not locked after 5 failed attempts")
	}
}

func TestUserStore_ListUsers(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	// Initially empty
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() failed: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("ListUsers() count = %v, want 0", len(users))
	}

	// Create some users
	_, _ = store.CreateUser("user1", "Pass123!", "user1@example.com", RoleUser)
	_, _ = store.CreateUser("user2", "Pass123!", "user2@example.com", RoleUser)
	_, _ = store.CreateUser("admin", "Pass123!", "admin@example.com", RoleAdmin)

	// List users
	users, err = store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() failed: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("ListUsers() count = %v, want 3", len(users))
	}
}

func TestUserStore_CountUsers(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	// Initially 0
	count, err := store.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers() failed: %v", err)
	}
	if count != 0 {
		t.Errorf("CountUsers() = %v, want 0", count)
	}

	// Create users
	_, _ = store.CreateUser("user1", "Pass123!", "", RoleUser)
	_, _ = store.CreateUser("user2", "Pass123!", "", RoleUser)

	count, err = store.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers() failed: %v", err)
	}
	if count != 2 {
		t.Errorf("CountUsers() = %v, want 2", count)
	}
}

func TestUserStore_Persistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create store and add user
	store1, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() failed: %v", err)
	}

	username := "persistent"
	password := "TestPass123!"
	_, err = store1.CreateUser(username, password, "test@example.com", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	// Create new store instance (simulating restart)
	store2, err := NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("NewUserStore() #2 failed: %v", err)
	}

	// Verify user persisted
	user, err := store2.GetUser(username)
	if err != nil {
		t.Fatalf("GetUser() after restart failed: %v", err)
	}

	if user.Username != username {
		t.Errorf("Username = %v, want %v", user.Username, username)
	}

	// Verify password still works
	if !CheckPasswordHash(password, user.PasswordHash) {
		t.Error("Password verification failed after restart")
	}
}
