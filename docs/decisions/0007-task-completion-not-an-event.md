# ADR-0007: Task completion does not emit a life event in v1

**Status:** Accepted
**Date:** 2026-05-10

## Context

When implementing `POST /api/v1/tasks/{id}/done`, two reasonable models presented themselves:

- **Pure event-sourced approach.** Mark-done emits a `task_completed` life event with the `rule_id` in the payload. Rules can have `trigger: task_completed` and filter on `payload.rule_id`. New rules like "when address registration is done, generate the My Number application task" become a YAML change.
- **Pragmatic state-update approach.** Mark-done is a direct UPDATE on `compliance_tasks` setting `status='done'` and `completed_at=NOW()`. No event row, no engine evaluation. The rule engine remains the single producer of tasks; user actions only mutate task state.

The pure approach is more flexible long-term — it makes task completion observable to the rule system. The pragmatic approach is simpler now and matches the actual v1 use case (a checkbox in the UI that updates a status field).

No existing rule needs to fire on task completion. The first time we want one (likely when the document parser auto-marks `register_address` done after a juminhyo upload, and that completion should trigger `apply_for_my_number_card`), we will add it then.

## Decision

Pragmatic. `MarkTaskDone` is a direct status update. No `task_completed` event is emitted in v1.

## Consequences

- **Easier:** mark-done is a 4-line SQL UPDATE plus a thin service method. No engine round-trip, no event-row bloat for an action that happens many times per task.
- **Easier:** the schema is unchanged. No new event_type constant, no new rule pattern.
- **Trade away:** rules cannot currently react to user-initiated task completion. The day we want that, we will:
  1. Add `EventTaskCompleted` to the registry in `rules/types.go`.
  2. Modify `MarkTaskDone` to also insert a `life_event` row of that type, with the rule_id in the payload.
  3. Add a wildcard rule in YAML with `trigger: task_completed` that filters on payload.
  4. Re-run the affected rule sets to backfill any missed downstream tasks (a one-off script, not a migration).

  This is additive, not destructive. The schema migration is one new event_type allowed value (already enforced by Go const, not by Postgres CHECK), and the engine already supports payload-aware rules through `UserContext`.
- **How to apply:** if a future PR proposes a rule that says "when X is done, do Y," that PR is the one that revisits this ADR and ships the event-emission changes. Do not pre-build it.

## Implementation note

Schema and handler shipped in the same commit as this ADR. The `compliance_tasks.status` field is already constrained by a CHECK to the seven valid statuses, so the transition to `done` is enforced at the DB level. The handler returns 409 if the task is already done — caller can treat that as idempotent success.
