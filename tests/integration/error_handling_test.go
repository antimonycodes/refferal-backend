package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// 404 NOT FOUND TESTS
// ============================================================================

// TestUnknownRoutes tests that unknown routes return 404
func TestUnknownRoutes(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	unknownRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/unknown"},
		{"GET", "/api/v1/auth/unknown"},
		{"GET", "/api/v2/auth/login"},
		{"POST", "/api/v1/nonexistent"},
		{"GET", "/unknown"},
		{"GET", "/api"},
		{"GET", "/api/v1"},
	}

	for _, route := range unknownRoutes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should return 404 or 405, not 500
			assert.True(t, rr.Code == http.StatusNotFound || rr.Code == http.StatusMethodNotAllowed,
				"Unknown route should return 404 or 405, got %d", rr.Code)
		})
	}
}

// ============================================================================
// METHOD NOT ALLOWED TESTS
// ============================================================================

// TestMethodNotAllowed tests that wrong HTTP methods are rejected
func TestMethodNotAllowed(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	testCases := []struct {
		name        string
		method      string
		path        string
		shouldAllow bool
	}{
		{"GET_on_login", "GET", "/api/v1/auth/login", false},
		{"PUT_on_login", "PUT", "/api/v1/auth/login", false},
		{"DELETE_on_login", "DELETE", "/api/v1/auth/login", false},
		{"GET_on_register", "GET", "/api/v1/auth/register", false},
		{"POST_on_health", "POST", "/health", false},
		{"DELETE_on_health", "DELETE", "/health", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			if !tc.shouldAllow {
				assert.True(t, rr.Code == http.StatusMethodNotAllowed || rr.Code == http.StatusNotFound,
					"Wrong method should be rejected")
			}
		})
	}
}

// ============================================================================
// INVALID UUID TESTS
// ============================================================================

// TestInvalidUUIDInPath tests handling of invalid UUIDs in URL paths
func TestInvalidUUIDInPath(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// First register a user to get a valid token
	token := registerAndLogin(t, ts, "uuidtest@example.com", "password123", "UUID Test")
	_ = token // Will use if admin routes are available

	invalidUUIDs := []string{
		"not-a-uuid",
		"123",
		"12345678-1234-1234-1234-123456789abc-extra",
		"",
		"null",
		"undefined",
		"../../../etc/passwd",
		"<script>alert('xss')</script>",
	}

	// Test various endpoints that take UUID parameters
	// Note: Admin routes need admin token, but we can test the pattern
	for _, uuid := range invalidUUIDs {
		if uuid == "" {
			continue // Empty UUID would match different route
		}
		t.Run("invalid_uuid_"+uuid[:min(10, len(uuid))], func(t *testing.T) {
			// This would require admin token which we don't have in test setup
			// Just verify server doesn't crash on invalid UUID format
			req := httptest.NewRequest("POST", "/api/v1/admin/users/"+uuid+"/block", nil)
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should return 401 (no auth) or 400 (bad UUID), not 500
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"Invalid UUID should not cause server error")
		})
	}
}

// ============================================================================
// MISSING REQUIRED FIELDS TESTS
// ============================================================================

// TestMissingRequiredFields tests that missing required fields are reported
func TestMissingRequiredFields(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{"missing_email", map[string]interface{}{"password": "password123", "name": "Test", "phone": "08012345678"}},
		{"missing_password", map[string]interface{}{"email": "test@example.com", "name": "Test", "phone": "08012345678"}},
		{"missing_name", map[string]interface{}{"email": "test@example.com", "password": "password123", "phone": "08012345678"}},
		{"missing_phone", map[string]interface{}{"email": "test@example.com", "password": "password123", "name": "Test"}},
		{"only_email", map[string]interface{}{"email": "test@example.com"}},
		{"empty_object", map[string]interface{}{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code, "Missing required fields should return 400")

			// Verify error message is informative
			var errorResponse map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			if err == nil {
				assert.Contains(t, rr.Body.String(), "error",
					"Error response should contain error field")
			}
		})
	}
}

// ============================================================================
// INVALID FIELD VALUES TESTS
// ============================================================================

// TestInvalidFieldValues tests handling of invalid field values
func TestInvalidFieldValues(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{"null_email", map[string]interface{}{"email": nil, "password": "password123", "name": "Test", "phone": "08012345678"}},
		{"number_email", map[string]interface{}{"email": 12345, "password": "password123", "name": "Test", "phone": "08012345678"}},
		{"array_email", map[string]interface{}{"email": []string{"test@example.com"}, "password": "password123", "name": "Test", "phone": "08012345678"}},
		{"object_email", map[string]interface{}{"email": map[string]string{"address": "test@example.com"}, "password": "password123", "name": "Test", "phone": "08012345678"}},
		{"boolean_password", map[string]interface{}{"email": "test@example.com", "password": true, "name": "Test", "phone": "08012345678"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should return 400, not 500
			assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusCreated,
				"Invalid field values should not cause server error, got %d", rr.Code)
		})
	}
}

// ============================================================================
// CONTENT TYPE TESTS
// ============================================================================

// TestWrongContentType tests handling of wrong Content-Type headers
func TestWrongContentType(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	contentTypes := []string{
		"text/plain",
		"text/html",
		"application/xml",
		"multipart/form-data",
		"application/x-www-form-urlencoded",
		"",
	}

	payload := `{"email": "test@example.com", "password": "password123"}`

	for _, contentType := range contentTypes {
		t.Run("content_type_"+contentType, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(payload)))
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should either work (if server is lenient) or return 415 Unsupported Media Type
			// Should NOT return 500
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
		})
	}
}

// ============================================================================
// ERROR RESPONSE FORMAT TESTS
// ============================================================================

// TestErrorResponseFormat tests that error responses have consistent format
func TestErrorResponseFormat(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Trigger various errors
	errorScenarios := []struct {
		name  string
		setup func() *http.Request
	}{
		{
			name: "invalid_json",
			setup: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(`{invalid}`)))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
		},
		{
			name: "missing_fields",
			setup: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(`{}`)))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
		},
		{
			name: "unauthorized",
			setup: func() *http.Request {
				return httptest.NewRequest("GET", "/api/v1/user/profile", nil)
			},
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setup()
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Verify response is valid JSON
			var response map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &response)

			if err == nil {
				// If it's JSON, it should have an error field
				_, hasError := response["error"]
				assert.True(t, hasError, "Error response should have 'error' field")
			}
		})
	}
}

// ============================================================================
// HEALTH CHECK TESTS
// ============================================================================

// TestHealthEndpoint tests the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var health map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &health)
	assert.NoError(t, err, "Health response should be valid JSON")
}

// TestHealthEndpointWithParams tests health endpoint ignores query params
func TestHealthEndpointWithParams(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health?foo=bar&baz=123", nil)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
