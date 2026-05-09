# v0.3.0 Architecture Assessment

**Date**: 2026-05-09
**Author**: Architect
**Scope**: Post-v0.3.0 codebase review — brittleness findings, data-loss risks, and medium/low technical debt for v0.4.0 planning.
**Reference**: This document is cited in `docs/prd/v0.4.0-kickoff.md` as the source for Wave 0 (T1–T5 blocking) and Wave 2 (T6–T7 architect-designed) findings.

---

## Executive Summary

v0.3.0 successfully shipped the cloud telemetry pipeline (daemon → BFF ingest → projection → analytics). However, a post-release architecture review surfaced **five blocking brittleness defects** that pose silent data-loss or data-correctness risks: duplicated `knownFormats` maps, unaligned daemon JSON struct tags (#1041), projection worker redeclaring contract payload structs, projection malformed-row handling that masquerades as success, and partial GRE events polluting aggregate queries. None block runtime, but all will silently corrupt analytics or cause regressions when contract or format additions happen. They MUST be resolved before any new feature work in v0.4.0.

In addition, **two HIGH-severity reliability findings** (T6 daemon local event queue, T7 NOTIFY/LISTEN projection) require architect-led design before implementation and are scheduled for Wave 2. Ten MEDIUM/LOW findings (T8–T17) are tracked for opportunistic resolution.

Architectural decisions confirmed correct and not revisited: SSE broker (ADR-001), Clerk auth (ADR-009), cursor pagination (ADR-018), event ordering (ADR-013), gameplay correlation (ADR-012).

---

## Severity Legend

| Severity | Meaning |
|----------|---------|
| BLOCKING | Silent data-loss or data-correctness risk. Must fix before Wave 1. |
| HIGH | Reliability or correctness risk under load or edge cases. Fix in Wave 2. |
| MEDIUM | Tech debt that will slow future development. Fix opportunistically. |
| LOW | Minor inconsistency or missing polish. Backlog. |

---

## Findings

### T1 — `knownFormats` map duplicated across handlers (BLOCKING)

**File(s)**: `services/bff/internal/api/handlers/history.go` (declares `knownFormats`); `services/bff/internal/api/handlers/list_v2.go` and `services/bff/internal/api/handlers/stats.go` (read it).
**Finding**: `knownFormats` is declared in `history.go` but referenced in `list_v2.go` (lines 147, 298) and `stats.go` (lines 436, 510). The current code accidentally compiles because all three live in the same `handlers` package, but the convention is fragile — any future engineer adding a `formats` validation in another file will not realize the var lives in `history.go` and is liable to redeclare it (yielding a compile error) or skip validation (yielding accepted-but-unsupported format strings). The post-mortem PRD lists this as duplication; in fact the bug is worse — the var is single-source today but is undiscoverable, and the package will not survive a routine refactor.
**Risk**: When MTGA adds a new format (e.g., a new digital-only mode), an engineer will update one call site and miss the others. Result: stats endpoint silently rejects the format while history endpoint accepts it, producing user-visible "no data" pages with no error logs.
**Resolution**: See Wave 0 ticket ACs in kickoff doc Section 5 / Wave 0 / T1 row.

### T2 — Daemon `models.go` struct JSON tags drift from MTGA log keys (#1041) (BLOCKING)

**File(s)**: `services/daemon/internal/logreader/models.go` (and the per-event files: `match.go`, `inventory.go`, `quests.go`, `deck.go`, `draft_pick.go`, `collection.go`).
**Finding**: Several structs use Go-idiomatic camelCase or PascalCase JSON tags that don't match the actual MTGA Player.log keys. Examples in `models.go`: `DraftEvent.EventID` is tagged `"CourseId"` (correct), but `PlayerInventory.WildCardCommons` is tagged `"wcCommon"` — there is no corroborating fixture or unit test asserting this matches the live MTGA key. The codebase does not have a JSON-key contract test against a captured Player.log sample, so any drift between the daemon's expectations and Arena's wire format produces silently empty/zero-valued payloads downstream.
**Risk**: Silent data corruption. The daemon happily emits `{"gems": 0, "gold": 0, "wcCommon": 0, ...}` if MTGA renames a key, and there is no smoke test catching it. Inventory snapshots will look like the player has nothing; analytics will diverge from Arena reality with no error.
**Resolution**: See Wave 0 ticket ACs in kickoff doc Section 5 / Wave 0 / T2 row (#1041).

### T3 — Projection worker redeclares contract payload structs (BLOCKING)

**File(s)**: `services/bff/internal/projection/worker.go` (lines 277–642 declare `matchPayload`, `draftPayload`, `draftPickPayload`, `collectionUpdatedPayload`, `inventoryUpdatedPayload`, `questProgressPayload`, `questCompletedPayload`, `deckUpdatedPayload`, `gamePlayPayload`); compare to the canonical types in `services/contract/contract.go` (`MatchEventPayload`, `DraftEventPayload`, `InventoryUpdatedPayload`, `QuestProgressPayload`, `QuestCompletedPayload`, `DeckUpdatedPayload`, `CollectionUpdatedPayload`).
**Finding**: The projection worker ignores the `services/contract` Go module and instead defines a parallel set of payload structs locally. There are now two definitions for every event payload — the contract types (used by the daemon producer) and the worker's `*Payload` types (used by the BFF consumer). Adding a field to a contract struct is a no-op on the BFF side: the worker's local struct has no equivalent field, so `json.Unmarshal` silently drops it.
**Risk**: Schema drift. Whenever a new field is added to the contract package (which the daemon will start emitting), the worker reads `daemon_events.payload`, deserializes into its local struct, and silently truncates the new field. The compiler offers zero protection.
**Resolution**: See Wave 0 ticket ACs in kickoff doc Section 5 / Wave 0 / T3 row.

### T4 — Projection marks malformed rows as projected; no dead-letter mechanism (BLOCKING)

**File(s)**: `services/bff/internal/projection/worker.go` (`projectRow`, lines ~190–270; `MarkProjected` always called regardless of outcome).
**Finding**: The worker tracks four outcomes (`projected`, `skippedUnknown`, `skippedMalformed`, `errored`) but treats all but `errored` identically — every non-errored row is marked `projected = true` in `daemon_events`. A malformed JSON payload, an unknown event type, and a successfully projected row all end up in the same terminal state. There is no dead-letter table, no retry queue, no operator visibility into which rows were silently dropped. The comment above `projectRow` even says "always attempts to mark the row as projected (even on skip/error) so malformed rows don't block the queue" — that's the bug, framed as the design.
**Risk**: Permanent data loss on any contract bug, JSON drift, or future event-type rename. Operator has no recovery path; the only signal is a log line that is overwritten on the next 30s tick.
**Resolution**: See Wave 0 ticket ACs in kickoff doc Section 5 / Wave 0 / T4 row.

### T5 — Partial GRE events poison aggregate queries (BLOCKING)

**File(s)**: `services/bff/internal/storage/repository/game_play_repo.go` (writes `partial` column from migration 000074); `services/bff/internal/storage/repository/stats_repo.go` and any handler aggregating `game_plays` (no `WHERE partial = false` filter present anywhere in the BFF).
**Finding**: The `game_plays` table has a `partial` boolean column written by the projection worker when GRE session buffering evicts an incomplete game. However, no aggregate query in the BFF filters on `partial = false`. `grep -r "WHERE partial" services/bff/internal/` returns zero hits. Partial rows have empty `match_id` / `game_number` / `winning_team_id` and pollute every win-rate, average-turn-count, and duration aggregation.
**Risk**: Analytics correctness. User-visible win rates and game-count metrics will be wrong by an unknown margin (proportional to GRE eviction rate). Worse: the bug is silent — stats simply look noisy or biased toward losses.
**Resolution**: See Wave 0 ticket ACs in kickoff doc Section 5 / Wave 0 / T5 row.

---

### T6 — Daemon has no local event queue; events lost on crash (HIGH)

**File(s)**: `services/daemon/internal/dispatch/dispatcher.go` and surrounding files.
**Finding**: The daemon dispatcher posts events directly to the BFF ingest endpoint over HTTP. If the network fails, the BFF is unreachable, or the daemon crashes between log read and BFF acknowledgement, the event is lost. There is no local SQLite write-ahead log, no in-memory buffer with disk spill, and no retry-on-restart behavior.
**Risk**: Data loss on any daemon crash, network partition, or BFF outage during a draft or match. Users will see gaps in their match history that they cannot recover.
**Resolution**: Architect designs a local SQLite write-ahead event queue with at-least-once delivery semantics. Design note required before implementation. **Wave 2 — architect design required before implementation.**

### T7 — Projection worker uses 30s polling; no NOTIFY/LISTEN (HIGH)

**File(s)**: `services/bff/internal/projection/worker.go` (`tickInterval = 30 * time.Second`).
**Finding**: The projection worker polls `daemon_events` every 30 seconds for unprojected rows. This produces a worst-case 30s latency between event ingest and projection completion. PostgreSQL supports `LISTEN/NOTIFY` for push notification, which would reduce projection lag from 30s to <100ms with no additional DB load.
**Risk**: User-visible delay. After a draft pick or match completion, the SPA dashboard does not reflect new data for up to 30s. Beta testers will report "where's my match?" — the answer is "wait 30 seconds." Not a data-loss risk, but a UX-quality risk for beta launch.
**Resolution**: Architect designs NOTIFY/LISTEN integration with fall-through polling for missed signals. Design note required before implementation. **Wave 2 — architect design required before implementation.**

---

### T8–T17 — Medium/Low Findings

| ID | Title | File(s) | Severity | Notes |
|----|-------|---------|----------|-------|
| T8 | Account lookup not cached; every projection iteration calls `GetOrCreateByClientID` | `services/bff/internal/projection/worker.go` | MEDIUM | Add an in-memory LRU cache keyed by `client_id`. Wave 1 small ticket. |
| T9 | No request timeout middleware on BFF handlers | `services/bff/internal/api/middleware/` | MEDIUM | Add `http.TimeoutHandler` wrapper at router init. Wave 1 small ticket. |
| T10 | SSE broker has no slow-client metric or backpressure | `services/bff/internal/api/sse/broker.go` | MEDIUM | Add a per-client lag gauge; emit warn log if a client is >5s behind. |
| T11 | Stats endpoint returns untyped `map[string]any` instead of a typed envelope | `services/bff/internal/api/handlers/stats.go` | MEDIUM | Replace with a versioned typed response struct; matches ADR-017 read-contract pattern. |
| T12 | SSE auth cookie name is a string literal, not a constant | `services/bff/internal/api/sse/broker.go` and middleware | LOW | Extract to `auth.SSECookieName` const for cross-package refactor safety. |
| T13 | Daemon log lines mix structured and unstructured logging | `services/daemon/internal/**` | LOW | Migrate to `log/slog` consistently. |
| T14 | BFF `migrate.go` does not record migration runtime in metadata table | `services/bff/internal/storage/migrate.go` | LOW | Add `applied_at` and `duration_ms` columns to schema_migrations. |
| T15 | Frontend SPA has no global error boundary for network failures | `frontend/src/` | MEDIUM | Add React error boundary with retry UI; integrate Sentry breadcrumb. |
| T16 | Daemon registrar JWT TTL is hardcoded 90 days | `services/daemon/internal/registrar/registrar.go` | LOW | Move TTL to config; document rotation policy. |
| T17 | No alembic-style "down" migration smoke test in CI | `services/bff/migrations/` | MEDIUM | Add CI step that applies up.sql then down.sql for every migration. |

---

## Wave Assignment Summary

| Finding | Severity | Wave |
|---------|---------|------|
| T1–T5 | BLOCKING | Wave 0 |
| T6–T7 | HIGH | Wave 2 (architect-designed) |
| T8–T17 | MEDIUM/LOW | Wave 1 opportunistic (T8, T9), backlog (T10–T17) |

---

## Architectural Decisions Confirmed Correct (Not Revisited)

- **ADR-001 SSE broker** — correct choice over WebSockets for unidirectional projection → SPA push.
- **ADR-009 Clerk auth** — correct provider; migration from legacy HMAC is on track for #1315.
- **ADR-013 daemon event ordering** — sequence numbers and at-least-once semantics are right.
- **ADR-012 gameplay event correlation** — match_id + game_number composite key is correct.
- **ADR-018 cursor pagination** — list endpoints use base64 cursor encoding correctly.

These five ADRs do not need revisiting in v0.4.0 or v0.5.0 absent a new requirement.
