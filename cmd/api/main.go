package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/config"
	"github.com/UncleKweks/E-Commerce/internal/database"
	"github.com/UncleKweks/E-Commerce/internal/logger"
	"github.com/UncleKweks/E-Commerce/internal/server"
)

// shutdownTimeout is how long in-flight requests get to finish after Ctrl+C
// or a deploy stops the server.
const shutdownTimeout = 10 * time.Second

func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get database connection")
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database connection")
		}
	}()
	log.Info().Msg("Database connection established successfully")

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: server.NewServer(cfg, db, &log).SetupRoutes(),
		// Timeouts stop slow or stalled clients from holding connections open
		// forever, which is an easy way to exhaust a server.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// ctx is cancelled when the process is asked to stop (Ctrl+C, or SIGTERM from Docker).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("HTTP server failed")
			stop()
		}
	}()

	<-ctx.Done()
	log.Info().Msg("Shutting down: finishing in-flight requests")

	// Graceful shutdown: stop accepting new requests, let current ones finish.
	// Without this, a deploy would cut off customers mid-checkout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Graceful shutdown did not finish in time")
	}
	log.Info().Msg("Server stopped")
}
