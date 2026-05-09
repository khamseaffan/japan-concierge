# ADR-0005: pgx/v5 + sqlc over database/sql + ORM

**Status:** Accepted
**Date:** 2026-05-09

## Context

Three options for Postgres access in Go:
- **`database/sql` + `lib/pq`:** standard library + classic driver. `lib/pq` is in maintenance mode and has weaker support for newer Postgres features (LISTEN/NOTIFY, COPY, JSONB, custom types).
- **ORM (GORM, ent, Bun):** developer-friendly but adds a layer of abstraction over SQL, hides query plans, and tends toward N+1 problems in any non-trivial query graph.
- **`pgx/v5` + `sqlc`:** `pgx` is the modern native Postgres driver with full feature support; `sqlc` generates type-safe Go from hand-written SQL.

## Decision

`pgx/v5` as the driver, `sqlc` for query generation in `pgx` mode. Hand-written SQL lives in `backend/internal/db/queries/*.sql`. Generated Go code goes to `backend/internal/db/sqlc/` and is committed to the repo (so the code is reviewable and the build doesn't require `sqlc` installed).

A narrow `Querier` interface is defined in `internal/service/`, listing only the queries a given service uses. Services depend on this interface, not on the generated `*Queries` struct directly. This is interface segregation: tests can mock the methods a service actually uses without inventing fakes for 40 unrelated queries.

## Consequences

- **Easier:** SQL stays SQL. Query plans are visible. Postgres-specific features (JSONB, generated columns, partial indexes, `ON CONFLICT`) work natively.
- **Easier:** `sqlc` generates fully-typed Go from SQL — no runtime reflection, compile-time errors when columns are renamed.
- **Easier:** `pgx/v5` uses `pgxpool.Pool` for connection pooling and `pgtype.Text` / `pgtype.Date` for nullable fields, which is more honest than `database/sql`'s `sql.NullString` ergonomics.
- **Harder:** every query is hand-written. There is no "find me by example" or "load with all relations" magic. For this project that's a feature — every query has a known cost.
- **Tradeoff:** `sqlc` requires a code-gen step in the dev loop (`make sqlc`). The Makefile target makes it one command, and CI can verify the generated code is up to date.
