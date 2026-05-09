# ADR-0003: Idempotency enforced in Postgres, not application code

**Status:** Accepted
**Date:** 2026-05-09

## Context

The rules engine is pure: the same `(visa, event, rules)` input always produces the same `[]GeneratedTask` output. But persistence is where double-firing becomes a real failure mode: re-processing an event (because of a worker retry, a UI double-submit, or eventually a multi-worker job queue race) would otherwise produce duplicate `compliance_tasks` rows.

Two places to fix it:
- **Application:** check before insert, dedup in service code with explicit transactions and SELECT-then-INSERT.
- **Schema:** add a UNIQUE constraint that makes duplicates impossible by construction.

## Decision

Schema-level. The `compliance_tasks` table has `UNIQUE (user_id, rule_id, triggered_by_event_id)`. Service code uses `INSERT ... ON CONFLICT DO NOTHING` and treats the constraint violation path as a no-op.

## Consequences

- **Easier:** correctness becomes a property of the database, not careful coding. Any code path that inserts tasks — current `RecordLifeEvent`, future job-queue retries, future bulk imports — is automatically idempotent. New developers can't break this without dropping the constraint.
- **Easier:** parallel workers racing on the same event are serialized by Postgres, not by application-level locks.
- **Harder (slightly):** the service must use `ON CONFLICT DO NOTHING` and inspect the returned row count to know whether it actually inserted anything. Tiny ergonomics cost.
- **Tradeoff:** the unique key includes `triggered_by_event_id`, which means re-firing the *same event* is a no-op, but logging two *different* events of the same type produces two sets of tasks. That is the desired behavior — "I logged 'address changed' twice because I moved twice" should produce two task batches, one per move.
