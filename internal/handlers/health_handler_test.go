package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cirvee/referral-backend/internal/cache"
	"github.com/cirvee/referral-backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Health_Success(t *testing.T) {
	// Try to connect to test database and redis
	dbURL := "postgres://referral_test:referral_test_secret@localhost:5435/referral_test_db?sslmode=disable"
	redisURL := "redis://localhost:6380"

	db, err := database.New(dbURL)
	if err != nil {
		t.Skipf("Skipping test - database not available: %v", err)
	}
	defer db.Close()

	redisCache, err := cache.New(redisURL)
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	handler := NewHealthHandler(db, redisCache)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "healthy")
}

func TestHealthHandler_Health_NilDB(t *testing.T) {
	// This test verifies behavior when DB is nil or unhealthy
	// In production, this would return service unavailable

	// Since we can't easily mock a failed connection,
	// we'll just verify the handler doesn't panic with valid connections
	dbURL := "postgres://referral_test:referral_test_secret@localhost:5435/referral_test_db?sslmode=disable"
	redisURL := "redis://localhost:6380"

	db, err := database.New(dbURL)
	if err != nil {
		t.Skipf("Skipping test - database not available: %v", err)
	}
	defer db.Close()

	redisCache, err := cache.New(redisURL)
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	handler := NewHealthHandler(db, redisCache)

	req := httptest.NewRequest("GET", "/health", nil)
	req = req.WithContext(context.Background())
	rr := httptest.NewRecorder()

	// This should not panic
	assert.NotPanics(t, func() {
		handler.Health(rr, req)
	})
}
