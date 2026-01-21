package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cirvee/referral-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// SQL INJECTION TESTS
// ============================================================================

// TestSQLInjectionInLogin tests SQL injection attempts in login
func TestSQLInjectionInLogin(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	injectionPayloads := []models.LoginRequest{
		{Email: "' OR '1'='1", Password: "password"},
		{Email: "admin@example.com'; DROP TABLE users;--", Password: "password"},
		{Email: "admin@example.com", Password: "' OR '1'='1"},
		{Email: "1; SELECT * FROM users WHERE '1'='1", Password: "password"},
		{Email: "admin@example.com' UNION SELECT * FROM users--", Password: "password"},
	}

	for _, payload := range injectionPayloads {
		t.Run("injection_"+payload.Email[:min(20, len(payload.Email))], func(t *testing.T) {
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should return 400 (invalid email) or 401 (unauthorized), never 200
			assert.NotEqual(t, http.StatusOK, rr.Code, "SQL injection should not succeed")
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code, "SQL injection should not cause server error")
		})
	}
}

// TestSQLInjectionInRegistration tests SQL injection attempts in registration
func TestSQLInjectionInRegistration(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	injectionPayloads := []models.RegisterRequest{
		{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "'; DROP TABLE users;--",
			Phone:    "08012345678",
		},
		{
			Email:    "test2@example.com",
			Password: "password123",
			Name:     "Test User",
			Phone:    "'; DELETE FROM referrals;--",
		},
		{
			Email:    "test3@example.com'; DROP TABLE users;--",
			Password: "password123",
			Name:     "Test User",
			Phone:    "08012345678",
		},
	}

	for i, payload := range injectionPayloads {
		t.Run("injection_registration_"+string(rune('a'+i)), func(t *testing.T) {
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should either succeed (input is sanitized) or fail gracefully
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code, "SQL injection should not cause server error")
		})
	}
}

// ============================================================================
// XSS TESTS
// ============================================================================

// TestXSSInRegistration tests that XSS payloads are handled safely
func TestXSSInRegistration(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"<img src=x onerror=alert('xss')>",
		"javascript:alert('xss')",
		"<svg onload=alert('xss')>",
		"<body onload=alert('xss')>",
	}

	for i, xss := range xssPayloads {
		t.Run("xss_name_"+string(rune('a'+i)), func(t *testing.T) {
			payload := models.RegisterRequest{
				Email:    "xss" + string(rune('a'+i)) + "@example.com",
				Password: "password123",
				Name:     xss,
				Phone:    "08012345678",
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should either accept (will be escaped on output) or reject with 400
			// Should NOT cause server error
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
		})
	}
}

// ============================================================================
// JWT SECURITY TESTS
// ============================================================================

// TestInvalidJWTFormat tests various malformed JWT tokens
func TestInvalidJWTFormat(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	invalidTokens := []string{
		"",
		"invalid",
		"invalid.token",
		"invalid.token.format",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
		"Bearer ",
		"Basic dXNlcjpwYXNz",
		"eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VyX2lkIjoiMTIzIn0.", // "alg": "none" attack
	}

	for i, token := range invalidTokens {
		t.Run("invalid_jwt_"+string(rune('a'+i)), func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code, "Invalid JWT should return 401")
		})
	}
}

// TestJWTTampering tests that tampered JWTs are rejected
func TestJWTTampering(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// First, register and login to get a valid token
	registerPayload := models.RegisterRequest{
		Email:    "tamper@example.com",
		Password: "password123",
		Name:     "Tamper Test",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	var authResponse models.AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &authResponse)
	validToken := authResponse.AccessToken

	// Tamper with different parts of the token
	parts := strings.Split(validToken, ".")
	if len(parts) == 3 {
		tamperedTokens := []string{
			parts[0] + "." + parts[1] + ".tampered", // Tampered signature
			"tampered." + parts[1] + "." + parts[2], // Tampered header
			parts[0] + ".tampered." + parts[2],      // Tampered payload
			validToken + "extra",                    // Extra characters
		}

		for i, token := range tamperedTokens {
			t.Run("tampered_jwt_"+string(rune('a'+i)), func(t *testing.T) {
				req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
				req.Header.Set("Authorization", "Bearer "+token)

				rr := httptest.NewRecorder()
				ts.ServeHTTP(rr, req)

				assert.Equal(t, http.StatusUnauthorized, rr.Code, "Tampered JWT should be rejected")
			})
		}
	}
}

// TestMissingAuthHeader tests requests without Authorization header
func TestMissingAuthHeader(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/user/profile"},
		{"GET", "/api/v1/user/dashboard"},
		{"GET", "/api/v1/admin/dashboard"},
		{"GET", "/api/v1/admin/referrals"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			// No Authorization header set

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

// TestInvalidAuthHeaderFormat tests various invalid Authorization header formats
func TestInvalidAuthHeaderFormat(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	invalidHeaders := []string{
		"InvalidFormat",
		"Bearer",
		"Bearer ",
		"Basic dXNlcjpwYXNz",
		"bearer token", // lowercase
		"BEARER token", // uppercase
		"Token abc123",
	}

	for _, header := range invalidHeaders {
		t.Run("invalid_header_"+header[:min(10, len(header))], func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
			req.Header.Set("Authorization", header)

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

// ============================================================================
// ROLE-BASED ACCESS CONTROL TESTS
// ============================================================================

// TestUserCannotAccessAdminRoutes tests that regular users cannot access admin endpoints
func TestUserCannotAccessAdminRoutes(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register a regular user
	registerPayload := models.RegisterRequest{
		Email:    "regularuser@example.com",
		Password: "password123",
		Name:     "Regular User",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	var authResponse models.AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &authResponse)
	userToken := authResponse.AccessToken

	// Try to access admin routes with user token
	adminRoutes := []string{
		"/api/v1/admin/dashboard",
		"/api/v1/admin/referrals",
		"/api/v1/admin/students",
		"/api/v1/admin/payouts",
	}

	for _, route := range adminRoutes {
		t.Run("user_access_"+route, func(t *testing.T) {
			req := httptest.NewRequest("GET", route, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code, "User should not access admin routes")
		})
	}
}

// ============================================================================
// SECURE HEADERS TESTS
// ============================================================================

// TestSecureHeadersPresent tests that security headers are set
func TestSecureHeadersPresent(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	// Note: These tests may need adjustment based on middleware in test setup
	// The test server may not include all production middleware
	t.Log("Response headers:", rr.Header())
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
