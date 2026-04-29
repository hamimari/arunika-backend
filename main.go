package main

import (
	"arunika_backend/config"
	"arunika_backend/registry"
	"arunika_backend/routes"
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env FIRST so that all subsequent env-var reads (including
	// validateEnv) see the values defined in the file.
	// godotenv.Load() is intentionally lenient — it does not fail when
	// .env is absent (production environments supply vars via the OS).
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using OS environment variables")
	}

	// Initialise structured JSON logging for the whole application.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Validate required environment variables before making any connections
	validateEnv()

	config.Init()
	db := config.DB
	rdb := config.RDB

	services := registry.NewServiceRegistry(db, rdb)
	r := routes.SetupRouter(services, rdb)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine so we can listen for OS signals concurrently
	go func() {
		log.Printf("server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt / terminate signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down server…")

	// Give in-flight requests up to 30 seconds to finish
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	// Close DB and Redis connections cleanly
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	if err := rdb.Close(); err != nil {
		log.Printf("redis close error: %v", err)
	}

	log.Println("server stopped")
}

// validateEnv checks that all required environment variables are present and
// fails fast before any connections are made.
func validateEnv() {
	required := []string{
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"REDIS_HOST",
		"JWT_SECRET",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS",
		"APP_DOMAIN",
	}
	missing := false
	for _, key := range required {
		if os.Getenv(key) == "" {
			log.Printf("ERROR: required environment variable %q is not set", key)
			missing = true
		}
	}
	if missing {
		log.Fatal("aborting: one or more required environment variables are missing")
	}

	// Warn if JWT_SECRET is too short (minimum 32 characters recommended)
	if len(os.Getenv("JWT_SECRET")) < 32 {
		log.Println("WARNING: JWT_SECRET should be at least 32 characters for adequate security")
	}
}
