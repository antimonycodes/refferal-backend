package services

import (
	"context"
	"errors"

	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/utils"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserBlocked        = errors.New("user account is blocked")
)

type AuthService struct {
	userRepo   *repository.UserRepository
	jwtManager *utils.JWTManager
}

func NewAuthService(userRepo *repository.UserRepository, jwtManager *utils.JWTManager) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	// Check if email exists
	exists, err := s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Hash password
	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Generate unique referral code
	referralCode := utils.GenerateReferralCode(req.Name)
	for {
		exists, err := s.userRepo.ExistsByReferralCode(ctx, referralCode)
		if err != nil {
			return nil, err
		}
		if !exists {
			break
		}
		referralCode = utils.GenerateReferralCode(req.Name)
	}

	// Create user
	user := &models.User{
		ID:            uuid.New(),
		Email:         req.Email,
		PasswordHash:  passwordHash,
		Name:          req.Name,
		Phone:         req.Phone,
		Role:          models.RoleUser,
		BankName:      req.BankName,
		AccountNumber: req.AccountNumber,
		AccountName:   req.AccountName,
		ReferralCode:  referralCode,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check if user is blocked
	if user.IsBlocked {
		return nil, ErrUserBlocked
	}

	// Check password
	if !utils.CheckPassword(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Generate new tokens
	newAccessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		User:         *user,
	}, nil
}

func (s *AuthService) CreateAdmin(ctx context.Context, email, password, name string) (*models.User, error) {
	// Check if email exists
	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Hash password
	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Generate referral code
	referralCode := utils.GenerateReferralCode(name)

	// Create admin user
	user := &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		Phone:        "",
		Role:         models.RoleAdmin,
		ReferralCode: referralCode,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// UpdatePassword updates a user's password
func (s *AuthService) UpdatePassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	// Hash new password
	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password in database
	return s.userRepo.UpdatePassword(ctx, userID, passwordHash)
}
