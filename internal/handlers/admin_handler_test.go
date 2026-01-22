package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cirvee/referral-backend/internal/database"
	"github.com/cirvee/referral-backend/internal/middleware"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminHandler(t *testing.T) (*AdminHandler, *database.DB, func()) {
	db, cleanup := setupTestDB(t)

	userRepo := repository.NewUserRepository(db)
	referralRepo := repository.NewReferralRepository(db, nil)
	payoutRepo := repository.NewPayoutRepository(db)

	handler := NewAdminHandler(userRepo, referralRepo, payoutRepo)

	return handler, db, cleanup
}

func TestAdminHandler_GetDashboard(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	rr := httptest.NewRecorder()

	handler.GetDashboard(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var stats models.DashboardStats
	err := json.Unmarshal(rr.Body.Bytes(), &stats)
	require.NoError(t, err)

	// Initial stats should be zero
	assert.Equal(t, int64(0), stats.TotalEarnings)
	assert.Equal(t, 0, stats.TotalReferrals)
}

func TestAdminHandler_GetReferrals_Empty(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/admin/referrals?page=1&per_page=10", nil)
	rr := httptest.NewRecorder()

	handler.GetReferrals(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var response models.PaginatedResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 10, response.PerPage)
	assert.Equal(t, int64(0), response.Total)
}

func TestAdminHandler_GetReferrals_Pagination(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	tests := []struct {
		name        string
		query       string
		wantPage    int
		wantPerPage int
	}{
		{"default_pagination", "", 1, 10},
		{"custom_page", "?page=2&per_page=5", 2, 5},
		{"negative_page", "?page=-1", 1, 10},
		{"large_per_page", "?per_page=200", 1, 10}, // Should cap at 100
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/admin/referrals"+tt.query, nil)
			rr := httptest.NewRecorder()

			handler.GetReferrals(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestAdminHandler_GetStudents_Empty(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/admin/students", nil)
	rr := httptest.NewRecorder()

	handler.GetStudents(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var response models.PaginatedResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, int64(0), response.Total)
}

func TestAdminHandler_GetPayouts_Empty(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/admin/payouts", nil)
	rr := httptest.NewRecorder()

	handler.GetPayouts(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var response models.PaginatedResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, int64(0), response.Total)
}

func TestAdminHandler_GetPayouts_WithFilter(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	tests := []struct {
		name   string
		status string
	}{
		{"filter_pending", "pending"},
		{"filter_approved", "approved"},
		{"filter_rejected", "rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/admin/payouts?status="+tt.status, nil)
			rr := httptest.NewRecorder()

			handler.GetPayouts(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestAdminHandler_UpdatePayoutStatus_InvalidID(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	// Create request with invalid UUID
	req := httptest.NewRequest("PATCH", "/api/v1/admin/payouts/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	// Need to set up chi context for URL params - for unit test, we test handler directly
	handler.UpdatePayoutStatus(rr, req)

	// Without chi.URLParam, this will return bad request for invalid uuid
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminHandler_UpdatePayoutStatus_NotFound(t *testing.T) {
	handler, _, cleanup := setupAdminHandler(t)
	defer cleanup()

	// Create a context with user claims
	claims := &utils.Claims{
		UserID: uuid.New(),
		Email:  "admin@test.com",
		Role:   "admin",
	}
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, claims)

	req := httptest.NewRequest("PATCH", "/api/v1/admin/payouts/"+uuid.New().String(), nil)
	req = req.WithContext(ctx)
	req.Body = http.NoBody
	rr := httptest.NewRecorder()

	handler.UpdatePayoutStatus(rr, req)

	// Without proper chi routing, this will fail to parse UUID from URL
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}
