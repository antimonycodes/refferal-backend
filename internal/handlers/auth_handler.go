package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/services"
	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	authService    *services.AuthService
	emailService   *services.EmailService
	userRepo       *repository.UserRepository
	resetTokenRepo *repository.ResetTokenRepository
	validate       *validator.Validate
}

func NewAuthHandler(
	authService *services.AuthService,
	emailService *services.EmailService,
	userRepo *repository.UserRepository,
	resetTokenRepo *repository.ResetTokenRepository,
) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		emailService:   emailService,
		userRepo:       userRepo,
		resetTokenRepo: resetTokenRepo,
		validate:       validator.New(),
	}
}

// Register godoc
// @Summary Register a new user
// @Description Register a new user account (users only, no admin registration)
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "Registration details"
// @Success 201 {object} models.AuthResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	response, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		if err == services.ErrEmailAlreadyExists {
			respondError(w, http.StatusConflict, "email already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to register user")
		return
	}

	// Send welcome email (async, don't block response)
	go h.emailService.SendWelcomeEmail(response.User.Email, response.User.Name)

	respondJSON(w, http.StatusCreated, response)
}

// Login godoc
// @Summary Login user
// @Description Authenticate user and return tokens
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.AuthResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	response, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if err == services.ErrUserBlocked {
			respondError(w, http.StatusForbidden, "user account is blocked")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	respondJSON(w, http.StatusOK, response)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Get new access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.RefreshRequest true "Refresh token"
// @Success 200 {object} models.AuthResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	response, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	respondJSON(w, http.StatusOK, response)
}

// ForgotPassword godoc
// @Summary Request password reset
// @Description Send password reset email to user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.ForgotPasswordRequest true "Email address"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Router /api/v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	// Find user by email
	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't reveal if email exists or not
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the email exists, a password reset link will be sent",
		})
		return
	}

	// Create reset token (expires in 1 hour)
	token, err := h.resetTokenRepo.Create(r.Context(), user.ID, time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	// Send email (async)
	go h.emailService.SendPasswordResetEmail(user.Email, token.Token)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the email exists, a password reset link will be sent",
	})
}

// ResetPassword godoc
// @Summary Reset password
// @Description Reset password using token from email
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Router /api/v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	// Validate token
	resetToken, err := h.resetTokenRepo.GetByToken(r.Context(), req.Token)
	if err != nil {
		if err == repository.ErrTokenNotFound || err == repository.ErrTokenExpired {
			respondError(w, http.StatusBadRequest, "invalid or expired reset token")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to validate token")
		return
	}

	// Update password
	if err := h.authService.UpdatePassword(r.Context(), resetToken.UserID, req.NewPassword); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	// Delete the used token
	h.resetTokenRepo.DeleteByToken(r.Context(), req.Token)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Password has been reset successfully",
	})
}
