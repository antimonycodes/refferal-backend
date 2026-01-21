package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/cirvee/referral-backend/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token expired")
)

type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type ResetTokenRepository struct {
	db *database.DB
}

func NewResetTokenRepository(db *database.DB) *ResetTokenRepository {
	return &ResetTokenRepository{db: db}
}

// GenerateToken generates a secure random token
func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Create creates a new password reset token
func (r *ResetTokenRepository) Create(ctx context.Context, userID uuid.UUID, expiry time.Duration) (*PasswordResetToken, error) {
	// Delete any existing tokens for this user
	_, _ = r.db.Pool.Exec(ctx, "DELETE FROM password_reset_tokens WHERE user_id = $1", userID)

	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	resetToken := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(expiry),
		CreatedAt: time.Now(),
	}

	query := `
		INSERT INTO password_reset_tokens (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = r.db.Pool.Exec(ctx, query, resetToken.ID, resetToken.UserID, resetToken.Token, resetToken.ExpiresAt, resetToken.CreatedAt)
	if err != nil {
		return nil, err
	}

	return resetToken, nil
}

// GetByToken retrieves a token by its value
func (r *ResetTokenRepository) GetByToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM password_reset_tokens WHERE token = $1
	`
	var t PasswordResetToken
	err := r.db.Pool.QueryRow(ctx, query, token).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	if time.Now().After(t.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	return &t, nil
}

// DeleteByToken deletes a token
func (r *ResetTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM password_reset_tokens WHERE token = $1", token)
	return err
}

// DeleteByUserID deletes all tokens for a user
func (r *ResetTokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM password_reset_tokens WHERE user_id = $1", userID)
	return err
}

// CleanupExpired removes expired tokens
func (r *ResetTokenRepository) CleanupExpired(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM password_reset_tokens WHERE expires_at < NOW()")
	return err
}
