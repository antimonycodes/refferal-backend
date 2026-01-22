package integration

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cirvee/referral-backend/internal/cache"
	"github.com/cirvee/referral-backend/internal/config"
	"github.com/cirvee/referral-backend/internal/database"
	"github.com/cirvee/referral-backend/internal/handlers"
	"github.com/cirvee/referral-backend/internal/middleware"
	"github.com/cirvee/referral-backend/internal/models"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/services"
	"github.com/cirvee/referral-backend/internal/utils"
	"github.com/go-chi/chi/v5"
)

func setupTestServer(t *testing.T) (http.Handler, func()) {
	// Use test database URL
	dbURL := os.Getenv("DATABASE_TEST_URL")
	if dbURL == "" {
		dbURL = "postgres://referral_test:referral_test_secret@localhost:5435/referral_test_db?sslmode=disable"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Connect to test database (max 10 connections)
	db, err := database.New(dbURL, 10)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Connect to Redis
	redisCache, err := cache.New(redisURL)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Clear test database
	_, err = db.Pool.Exec(context.Background(), "TRUNCATE users, referrals, payouts, referral_clicks, password_reset_tokens CASCADE")
	if err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}

	// Initialize components
	jwtManager := utils.NewJWTManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Repositories
	userRepo := repository.NewUserRepository(db)
	referralRepo := repository.NewReferralRepository(db, redisCache)
	payoutRepo := repository.NewPayoutRepository(db)
	clickRepo := repository.NewClickRepository(db)

	// Services
	authService := services.NewAuthService(userRepo, jwtManager)
	// Stub email service for testing (won't actually send emails)
	emailService := services.NewEmailService(&config.SMTPConfig{})

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, emailService, userRepo, nil)
	adminHandler := handlers.NewAdminHandler(userRepo, referralRepo, payoutRepo)
	userHandler := handlers.NewUserHandler(userRepo, referralRepo, clickRepo)
	healthHandler := handlers.NewHealthHandler(db, redisCache)

	// Middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.RefreshToken)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Use(authMiddleware.RequireRole(models.RoleAdmin))

			r.Get("/dashboard", adminHandler.GetDashboard)
			r.Get("/referrals", adminHandler.GetReferrals)
			r.Get("/students", adminHandler.GetStudents)
			r.Get("/payouts", adminHandler.GetPayouts)
			r.Patch("/payouts/{id}", adminHandler.UpdatePayoutStatus)
		})

		r.Route("/user", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Use(authMiddleware.RequireRole(models.RoleUser))

			r.Get("/dashboard", userHandler.GetDashboard)
			r.Get("/referrals", userHandler.GetMyReferrals)
			r.Get("/profile", userHandler.GetProfile)
			r.Patch("/profile", userHandler.UpdateProfile)
		})
	})

	cleanup := func() {
		db.Close()
		redisCache.Close()
	}

	return r, cleanup
}
