# ADR-013: Event Ordering Guarantees for Daemon→BFF Ingest

**Date**: 2026-05-07
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-012 (game-play event correlation), ADR-014 (legacy parser extraction)

---

## Context

The desktop daemon dispatches events to the BFF over HTTP with retry
semantics. The current retry policy is at-least-once — a transient
network failure causes the daemon to re-send the event, and the BFF must
tolerate duplicate delivery and out-of-order arrival.

The v0.3.0 telemetry parity work adds the game-play projector (per
ADR-012). Game-play correlation depends on event order — turn N+1 cannot
be applied before turn N. If two events arrive out of order, the
projector must either (a) buffer until ordering can be resolved, or (b)
sort at write time using a stable, daemon-assigned ordering key.

The current `contract.DaemonEvent` wire format carries `occurred_at`
(client wall-clock time) and an event id. Wall-clock time is not
sufficient as an ordering key — multiple events in the same session can
share the same millisecond timestamp, and clock skew within a single
session is non-zero in practice.

---

## Decision

**Add a monotonic `sequence uint64` field to `contract.DaemonEvent`. The
daemon assigns sequence numbers per session starting at 1. Projectors
sort by `(occurred_at, sequence)` within a session before applying state
transitions.**

### Specifics

1. **Wire format change**: Add `Sequence uint64` to
   `contract.DaemonEvent`. Tagged release of `services/contract` (per
   the Go workspace rules in the architect guide) — no `replace`
   directives in CI.
2. **Daemon assignment**: The daemon maintains a per-session counter,
   starting at 1, incremented atomically on each event emission. The
   counter resets at session start.
3. **BFF ingest**: The BFF ingest endpoint accepts events in any order
   and persists the `Sequence` field alongside the event payload.
   The endpoint does not block on ordering.
4. **Projection workers**: Projectors that require ordered application
   (game-play correlation per ADR-012) sort by
   `(occurred_at, sequence)` within a session before applying state
   transitions. Sorting happens at write time, in the projector
   worker, not at ingest time.
5. **Session scoping**: `sequence` is session-scoped — it resets per
   session, not globally monotonic. Cross-session ordering is
   irrelevant; sessions are independent.

### What this changes

- **Adds** `Sequence uint64` to `contract.DaemonEvent`.
- **Adds** a per-session counter to the daemon dispatcher.
- **Adds** sequence-aware sort to ordered projectors.
- **Does not** change retry semantics — at-least-once delivery
  remains.
- **Does not** add an idempotency key — duplicate suppression is a
  v0.3.1 concern.

---

## Consequences

### Positive

- **Out-of-order arrival becomes safe.** The projector sorts before
  applying. HTTP retry reordering no longer corrupts game-play state.
- **Cheap on the wire.** `uint64` per event. No additional round
  trips, no coordination protocol.
- **Cheap in the daemon.** A per-session atomic counter. No
  cross-session synchronization.
- **No BFF state.** Ingest stays stateless; only projectors care about
  ordering, and they already touch the database row-by-row.

### Negative

- **Breaking wire-format change.** `contract.DaemonEvent` adds a
  required field. The daemon binary and the BFF must be deployed
  together. v0.3.0 is the cut for this change; older daemons cannot
  talk to the v0.3.0 BFF and vice versa. Acceptable for beta —
  documented in the release notes.
- **Session-scoped, not globally monotonic.** Sorting within a session
  is sufficient for ADR-012 correlation but means ordering across
  sessions cannot be inferred from `sequence` alone. This is by
  design; no projector requires cross-session ordering.

### Deferred

- **Exactly-once delivery.** Current retry is at-least-once. Adding
  per-event idempotency keys (so the BFF can suppress duplicates) is
  a v0.3.1 concern. Until then, projectors must be idempotent under
  duplicate apply — the game-play projector already is, because
  sequence-keyed inserts collide on the unique constraint.
- **Globally monotonic ids.** If a future projector needs cross-session
  ordering, layer a separate epoch+counter id on top — do not bend the
  per-session sequence to do double duty.

---

## Implementation Tickets

These tickets land in milestone v0.3.0 on project board #29. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Contract: add `Sequence uint64` to `contract.DaemonEvent`; tag new `services/contract` release | backend-engineer |
| **TBD-B** | Daemon: per-session counter; assign on dispatch; reset on session start | backend-engineer |
| **TBD-C** | BFF ingest: persist `Sequence` field; no ordering enforcement at ingest time | backend-engineer |
| **TBD-D** | Projector: sequence-aware sort by `(occurred_at, sequence)` within a session before applying state transitions | backend-engineer |
| **TBD-E** | Tests: out-of-order delivery test — feed events in shuffled order, verify projected state matches in-order delivery | backend-engineer |
| **TBD-F** | Docs: release-note entry for the breaking wire-format change; daemon and BFF must be deployed together for v0.3.0 | architect |

Each ticket gets acceptance criteria when the Project Manager files it.

---

## Alternatives Considered

### A. Use `occurred_at` (client wall-clock) as the sole ordering key

**Rejected.** Wall-clock collisions within a millisecond are common
inside a single match (multiple GRE events on the same trigger).
Wall-clock skew across daemon restarts within a session is non-zero
in practice. Not a stable ordering key.

### B. BFF assigns ordering via ingest-time sequence

**Rejected.** Out-of-order HTTP arrival means BFF-assigned sequence
reflects network jitter, not emission order. The point of the
sequence field is to capture emission order — that has to happen at
the source.

### C. Buffer at ingest until ordering resolves

**Rejected.** Adds BFF state and a holding window. Ordering can be
delayed indefinitely on retry storms. Sorting at write time in the
projector is strictly simpler.

### D. Per-session monotonic sequence

**Accepted.** See Decision section above.

---

## References

- ADR-012 — Game-play event correlation. The primary consumer of the
  ordering guarantee added here.
- ADR-014 — Legacy parser extraction. Establishes the `pkg/logparse/`
  location from which both daemon and BFF projector consume parsing
  logic.
- Go workspace rules (architect agent guide) — `services/contract`
  must be tagged before the daemon and BFF depend on the new
  `Sequence` field.
