package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@127.0.0.1:5434/lms?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	targetEmail := "saksham.s25397@nst.rishihood.edu.in"

	var id, email, name, role string
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active)
		VALUES (LOWER($1), 'Saksham Saini', 'dev', TRUE)
		ON CONFLICT (email) DO UPDATE
		SET role = 'dev',
		    is_active = TRUE,
		    deleted_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, email, name, role
	`, targetEmail).Scan(&id, &email, &name, &role)
	if err != nil {
		log.Fatalf("failed to grant dev role: %v", err)
	}

	fmt.Printf("Granted dev role successfully:\n  ID:    %s\n  Email: %s\n  Name:  %s\n  Role:  %s\n", id, email, name, role)
}