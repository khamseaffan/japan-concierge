# ADR-0001: Go for the backend over Python/FastAPI

**Status:** Accepted
**Date:** 2026-05-09

## Context

Author has production experience with FastAPI and Pydantic at a previous role. The CTO-style review pushed for Python/FastAPI on the basis that "use what you know" wins on shipping speed for a solo project under time pressure (visa application + international move).

Counter-arguments for Go: better static typing for a rules engine where correctness matters, easier deployment (single binary), idiomatic concurrency for the Phase 2 OCR + LLM document parsing pipeline that fans out per document, and the project doubles as a portfolio piece for backend roles in Japan where Go is increasingly common.

## Decision

Go for the backend. Acknowledged that this trades a few weeks of velocity for the language-fit and portfolio benefits.

## Consequences

- **Easier:** rules engine modeled as pure functions over typed structs; deployment as one static binary; concurrent document-parsing pipelines without `asyncio` ceremony.
- **Harder:** less mature ecosystem for some integrations (Anthropic SDK is younger than its Python sibling); author has to absorb Go idioms while shipping.
- **Tradeoff accepted:** if the timeline slips by 2-4 weeks because of Go ramp-up, that's the cost. Apartment-hunting and forum features were already cut from v1 to make room.
- **Mitigation:** AI-assisted coding closes a meaningful chunk of the velocity gap; the rules engine being pure data with table-driven tests means the foundation isn't bottlenecked on language fluency.
