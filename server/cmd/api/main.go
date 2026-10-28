package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devclubxnst.com/cadence/internal/bootstrap"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.NewApplication(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer app.DB.Close()
	defer app.Redis.Close()
	if app.MailQueue != nil {
		defer app.MailQueue.Close()
	}
	defer app.Mailer.Close()

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", app.Config.Port),
		Handler:      app.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on port %s (env: %s)", app.Config.Port, app.Config.Env)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server listen failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server terminated cleanly")
}
