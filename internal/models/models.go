package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	Name          string    `json:"name"`
	Phone         string    `json:"phone"`
	Role          Role      `json:"role"`
	BankName      string    `json:"bank_name,omitempty"`
	AccountNumber string    `json:"account_number,omitempty"`
	AccountName   string    `json:"account_name,omitempty"`
	ReferralCode  string    `json:"referral_code"`
	IsBlocked     bool      `json:"is_blocked"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Referral struct {
	ID            uuid.UUID  `json:"id"`
	ReferrerID    *uuid.UUID `json:"referrer_id"`
	ReferrerName  string     `json:"referrer_name,omitempty"`
	ReferredName  string     `json:"referred_name"`
	ReferredEmail string     `json:"referred_email"`
	ReferredPhone string     `json:"referred_phone"`
	Course        string     `json:"course"`
	CoursePrice   int64      `json:"course_price"`
	Earnings      int64      `json:"earnings"`
	Status        string     `json:"status"` // pending, paid
	CreatedAt     time.Time  `json:"created_at"`
}

type ReferrerStats struct {
	ReferrerID    uuid.UUID `json:"referrer_id"`
	ReferrerName  string    `json:"referrer_name"`
	ReferralCode  string    `json:"referral_code"`
	TotalUsage    int       `json:"total_usage"`
	TotalEarnings int64     `json:"total_earnings"`
	Status        string    `json:"status"` // active (usage > 0), inactive
	IsBlocked     bool      `json:"is_blocked"`
}

type PayoutStatus string

const (
	PayoutStatusPending  PayoutStatus = "pending"
	PayoutStatusApproved PayoutStatus = "approved"
	PayoutStatusRejected PayoutStatus = "rejected"
)

type Payout struct {
	ID         uuid.UUID    `json:"id"`
	UserID     uuid.UUID    `json:"user_id"`
	Amount     int64        `json:"amount"`
	Status     PayoutStatus `json:"status"`
	ApprovedBy *uuid.UUID   `json:"approved_by,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	PaidAt     *time.Time   `json:"paid_at,omitempty"`
}

// Request/Response DTOs
type RegisterRequest struct {
	Email         string `json:"email" validate:"required,email"`
	Password      string `json:"password" validate:"required,min=8"`
	Name          string `json:"name" validate:"required,min=2"`
	Phone         string `json:"phone" validate:"required"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type UpdateProfileRequest struct {
	Name          string `json:"name" validate:"omitempty,min=2"`
	Phone         string `json:"phone"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type UpdatePayoutStatusRequest struct {
	Status PayoutStatus `json:"status" validate:"required,oneof=approved rejected"`
}

type UpdateReferralStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

type StudentRegistrationRequest struct {
	Name         string `json:"name" validate:"required,min=2"`
	Email        string `json:"email" validate:"required,email"`
	Phone        string `json:"phone" validate:"required"`
	Course       string `json:"course" validate:"required"`
	ReferralCode string `json:"referral_code"`
}

type DashboardStats struct {
	TotalEarnings      int64 `json:"total_earnings"`
	PendingBalance     int64 `json:"pending_balance"`
	TotalPaidEarnings  int64 `json:"total_paid_earnings"`
	PaidCount          int   `json:"paid_count"`
	TotalReferrals     int   `json:"total_referrals"`
	TotalClicks        int   `json:"total_clicks"`
	TotalPayouts       int64 `json:"total_payouts"`
	ActiveCodes        int   `json:"active_codes"`
	TotalCodes         int   `json:"total_codes"`
	TotalStudents      int   `json:"total_students"`
	TotalUniqueCourses int   `json:"total_unique_courses"`
	MonthlyRevenue     int64 `json:"monthly_revenue"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type BlockUserRequest struct {
	IsBlocked bool `json:"is_blocked"`
}

type StudentResponse struct {
	User
	Course     string `json:"course"`
	ReferredBy string `json:"referred_by"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
