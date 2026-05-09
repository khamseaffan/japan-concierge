# japan-concierge

A compliance state-machine for the Japan immigration journey. Tracks every obligation, deadline, and document from pre-arrival through your first year — visa-aware, deadline-aware, and grounded in real legal sources.

Built initially as a personal tool for a J-FIND visa applicant moving from NYC to Tokyo, designed from day one to be sharable with other foreigners on different visa paths.

## Status

Early development. Phase 1 (compliance tracker) in progress.

- [x] Postgres schema (multi-tenant ready)
- [x] Rule engine (pure Go, YAML-driven, visa-agnostic)
- [x] Rules for J-FIND and Engineer/Specialist in Humanities/International Services
- [ ] HTTP API (Chi)
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

backend/internal/domain    Pure domain types (Visa, Task, Event, Document).

backend/internal/db        Postgres schema, migrations, sqlc-generated queries.

backend/internal/http      Chi handlers + middleware. The dirty I/O boundary.

backend/internal/service   Orchestrates the engine and persistence.
```

The rules engine has zero dependencies on the database or HTTP. It is a pure function: `(rule sets, input event) → generated tasks`. This is the SOLID core — open for extension (new visas), closed for modification (one engine).

## Quick start

Requirements: Docker, Go 1.23+, [`goose`](https://github.com/pressly/goose).

```bash
# Start Postgres + MinIO
make db-up

# Apply schema
make migrate

# Run the rule engine tests
make test-rules
```

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

TODO: BSL-1.1 with 4-year change date to Apache 2.0. License file pending.

In the meantime: this code is shared publicly for review and learning. Please do not redistribute or commercialize.

## Contributing

Not accepting external pull requests at this stage. Issues with bug reports or rule corrections (e.g., outdated municipal procedures, incorrect deadline math) are welcome.
