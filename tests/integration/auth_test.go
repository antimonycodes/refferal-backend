package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cirvee/referral-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegister tests the registration endpoint
func TestRegister(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		payload    models.RegisterRequest
		wantStatus int
		wantError  bool
	}{
		{
			name: "valid_registration",
			payload: models.RegisterRequest{
				Email:         "test@example.com",
				Password:      "password123",
				Name:          "Test User",
				Phone:         "08012345678",
				BankName:      "GTBank",
				AccountNumber: "0123456789",
				AccountName:   "Test User",
			},
			wantStatus: http.StatusCreated,
			wantError:  false,
		},
		{
			name: "invalid_email",
			payload: models.RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
				Name:     "Test User",
				Phone:    "08012345678",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "weak_password",
			payload: models.RegisterRequest{
				Email:    "test2@example.com",
				Password: "123",
				Name:     "Test User",
				Phone:    "08012345678",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "missing_name",
			payload: models.RegisterRequest{
				Email:    "test3@example.com",
				Password: "password123",
				Phone:    "08012345678",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if !tt.wantError {
				var response models.AuthResponse
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.AccessToken)
				assert.NotEmpty(t, response.RefreshToken)
				assert.Equal(t, tt.payload.Email, response.User.Email)
			}
		})
	}
}

// TestRegisterDuplicateEmail tests duplicate email detection
func TestRegisterDuplicateEmail(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	payload := models.RegisterRequest{
		Email:    "duplicate@example.com",
		Password: "password123",
		Name:     "Test User",
		Phone:    "08012345678",
	}

	// First registration should succeed
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Second registration with same email should fail
	req = httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestLogin tests the login endpoint
func TestLogin(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// First register a user
	registerPayload := models.RegisterRequest{
		Email:    "login@example.com",
		Password: "password123",
		Name:     "Login User",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	tests := []struct {
		name       string
		payload    models.LoginRequest
		wantStatus int
	}{
		{
			name: "valid_login",
			payload: models.LoginRequest{
				Email:    "login@example.com",
				Password: "password123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "wrong_password",
			payload: models.LoginRequest{
				Email:    "login@example.com",
				Password: "wrongpassword",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "non_existent_user",
			payload: models.LoginRequest{
				Email:    "nonexistent@example.com",
				Password: "password123",
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

// TestProtectedRoutes tests that protected routes require authentication
func TestProtectedRoutes(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/dashboard"},
		{"GET", "/api/v1/admin/referrals"},
		{"GET", "/api/v1/admin/students"},
		{"GET", "/api/v1/admin/payouts"},
		{"GET", "/api/v1/user/dashboard"},
		{"GET", "/api/v1/user/referrals"},
		{"GET", "/api/v1/user/profile"},
	}

	for _, route := range routes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}
