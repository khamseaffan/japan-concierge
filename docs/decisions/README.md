# Architecture Decision Records (ADRs)

This directory captures the *reasoning* behind architectural choices in this project, not just the outcomes. Each ADR is short (10-30 lines), dated, and follows the same structure: **Context**, **Decision**, **Consequences**.

The goal is that a future contributor — or recruiter reviewing this repo — can understand *why* something was built the way it was, including the tradeoffs that were accepted at the time.

## Index

| ID | Title | Status |
|----|-------|--------|
| [ADR-0001](0001-go-over-python.md) | Go for the backend over Python/FastAPI | Accepted |
| [ADR-0002](0002-bsl-license.md) | Business Source License 1.1 over MIT/Apache | Accepted |
| [ADR-0003](0003-schema-level-idempotency.md) | Idempotency enforced in Postgres, not application code | Accepted |
| [ADR-0004](0004-rules-as-yaml-data.md) | Compliance rules expressed as YAML data, not Go branches | Accepted |
| [ADR-0005](0005-pgx-and-sqlc.md) | pgx/v5 + sqlc over database/sql + ORM | Accepted |

## Format

Each ADR is a markdown file named `NNNN-short-title.md` with this structure:

```markdown
# ADR-NNNN: Title

**Status:** Proposed | Accepted | Superseded by ADR-XXXX
**Date:** YYYY-MM-DD

## Context
What problem are we solving? What forces are at play?

## Decision
What did we decide to do?

## Consequences
What becomes easier? What becomes harder? What did we trade away?
```
