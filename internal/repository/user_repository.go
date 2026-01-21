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
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrReferralCodeExists = errors.New("referral code already exists")
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, name, phone, role, bank_name, account_number, account_name, referral_code, is_blocked)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Phone, user.Role,
		user.BankName, user.AccountNumber, user.AccountName, user.ReferralCode, user.IsBlocked,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrUserExists
		}
		return err
	}

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, name, phone, role, bank_name, account_number, account_name, referral_code, is_blocked, created_at, updated_at
		FROM users WHERE id = $1
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone, &user.Role,
		&user.BankName, &user.AccountNumber, &user.AccountName, &user.ReferralCode, &user.IsBlocked,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, name, phone, role, bank_name, account_number, account_name, referral_code, is_blocked, created_at, updated_at
		FROM users WHERE email = $1
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone, &user.Role,
		&user.BankName, &user.AccountNumber, &user.AccountName, &user.ReferralCode, &user.IsBlocked,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users SET 
			name = $2, phone = $3, bank_name = $4, account_number = $5, account_name = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		user.ID, user.Name, user.Phone, user.BankName, user.AccountNumber, user.AccountName,
	).Scan(&user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}

	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *UserRepository) List(ctx context.Context, role models.Role, page, perPage int) ([]models.User, int64, error) {
	offset := (page - 1) * perPage

	// Count query
	countQuery := `SELECT COUNT(*) FROM users WHERE role = $1`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery, role).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List query
	query := `
		SELECT id, email, password_hash, name, phone, role, bank_name, account_number, account_name, referral_code, is_blocked, created_at, updated_at
		FROM users WHERE role = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, role, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone, &user.Role,
			&user.BankName, &user.AccountNumber, &user.AccountName, &user.ReferralCode, &user.IsBlocked,
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}

	return users, total, nil
}

func (r *UserRepository) ListStudents(ctx context.Context, page, perPage int) ([]models.StudentResponse, int64, error) {
	offset := (page - 1) * perPage

	// Count query (users with role 'user')
	countQuery := `SELECT COUNT(*) FROM users WHERE role = 'user'`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List query with JOINs
	query := `
		SELECT 
			u.id, u.email, u.password_hash, u.name, u.phone, u.role, 
			u.bank_name, u.account_number, u.account_name, u.referral_code, u.is_blocked, u.created_at, u.updated_at,
			COALESCE(r.course, '-') as course,
			COALESCE(ref_u.name, '-') as referred_by
		FROM users u
		LEFT JOIN referrals r ON u.email = r.referred_email
		LEFT JOIN users ref_u ON r.referrer_id = ref_u.id
		WHERE u.role = 'user'
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var students []models.StudentResponse
	for rows.Next() {
		var s models.StudentResponse
		if err := rows.Scan(
			&s.ID, &s.Email, &s.PasswordHash, &s.Name, &s.Phone, &s.Role,
			&s.BankName, &s.AccountNumber, &s.AccountName, &s.ReferralCode, &s.IsBlocked,
			&s.CreatedAt, &s.UpdatedAt,
			&s.Course, &s.ReferredBy,
		); err != nil {
			return nil, 0, err
		}
		students = append(students, s)
	}

	return students, total, nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(&exists)
	return exists, err
}

func (r *UserRepository) ExistsByReferralCode(ctx context.Context, code string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE referral_code = $1)`
	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, code).Scan(&exists)
	return exists, err
}

func (r *UserRepository) GetByReferralCode(ctx context.Context, code string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, name, phone, role, bank_name, account_number, account_name, referral_code, is_blocked, created_at, updated_at
		FROM users WHERE referral_code = $1
	`

	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, code).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone, &user.Role,
		&user.BankName, &user.AccountNumber, &user.AccountName, &user.ReferralCode, &user.IsBlocked,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// UpdatePassword updates a user's password hash
func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`
	result, err := r.db.Pool.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateStatus updates a user's blocked status
func (r *UserRepository) UpdateStatus(ctx context.Context, userID uuid.UUID, isBlocked bool) error {
	query := `UPDATE users SET is_blocked = $2, updated_at = NOW() WHERE id = $1`
	result, err := r.db.Pool.Exec(ctx, query, userID, isBlocked)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func isDuplicateKeyError(err error) bool {
	return err != nil && (
	// PostgreSQL unique violation error code
	err.Error() == "ERROR: duplicate key value violates unique constraint" ||
		// Check for constraint violation in error message
		contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "unique constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
