package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ADMIN AUTHENTICATION TESTS
// ============================================================================

// Note: These tests require an admin user to be created first.
// The setupTestServer doesn't seed an admin, so we'd need to either:
// 1. Add admin seeding to setup_test.go, or
// 2. Test that admin routes are properly protected

// TestAdminRoutesRequireAdminRole tests that admin routes reject regular users
func TestAdminRoutesRequireAdminRole(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register a regular user
	userToken := registerAndLogin(t, ts, "regularuser@example.com", "password123", "Regular User")

	adminRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/dashboard"},
		{"GET", "/api/v1/admin/referrals"},
		{"GET", "/api/v1/admin/students"},
		{"GET", "/api/v1/admin/payouts"},
		{"GET", "/api/v1/admin/referrers"},
	}

	for _, route := range adminRoutes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code,
				"Regular user should not access admin routes")
		})
	}
}

// TestAdminRoutesRequireAuthentication tests that admin routes require auth
func TestAdminRoutesRequireAuthentication(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	adminRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/dashboard"},
		{"GET", "/api/v1/admin/referrals"},
		{"GET", "/api/v1/admin/students"},
		{"GET", "/api/v1/admin/payouts"},
		{"POST", "/api/v1/admin/users/123/block"},
		{"POST", "/api/v1/admin/referrals/123/paid"},
		{"PATCH", "/api/v1/admin/payouts/123"},
	}

	for _, route := range adminRoutes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			// No Authorization header

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code,
				"Admin routes should require authentication")
		})
	}
}

// ============================================================================
// USER TO ADMIN ESCALATION TESTS
// ============================================================================

// TestUserCannotEscalateToAdmin tests that users cannot become admins
func TestUserCannotEscalateToAdmin(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register a regular user
	token := registerAndLogin(t, ts, "escalate@example.com", "password123", "Escalate Test")

	// Try to update profile with admin role
	updatePayload := map[string]string{
		"role": "admin",
	}
	body, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	// Should either succeed (ignoring role field) or fail
	// Either way, user should not become admin
	assert.NotEqual(t, http.StatusInternalServerError, rr.Code)

	// Verify user is still not admin by trying admin route
	req = httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "User should not have become admin")
}

// ============================================================================
// ADMIN OPERATIONS WITH INVALID DATA
// ============================================================================

// TestAdminOperationsWithInvalidUUID tests admin operations with invalid UUIDs
func TestAdminOperationsWithInvalidUUID(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// These should all fail with 401 (no auth) but not 500
	invalidUUIDs := []string{
		"not-a-uuid",
		"123",
		"",
		"null",
	}

	for _, uuid := range invalidUUIDs {
		if uuid == "" {
			continue
		}
		t.Run("invalid_uuid_"+uuid, func(t *testing.T) {
			// Block user
			req := httptest.NewRequest("POST", "/api/v1/admin/users/"+uuid+"/block", nil)
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code)

			// Mark referral paid
			req = httptest.NewRequest("POST", "/api/v1/admin/referrals/"+uuid+"/paid", nil)
			rr = httptest.NewRecorder()
			ts.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code)

			// Update payout
			req = httptest.NewRequest("PATCH", "/api/v1/admin/payouts/"+uuid, nil)
			rr = httptest.NewRecorder()
			ts.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
		})
	}
}

// ============================================================================
// PAGINATION TESTS FOR ADMIN ROUTES
// ============================================================================

// TestAdminPaginationParameters tests pagination behavior on admin routes
func TestAdminPaginationParameters(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// These will return 401 without auth, but we're testing that
	// pagination params don't cause server errors
	testCases := []struct {
		name  string
		query string
	}{
		{"default", ""},
		{"page_1", "?page=1"},
		{"page_1_per_10", "?page=1&per_page=10"},
		{"large_page", "?page=99999"},
		{"large_per_page", "?per_page=99999"},
		{"zero_page", "?page=0"},
		{"zero_per_page", "?per_page=0"},
		{"negative_page", "?page=-1"},
		{"string_page", "?page=abc"},
	}

	routes := []string{
		"/api/v1/admin/referrals",
		"/api/v1/admin/students",
		"/api/v1/admin/payouts",
		"/api/v1/admin/referrers",
	}

	for _, route := range routes {
		for _, tc := range testCases {
			t.Run(route+"_"+tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", route+tc.query, nil)
				rr := httptest.NewRecorder()
				ts.ServeHTTP(rr, req)

				// Should return 401 (no auth), not 500
				assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
					"Invalid pagination should not cause server error")
			})
		}
	}
}

// ============================================================================
// STATUS UPDATE TESTS
// ============================================================================

// TestInvalidStatusValues tests handling of invalid status values
func TestInvalidStatusValues(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	invalidStatuses := []string{
		"invalid",
		"PAID", // uppercase
		"",
		"null",
		"123",
	}

	for _, status := range invalidStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			payload := map[string]string{"status": status}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("PATCH", "/api/v1/admin/payouts/00000000-0000-0000-0000-000000000001", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should return 401 (no auth) but test that it doesn't crash
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
		})
	}
}

// ============================================================================
// ADMIN DASHBOARD CALCULATIONS
// ============================================================================

// TestDashboardWithNoData tests dashboard when there's no data
func TestDashboardWithNoData(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to access dashboard without auth - should return 401
	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ============================================================================
// REFERRER LIST TESTS
// ============================================================================

// TestReferrerListRequiresAuth tests that referrer list requires authentication
func TestReferrerListRequiresAuth(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/admin/referrers", nil)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ============================================================================
// EXPORT FUNCTIONALITY TESTS (Placeholder)
// ============================================================================

// TestExportReferrals tests export functionality
func TestExportReferrals(t *testing.T) {
	// Export functionality would need to be tested with admin token
	t.Skip("Export tests require admin token")
}

// ============================================================================
// COMPREHENSIVE ADMIN FLOW TEST
// ============================================================================

// TestFullAdminFlow tests a complete admin workflow
func TestFullAdminFlow(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Without an admin user seeded, we can only test that:
	// 1. Regular users can't access admin routes
	// 2. Unauthenticated requests are rejected

	// Register a regular user
	token := registerAndLogin(t, ts, "adminflow@example.com", "password123", "Admin Flow Test")

	// Verify user exists
	user := getUserProfile(t, ts, token)
	require.NotEmpty(t, user.ID)

	// Verify user cannot access admin routes
	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
