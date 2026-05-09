# ADR-0004: Compliance rules expressed as YAML data, not Go branches

**Status:** Accepted
**Date:** 2026-05-09

## Context

Japan immigration compliance is a moving target: rules differ per visa type (J-FIND, Engineer, HSP, Student, Spouse, Working Holiday, ...), per municipality (Tokyo wards differ from Osaka and from each other), and over time as MOJ and NTA update procedures. Hard-coding rules as `if/elif` branches in Go would mean every rule change is a code change, and adding a new visa type would require touching the engine.

This is an Open/Closed Principle problem: the engine should be closed for modification but open for extension. Adding a visa = adding data, not code.

## Decision

Rules live as YAML files in `backend/internal/rules/data/*.yaml`, embedded into the binary at compile time via Go's `embed.FS`. Each file is one visa's rule set: a list of `Rule` objects, each with a trigger event type, applicability scope (`applies_to_visas: [...]` or `['*']` for wildcards like tax-residency), and a list of `TaskTemplate` outputs with deadlines, dependencies, and legal source citations.

The engine is a pure function: `Evaluate(ruleSets, inputEvent) -> []GeneratedTask`. It has zero knowledge of which visas exist beyond what's loaded.

## Consequences

- **Easier:** adding HSP, Student, Spouse later is a YAML file plus a few tests. No engine changes. A non-engineer (or a future contributor with deep immigration knowledge but no Go) can author rules.
- **Easier:** rules are reviewable as data — diffs in PRs show the actual policy change, not Go control flow.
- **Easier:** the binary ships with rules baked in (`embed.FS`), so there is no "where do the rules live in production" question and no runtime file-loading bug class.
- **Harder:** rule YAML must be validated at load time, because YAML is dynamically typed. The loader runs a validation pass: `depends_on` references must resolve, deadline specs must set exactly one of three fields, every `rule_id` must be globally unique. This is enforced in `validateRuleSets` and exercised by tests.
- **Tradeoff:** very complex rules (e.g., conditional logic based on user state like "if user has Japanese employer, do X, else Y") cannot be expressed in pure data without introducing an expression language. v1 sidesteps this by passing a `UserContext` struct to `Evaluate` for future expansion. When we hit a rule that needs conditional logic, we either add a small DSL or split the rule into two rules with disjoint applicability.
