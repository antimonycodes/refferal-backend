package main

import (
	"context"
	"log"
	"time"

	"github.com/cirvee/referral-backend/internal/config"
	"github.com/cirvee/referral-backend/internal/database"
	"github.com/cirvee/referral-backend/internal/repository"
	"github.com/cirvee/referral-backend/internal/services"
	"github.com/cirvee/referral-backend/internal/utils"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check if admin credentials are provided
	if cfg.Admin.Email == "" || cfg.Admin.Password == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD must be set in environment")
	}

	// Connect to database
	db, err := database.New(cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize components
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)
	userRepo := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepo, jwtManager)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if admin already exists
	exists, err := userRepo.ExistsByEmail(ctx, cfg.Admin.Email)
	if err != nil {
		log.Fatalf("Failed to check existing admin: %v", err)
	}

	if exists {
		log.Printf("Admin user already exists: %s", cfg.Admin.Email)
		return
	}

	// Create admin
	user, err := authService.CreateAdmin(ctx, cfg.Admin.Email, cfg.Admin.Password, cfg.Admin.Name)
	if err != nil {
		log.Fatalf("Failed to create admin: %v", err)
	}

	log.Printf("Admin user created successfully!")
	log.Printf("Email: %s", user.Email)
	log.Printf("Name: %s", user.Name)
	log.Printf("Referral Code: %s", user.ReferralCode)
}
