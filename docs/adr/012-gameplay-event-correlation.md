# ADR-012: Game-play Event Correlation Model

**Date**: 2026-05-07
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-010 (draft overlay architecture), ADR-013 (daemon event ordering), ADR-014 (legacy parser extraction)

---

## Context

The desktop daemon emits individual GRE (Game Rules Engine) game-play
events — life total changes, zone transitions, attack declarations, block
declarations, damage assignment — to the BFF over HTTP, one event per
request. Each event in isolation is a delta against an implicit
session-scoped state.

The legacy desktop parser at
`internal/mtga/logreader/gre_parser.go` already correlates these events
into coherent `GamePlayEvent` rows. It does this by ingesting the entire
GRE event slice for a session at once and walking it with mutable session
state (current turn, active player, life totals, zone contents). The
parser is battle-tested and has approximately five years of bug fixes
encoded in its branches.

The BFF projector for v0.3.0 telemetry parity must produce the same
`GamePlayEvent` rows for matches that originate from the cloud daemon as
the desktop binary produces locally. The architectural question is
**where session-scoped correlation state lives** when events arrive over
HTTP one at a time rather than as a single in-memory slice.

Three placements were considered: (A) BFF-side stateful buffering with
session state held in Redis or BFF process memory; (B) BFF-side stateless
projection that re-derives state from the database on every event; (C)
daemon-side pre-computation that ships fully-correlated `GamePlayEvent`
structs to the BFF.

---

## Decision

**Option C — daemon-side pre-computation — is the correlation model for
v0.3.0 telemetry parity.**

### Specifics

1. **Buffering location**: The daemon buffers GRE events per session in
   process memory. The buffer keys on the in-progress match id /
   session id already tracked by the existing `logreader` pipeline.
2. **Correlation logic**: The daemon reuses the existing
   `gre_parser.go` correlation logic verbatim (lifted to `pkg/logparse/`
   per ADR-014). No reimplementation. No port to the BFF.
3. **Flush triggers**: The daemon flushes buffered `GamePlayEvent`
   structs to the BFF when (a) the session ends (match concludes,
   user concedes, opponent concedes), or (b) a per-session size
   threshold is hit (defense against unbounded memory growth on
   pathological matches).
4. **Wire format**: The daemon ships fully-correlated
   `GamePlayEvent` structs — not raw GRE events — to a new BFF ingest
   endpoint. The contract type lives in `services/contract`.
5. **BFF projector**: The BFF projector for game-play events is
   stateless. It receives complete `GamePlayEvent` structs and writes
   them to the per-account_id partitioned table. No session state in
   the BFF.

### What this changes

- **Adds** a session-scoped GRE event buffer in the daemon.
- **Adds** a flush mechanism keyed on session-end and size threshold.
- **Adds** a BFF ingest endpoint accepting `GamePlayEvent` structs.
- **Reuses** the legacy `gre_parser.go` correlation logic without
  changes (lifted, not rewritten).
- **Does not** introduce Redis as a BFF dependency.
- **Does not** require the BFF projector to hold session state.

---

## Consequences

### Positive

- **Reuses battle-tested logic.** The legacy `ParseGamePlays` already
  does correlation correctly. Lifting it to `pkg/logparse/` and calling
  it from the daemon avoids reimplementing five years of edge-case
  handling.
- **Daemon already holds session state.** The existing `logreader`
  pipeline tracks active matches; adding a GRE event buffer is a small
  delta, not a new architectural layer.
- **No new BFF dependency.** No Redis. No BFF process memory pinned to
  a session. No sticky load balancing. The BFF stays stateless.
- **Simpler projector.** The BFF projector for game-play events
  receives complete structs and writes rows. No state machine on the
  server side.
- **Bounded memory.** A typical user has at most one active match.
  Buffer size is bounded by match length (≈ a few hundred events
  worst case).

### Negative

- **Daemon memory usage scales with active sessions.** In the typical
  single-user case this is one session and is trivially small. The
  size-threshold flush is the bound for pathological cases.
- **Out-of-session events may be lost.** If the daemon process crashes
  mid-match before flushing, the buffered GRE events for that session
  are lost. Acceptable for v0.3.0 — addressed in v0.3.1 via on-disk
  spool (see Deferred).
- **Daemon must hold parser dependencies.** `pkg/logparse/` becomes a
  required import for the daemon binary. Acceptable — the daemon
  already imports the logreader.

### Deferred

- **On-disk spool for crash resilience.** v0.3.1 adds a small on-disk
  buffer in `~/.vaultmtg/spool/` so a daemon crash mid-match does not
  drop the in-flight session.
- **BFF-side stateful buffering as fallback.** If daemon memory pressure
  emerges at scale (multi-account daemons, headless deployment), revisit
  with a Redis-backed BFF buffer and stateless daemon. Not a v0.3.0
  concern.

---

## Implementation Tickets

These tickets land in milestone v0.3.0 on project board #29. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Daemon: add session-scoped GRE event buffer keyed on match/session id | backend-engineer |
| **TBD-B** | Daemon: wire `pkg/logparse/` correlation into the buffer flush path; reuse `ParseGamePlays` logic | backend-engineer |
| **TBD-C** | Daemon: flush triggers — session-end and size-threshold; metrics for both paths | backend-engineer |
| **TBD-D** | Contract: `GamePlayEvent` struct in `services/contract`, tagged release | backend-engineer |
| **TBD-E** | BFF: new ingest endpoint accepting `GamePlayEvent` structs; per-account_id scoping | backend-engineer |
| **TBD-F** | BFF: stateless projector writing `GamePlayEvent` rows to the partitioned table | backend-engineer |
| **TBD-G** | Tests: parity test comparing desktop-binary-produced rows vs daemon→BFF-produced rows for a fixture match | backend-engineer |

Each ticket gets acceptance criteria when the Project Manager files it.

---

## Alternatives Considered

### A. BFF-side stateful buffering (Redis or in-process)

**Rejected.**

- Adds Redis as a BFF dependency or pins the BFF to in-process state
  (and therefore to sticky load balancing). Both are heavy infra
  changes for a beta.
- Requires reimplementing or porting `gre_parser.go` correlation logic
  to the BFF. Five years of edge-case fixes would have to be re-tested
  against the new implementation.
- Splits the correlation logic between daemon (desktop) and BFF
  (cloud) — two implementations of the same rules engine, diverging
  over time.

### B. BFF-side stateless projection (re-derive state per event)

**Rejected.**

- Each event would require reading the prior session state from the
  database, applying the delta, and writing it back. N events ⇒ N
  round-trips. Quadratic-ish projection cost.
- Race conditions on out-of-order arrival become a database concern
  rather than a buffer concern. ADR-013 ordering guarantees still
  apply but the correctness surface is much larger.
- No reuse of the legacy parser — full reimplementation against
  database state.

### C. Daemon-side pre-computation

**Accepted.** See Decision section above.

---

## References

- ADR-010 — Draft overlay architecture. Establishes the daemon →
  BFF SSE pattern reused here.
- ADR-013 — Daemon event ordering. Provides the sequence-number
  guarantees the daemon buffer relies on for correct correlation
  even when retries reorder HTTP delivery.
- ADR-014 — Legacy parser extraction. Defines the `pkg/logparse/`
  location from which the daemon imports `ParseGamePlays`.
- `internal/mtga/logreader/gre_parser.go` — legacy correlation
  reference implementation; lifted, not rewritten.
