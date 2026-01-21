package handlers

import (
	"net/http"
	"strconv"

	"github.com/cirvee/referral-backend/internal/middleware"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	userRepo     *repository.UserRepository
	referralRepo *repository.ReferralRepository
	payoutRepo   *repository.PayoutRepository
}

func NewAdminHandler(userRepo *repository.UserRepository, referralRepo *repository.ReferralRepository, payoutRepo *repository.PayoutRepository) *AdminHandler {
	return &AdminHandler{
		userRepo:     userRepo,
		referralRepo: referralRepo,
		payoutRepo:   payoutRepo,
	}
}

// GetDashboard godoc
// @Summary Get admin dashboard stats
// @Description Get statistics for admin dashboard
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.DashboardStats
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/admin/dashboard [get]
func (h *AdminHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalReferrals, totalEarnings, pendingEarnings, totalPaidEarnings, paidCount, totalCodes, activeCodes, totalEnrollments, totalUniqueCourses, _ := h.referralRepo.GetTotalStats(ctx)

	stats := models.DashboardStats{
		TotalEarnings:      totalEarnings,
		TotalReferrals:     totalReferrals,
		TotalPayouts:       totalPaidEarnings, // Use paid earnings as total payouts
		PendingBalance:     pendingEarnings,
		TotalPaidEarnings:  totalPaidEarnings,
		PaidCount:          paidCount,
		ActiveCodes:        activeCodes,
		TotalCodes:         totalCodes,
		TotalStudents:      totalEnrollments,
		TotalUniqueCourses: totalUniqueCourses,
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetReferrals godoc
// @Summary Get all referrals
// @Description Get paginated list of all referrals
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/admin/referrals [get]
func (h *AdminHandler) GetReferrals(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	// Revert to ListAll for raw transactions (used by Dashboard)
	referrals, total, err := h.referralRepo.ListAll(r.Context(), page, perPage)
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

// GetReferrers godoc
// @Summary Get all referrers with stats
// @Description Get paginated list of referrers with aggregated stats
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/admin/referrers [get]
func (h *AdminHandler) GetReferrers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	referrers, total, err := h.referralRepo.GetReferrersStats(r.Context(), page, perPage)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get referrers")
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	respondJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:       referrers,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetStudents godoc
// @Summary Get all students (users)
// @Description Get paginated list of all users
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/admin/students [get]
func (h *AdminHandler) GetStudents(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	// Use ListAll to get all students (referrals)
	students, total, err := h.referralRepo.ListAll(r.Context(), page, perPage)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get students")
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	respondJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:       students,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetPayouts godoc
// @Summary Get all payouts
// @Description Get paginated list of all payouts with optional status filter
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Param status query string false "Filter by status (pending, approved, rejected)"
// @Success 200 {object} models.PaginatedResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/admin/payouts [get]
func (h *AdminHandler) GetPayouts(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	var statusFilter *models.PayoutStatus
	if status := r.URL.Query().Get("status"); status != "" {
		s := models.PayoutStatus(status)
		statusFilter = &s
	}

	payouts, total, err := h.payoutRepo.ListAll(r.Context(), page, perPage, statusFilter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get payouts")
		return
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	respondJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:       payouts,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// UpdatePayoutStatus godoc
// @Summary Update payout status
// @Description Approve or reject a payout request
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Payout ID"
// @Param request body models.UpdatePayoutStatusRequest true "New status"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/admin/payouts/{id} [patch]
func (h *AdminHandler) UpdatePayoutStatus(w http.ResponseWriter, r *http.Request) {
	payoutID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid payout ID")
		return
	}

	var req models.UpdatePayoutStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims, _ := middleware.GetUserFromContext(r.Context())

	err = h.payoutRepo.UpdateStatus(r.Context(), payoutID, req.Status, claims.UserID)
	if err != nil {
		if err == repository.ErrPayoutNotFound {
			respondError(w, http.StatusNotFound, "payout not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update payout")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "payout status updated"})
}

// MarkReferrerPaid godoc
// @Summary Mark user's referrals as paid
// @Description Mark all pending referrals for a user as paid
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Referrer ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/admin/referrers/{id}/paid [post]
func (h *AdminHandler) MarkReferrerPaid(w http.ResponseWriter, r *http.Request) {
	referrerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid referrer ID")
		return
	}

	err = h.referralRepo.MarkReferralsAsPaid(r.Context(), referrerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to mark referrals as paid")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "referrals marked as paid"})
}

// BlockUser godoc
// @Summary Block/Unblock a user
// @Description Block or unblock a user by ID
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Param request body models.BlockUserRequest true "Block status"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/admin/users/{id}/block [post]
func (h *AdminHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req models.BlockUserRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Prevent blocking self? Maybe. For now, just allow it or rely on frontend.
	claims, _ := middleware.GetUserFromContext(r.Context())
	if claims.UserID == userID {
		respondError(w, http.StatusBadRequest, "cannot block yourself")
		return
	}

	err = h.userRepo.UpdateStatus(r.Context(), userID, req.IsBlocked)
	if err != nil {
		if err == repository.ErrUserNotFound {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update user status")
		return
	}

	action := "blocked"
	if !req.IsBlocked {
		action = "unblocked"
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "user " + action})
}

// UpdateReferralStatus godoc
// @Summary Update referral status
// @Description Update the status of a specific referral (e.g. paid, rejected)
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Referral ID"
// @Param request body models.UpdateReferralStatusRequest true "New status"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/admin/referrals/{id}/status [patch]
func (h *AdminHandler) UpdateReferralStatus(w http.ResponseWriter, r *http.Request) {
	referralID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid referral ID")
		return
	}

	var req models.UpdateReferralStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err = h.referralRepo.UpdateStatus(r.Context(), referralID, req.Status)
	if err != nil {
		if err == repository.ErrReferralNotFound {
			respondError(w, http.StatusNotFound, "referral not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update referral status")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "referral status updated"})
}

// MarkReferralPaid godoc
// @Summary Mark specific referral as paid
// @Description Mark a single pending referral as paid
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Referral ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/admin/referrals/{id}/paid [post]
func (h *AdminHandler) MarkReferralPaid(w http.ResponseWriter, r *http.Request) {
	referralID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid referral ID")
		return
	}

	err = h.referralRepo.MarkReferralAsPaid(r.Context(), referralID)
	if err != nil {
		if err == repository.ErrReferralNotFound {
			respondError(w, http.StatusNotFound, "referral not found or already paid")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to mark referral as paid")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "referral marked as paid"})
}
