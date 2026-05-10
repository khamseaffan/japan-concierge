# ADR-0006: Explicit POST /visas over implicit creation on first event

**Status:** Accepted
**Date:** 2026-05-10

## Context

When the first frontend session ran against `POST /api/v1/life-events`, the request failed because `RecordLifeEvent` requires the user to have an active visa row to attach the event to. New users on a fresh database have no visa, so the very first interaction broke.

Two ways to fix this:

- **(a) Explicit visa creation**: add `POST /api/v1/visas` that the frontend calls during onboarding. The user picks a visa type from `GET /api/v1/visa-types`, sets `status="planning"`, then proceeds to log events.
- **(b) Implicit creation**: special-case `visa_application_started` in the service to auto-create a visa row if none exists, inferring the visa type from the event payload.

## Decision

Option (a) — explicit `POST /api/v1/visas`. The frontend pre-arrival flow is responsible for creating the visa row before any events can be recorded.

## Consequences

- **Easier:** the user genuinely picks a visa type as a discrete decision in the UI ("Which visa are you applying for?"). Modeling that as an explicit API call mirrors reality.
- **Easier:** explicit endpoints have explicit tests. Implicit creation has implicit tests, which means usually no tests.
- **Easier:** future visa-change flows (J-FIND → Engineer transition) need the same shape. With `POST /visas` already in place, "I'm switching to Engineer" is a new visa row plus a `EventVisaChanged` event — both clean. With implicit creation, the change flow has to invent a different pattern.
- **Harder (slightly):** the frontend must orchestrate two calls during onboarding (`POST /visas`, then `POST /life-events`). Trivial UX cost; the screens map naturally to "pick visa type" → "log application started" anyway.
- **Rejected because of:** option (b) hides errors. If `visa_application_started` arrives with no visa context and the user actually meant a different visa type than the system would default to, you've quietly created the wrong visa row. Recovery means cascading deletes on the visa, life events, and tasks. Better to force the choice up front.

## Implementation note

Schema, service, and handler are all in place as of commit that introduces this ADR:

- `service.VisaService.CreateVisa` validates `visa_type_code` against the `visa_types` reference table.
- Handler returns `400 unknown visa_type_code` for unknown codes; `201 Created` with the new visa otherwise.
- `GET /api/v1/visas/active` returns `404` if the user has no visa, signalling the frontend to redirect to the onboarding flow.
