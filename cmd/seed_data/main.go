package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cirvee/referral-backend/internal/config"
	"github.com/cirvee/referral-backend/internal/database"
	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 1. Get or Create a User
	var userID uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT id FROM users WHERE email = 'testuser@example.com'").Scan(&userID)
	if err != nil {
		// Create user
		userID = uuid.New()
		_, err = db.Pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, name, phone, role, bank_name, account_number, account_name, referral_code, created_at, updated_at)
			VALUES ($1, 'testuser@example.com', 'hash', 'Test User', '08012345678', 'user', 'GTBank', '0123456789', 'Test User', 'TEST01', NOW(), NOW())
		`, userID)
		if err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		fmt.Println("Created Test User")
	} else {
		fmt.Println("Found Test User")
	}

	// 2. Insert Payouts
	payoutID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO payouts (id, user_id, amount, status, created_at)
		VALUES ($1, $2, 50000, 'pending', NOW())
	`, payoutID, userID)
	if err != nil {
		log.Printf("Failed to insert payout: %v", err)
	} else {
		fmt.Printf("Inserted Pending Payout: %s\n", payoutID)
	}

	payoutID2 := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO payouts (id, user_id, amount, status, created_at)
		VALUES ($1, $2, 25000, 'approved', NOW() - INTERVAL '1 day')
	`, payoutID2, userID)
	if err != nil {
		log.Printf("Failed to insert payout 2: %v", err)
	} else {
		fmt.Printf("Inserted Approved Payout: %s\n", payoutID2)
	}
}
