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

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// registerAndLogin registers a user and returns their access token
func registerAndLogin(t *testing.T, ts http.Handler, email, password, name string) string {
	payload := models.RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Failed to register user: %s", rr.Body.String())
	}

	var response models.AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	return response.AccessToken
}

// getUserProfile gets user profile and returns referral code
func getUserProfile(t *testing.T, ts http.Handler, token string) *models.User {
	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Failed to get profile: %s", rr.Body.String())
	}

	var user models.User
	json.Unmarshal(rr.Body.Bytes(), &user)
	return &user
}

// ============================================================================
// USER REGISTRATION AND PROFILE TESTS
// ============================================================================

// TestUserRegistrationGeneratesReferralCode tests that new users get a referral code
func TestUserRegistrationGeneratesReferralCode(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndLogin(t, ts, "referrer@example.com", "password123", "Referrer Test")
	user := getUserProfile(t, ts, token)

	assert.NotEmpty(t, user.ReferralCode, "Registered user should have a referral code")
	assert.Greater(t, len(user.ReferralCode), 5, "Referral code should be reasonable length")
}

// TestUserProfileUpdate tests profile update functionality
func TestUserProfileUpdate(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndLogin(t, ts, "profile@example.com", "password123", "Profile Test")

	// Update profile
	updatePayload := map[string]string{
		"name":           "Updated Name",
		"phone":          "08099999999",
		"bank_name":      "First Bank",
		"account_number": "1234567890",
		"account_name":   "Updated Name",
	}
	body, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify update
	user := getUserProfile(t, ts, token)
	assert.Equal(t, "Updated Name", user.Name)
}

// ============================================================================
// USER DASHBOARD TESTS
// ============================================================================

// TestUserDashboardEmpty tests dashboard for user with no referrals
func TestUserDashboardEmpty(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndLogin(t, ts, "dashboard@example.com", "password123", "Dashboard Test")

	req := httptest.NewRequest("GET", "/api/v1/user/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var dashboard map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &dashboard)
	require.NoError(t, err)

	// Empty dashboard should have zero values
	t.Log("Dashboard response:", rr.Body.String())
}

// TestUserReferralsEmpty tests referrals list for user with no referrals
func TestUserReferralsEmpty(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndLogin(t, ts, "noreferrals@example.com", "password123", "No Referrals")

	req := httptest.NewRequest("GET", "/api/v1/user/referrals", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ============================================================================
// TOKEN REFRESH TESTS
// ============================================================================

// TestTokenRefresh tests the token refresh flow
func TestTokenRefresh(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register and get tokens
	payload := models.RegisterRequest{
		Email:    "refresh@example.com",
		Password: "password123",
		Name:     "Refresh Test",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	var authResponse models.AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &authResponse)

	// Use refresh token to get new access token
	refreshPayload := models.RefreshRequest{
		RefreshToken: authResponse.RefreshToken,
	}
	body, _ = json.Marshal(refreshPayload)
	req = httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var newAuthResponse models.AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &newAuthResponse)
	assert.NotEmpty(t, newAuthResponse.AccessToken)
	// Note: Token may be same if generated in same second (JWT exp based on seconds)
}

// TestTokenRefreshWithInvalidToken tests refresh with invalid token
func TestTokenRefreshWithInvalidToken(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	invalidTokens := []string{
		"invalid_token",
		"",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
	}

	for i, token := range invalidTokens {
		t.Run("invalid_refresh_"+string(rune('a'+i)), func(t *testing.T) {
			refreshPayload := models.RefreshRequest{
				RefreshToken: token,
			}
			body, _ := json.Marshal(refreshPayload)
			req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Should return 400 (bad request) or 401 (unauthorized)
			assert.True(t, rr.Code == http.StatusUnauthorized || rr.Code == http.StatusBadRequest,
				"Invalid refresh token should return 400 or 401, got %d", rr.Code)
		})
	}
}

// ============================================================================
// MULTIPLE USERS TESTS
// ============================================================================

// TestMultipleUsersIndependentData tests that users can't see each other's data
func TestMultipleUsersIndependentData(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register two users
	token1 := registerAndLogin(t, ts, "user1@example.com", "password123", "User One")
	token2 := registerAndLogin(t, ts, "user2@example.com", "password123", "User Two")

	// Get profiles
	user1 := getUserProfile(t, ts, token1)
	user2 := getUserProfile(t, ts, token2)

	// Verify they have different IDs and emails
	assert.NotEqual(t, user1.ID, user2.ID)
	assert.NotEqual(t, user1.Email, user2.Email)
	assert.NotEqual(t, user1.ReferralCode, user2.ReferralCode)
}

// TestLoginWithWrongPassword tests login failure with wrong password
func TestLoginWithWrongPassword(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register user
	registerAndLogin(t, ts, "wrongpw@example.com", "password123", "Wrong PW Test")

	// Try to login with wrong password
	loginPayload := models.LoginRequest{
		Email:    "wrongpw@example.com",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestLoginPreservesOriginalPassword tests that password is checked correctly
func TestLoginPreservesOriginalPassword(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register user
	registerAndLogin(t, ts, "preserve@example.com", "originalPassword123", "Preserve Test")

	// Login with original password should work
	loginPayload := models.LoginRequest{
		Email:    "preserve@example.com",
		Password: "originalPassword123",
	}
	body, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ============================================================================
// CASE SENSITIVITY TESTS
// ============================================================================

// TestEmailCaseSensitivity tests email case handling
func TestEmailCaseSensitivity(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register with lowercase email
	registerAndLogin(t, ts, "lowercase@example.com", "password123", "Case Test")

	// Try to login with different casing
	casings := []string{
		"LOWERCASE@EXAMPLE.COM",
		"Lowercase@Example.Com",
		"lOwErCaSe@eXaMpLe.CoM",
	}

	for _, email := range casings {
		t.Run("casing_"+email[:10], func(t *testing.T) {
			loginPayload := models.LoginRequest{
				Email:    email,
				Password: "password123",
			}
			body, _ := json.Marshal(loginPayload)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			// Either should work (case insensitive) or fail consistently
			t.Log("Login with", email, "returned", rr.Code)
		})
	}
}
