package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/database"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("[SEED ERROR] Failed to load configuration: %v", err)
	}

	if cfg.SeedAdminEmail == "" || cfg.SeedAdminPassword == "" {
		log.Fatal("[SEED ERROR] SEED_ADMIN_EMAIL and SEED_ADMIN_PASSWORD must be set in .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[SEED ERROR] Failed to connect to database: %v", err)
	}
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("[SEED ERROR] Failed to hash password: %v", err)
	}

	var userID string
	var existingID string
	checkQuery := `SELECT id FROM users WHERE LOWER(email) = LOWER($1)`
	err = pool.QueryRow(ctx, checkQuery, cfg.SeedAdminEmail).Scan(&existingID)

	if err == nil {
		userID = existingID
		updateQuery := `
			UPDATE users
			SET password_hash = $1,
			    name = $2,
			    is_password_configured = TRUE,
			    is_active = TRUE,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`
		if _, err := pool.Exec(ctx, updateQuery, string(hash), cfg.SeedAdminName, existingID); err != nil {
			log.Fatalf("[SEED ERROR] Failed to update existing admin user: %v", err)
		}
	} else {
		insertQuery := `
			INSERT INTO users (email, password_hash, name, is_password_configured, is_active)
			VALUES ($1, $2, $3, TRUE, TRUE)
			RETURNING id
		`
		if err := pool.QueryRow(ctx, insertQuery, cfg.SeedAdminEmail, string(hash), cfg.SeedAdminName).Scan(&userID); err != nil {
			log.Fatalf("[SEED ERROR] Failed to insert admin user: %v", err)
		}
	}

	// Map admin to default competition in user_competitions if the table exists
	compRoleQuery := `
		INSERT INTO user_competitions (user_id, competition_id, role)
		VALUES ($1, '00000000-0000-0000-0000-000000000001', 'owner')
		ON CONFLICT (competition_id, user_id) 
		DO UPDATE SET role = 'owner', updated_at = CURRENT_TIMESTAMP
	`
	_, _ = pool.Exec(ctx, compRoleQuery, userID)

	fmt.Println("==================================================")
	fmt.Println(" [SEED] Admin user seeded successfully")
	fmt.Printf(" Email : %s\n", cfg.SeedAdminEmail)
	fmt.Printf(" Name  : %s\n", cfg.SeedAdminName)
	fmt.Println(" Role  : owner (in default competition)")
	fmt.Println(" Status: password configured & active")
	fmt.Println("==================================================")
}
