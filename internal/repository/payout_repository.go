package repository

import (
	"context"
	"errors"

	"github.com/cirvee/referral-backend/internal/database"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPayoutNotFound = errors.New("payout not found")
)

type PayoutRepository struct {
	db *database.DB
}

func NewPayoutRepository(db *database.DB) *PayoutRepository {
	return &PayoutRepository{db: db}
}

func (r *PayoutRepository) Create(ctx context.Context, payout *models.Payout) error {
	query := `
		INSERT INTO payouts (id, user_id, amount, status)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`

	return r.db.Pool.QueryRow(ctx, query,
		payout.ID, payout.UserID, payout.Amount, payout.Status,
	).Scan(&payout.CreatedAt)
}

func (r *PayoutRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Payout, error) {
	query := `
		SELECT id, user_id, amount, status, approved_by, created_at, paid_at
		FROM payouts WHERE id = $1
	`

	payout := &models.Payout{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&payout.ID, &payout.UserID, &payout.Amount, &payout.Status,
		&payout.ApprovedBy, &payout.CreatedAt, &payout.PaidAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPayoutNotFound
		}
		return nil, err
	}

	return payout, nil
}

func (r *PayoutRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, perPage int) ([]models.Payout, int64, error) {
	offset := (page - 1) * perPage

	countQuery := `SELECT COUNT(*) FROM payouts WHERE user_id = $1`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, amount, status, approved_by, created_at, paid_at
		FROM payouts WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payouts []models.Payout
	for rows.Next() {
		var p models.Payout
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Amount, &p.Status,
			&p.ApprovedBy, &p.CreatedAt, &p.PaidAt,
		); err != nil {
			return nil, 0, err
		}
		payouts = append(payouts, p)
	}

	return payouts, total, nil
}

func (r *PayoutRepository) ListAll(ctx context.Context, page, perPage int, status *models.PayoutStatus) ([]models.Payout, int64, error) {
	offset := (page - 1) * perPage

	var countQuery, query string
	var args []interface{}

	if status != nil {
		countQuery = `SELECT COUNT(*) FROM payouts WHERE status = $1`
		query = `
			SELECT id, user_id, amount, status, approved_by, created_at, paid_at
			FROM payouts WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{*status, perPage, offset}
	} else {
		countQuery = `SELECT COUNT(*) FROM payouts`
		query = `
			SELECT id, user_id, amount, status, approved_by, created_at, paid_at
			FROM payouts
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []interface{}{perPage, offset}
	}

	var total int64
	if status != nil {
		if err := r.db.Pool.QueryRow(ctx, countQuery, *status).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		if err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payouts []models.Payout
	for rows.Next() {
		var p models.Payout
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Amount, &p.Status,
			&p.ApprovedBy, &p.CreatedAt, &p.PaidAt,
		); err != nil {
			return nil, 0, err
		}
		payouts = append(payouts, p)
	}

	return payouts, total, nil
}

func (r *PayoutRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.PayoutStatus, approvedBy uuid.UUID) error {
	query := `
		UPDATE payouts 
		SET status = $2, approved_by = $3, paid_at = CASE WHEN $2 = 'approved' THEN NOW() ELSE paid_at END
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, id, status, approvedBy)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrPayoutNotFound
	}

	return nil
}

func (r *PayoutRepository) GetStats(ctx context.Context) (totalPayouts int64, pendingPayouts int64, err error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'approved' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN amount ELSE 0 END), 0)
		FROM payouts
	`
	err = r.db.Pool.QueryRow(ctx, query).Scan(&totalPayouts, &pendingPayouts)
	return
}
