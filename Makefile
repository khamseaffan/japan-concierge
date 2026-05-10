.PHONY: help db-up db-down db-reset migrate migrate-down test test-rules test-integration sqlc build run tidy

DB_URL := postgresql://japan_concierge:dev_password@localhost:5432/japan_concierge?sslmode=disable
GOOSE_DIR := backend/internal/db/migrations

help:
	@echo "Targets:"
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

db-up:
	docker compose up -d postgres minio

db-down:
	docker compose down

db-reset:
	docker compose down -v && docker compose up -d postgres minio

migrate:
	goose -dir $(GOOSE_DIR) postgres "$(DB_URL)" up

migrate-down:
	goose -dir $(GOOSE_DIR) postgres "$(DB_URL)" down

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
