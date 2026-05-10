// Package testutil is shared test infrastructure for integration tests.
//
// Build-tagged so it does not bloat normal `go build` (testcontainers and
// goose pull in large dep trees).
//go:build integration

package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // register pgx driver as "pgx" for database/sql
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/migrations"
)

// PostgresFixture holds a live Postgres container and a connection pool to it.
// Call Close from a t.Cleanup to terminate the container.
type PostgresFixture struct {
	Pool      *pgxpool.Pool
	DSN       string
	container testcontainers.Container
}

// StartPostgres boots a fresh Postgres 16 container, runs all goose
// migrations against it via the embedded migrations FS, and returns a pool
// connected to the new database.
//
// One container per call: tests are isolated by getting their own database.
// This trades a few seconds of startup per test for true independence — no
// flaky cross-test state.
func StartPostgres(t *testing.T) *PostgresFixture {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("japan_concierge_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("get connection string: %v", err)
	}

	if err := runMigrations(ctx, dsn); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("open pool: %v", err)
	}

	fix := &PostgresFixture{Pool: pool, DSN: dsn, container: container}
	t.Cleanup(fix.Close)
	return fix
}

// Close shuts down the pool and terminates the container. Safe to call twice.
func (f *PostgresFixture) Close() {
	if f.Pool != nil {
		f.Pool.Close()
		f.Pool = nil
	}
	if f.container != nil {
		_ = f.container.Terminate(context.Background())
		f.container = nil
	}
}

func runMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql.DB for migrations: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	// Quiet logger; tests don't need migration progress in their output.
	goose.SetLogger(goose.NopLogger())

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
