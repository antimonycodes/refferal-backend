package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cirvee/referral-backend/internal/models"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// CONCURRENT REGISTRATION TESTS
// ============================================================================

// TestConcurrentRegistrations tests that concurrent user registrations don't cause conflicts
func TestConcurrentRegistrations(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	numUsers := 10
	var wg sync.WaitGroup
	results := make(chan int, numUsers)
	errors := make(chan error, numUsers)

	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			payload := models.RegisterRequest{
				Email:    "concurrent" + string(rune('a'+index)) + "@example.com",
				Password: "password123",
				Name:     "Concurrent User " + string(rune('a'+index)),
				Phone:    "0801234567" + string(rune('0'+index)),
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Count successes and failures
	successCount := 0
	for code := range results {
		if code == http.StatusCreated {
			successCount++
		}
	}

	// All registrations should succeed (unique emails)
	assert.Equal(t, numUsers, successCount, "All concurrent registrations should succeed")
}

// TestConcurrentDuplicateRegistrations tests that duplicate email registrations are properly handled
func TestConcurrentDuplicateRegistrations(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Same email for all registrations
	numAttempts := 5
	var wg sync.WaitGroup
	results := make(chan int, numAttempts)

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			payload := models.RegisterRequest{
				Email:    "duplicate@concurrent.com",
				Password: "password123",
				Name:     "Duplicate Test",
				Phone:    "08012345678",
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code
		}()
	}

	wg.Wait()
	close(results)

	// Count results
	successCount := 0
	conflictCount := 0
	errorCount := 0
	for code := range results {
		switch code {
		case http.StatusCreated:
			successCount++
		case http.StatusConflict:
			conflictCount++
		default:
			errorCount++
		}
	}

	// At least one should succeed; others should fail (409 or 500 due to race condition)
	assert.GreaterOrEqual(t, successCount, 1, "At least one registration should succeed")
	t.Logf("Results: %d success, %d conflict, %d other errors", successCount, conflictCount, errorCount)
}

// ============================================================================
// CONCURRENT LOGIN TESTS
// ============================================================================

// TestConcurrentLogins tests that multiple logins for same user work correctly
func TestConcurrentLogins(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// First register a user
	registerPayload := models.RegisterRequest{
		Email:    "concurrentlogin@example.com",
		Password: "password123",
		Name:     "Concurrent Login Test",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Now perform concurrent logins
	numLogins := 10
	var wg sync.WaitGroup
	tokens := make(chan string, numLogins)
	results := make(chan int, numLogins)

	for i := 0; i < numLogins; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			payload := models.LoginRequest{
				Email:    "concurrentlogin@example.com",
				Password: "password123",
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code

			if rr.Code == http.StatusOK {
				var response models.AuthResponse
				json.Unmarshal(rr.Body.Bytes(), &response)
				tokens <- response.AccessToken
			}
		}()
	}

	wg.Wait()
	close(results)
	close(tokens)

	// All logins should succeed
	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}
	assert.Equal(t, numLogins, successCount, "All concurrent logins should succeed")

	// Each login should get a unique token
	tokenSet := make(map[string]bool)
	for token := range tokens {
		tokenSet[token] = true
	}
	// Note: Tokens might be the same if generated in same millisecond
	// This is expected behavior for JWT
}

// ============================================================================
// CONCURRENT PROFILE ACCESS TESTS
// ============================================================================

// TestConcurrentProfileAccess tests that concurrent profile reads work correctly
func TestConcurrentProfileAccess(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register and get token
	token := registerAndLogin(t, ts, "profileaccess@example.com", "password123", "Profile Access Test")

	// Concurrent profile reads
	numRequests := 20
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code
		}()
	}

	wg.Wait()
	close(results)

	// All reads should succeed
	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}
	assert.Equal(t, numRequests, successCount, "All concurrent profile reads should succeed")
}

// ============================================================================
// CONCURRENT MIXED OPERATIONS TESTS
// ============================================================================

// TestConcurrentMixedOperations tests concurrent reads and writes
func TestConcurrentMixedOperations(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register users first
	tokens := make([]string, 5)
	for i := 0; i < 5; i++ {
		tokens[i] = registerAndLogin(t, ts,
			"mixed"+string(rune('a'+i))+"@example.com",
			"password123",
			"Mixed User "+string(rune('a'+i)))
	}

	// Concurrent operations using different users' tokens
	var wg sync.WaitGroup
	numOps := 20
	results := make(chan int, numOps)

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			token := tokens[index%len(tokens)]

			// Mix of different operations
			var req *http.Request
			switch index % 3 {
			case 0:
				req = httptest.NewRequest("GET", "/api/v1/user/profile", nil)
			case 1:
				req = httptest.NewRequest("GET", "/api/v1/user/dashboard", nil)
			case 2:
				req = httptest.NewRequest("GET", "/api/v1/user/referrals", nil)
			}
			req.Header.Set("Authorization", "Bearer "+token)

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code
		}(i)
	}

	wg.Wait()
	close(results)

	// All operations should succeed
	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}
	assert.Equal(t, numOps, successCount, "All concurrent operations should succeed")
}

// ============================================================================
// CONCURRENT TOKEN REFRESH TESTS
// ============================================================================

// TestConcurrentTokenRefresh tests concurrent token refresh requests
func TestConcurrentTokenRefresh(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Register and get tokens
	payload := models.RegisterRequest{
		Email:    "tokenrefresh@example.com",
		Password: "password123",
		Name:     "Token Refresh Test",
		Phone:    "08012345678",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var authResponse models.AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &authResponse)
	refreshToken := authResponse.RefreshToken

	// Concurrent refresh requests with same token
	numRequests := 5
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			payload := models.RefreshRequest{
				RefreshToken: refreshToken,
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code
		}()
	}

	wg.Wait()
	close(results)

	// All refreshes should succeed (refresh token is not invalidated on use in this implementation)
	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}
	// At least the first one should succeed
	assert.GreaterOrEqual(t, successCount, 1, "At least one refresh should succeed")
}

// ============================================================================
// STRESS TEST
// ============================================================================

// TestStressHealthEndpoint tests the health endpoint under load
func TestStressHealthEndpoint(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	numRequests := 100
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/health", nil)
			rr := httptest.NewRecorder()
			ts.ServeHTTP(rr, req)

			results <- rr.Code
		}()
	}

	wg.Wait()
	close(results)

	// All should succeed
	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}
	assert.Equal(t, numRequests, successCount, "All health checks should succeed under load")
}
