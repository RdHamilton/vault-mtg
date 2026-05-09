# ADR-014: Legacy Parser Extraction Strategy

**Date**: 2026-05-07
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-012 (game-play event correlation), ADR-013 (daemon event ordering)

---

## Context

`internal/mtga/logreader/` contains roughly 5,000 lines of battle-tested
parser code originally written for the desktop binary. This includes the
GRE parser, draft event parser, deck submission parser, match result
parser, and supporting fixture loaders. The accompanying test suite is
roughly 6,000 lines and encodes years of edge-case fixes against real
`Player.log` captures.

For v0.3.0 telemetry parity, both the daemon (cloud-mode dispatcher)
and the desktop binary need to call the same parser code. Per ADR-012,
the daemon imports `ParseGamePlays` to do session-scoped correlation
before shipping events to the BFF. Per ADR-013, the contract types live
in `services/contract`. Neither location is correct for parser code.

The current import path `internal/mtga/logreader/` is technically
reachable from anywhere in the monorepo, but the `internal/` path
implies desktop-only ownership. As `services/daemon/` and potentially
future tools (CLI replay tool, fixture validator) need the same logic,
the package needs an unambiguous shared home.

Three placements were considered: (A) leave it in `internal/mtga/logreader/`
and import from services; (B) copy-paste into each consumer service;
(C) lift to a new shared package `pkg/logparse/`.

---

## Decision

**Lift `internal/mtga/logreader/` to `pkg/logparse/` — a shared package
importable by any service in the monorepo. Migrate the existing test
suite alongside it. No logic changes during the lift.**

### Specifics

1. **New location**: `pkg/logparse/`. The `pkg/` prefix signals shared
   ownership across services (desktop binary, daemon, future tools).
2. **No copy-paste.** Both `services/daemon/` and the desktop binary
   import from `pkg/logparse/`. There is exactly one implementation
   of every parser in the monorepo.
3. **Not in `services/contract/`.** The contract package is reserved
   for wire types only. Parser logic does not belong there.
4. **Tests move with code.** The existing test suite at
   `internal/mtga/logreader/*_test.go` moves to `pkg/logparse/` in
   the same commit. Fixture files move with them.
5. **Mechanical lift.** No refactoring during the move. No API
   redesign. No logic changes. The diff is rename-only plus
   import-path updates in callers.
6. **Caller updates.** The desktop binary and the daemon both update
   their import paths in the same PR as the lift, so no consumer is
   left pointing at the dead path.

### What this changes

- **Moves** ~5,000 LOC of parser code from `internal/mtga/logreader/`
  to `pkg/logparse/`.
- **Moves** ~6,000 LOC of parser tests and fixtures alongside.
- **Updates** import paths in the desktop binary and the daemon.
- **Does not** change parser behavior, public API shape, or fixture
  data.
- **Does not** introduce a new module — `pkg/logparse/` lives in the
  same module as the rest of the monorepo, so no `go.work` or
  `replace` ceremony is needed.

---

## Consequences

### Positive

- **Single source of truth.** Both the desktop binary and the daemon
  call the same parser. Bug fixes apply once.
- **Clear ownership boundary.** `pkg/logparse/` is the home for log
  parsing — anyone reading the tree knows where to look. No more
  "is this parser desktop-only or shared?" ambiguity.
- **Enables ADR-012.** The daemon's session-scoped GRE buffer calls
  `pkg/logparse.ParseGamePlays` directly. No reimplementation, no
  drift between daemon and desktop correlation logic.
- **Mechanical, low-risk migration.** Rename + import-path update.
  The test suite catches any regression at PR-review time.

### Negative

- **One large rename PR.** ~11,000 LOC of churn in a single diff.
  Reviewable because the change is mechanical, but git blame for
  parser files resets to the lift commit. Mitigated with
  `git log --follow` and the `--find-renames` default.
- **One-time CI churn.** Any in-flight branches that touch
  `internal/mtga/logreader/` will need to rebase onto the new path.

### Deferred

- **Splitting `pkg/logparse/` further.** If the package grows beyond
  pure parsing — e.g. business rules, enrichment, derived metrics —
  split into `pkg/logparse/` (pure parsing) and `pkg/telemetry/`
  (aggregation and enrichment). Not a v0.3.0 concern; revisit when
  the package crosses ~7,500 LOC or starts importing storage-layer
  types.
- **Public-module extraction.** If a third-party tool ever needs the
  parser, lift again from `pkg/logparse/` (private to the monorepo)
  to a separate `github.com/RdHamilton/mtga-logparse` module. Not on
  the roadmap.

---

## Implementation Tickets

These tickets land in milestone v0.3.0 on project board #29. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Lift: `git mv internal/mtga/logreader/ pkg/logparse/`; update package declarations | backend-engineer |
| **TBD-B** | Lift: update import paths in desktop binary callers | backend-engineer |
| **TBD-C** | Lift: update import paths in `services/daemon/` callers | backend-engineer |
| **TBD-D** | Tests: confirm full test suite passes post-lift; no test changes other than package declaration | backend-engineer |
| **TBD-E** | CI: confirm `go test -race ./...` runs clean from monorepo root after lift | backend-engineer |
| **TBD-F** | Docs: update any docs referencing `internal/mtga/logreader/` to point to `pkg/logparse/` | architect |

Each ticket gets acceptance criteria when the Project Manager files it.

---

## Alternatives Considered

### A. Leave parsers in `internal/mtga/logreader/`

**Rejected.** The `internal/` path implies desktop-only ownership.
Importing from `services/daemon/` works at the Go-compiler level but
muddies the ownership signal — every new contributor has to be told
"yes, the daemon really does import from `internal/mtga/`". Costs
clarity for no real benefit.

### B. Copy-paste parser code into each consumer service

**Rejected.** Two implementations of `ParseGamePlays` will diverge.
Bug fixes will land in one location and not the other. Test suites
duplicate. This is the worst possible outcome for a parity-critical
piece of logic.

### C. Lift to `pkg/logparse/`

**Accepted.** See Decision section above.

### D. Lift to `services/contract/`

**Rejected.** `services/contract/` is reserved for wire types — the
shapes that cross service boundaries on HTTP. Parser logic is not a
wire type. Mixing them would force every consumer of the contract
package to also pull in parser dependencies, inflating the
contract module unnecessarily.

---

## References

- ADR-012 — Game-play event correlation. The primary consumer of
  `pkg/logparse.ParseGamePlays` from the daemon.
- ADR-013 — Daemon event ordering. Establishes the wire-format
  changes that depend on consistent parser output across daemon and
  desktop.
- Go workspace rules (architect agent guide) — `pkg/logparse/`
  lives in the monorepo's primary module, so no `go.work` or
  `replace` ceremony applies.
- `internal/mtga/logreader/` — current location; source of the
  lift.
