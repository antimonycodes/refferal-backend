package utils

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTManager_GenerateAccessToken(t *testing.T) {
	jm := NewJWTManager("test-access-secret", "test-refresh-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, err := jm.GenerateAccessToken(userID, "test@example.com", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateAccessToken() returned empty token")
	}
}

func TestJWTManager_ValidateAccessToken(t *testing.T) {
	jm := NewJWTManager("test-access-secret", "test-refresh-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()
	email := "test@example.com"
	role := "admin"

	token, _ := jm.GenerateAccessToken(userID, email, role)

	claims, err := jm.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Email != email {
		t.Errorf("Email = %v, want %v", claims.Email, email)
	}
	if claims.Role != role {
		t.Errorf("Role = %v, want %v", claims.Role, role)
	}
	if claims.Type != AccessToken {
		t.Errorf("Type = %v, want %v", claims.Type, AccessToken)
	}
}

func TestJWTManager_ValidateRefreshToken(t *testing.T) {
	jm := NewJWTManager("test-access-secret", "test-refresh-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, _ := jm.GenerateRefreshToken(userID, "test@example.com", "user")

	claims, err := jm.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}

	if claims.Type != RefreshToken {
		t.Errorf("Type = %v, want %v", claims.Type, RefreshToken)
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	jm := NewJWTManager("test-access-secret", "test-refresh-secret", 15*time.Minute, 7*24*time.Hour)

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"random string", "not-a-valid-token"},
		{"malformed jwt", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jm.ValidateAccessToken(tt.token)
			if err == nil {
				t.Error("ValidateAccessToken() should return error for invalid token")
			}
		})
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	// Create manager with very short expiry
	jm := NewJWTManager("test-access-secret", "test-refresh-secret", 1*time.Millisecond, 7*24*time.Hour)
	userID := uuid.New()

	token, _ := jm.GenerateAccessToken(userID, "test@example.com", "user")

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err := jm.ValidateAccessToken(token)
	if err != ErrExpiredToken {
		t.Errorf("ValidateAccessToken() error = %v, want ErrExpiredToken", err)
	}
}

func TestJWTManager_WrongTokenType(t *testing.T) {
	jm := NewJWTManager("test-access-secret", "test-refresh-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	// Generate refresh token but try to validate as access token
	refreshToken, _ := jm.GenerateRefreshToken(userID, "test@example.com", "user")
	
	_, err := jm.ValidateAccessToken(refreshToken)
	if err != ErrInvalidToken {
		t.Errorf("ValidateAccessToken() with refresh token should return ErrInvalidToken, got %v", err)
	}

	// Generate access token but try to validate as refresh token
	accessToken, _ := jm.GenerateAccessToken(userID, "test@example.com", "user")
	
	_, err = jm.ValidateRefreshToken(accessToken)
	if err != ErrInvalidToken {
		t.Errorf("ValidateRefreshToken() with access token should return ErrInvalidToken, got %v", err)
	}
}

func TestJWTManager_WrongSecret(t *testing.T) {
	jm1 := NewJWTManager("secret-1", "refresh-1", 15*time.Minute, 7*24*time.Hour)
	jm2 := NewJWTManager("secret-2", "refresh-2", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, _ := jm1.GenerateAccessToken(userID, "test@example.com", "user")
	
	_, err := jm2.ValidateAccessToken(token)
	if err != ErrInvalidToken {
		t.Errorf("ValidateAccessToken() with wrong secret should return ErrInvalidToken, got %v", err)
	}
}
