package repository

import (
	"context"

	"github.com/cirvee/referral-backend/internal/database"
	"github.com/google/uuid"
)

type ReferralClick struct {
	ID           uuid.UUID `json:"id"`
	ReferralCode string    `json:"referral_code"`
	UserID       uuid.UUID `json:"user_id"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
}

type ClickRepository struct {
	db *database.DB
}

func NewClickRepository(db *database.DB) *ClickRepository {
	return &ClickRepository{db: db}
}

// RecordClick records a click on a referral link
func (r *ClickRepository) RecordClick(ctx context.Context, referralCode, ipAddress, userAgent string, userID *uuid.UUID) error {
	query := `
		INSERT INTO referral_clicks (referral_code, user_id, ip_address, user_agent)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.db.Pool.Exec(ctx, query, referralCode, userID, ipAddress, userAgent)
	return err
}

// GetClickCountByUserID returns the total click count for a user's referral code
func (r *ClickRepository) GetClickCountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	// First get the user's referral code
	var referralCode string
	codeQuery := `SELECT referral_code FROM users WHERE id = $1`
	err := r.db.Pool.QueryRow(ctx, codeQuery, userID).Scan(&referralCode)
	if err != nil {
		return 0, err
	}

	// Then count clicks for that code
	countQuery := `SELECT COUNT(*) FROM referral_clicks WHERE referral_code = $1`
	var count int
	err = r.db.Pool.QueryRow(ctx, countQuery, referralCode).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetClickCountByReferralCode returns the click count for a specific referral code
func (r *ClickRepository) GetClickCountByReferralCode(ctx context.Context, referralCode string) (int, error) {
	query := `SELECT COUNT(*) FROM referral_clicks WHERE referral_code = $1`
	var count int
	err := r.db.Pool.QueryRow(ctx, query, referralCode).Scan(&count)
	return count, err
}
