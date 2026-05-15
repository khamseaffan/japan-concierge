# Japan Concierge

Moving to Japan as a foreigner means navigating a maze of legal obligations with real deadlines — register your address within 14 days, enroll in health insurance immediately, notify immigration when you change jobs. Miss one and it can affect your visa renewal, tax status, or permanent residency application. There's no single source that tells you what to do, in what order, for your specific visa type.

Japan Concierge solves this. You tell it your visa type and what's happening in your life ("I landed in Japan", "I changed employers"), and it generates a personalized compliance checklist with deadlines, legal citations, and location hints — grounded in actual immigration law, not blog posts.

Built as a personal tool for a J-FIND visa applicant moving from NYC to Tokyo. Designed from day one to work for other visa types and other people.

## How it works

Two modes, same data:

**Manual mode** (available now) — a checklist you work through yourself:

1. **Pick your visa type** — J-FIND or Engineer (more coming)
2. **Log life events** — "started application", "landed in Japan", "changed employer"
3. **Get tasks with real deadlines** — each one cites the specific law, tells you where to go, and counts down
4. **Check them off** — on your phone, on the train, in Tokyo

**Conversational mode** (available with `AI_API_KEY`, `AI_MODEL`, and `AI_BASE_URL`) — a chat interface where you describe what happened in natural language ("I went to the Shinjuku ward office and registered my address today") and the system can call backend tools to list tasks, mark tasks done, log life events, and ask what it needs next. No forms, no dropdowns — just tell it what you did.

Both modes read and write the same backend state. The conversational flow is where this becomes more than a checklist app.

The rules engine is YAML-driven: adding a new visa type means adding a YAML file, not changing code.

## Quick start

Requirements: Docker, Go 1.25+, Node.js 24+, [overmind](https://github.com/DarthSim/overmind).

```bash
make dev
```

That's it. Starts Postgres, runs migrations, launches the Go API on `:8080` and Next.js on `:3000`. Ctrl-C stops everything.

To enable `/chat`, set `AI_API_KEY`, `AI_MODEL`, and `AI_BASE_URL` in `.env` for a Responses-compatible AI provider.

See `make help` for all available targets.

## Try it: full session in curl

After `make dev`, walk through the pre-arrival flow:

```bash
# 1. What visa types are available?
curl -s localhost:8080/api/v1/visa-types

# 2. Create a J-FIND visa
curl -s -X POST localhost:8080/api/v1/visas \
  -H 'Content-Type: application/json' \
  -d '{"visa_type_code":"jfind","notes":"NYC, gathering documents"}'

# 3. Log "I started my application" -> 4 pre-arrival tasks generated
curl -s -X POST localhost:8080/api/v1/life-events \
  -H 'Content-Type: application/json' \
  -d '{"event_type":"visa_application_started","occurred_at":"2026-05-10"}'

# 4. Log "I landed in Japan" -> 4 post-landing tasks with 14-day deadlines
curl -s -X POST localhost:8080/api/v1/life-events \
  -H 'Content-Type: application/json' \
  -d '{"event_type":"landed_japan","occurred_at":"2026-08-01"}'

# 5. See all your tasks
curl -s localhost:8080/api/v1/tasks

# 6. Mark one done
curl -s -X POST localhost:8080/api/v1/tasks/1/done
```

## Architecture

```
frontend/                  Next.js 16 PWA — mobile-first, talks to Go API
                           via proxy rewrite. Screens: visa picker,
                           life-event form, task list with mark-done,
                           and conversational chat.

backend/internal/rules     Pure rule engine. No DB, no HTTP, no clock.
                           Rules live as YAML data, not code branches.
                           Adding a visa = adding a YAML file + tests.

backend/internal/db        Postgres schema (migrations), hand-written
                           queries (queries/*.sql), sqlc-generated Go.

backend/internal/service   Orchestrates engine + persistence inside
                           transactions. Each service exposes a narrow
                           Querier interface (interface segregation).
                           Conversation service wraps Responses-style
                           tool-calling around task and event actions.

backend/internal/http      Chi handlers + middleware. Per-request slog
                           with request_id threaded through context.

Procfile.dev               overmind process declarations (api + next).
Makefile                   Project command registry (make help).
```

See [`docs/decisions/`](docs/decisions/) for ADRs explaining every architectural choice.

## Status

- [x] Postgres schema (multi-tenant ready)
- [x] Rule engine (pure Go, YAML-driven, visa-agnostic)
- [x] Rules for J-FIND and Engineer/Specialist in Humanities
- [x] HTTP API — visas, life-events, tasks, health check
- [x] Integration tests (testcontainers)
- [x] CI/CD pipeline (GitHub Actions + GHCR image publishing)
- [x] Frontend pre-arrival mode (Next.js 16 PWA)
- [x] Conversational AI tool calling for tasks and life events
- [x] Dev tooling (`make dev`, overmind, Procfile)
- [ ] Frontend post-landing tracker
- [ ] Document upload + storage
- [ ] OCR pipeline (Google Cloud Vision)
- [ ] Document classification + extraction (Claude)
- [ ] Auto-update tracker from parsed documents
- [ ] Reminders + notifications
- [ ] Hardening for sharing (auth, rate limiting, privacy)

## CI/CD

GitHub Actions runs formatting checks, `go mod tidy` verification, `go vet`, unit tests, integration tests, and a server build on pull requests and pushes to `main`. Pushes to `main`, version tags like `v1.2.3`, and manual runs also build and publish the backend container image to GitHub Container Registry at `ghcr.io/khamseaffan/japan-concierge/backend`.

## Visa coverage

| Code       | Name                                                            | Status  |
|------------|-----------------------------------------------------------------|---------|
| `jfind`    | J-FIND (Designated Activities — Future Creation Activities)     | v1      |
| `engineer` | Engineer/Specialist in Humanities/International Services        | v1      |
| `hsp`      | Highly Skilled Professional                                     | planned |
| `student`  | Student                                                         | planned |
| `spouse`   | Spouse of Japanese National                                     | planned |

Adding a new visa is a YAML file in `backend/internal/rules/data/` plus tests. No engine code changes.

## License

[Business Source License 1.1](LICENSE), converting to Apache License 2.0 on 2030-05-10.

You may use this code for non-production purposes and for personal or internal production use. You may NOT offer it to third parties on a hosted or embedded basis. See [ADR-0002](docs/decisions/0002-bsl-license.md) for the reasoning.

## Contributing

Not accepting external pull requests at this stage. Issues with bug reports or rule corrections (e.g., outdated municipal procedures, incorrect deadline math) are welcome.
