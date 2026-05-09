# ADR-0002: Business Source License 1.1 over MIT/Apache

**Status:** Accepted (LICENSE file pending)
**Date:** 2026-05-09

## Context

The repo is public on GitHub for recruiter visibility and to allow other foreigners moving to Japan to read and learn from it. But the author has a potential commercial path for this product (subscription / token-based when the AI-assisted document parsing layer ships) and does not want a third party to fork it and offer a competing hosted version.

Standard options:
- **MIT / Apache 2.0:** maximally permissive but no protection against commercial reuse.
- **AGPL-3.0:** copyleft for SaaS — viable but philosophically heavier and discourages enterprise adoption.
- **BSL 1.1:** commercial use restricted until a "Change Date," after which it converts to a permissive license. Used by HashiCorp (Vault, Consul), MariaDB, CockroachDB.

## Decision

BSL 1.1 with a 4-year Change Date (2030-05-09) converting to Apache 2.0. Additional Use Grant explicitly permits non-commercial use and personal forks; prohibits offering the Licensed Work as a hosted or embedded service to third parties.

## Consequences

- **Easier:** recruiters and learners can read all source; commercial cloning is legally deterred for 4 years; in 4 years the project becomes fully open under Apache 2.0 regardless of who owns it.
- **Harder:** BSL is not OSI-approved, so the project is not technically "open source" in the strict sense. Some communities (Debian, FSF zealots) won't engage. Acceptable tradeoff for v1.
- **Implementation note:** LICENSE file is currently a TODO (see README). The canonical BSL text needs to be pulled from the MariaDB-blessed source and parameterized with this project's Licensor / Change Date / Change License values. Until then, the public repo is technically "all rights reserved" by default — fine for recruiter-only viewing, must be resolved before any external user is invited to use the app.
