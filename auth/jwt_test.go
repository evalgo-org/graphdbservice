package auth

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	user := User{
		ID:       "test-user-id",
		Username: "testuser",
		Role:     RoleAdmin,
	}

	secret := "test-secret-key-for-jwt-signing"

	tests := []struct {
		name            string
		user            User
		secret          string
		expirationHours int
		wantErr         bool
	}{
		{
			name:            "Valid token generation",
			user:            user,
			secret:          secret,
			expirationHours: 1,
			wantErr:         false,
		},
		{
			name:            "Empty secret",
			user:            user,
			secret:          "",
			expirationHours: 1,
			wantErr:         true,
		},
		{
			name:            "Zero expiration",
			user:            user,
			secret:          secret,
			expirationHours: 0,
			wantErr:         false, // Still generates token, just expires immediately
		},
		{
			name:            "Long expiration",
			user:            user,
			secret:          secret,
			expirationHours: 24 * 365, // 1 year
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateToken(tt.user, tt.secret, tt.expirationHours)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && token == "" {
				t.Error("GenerateToken() returned empty token")
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	user := User{
		ID:       "test-user-id",
		Username: "testuser",
		Role:     RoleAdmin,
	}

	secret := "test-secret-key-for-jwt-signing"

	// Generate a valid token
	validToken, err := GenerateToken(user, secret, 1)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Generate an expired token
	expiredUser := user
	expiredToken, err := GenerateToken(expiredUser, secret, -1) // Negative hours = already expired
	if err != nil {
		t.Fatalf("Failed to generate expired token: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		secret  string
		wantErr bool
		checkID bool
	}{
		{
			name:    "Valid token",
			token:   validToken,
			secret:  secret,
			wantErr: false,
			checkID: true,
		},
		{
			name:    "Invalid token string",
			token:   "invalid.token.string",
			secret:  secret,
			wantErr: true,
		},
		{
			name:    "Wrong secret",
			token:   validToken,
			secret:  "wrong-secret",
			wantErr: true,
		},
		{
			name:    "Empty secret",
			token:   validToken,
			secret:  "",
			wantErr: true,
		},
		{
			name:    "Empty token",
			token:   "",
			secret:  secret,
			wantErr: true,
		},
		{
			name:    "Expired token",
			token:   expiredToken,
			secret:  secret,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ValidateToken(tt.token, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if claims == nil {
					t.Error("ValidateToken() returned nil claims")
					return
				}
				if tt.checkID && claims.UserID != user.ID {
					t.Errorf("ValidateToken() UserID = %v, want %v", claims.UserID, user.ID)
				}
				if tt.checkID && claims.Username != user.Username {
					t.Errorf("ValidateToken() Username = %v, want %v", claims.Username, user.Username)
				}
				if tt.checkID && claims.Role != user.Role {
					t.Errorf("ValidateToken() Role = %v, want %v", claims.Role, user.Role)
				}
			}
		})
	}
}

func TestTokenRoundTrip(t *testing.T) {
	user := User{
		ID:       "round-trip-user",
		Username: "roundtripuser",
		Role:     RoleUser,
		Email:    "test@example.com",
	}

	secret := "test-secret-key-for-round-trip"
	expirationHours := 2

	// Generate token
	token, err := GenerateToken(user, secret, expirationHours)
	if err != nil {
		t.Fatalf("GenerateToken() failed: %v", err)
	}

	// Validate token
	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken() failed: %v", err)
	}

	// Verify all fields
	if claims.UserID != user.ID {
		t.Errorf("UserID mismatch: got %v, want %v", claims.UserID, user.ID)
	}
	if claims.Username != user.Username {
		t.Errorf("Username mismatch: got %v, want %v", claims.Username, user.Username)
	}
	if claims.Role != user.Role {
		t.Errorf("Role mismatch: got %v, want %v", claims.Role, user.Role)
	}

	// Verify expiration is in the future
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
		t.Error("Token expiration is in the past or nil")
	}

	// Verify issuer
	if claims.Issuer != "graphservice" {
		t.Errorf("Issuer mismatch: got %v, want graphservice", claims.Issuer)
	}
}

func TestTokenExpiration(t *testing.T) {
	user := User{
		ID:       "expiration-test-user",
		Username: "expirationuser",
		Role:     RoleAdmin,
	}

	secret := "test-secret-for-expiration"

	// Generate token that expires in negative time (already expired)
	expiredToken, err := GenerateToken(user, secret, -1)
	if err != nil {
		t.Fatalf("GenerateToken() failed: %v", err)
	}

	// Try to validate expired token
	_, err = ValidateToken(expiredToken, secret)
	if err == nil {
		t.Error("ValidateToken() should fail for expired token")
	}
}
