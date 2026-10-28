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

	var existingID string
	checkQuery := `SELECT id FROM users WHERE LOWER(email) = LOWER($1)`
	err = pool.QueryRow(ctx, checkQuery, cfg.SeedAdminEmail).Scan(&existingID)

	if err == nil {
		updateQuery := `
			UPDATE users
			SET password_hash = $1,
			    name = $2,
			    role = 'admin',
			    is_password_configured = TRUE,
			    is_active = TRUE,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`
		if _, err := pool.Exec(ctx, updateQuery, string(hash), cfg.SeedAdminName, existingID); err != nil {
			log.Fatalf("[SEED ERROR] Failed to update existing admin user: %v", err)
		}
		fmt.Println("==================================================")
		fmt.Println(" [SEED] Admin user updated successfully")
		fmt.Printf(" Email : %s\n", cfg.SeedAdminEmail)
		fmt.Printf(" Name  : %s\n", cfg.SeedAdminName)
		fmt.Println(" Role  : admin")
		fmt.Println(" Status: password configured & active")
		fmt.Println("==================================================")
		return
	}

	insertQuery := `
		INSERT INTO users (email, password_hash, name, role, is_password_configured, is_active)
		VALUES ($1, $2, $3, 'admin', TRUE, TRUE)
	`
	if _, err := pool.Exec(ctx, insertQuery, cfg.SeedAdminEmail, string(hash), cfg.SeedAdminName); err != nil {
		log.Fatalf("[SEED ERROR] Failed to insert admin user: %v", err)
	}

	fmt.Println("==================================================")
	fmt.Println(" [SEED] New admin user seeded successfully")
	fmt.Printf(" Email : %s\n", cfg.SeedAdminEmail)
	fmt.Printf(" Name  : %s\n", cfg.SeedAdminName)
	fmt.Println(" Role  : admin")
	fmt.Println(" Status: password configured & active")
	fmt.Println("==================================================")
}
