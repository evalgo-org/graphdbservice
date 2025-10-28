package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "Valid password",
			password: "SecurePass123!",
			wantErr:  false,
		},
		{
			name:     "Short password",
			password: "abc",
			wantErr:  false, // Hashing succeeds, validation is separate
		},
		{
			name:     "Empty password",
			password: "",
			wantErr:  false, // Hashing succeeds, validation is separate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hash == "" {
				t.Error("HashPassword() returned empty hash")
			}
			if !tt.wantErr && hash == tt.password {
				t.Error("HashPassword() returned plaintext password")
			}
		})
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "TestPassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "Correct password",
			password: password,
			hash:     hash,
			want:     true,
		},
		{
			name:     "Wrong password",
			password: "WrongPassword",
			hash:     hash,
			want:     false,
		},
		{
			name:     "Empty password",
			password: "",
			hash:     hash,
			want:     false,
		},
		{
			name:     "Invalid hash",
			password: password,
			hash:     "invalid-hash",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPasswordHash(tt.password, tt.hash); got != tt.want {
				t.Errorf("CheckPasswordHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name          string
		password      string
		minLength     int
		requireStrong bool
		wantErr       bool
		errContains   string
	}{
		{
			name:          "Valid strong password",
			password:      "SecurePass123!",
			minLength:     8,
			requireStrong: true,
			wantErr:       false,
		},
		{
			name:          "Too short",
			password:      "Abc1!",
			minLength:     8,
			requireStrong: true,
			wantErr:       true,
			errContains:   "too short",
		},
		{
			name:          "No uppercase",
			password:      "securepass123!",
			minLength:     8,
			requireStrong: true,
			wantErr:       true,
			errContains:   "uppercase",
		},
		{
			name:          "No lowercase",
			password:      "SECUREPASS123!",
			minLength:     8,
			requireStrong: true,
			wantErr:       true,
			errContains:   "lowercase",
		},
		{
			name:          "No number",
			password:      "SecurePass!",
			minLength:     8,
			requireStrong: true,
			wantErr:       true,
			errContains:   "number",
		},
		{
			name:          "No special character",
			password:      "SecurePass123",
			minLength:     8,
			requireStrong: true,
			wantErr:       true,
			errContains:   "special character",
		},
		{
			name:          "Weak password allowed when not required",
			password:      "simplepass",
			minLength:     8,
			requireStrong: false,
			wantErr:       false,
		},
		{
			name:          "Too short even when weak allowed",
			password:      "abc",
			minLength:     8,
			requireStrong: false,
			wantErr:       true,
			errContains:   "too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password, tt.minLength, tt.requireStrong)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePassword() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "Valid username",
			username: "admin",
			wantErr:  false,
		},
		{
			name:     "Valid with underscore",
			username: "admin_user",
			wantErr:  false,
		},
		{
			name:     "Valid with hyphen",
			username: "admin-user",
			wantErr:  false,
		},
		{
			name:     "Valid with numbers",
			username: "admin123",
			wantErr:  false,
		},
		{
			name:        "Too short",
			username:    "ab",
			wantErr:     true,
			errContains: "at least 3",
		},
		{
			name:        "Too long",
			username:    "a" + string(make([]byte, 50)),
			wantErr:     true,
			errContains: "at most 50",
		},
		{
			name:        "Invalid characters - space",
			username:    "admin user",
			wantErr:     true,
			errContains: "letters, numbers",
		},
		{
			name:        "Invalid characters - special",
			username:    "admin@user",
			wantErr:     true,
			errContains: "letters, numbers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateUsername() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
