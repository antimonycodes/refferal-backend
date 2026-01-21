package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cirvee/referral-backend/internal/middleware"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/go-playground/validator/v10"
)

type UserHandler struct {
	userRepo     *repository.UserRepository
	referralRepo *repository.ReferralRepository
	clickRepo    *repository.ClickRepository
	validate     *validator.Validate
}

func NewUserHandler(userRepo *repository.UserRepository, referralRepo *repository.ReferralRepository, clickRepo *repository.ClickRepository) *UserHandler {
	return &UserHandler{
		userRepo:     userRepo,
		referralRepo: referralRepo,
		clickRepo:    clickRepo,
		validate:     validator.New(),
	}
}

// GetDashboard godoc
// @Summary Get user dashboard stats
// @Description Get statistics for user dashboard
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.DashboardStats
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/user/dashboard [get]
func (h *UserHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	totalCount, totalEarnings, pendingEarnings, err := h.referralRepo.GetStatsByReferrer(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	// Get click count for user's referral code
	clickCount, _ := h.clickRepo.GetClickCountByUserID(r.Context(), claims.UserID)

	stats := models.DashboardStats{
		TotalEarnings:  totalEarnings,
		PendingBalance: pendingEarnings,
		TotalReferrals: totalCount,
		TotalClicks:    clickCount,
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetMyReferrals godoc
// @Summary Get my referrals
// @Description Get paginated list of user's referrals
// @Tags User
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/user/referrals [get]
func (h *UserHandler) GetMyReferrals(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	referrals, total, err := h.referralRepo.ListByReferrer(r.Context(), claims.UserID, page, perPage)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get referrals")
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	respondJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:       referrals,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetProfile godoc
// @Summary Get user profile
// @Description Get current user's profile
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/user/profile [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get profile")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update current user's profile
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.UpdateProfileRequest true "Profile update"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/user/profile [patch]
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get profile")
		return
	}

	// Update only provided fields
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.BankName != "" {
		user.BankName = req.BankName
	}
	if req.AccountNumber != "" {
		user.AccountNumber = req.AccountNumber
	}
	if req.AccountName != "" {
		user.AccountName = req.AccountName
	}

	if err := h.userRepo.Update(r.Context(), user); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
