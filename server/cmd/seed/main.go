package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/database"
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

	var userID string
	var existingID string
	checkQuery := `SELECT id FROM users WHERE LOWER(email) = LOWER($1)`
	err = pool.QueryRow(ctx, checkQuery, cfg.SeedAdminEmail).Scan(&existingID)

	if err == nil {
		userID = existingID
		updateQuery := `
			UPDATE users
			SET name = $1,
			    global_role = 'dev',
			    is_active = TRUE,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $2
		`
		if _, err := pool.Exec(ctx, updateQuery, cfg.SeedAdminName, existingID); err != nil {
			log.Fatalf("[SEED ERROR] Failed to update existing admin user: %v", err)
		}
	} else {
		insertQuery := `
			INSERT INTO users (email, name, global_role, is_active)
			VALUES ($1, $2, 'dev', TRUE)
			RETURNING id
		`
		if err := pool.QueryRow(ctx, insertQuery, cfg.SeedAdminEmail, cfg.SeedAdminName).Scan(&userID); err != nil {
			log.Fatalf("[SEED ERROR] Failed to insert admin user: %v", err)
		}
	}

	// Ensure default event and competition exist and map admin to default event in user_events
	_, _ = pool.Exec(ctx, `
		INSERT INTO events (id, slug, name)
		VALUES ('00000000-0000-0000-0000-000000000001', 'devclub-event', 'DevClub Event')
		ON CONFLICT (id) DO NOTHING
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO competitions (id, event_id, slug, name)
		VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'devclub-hack', 'DevClub Hackathon')
		ON CONFLICT (id) DO UPDATE SET event_id = '00000000-0000-0000-0000-000000000001'
	`)
	eventRoleQuery := `
		INSERT INTO user_events (user_id, event_id, role)
		VALUES ($1, '00000000-0000-0000-0000-000000000001', 'owner')
		ON CONFLICT (event_id, user_id) 
		DO UPDATE SET role = 'owner', updated_at = CURRENT_TIMESTAMP
	`
	_, _ = pool.Exec(ctx, eventRoleQuery, userID)

	fmt.Println("==================================================")
	fmt.Println(" [SEED] Admin user seeded successfully")
	fmt.Printf(" Email : %s\n", cfg.SeedAdminEmail)
	fmt.Printf(" Name  : %s\n", cfg.SeedAdminName)
	fmt.Println(" Role  : dev (global) / owner (in default event)")
	fmt.Println(" Status: password configured & active")
	fmt.Println("==================================================")
}
