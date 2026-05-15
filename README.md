# japan-concierge

A compliance state-machine for the Japan immigration journey. Tracks every obligation, deadline, and document from pre-arrival through your first year — visa-aware, deadline-aware, and grounded in real legal sources.

Built initially as a personal tool for a J-FIND visa applicant moving from NYC to Tokyo, designed from day one to be sharable with other foreigners on different visa paths.

## Status

Early development. Phase 1 (compliance tracker) in progress.

- [x] Postgres schema (multi-tenant ready)
- [x] Rule engine (pure Go, YAML-driven, visa-agnostic)
- [x] Rules for J-FIND and Engineer/Specialist in Humanities/International Services
- [x] HTTP API (Chi) — `POST /visas`, `GET /visas/active`, `POST /life-events`, `GET /tasks`, `POST /tasks/{id}/done`, `GET /healthz`
- [x] Integration tests (testcontainers)
- [x] CI/CD pipeline (GitHub Actions + GHCR image publishing)
- [ ] Frontend pre-arrival mode (Next.js PWA)
- [ ] Frontend post-landing tracker
- [ ] Document upload + storage
- [ ] OCR pipeline (Google Cloud Vision)
- [ ] Document classification + structured extraction (Claude)
- [ ] Auto-update tracker from parsed documents
- [ ] Reminders + notifications
- [ ] Hardening for sharing (auth, ToS, rate limiting)

## Architecture

```
backend/internal/rules     Pure rule engine. No DB, no HTTP, no clock.
                           Rules live as YAML data, not code branches.
                           Adding a visa = adding a YAML file + tests.

backend/internal/db        Postgres schema (migrations), hand-written
                           queries (queries/*.sql), sqlc-generated Go.

backend/internal/service   Orchestrates engine + persistence inside
                           transactions. Each service exposes a narrow
                           Querier interface (interface segregation).

backend/internal/http      Chi handlers + middleware. The dirty I/O
                           boundary. Per-request slog with request_id
                           threaded through context.

backend/internal/testutil  Testcontainers + goose runner for integration
                           tests (build-tagged `integration`).
```

The rules engine has zero dependencies on the database or HTTP. It is a pure function: `(rule sets, input event) → generated tasks`. This is the SOLID core — open for extension (new visas), closed for modification (one engine).

See [`docs/decisions/`](docs/decisions/) for the architecture decisions and the reasoning behind each choice.

## Quick start

Requirements: Docker, Go 1.23+, [`goose`](https://github.com/pressly/goose).

```bash
# Start Postgres + MinIO
make db-up

# Apply schema
make migrate

# Fast tests (rule engine only)
make test-rules

# Integration tests (boots throwaway Postgres containers, ~10s)
make test-integration

# Build and run the server
make build
make run
```

## Try it: full session in curl

After `make db-up`, `make migrate`, and `make run`, walk through the pre-arrival flow:

```bash
# 1. Catalog of supported visa types (UI populates a picker from this)
curl -s localhost:8080/api/v1/visa-types

# 2. No visa yet -> 404 (frontend uses this to redirect to onboarding)
curl -i localhost:8080/api/v1/visas/active

# 3. Create a J-FIND planning visa
curl -s -X POST localhost:8080/api/v1/visas \
  -H 'Content-Type: application/json' \
  -d '{"visa_type_code":"jfind","notes":"NYC, gathering documents"}'

# 4. Log "I started my application today" -> 4 J-FIND pre-arrival tasks come back
curl -s -X POST localhost:8080/api/v1/life-events \
  -H 'Content-Type: application/json' \
  -d '{"event_type":"visa_application_started","occurred_at":"2026-05-10"}'

# 5. Log "I landed" -> 4 post-landing tasks with 14-day and 365-day deadlines
curl -s -X POST localhost:8080/api/v1/life-events \
  -H 'Content-Type: application/json' \
  -d '{"event_type":"landed_japan","occurred_at":"2026-08-01"}'

# 6. List every task, filter by category or status
curl -s localhost:8080/api/v1/tasks
curl -s 'localhost:8080/api/v1/tasks?category=municipal'
curl -s 'localhost:8080/api/v1/tasks?status=pending&visa_id=1'

# 7. Mark a task done (replace 1 with the task id you want to complete).
#    Returns 200 + the updated task with completed_at set; 409 if already done;
#    404 if the task does not exist; 400 if the id is not a positive integer.
curl -s -X POST localhost:8080/api/v1/tasks/1/done

# 8. Health check pings Postgres (use this for cloud readiness probes)
curl -s localhost:8080/healthz
```

Every server log line includes `request_id`, `method`, `path`, and the `source` file/function/line of the log call, making it trivial to follow a single request across handlers and services.

## CI/CD

GitHub Actions runs formatting checks, `go mod tidy` verification, `go vet`, unit tests, integration tests, and a server build on pull requests and pushes to `main`. Pushes to `main`, version tags like `v1.2.3`, and manual runs also build and publish the backend container image to GitHub Container Registry at `ghcr.io/khamseaffan/japan-concierge/backend`.

## Visa coverage

v1 ships with rules for two visa types:

| Code       | Display name                                                    | Status   |
|------------|-----------------------------------------------------------------|----------|
| `jfind`    | J-FIND (Designated Activities — Future Creation Activities)     | v1       |
| `engineer` | Engineer/Specialist in Humanities/International Services        | v1       |
| `hsp`      | Highly Skilled Professional                                     | planned  |
| `student`  | Student                                                         | planned  |
| `spouse`   | Spouse of Japanese National                                     | planned  |

Adding a new visa is a YAML file in `backend/internal/rules/data/` plus tests in `backend/internal/rules/engine_test.go`. No engine code changes.

## License

[Business Source License 1.1](LICENSE), converting to Apache License 2.0 on 2030-05-10.

You may use this code for non-production purposes (reading, learning, personal evaluation) and for personal or internal production use. You may NOT offer it to third parties on a hosted or embedded basis. See [ADR-0002](docs/decisions/0002-bsl-license.md) for the reasoning.

## Contributing

Not accepting external pull requests at this stage. Issues with bug reports or rule corrections (e.g., outdated municipal procedures, incorrect deadline math) are welcome.
