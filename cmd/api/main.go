package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// @title Cirvee Referral System API
// @version 1.0
// @description Production-grade referral system API
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.New(cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	// Run migrations
	log.Println("Running migrations...")
	if err := runMigrations(cfg.Database.URL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")

	// Connect to Redis
	redisCache, err := cache.New(cfg.Redis.URL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()
	log.Println("Connected to Redis")

	// Initialize components
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// Repositories
	userRepo := repository.NewUserRepository(db)
	referralRepo := repository.NewReferralRepository(db, redisCache)
	payoutRepo := repository.NewPayoutRepository(db)
	clickRepo := repository.NewClickRepository(db)
	resetTokenRepo := repository.NewResetTokenRepository(db)

	// Services
	authService := services.NewAuthService(userRepo, jwtManager)
	emailService := services.NewEmailService(&cfg.SMTP)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, emailService, userRepo, resetTokenRepo)
	adminHandler := handlers.NewAdminHandler(userRepo, referralRepo, payoutRepo)
	userHandler := handlers.NewUserHandler(userRepo, referralRepo, clickRepo)
	studentHandler := handlers.NewStudentHandler(userRepo, referralRepo, clickRepo, emailService, &cfg.Admin)
	paystackHandler := handlers.NewPaystackHandler(&cfg.Paystack)
	healthHandler := handlers.NewHealthHandler(db, redisCache)

	// Seed Admin User
	// Seed Admin User
	seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer seedCancel()
	ensureAdminExists(seedCtx, userRepo, authService, &cfg.Admin)

	// Middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	rateLimiter := middleware.NewRateLimiter(redisCache, cfg.RateLimit.Requests, cfg.RateLimit.Window)
	authRateLimiter := middleware.NewAuthRateLimiter(redisCache, 5, time.Minute) // 5 requests per minute for auth

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(chiMiddleware.AllowContentType("application/json"))
	r.Use(func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, 1<<20) // 1MB request body limit
	})
	r.Use(middleware.SecureHeaders)
	r.Use(rateLimiter.Limit)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", healthHandler.Health)

	// Swagger docs
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/index.html")
	})
	r.Get("/docs/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, "docs/swagger.json")
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public, with stricter rate limiting)
		r.Route("/auth", func(r chi.Router) {
			r.Use(authRateLimiter.Limit) // 5 requests per minute
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.RefreshToken)
			r.Post("/forgot-password", authHandler.ForgotPassword)
			r.Post("/reset-password", authHandler.ResetPassword)
		})

		// Student routes (public)
		r.Route("/students", func(r chi.Router) {
			r.Post("/register", studentHandler.RegisterStudent)
			r.Post("/track-click", studentHandler.TrackClick)
		})

		// Banks routes (public - Paystack proxy)
		r.Route("/banks", func(r chi.Router) {
			r.Get("/", paystackHandler.ListBanks)
			r.Get("/resolve", paystackHandler.ResolveAccount)
		})

		// Admin routes (authenticated + admin only)
		r.Route("/admin", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Use(authMiddleware.RequireRole(models.RoleAdmin))

			r.Get("/dashboard", adminHandler.GetDashboard)
			r.Post("/users/{id}/block", adminHandler.BlockUser)
			r.Get("/referrals", adminHandler.GetReferrals)
			r.Post("/referrals/{id}/paid", adminHandler.MarkReferralPaid)
			r.Patch("/referrals/{id}/status", adminHandler.UpdateReferralStatus)
			r.Get("/referrers", adminHandler.GetReferrers)
			r.Post("/referrers/{id}/paid", adminHandler.MarkReferrerPaid)
			r.Get("/students", adminHandler.GetStudents)
			r.Get("/payouts", adminHandler.GetPayouts)
			r.Patch("/payouts/{id}", adminHandler.UpdatePayoutStatus)
		})

		// User routes (authenticated + user only)
		r.Route("/user", func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Use(authMiddleware.RequireRole(models.RoleUser))

			r.Get("/dashboard", userHandler.GetDashboard)
			r.Get("/referrals", userHandler.GetMyReferrals)
			r.Get("/profile", userHandler.GetProfile)
			r.Patch("/profile", userHandler.UpdateProfile)
		})
	})

	// Server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}

func runMigrations(databaseURL string) error {
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

func ensureAdminExists(ctx context.Context, userRepo *repository.UserRepository, authService *services.AuthService, cfg *config.AdminConfig) {
	if cfg.Email == "" || cfg.Password == "" {
		log.Println("Warning: ADMIN_EMAIL or ADMIN_PASSWORD not set, skipping admin creation")
		return
	}

	exists, err := userRepo.ExistsByEmail(ctx, cfg.Email)
	if err != nil {
		log.Printf("Error checking for admin existence: %v", err)
		return
	}

	if exists {
		log.Printf("Admin user already exists: %s", cfg.Email)
		return
	}

	log.Printf("Creating default admin user: %s", cfg.Email)
	user, err := authService.CreateAdmin(ctx, cfg.Email, cfg.Password, cfg.Name)
	if err != nil {
		log.Printf("Failed to create admin user: %v", err)
		return
	}

	log.Printf("Admin user created successfully! ID: %s", user.ID)
}
