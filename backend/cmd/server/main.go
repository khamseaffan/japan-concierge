// Command server is the japan-concierge HTTP API entrypoint.
//
// Wiring order: load config -> open Postgres pool -> load rule sets -> build
// rule engine -> build tracker service -> build HTTP router -> listen.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamseaffan/japan-concierge/backend/internal/config"
	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
	httpapi "github.com/khamseaffan/japan-concierge/backend/internal/http"
	"github.com/khamseaffan/japan-concierge/backend/internal/rules"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	logger.Info("postgres connected")

	ruleSets, err := rules.LoadAll()
	if err != nil {
		return err
	}
	engine := rules.NewEngine(ruleSets)
	logger.Info("rules engine loaded", "visa_codes", engine.VisaCodes())

	q := sqlc.New(pool)
	svcs := httpapi.Services{
		Tracker: service.NewTrackerService(pool, engine),
		Visas:   service.NewVisaService(q),
		Tasks:   service.NewTaskService(q),
	}

	router := httpapi.NewRouter(cfg, logger, svcs, pool)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("http server starting", "addr", cfg.HTTPAddr, "single_user_mode", cfg.SingleUserMode)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	select {
	case err := <-serverErrCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}
