.PHONY: help dev dev-stop db-up db-down db-reset migrate migrate-down test test-rules test-integration sqlc build run tidy frontend-install

DB_URL := postgresql://japan_concierge:dev_password@localhost:5432/japan_concierge?sslmode=disable
GOOSE_DIR := backend/internal/db/migrations

help:
	@echo "Targets:"
	@echo "  dev          Start everything: postgres, migrations, Go API, Next.js frontend"
	@echo "  dev-stop     Stop background processes and containers"
	@echo "  db-up        Start postgres + minio in docker"
	@echo "  db-down      Stop containers (preserves data)"
	@echo "  db-reset     Stop and DELETE all data, then restart"
	@echo "  migrate      Apply all pending migrations"
	@echo "  migrate-down Roll back the last migration"
	@echo "  test             Run fast Go tests (excludes integration)"
	@echo "  test-rules       Run rule engine tests verbosely"
	@echo "  test-integration Run testcontainer-backed integration tests (~10s, needs Docker)"
	@echo "  sqlc             Regenerate sqlc Go code from queries/*.sql"
	@echo "  build            Compile server binary to bin/server"
	@echo "  run              Run server from source"
	@echo "  tidy             go mod tidy in backend/"
	@echo "  frontend-install Install frontend npm dependencies"

dev:
	@echo "==> Starting Postgres + MinIO..."
	docker compose up -d postgres minio
	@echo "==> Waiting for Postgres to be ready..."
	@until docker exec japan-concierge-db pg_isready -U japan_concierge -q 2>/dev/null; do sleep 1; done
	@echo "==> Running migrations..."
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir internal/db/migrations postgres "$(DB_URL)" up
	@echo "==> Installing frontend dependencies (if needed)..."
	@cd frontend && npm install --silent
	@echo "==> Starting Go API (port 8080) and Next.js (port 3000)..."
	overmind start -f Procfile.dev

dev-stop:
	overmind quit || true
	docker compose down

db-up:
	docker compose up -d postgres minio

db-down:
	docker compose down

db-reset:
	docker compose down -v && docker compose up -d postgres minio

migrate:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir internal/db/migrations postgres "$(DB_URL)" up

migrate-down:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir internal/db/migrations postgres "$(DB_URL)" down

test:
	cd backend && go test ./...

test-rules:
	cd backend && go test ./internal/rules/... -v

test-integration:
	cd backend && go test -tags=integration -count=1 -timeout=300s ./...

sqlc:
	cd backend && sqlc generate

build:
	cd backend && go build -o ../bin/server ./cmd/server

run:
	cd backend && go run ./cmd/server

tidy:
	cd backend && go mod tidy

frontend-install:
	cd frontend && npm install
