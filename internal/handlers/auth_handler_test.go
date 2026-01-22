package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cirvee/referral-backend/internal/config"
	"github.com/cirvee/referral-backend/internal/database"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/services"
	"github.com/cirvee/referral-backend/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock database for testing
type mockDB struct{}

func setupAuthHandler(t *testing.T) (*AuthHandler, *services.AuthService, *utils.JWTManager) {
	jwtManager := utils.NewJWTManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	// For unit tests, we'll use a mock approach
	// In a real scenario, you'd use sqlmock or testcontainers
	return nil, nil, jwtManager
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	handler := &AuthHandler{validate: nil}

	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Register_MissingFields(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	tests := []struct {
		name    string
		payload map[string]interface{}
	}{
		{"missing_email", map[string]interface{}{"password": "test123", "name": "Test", "phone": "123"}},
		{"missing_password", map[string]interface{}{"email": "test@test.com", "name": "Test", "phone": "123"}},
		{"missing_name", map[string]interface{}{"email": "test@test.com", "password": "test123", "phone": "123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Register(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestAuthHandler_Register_InvalidEmail(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	payload := models.RegisterRequest{
		Email:    "not-an-email",
		Password: "password123",
		Name:     "Test User",
		Phone:    "08012345678",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Register_WeakPassword(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	payload := models.RegisterRequest{
		Email:    "test@example.com",
		Password: "123", // Too short
		Name:     "Test User",
		Phone:    "08012345678",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Login_MissingCredentials(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	tests := []struct {
		name    string
		payload map[string]interface{}
	}{
		{"missing_email", map[string]interface{}{"password": "test123"}},
		{"missing_password", map[string]interface{}{"email": "test@test.com"}},
		{"empty_email", map[string]interface{}{"email": "", "password": "test123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Login(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestAuthHandler_RefreshToken_InvalidJSON(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_RefreshToken_MissingToken(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)

	payload := map[string]interface{}{}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.RefreshToken(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// Integration test helper
func setupTestDB(t *testing.T) (*database.DB, func()) {
	dbURL := "postgres://referral_test:referral_test_secret@localhost:5435/referral_test_db?sslmode=disable"

	db, err := database.New(dbURL, 10)
	if err != nil {
		t.Skipf("Skipping integration test - database not available: %v", err)
	}

	// Clear tables
	ctx := context.Background()
	db.Pool.Exec(ctx, "TRUNCATE users, referrals, payouts CASCADE")

	cleanup := func() {
		db.Pool.Exec(ctx, "TRUNCATE users, referrals, payouts CASCADE")
		db.Close()
	}

	return db, cleanup
}

func TestAuthHandler_Register_Integration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	jwtManager := utils.NewJWTManager("test-secret", "test-refresh", 15*time.Minute, 168*time.Hour)
	userRepo := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, jwtManager)
	emailService := services.NewEmailService(&config.SMTPConfig{})
	handler := NewAuthHandler(authService, emailService, userRepo, nil)

	payload := models.RegisterRequest{
		Email:    "integration@test.com",
		Password: "password123",
		Name:     "Integration Test",
		Phone:    "08012345678",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var response models.AuthResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
	assert.Equal(t, payload.Email, response.User.Email)
	assert.NotEmpty(t, response.User.ReferralCode)
}

func TestAuthHandler_Login_Integration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	jwtManager := utils.NewJWTManager("test-secret", "test-refresh", 15*time.Minute, 168*time.Hour)
	userRepo := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, jwtManager)
	emailService := services.NewEmailService(&config.SMTPConfig{})
	handler := NewAuthHandler(authService, emailService, userRepo, nil)

	// First register a user
	registerPayload := models.RegisterRequest{
		Email:    "login@test.com",
		Password: "password123",
		Name:     "Login Test",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Register(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Test successful login
	loginPayload := models.LoginRequest{
		Email:    "login@test.com",
		Password: "password123",
	}
	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()

	handler.Login(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var response models.AuthResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.AccessToken)
}

func TestAuthHandler_Login_WrongPassword_Integration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	jwtManager := utils.NewJWTManager("test-secret", "test-refresh", 15*time.Minute, 168*time.Hour)
	userRepo := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, jwtManager)
	emailService := services.NewEmailService(&config.SMTPConfig{})
	handler := NewAuthHandler(authService, emailService, userRepo, nil)

	// Register user
	registerPayload := models.RegisterRequest{
		Email:    "wrongpass@test.com",
		Password: "password123",
		Name:     "Wrong Pass Test",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Register(rr, req)

	// Try login with wrong password
	loginPayload := models.LoginRequest{
		Email:    "wrongpass@test.com",
		Password: "wrongpassword",
	}
	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()

	handler.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthHandler_DuplicateEmail_Integration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	jwtManager := utils.NewJWTManager("test-secret", "test-refresh", 15*time.Minute, 168*time.Hour)
	userRepo := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, jwtManager)
	emailService := services.NewEmailService(&config.SMTPConfig{})
	handler := NewAuthHandler(authService, emailService, userRepo, nil)

	payload := models.RegisterRequest{
		Email:    "duplicate@test.com",
		Password: "password123",
		Name:     "Duplicate Test",
		Phone:    "08012345678",
	}

	// First registration succeeds
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Register(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Second registration with same email fails
	req = httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.Register(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}
