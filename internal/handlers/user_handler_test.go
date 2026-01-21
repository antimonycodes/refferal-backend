package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cirvee/referral-backend/internal/database"
	"github.com/cirvee/referral-backend/internal/middleware"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/services"
	"github.com/cirvee/referral-backend/internal/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserHandler(t *testing.T) (*UserHandler, *database.DB, uuid.UUID, func()) {
	db, cleanup := setupTestDB(t)

	jwtManager := utils.NewJWTManager("test-secret", "test-refresh", 15*time.Minute, 168*time.Hour)
	userRepo := repository.NewUserRepository(db)
	referralRepo := repository.NewReferralRepository(db)
	clickRepo := repository.NewClickRepository(db)
	authService := services.NewAuthService(userRepo, jwtManager)

	// Create a test user
	ctx := context.Background()
	response, err := authService.Register(ctx, &models.RegisterRequest{
		Email:    "testuser@example.com",
		Password: "password123",
		Name:     "Test User",
		Phone:    "08012345678",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	handler := NewUserHandler(userRepo, referralRepo, clickRepo)

	return handler, db, response.User.ID, cleanup
}

func createUserContext(userID uuid.UUID, role string) context.Context {
	claims := &utils.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   role,
	}
	return context.WithValue(context.Background(), middleware.UserContextKey, claims)
}

func TestUserHandler_GetDashboard(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	ctx := createUserContext(userID, "user")
	req := httptest.NewRequest("GET", "/api/v1/user/dashboard", nil)
	req = req.WithContext(ctx)
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

func TestUserHandler_GetDashboard_Unauthorized(t *testing.T) {
	handler, _, _, cleanup := setupUserHandler(t)
	defer cleanup()

	// Request without context
	req := httptest.NewRequest("GET", "/api/v1/user/dashboard", nil)
	rr := httptest.NewRecorder()

	handler.GetDashboard(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestUserHandler_GetMyReferrals_Empty(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	ctx := createUserContext(userID, "user")
	req := httptest.NewRequest("GET", "/api/v1/user/referrals", nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.GetMyReferrals(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var response models.PaginatedResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, int64(0), response.Total)
}

func TestUserHandler_GetMyReferrals_Pagination(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	tests := []struct {
		name  string
		query string
	}{
		{"default", ""},
		{"custom_page", "?page=2&per_page=5"},
		{"negative_page", "?page=-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createUserContext(userID, "user")
			req := httptest.NewRequest("GET", "/api/v1/user/referrals"+tt.query, nil)
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			handler.GetMyReferrals(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestUserHandler_GetProfile(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	ctx := createUserContext(userID, "user")
	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.GetProfile(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var user models.User
	err := json.Unmarshal(rr.Body.Bytes(), &user)
	require.NoError(t, err)

	assert.Equal(t, "testuser@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.NotEmpty(t, user.ReferralCode)
}

func TestUserHandler_GetProfile_Unauthorized(t *testing.T) {
	handler, _, _, cleanup := setupUserHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	rr := httptest.NewRecorder()

	handler.GetProfile(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestUserHandler_UpdateProfile(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	ctx := createUserContext(userID, "user")

	payload := models.UpdateProfileRequest{
		Name:          "Updated Name",
		Phone:         "08087654321",
		BankName:      "GTBank",
		AccountNumber: "0123456789",
		AccountName:   "Updated Account Name",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateProfile(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var user models.User
	err := json.Unmarshal(rr.Body.Bytes(), &user)
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", user.Name)
	assert.Equal(t, "08087654321", user.Phone)
	assert.Equal(t, "GTBank", user.BankName)
}

func TestUserHandler_UpdateProfile_InvalidJSON(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	ctx := createUserContext(userID, "user")

	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", bytes.NewReader([]byte("invalid")))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUserHandler_UpdateProfile_Unauthorized(t *testing.T) {
	handler, _, _, cleanup := setupUserHandler(t)
	defer cleanup()

	payload := models.UpdateProfileRequest{Name: "Test"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestUserHandler_UpdateProfile_PartialUpdate(t *testing.T) {
	handler, _, userID, cleanup := setupUserHandler(t)
	defer cleanup()

	ctx := createUserContext(userID, "user")

	// Only update phone, leave other fields empty
	payload := models.UpdateProfileRequest{
		Phone: "08099999999",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateProfile(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var user models.User
	err := json.Unmarshal(rr.Body.Bytes(), &user)
	require.NoError(t, err)

	// Name should remain unchanged, phone should be updated
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "08099999999", user.Phone)
}
