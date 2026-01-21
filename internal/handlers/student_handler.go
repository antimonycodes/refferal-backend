package handlers

import (
	"net/http"

	"github.com/cirvee/referral-backend/internal/config"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/services"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Course prices and earnings percentages (can be moved to config)
var coursePrices = map[string]int64{
	"Web Development":    150000,
	"Data Science":       180000,
	"Mobile Development": 160000,
	"UI/UX Design":       120000,
	"Digital Marketing":  100000,
	"Cybersecurity":      200000,
	"Cloud Computing":    170000,
	"Machine Learning":   190000,
}

const earningsPercentage = 10 // 10% commission

type StudentHandler struct {
	userRepo     *repository.UserRepository
	referralRepo *repository.ReferralRepository
	clickRepo    *repository.ClickRepository
	emailService *services.EmailService
	adminEmail   string
	validate     *validator.Validate
}

func NewStudentHandler(
	userRepo *repository.UserRepository,
	referralRepo *repository.ReferralRepository,
	clickRepo *repository.ClickRepository,
	emailService *services.EmailService,
	adminCfg *config.AdminConfig,
) *StudentHandler {
	return &StudentHandler{
		userRepo:     userRepo,
		referralRepo: referralRepo,
		clickRepo:    clickRepo,
		emailService: emailService,
		adminEmail:   adminCfg.Email,
		validate:     validator.New(),
	}
}

// RegisterStudent godoc
// @Summary Register a new student
// @Description Public endpoint for student registration. Creates a referral if valid referral code is provided.
// @Tags Students
// @Accept json
// @Produce json
// @Param request body models.StudentRegistrationRequest true "Student registration data"
// @Success 201 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/students/register [post]
func (h *StudentHandler) RegisterStudent(w http.ResponseWriter, r *http.Request) {
	var req models.StudentRegistrationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	// Get course price (default to 100000 if not found)
	coursePrice, ok := coursePrices[req.Course]
	if !ok {
		coursePrice = 100000
	}

	// Calculate earnings and set referrer
	var referrerID *uuid.UUID
	var earnings int64
	var referrerName string
	var referrerEmail string

	if req.ReferralCode != "" {
		referrer, err := h.userRepo.GetByReferralCode(r.Context(), req.ReferralCode)
		if err != nil {
			if err != repository.ErrUserNotFound {
				respondError(w, http.StatusInternalServerError, "failed to verify referral code")
				return
			}
			// Referral code invalid: proceed as direct signup (referrerID remains nil)
		} else {
			// Valid referral
			referrerID = &referrer.ID
			earnings = (coursePrice * int64(earningsPercentage)) / 100
			referrerName = referrer.Name
			referrerEmail = referrer.Email
		}
	}

	// ALWAYS Create referral record to persist Course info
	referral := &models.Referral{
		ID:            uuid.New(),
		ReferrerID:    referrerID,
		ReferredName:  req.Name,
		ReferredEmail: req.Email,
		ReferredPhone: req.Phone,
		Course:        req.Course,
		CoursePrice:   coursePrice,
		Earnings:      earnings,
		Status:        "pending",
	}

	if err := h.referralRepo.Create(r.Context(), referral); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create referral record")
		return
	}

	// Send emails (async)
	go h.emailService.SendStudentConfirmation(req.Email, req.Name, req.Course)

	if referrerID != nil {
		// Only notify referrer if one exists
		go h.emailService.SendReferralNotification(referrerEmail, referrerName, req.Name, req.Course, earnings)
		go h.emailService.SendAdminNewStudentAlert(h.adminEmail, req.Name, req.Email, req.Course, referrerName)

		respondJSON(w, http.StatusCreated, map[string]string{
			"message":  "Student registered successfully",
			"referral": "applied",
			"referrer": referrerName,
		})
	} else {
		// Direct signup
		go h.emailService.SendAdminNewStudentAlert(h.adminEmail, req.Name, req.Email, req.Course, "Direct Sign-up")

		respondJSON(w, http.StatusCreated, map[string]string{
			"message": "Student registered successfully",
		})
	}
}

// TrackClick godoc
// @Summary Track referral link click
// @Description Records a click on a referral link for analytics
// @Tags Students
// @Accept json
// @Produce json
// @Param request body map[string]string true "Referral code"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Router /api/v1/students/track-click [post]
func (h *StudentHandler) TrackClick(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReferralCode string `json:"referral_code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ReferralCode == "" {
		respondError(w, http.StatusBadRequest, "referral_code is required")
		return
	}

	// Get IP address and user agent
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	userAgent := r.Header.Get("User-Agent")

	// Record the click
	if err := h.clickRepo.RecordClick(r.Context(), req.ReferralCode, ipAddress, userAgent, nil); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to record click")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "click recorded",
	})
}
