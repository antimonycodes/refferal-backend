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
	ErrReferralNotFound = errors.New("referral not found")
)

type ReferralRepository struct {
	db *database.DB
}

func NewReferralRepository(db *database.DB) *ReferralRepository {
	return &ReferralRepository{db: db}
}

func (r *ReferralRepository) Create(ctx context.Context, referral *models.Referral) error {
	query := `
		INSERT INTO referrals (id, referrer_id, referred_name, referred_email, referred_phone, course, course_price, earnings, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at
	`

	return r.db.Pool.QueryRow(ctx, query,
		referral.ID, referral.ReferrerID, referral.ReferredName, referral.ReferredEmail,
		referral.ReferredPhone, referral.Course, referral.CoursePrice, referral.Earnings, referral.Status,
	).Scan(&referral.CreatedAt)
}

func (r *ReferralRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Referral, error) {
	query := `
		SELECT id, referrer_id, referred_name, referred_email, referred_phone, course, course_price, earnings, status, created_at
		FROM referrals WHERE id = $1
	`

	referral := &models.Referral{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&referral.ID, &referral.ReferrerID, &referral.ReferredName, &referral.ReferredEmail,
		&referral.ReferredPhone, &referral.Course, &referral.CoursePrice, &referral.Earnings,
		&referral.Status, &referral.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReferralNotFound
		}
		return nil, err
	}

	return referral, nil
}

func (r *ReferralRepository) ListByReferrer(ctx context.Context, referrerID uuid.UUID, page, perPage int) ([]models.Referral, int64, error) {
	offset := (page - 1) * perPage

	countQuery := `SELECT COUNT(*) FROM referrals WHERE referrer_id = $1`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery, referrerID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, referrer_id, referred_name, referred_email, referred_phone, course, course_price, earnings, status, created_at
		FROM referrals WHERE referrer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, referrerID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var referrals []models.Referral
	for rows.Next() {
		var ref models.Referral
		if err := rows.Scan(
			&ref.ID, &ref.ReferrerID, &ref.ReferredName, &ref.ReferredEmail,
			&ref.ReferredPhone, &ref.Course, &ref.CoursePrice, &ref.Earnings,
			&ref.Status, &ref.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		referrals = append(referrals, ref)
	}

	return referrals, total, nil
}

func (r *ReferralRepository) ListAll(ctx context.Context, page, perPage int) ([]models.Referral, int64, error) {
	offset := (page - 1) * perPage

	countQuery := `SELECT COUNT(*) FROM referrals`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT r.id, r.referrer_id, COALESCE(u.name, '-') as referrer_name, r.referred_name, r.referred_email, r.referred_phone, r.course, r.course_price, r.earnings, r.status, r.created_at
		FROM referrals r
		LEFT JOIN users u ON r.referrer_id = u.id
		ORDER BY r.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var referrals []models.Referral
	for rows.Next() {
		var ref models.Referral
		if err := rows.Scan(
			&ref.ID, &ref.ReferrerID, &ref.ReferrerName, &ref.ReferredName, &ref.ReferredEmail,
			&ref.ReferredPhone, &ref.Course, &ref.CoursePrice, &ref.Earnings,
			&ref.Status, &ref.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		referrals = append(referrals, ref)
	}

	return referrals, total, nil
}

func (r *ReferralRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE referrals SET status = $2 WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id, status)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrReferralNotFound
	}

	return nil
}

func (r *ReferralRepository) GetStatsByReferrer(ctx context.Context, referrerID uuid.UUID) (totalCount int, totalEarnings int64, pendingEarnings int64, err error) {
	query := `
		SELECT 
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN earnings ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN earnings ELSE 0 END), 0)
		FROM referrals WHERE referrer_id = $1
	`
	err = r.db.Pool.QueryRow(ctx, query, referrerID).Scan(&totalCount, &totalEarnings, &pendingEarnings)
	return
}

func (r *ReferralRepository) GetTotalStats(ctx context.Context) (totalReferrals int, totalEarnings int64, pendingEarnings int64, totalPaidEarnings int64, paidCount int, totalCodes int, activeCodes int, totalEnrollments int, totalUniqueCourses int, err error) {
	// Total Referrals (with a referrer), Total Earnings, Pending Earnings, Paid Earnings - Include all referrals
	query1 := `
		SELECT 
			COUNT(CASE WHEN r.referrer_id IS NOT NULL THEN r.id END), 
			COALESCE(SUM(r.earnings), 0),
			COALESCE(SUM(CASE WHEN r.status = 'pending' THEN r.earnings ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN r.status = 'paid' THEN r.earnings ELSE 0 END), 0),
			COUNT(CASE WHEN r.status = 'paid' THEN r.id END)
		FROM referrals r
		LEFT JOIN users u ON r.referrer_id = u.id
		WHERE u.role IS NULL OR u.role != 'admin'
	`
	if err = r.db.Pool.QueryRow(ctx, query1).Scan(&totalReferrals, &totalEarnings, &pendingEarnings, &totalPaidEarnings, &paidCount); err != nil {
		return
	}

	// Total Referral Codes (users with a code) - Exclude Admins
	query2 := `SELECT COUNT(*) FROM users WHERE referral_code IS NOT NULL AND referral_code != '' AND role != 'admin'`
	if err = r.db.Pool.QueryRow(ctx, query2).Scan(&totalCodes); err != nil {
		return
	}

	// Active Referral Codes (referrers with at least one referral) - Exclude Admins
	query3 := `
		SELECT COUNT(DISTINCT r.referrer_id) 
		FROM referrals r
		JOIN users u ON r.referrer_id = u.id
		WHERE u.role != 'admin'
	`
	if err = r.db.Pool.QueryRow(ctx, query3).Scan(&activeCodes); err != nil {
		return
	}

	// Total Enrollments (All referrals/students)
	query4 := `SELECT COUNT(*) FROM referrals`
	if err = r.db.Pool.QueryRow(ctx, query4).Scan(&totalEnrollments); err != nil {
		return
	}

	// Total Unique Courses
	query5 := `SELECT COUNT(DISTINCT course) FROM referrals`
	err = r.db.Pool.QueryRow(ctx, query5).Scan(&totalUniqueCourses)
	return
}

func (r *ReferralRepository) GetReferrersStats(ctx context.Context, page, perPage int) ([]models.ReferrerStats, int64, error) {
	offset := (page - 1) * perPage

	// Count users who have a referral code
	countQuery := `SELECT COUNT(*) FROM users WHERE referral_code IS NOT NULL AND referral_code != ''`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Query to aggregate stats per referrer
	query := `
		SELECT 
			u.id, 
			u.name, 
			u.referral_code, 
			COUNT(r.id) as total_usage, 
			COALESCE(SUM(r.earnings), 0) as total_earnings,
			u.is_blocked
		FROM users u
		LEFT JOIN referrals r ON u.id = r.referrer_id
		WHERE u.referral_code IS NOT NULL AND u.referral_code != '' AND u.role != 'admin'
		GROUP BY u.id, u.name, u.referral_code, u.is_blocked
		ORDER BY total_earnings DESC, total_usage DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var stats []models.ReferrerStats
	for rows.Next() {
		var s models.ReferrerStats
		if err := rows.Scan(
			&s.ReferrerID, &s.ReferrerName, &s.ReferralCode,
			&s.TotalUsage, &s.TotalEarnings, &s.IsBlocked,
		); err != nil {
			return nil, 0, err
		}

		// Derive status based on usage for now
		if s.TotalUsage > 0 {
			s.Status = "Active"
		} else {
			s.Status = "Inactive"
		}

		stats = append(stats, s)
	}

	return stats, total, nil
}

func (r *ReferralRepository) MarkReferralsAsPaid(ctx context.Context, referrerID uuid.UUID) error {
	query := `
		UPDATE referrals 
		SET status = 'paid' 
		WHERE referrer_id = $1 AND status = 'pending'
	`
	_, err := r.db.Pool.Exec(ctx, query, referrerID)
	return err
}

// MarkReferralAsPaid marks a single referral as paid
func (r *ReferralRepository) MarkReferralAsPaid(ctx context.Context, id uuid.UUID) error {
	return r.UpdateStatus(ctx, id, "paid")
}
