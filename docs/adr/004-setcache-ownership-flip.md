# ADR-004: SetCache Ownership Flip Mechanism for Sync / BFF

**Date**: 2026-05-03
**Status**: Proposed
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

Per ADR-001, the Sync Lambda owns all writes to the `draft_card_ratings` table. The BFF reads that table to serve draft ratings to the frontend. This creates an operational dependency: if the Sync Lambda fails, is delayed, or misses a scheduled run, the BFF has no fallback. Without a mechanism to degrade gracefully, a stale cache either causes the BFF to return an error (bad UX) or silently serve data that may be days old with no signal to the caller.

The `draft_card_ratings` table already contains a `cached_at` column (`TIMESTAMPTZ NOT NULL DEFAULT NOW()`) set by Sync on each `UpsertRatings` call. This is the natural freshness signal for any staleness-based approach.

The BFF currently has no draft ratings read handler — the `internal/api/handlers/` directory contains only `ingest.go` and `api_keys.go`. The ownership flip mechanism must be designed before that handler is built, so the handler can be built with the correct staleness logic from the start.

### Failure modes that require a fallback

| Scenario | Without fallback | With fallback |
|---|---|---|
| Sync Lambda missed one daily run | BFF returns 503 or empty response | BFF serves yesterday's ratings with degraded-mode signal |
| 17Lands API down for 48+ hours | Stale data served silently or error | BFF serves last-known-good with observable header |
| Operator wants to pause Sync during maintenance | Must redeploy BFF to suppress errors | Toggle flag; BFF bypasses freshness check |
| Sync runs successfully but with partial data | No visibility | Sync writes health record; BFF detects partial run |

---

## Options Considered

### Option 1 — Feature Flag (`sync_owns_set_cache` env var or DB table)

A boolean flag (environment variable or a `feature_flags` table) controls whether the BFF enforces Sync freshness. When `false`, the BFF bypasses all age checks and serves whatever is in `draft_card_ratings`, regardless of how old it is.

**Pros**

| | |
|---|---|
| Instant operational override | An operator can flip the flag without a code deploy |
| Zero new schema | If env-var based, no migration needed |
| Simple BFF logic | Single `if flagEnabled { checkFreshness() }` branch |

**Cons**

| | |
|---|---|
| Binary semantics | Flag is either fully on or fully off — no graduated degradation |
| No automatic recovery | Operator must manually re-enable the flag after Sync recovers |
| Silent stale serving | When flag is off, callers get no signal that data may be stale |
| DB-table variant adds operational surface | A `feature_flags` table needs CRUD tooling; ENV var approach has no audit trail |

### Option 2 — Staleness Threshold (BFF reads `cached_at` from `draft_card_ratings`)

The BFF handler reads `MAX(cached_at)` for the requested set/format. If the most recent cache write is older than a configurable threshold (e.g. `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS=48`), the BFF serves the data anyway but includes a `X-Cache-Degraded: true` response header and logs a warning. No error is returned to the caller.

**Pros**

| | |
|---|---|
| No new schema or table | `cached_at` already exists in `draft_card_ratings` |
| Graduated response | Callers and monitoring can observe degraded mode via header |
| Automatic recovery | When Sync writes fresh data, `cached_at` advances and degraded mode clears automatically |
| Threshold is tunable | `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS` env var; no code change required to adjust |
| No operator action required for recovery | Sync job success restores normal mode |

**Cons**

| | |
|---|---|
| No instant override | Operator cannot force normal mode without waiting for Sync to run |
| Threshold must be calibrated | Too short triggers false degraded signals; too long masks real failures |
| `cached_at` reflects last write, not last successful full sync | A partial run updates `cached_at` on written rows but may leave gaps |

### Option 3 — Sync Health Table (Sync writes heartbeat; BFF reads it)

Sync Lambda writes a record to a `sync_health` table after each successful full run. The BFF reads this table to determine whether Sync is healthy before deciding whether to apply freshness enforcement. If no health record exists within a threshold window, BFF serves stale data with a degraded header.

**Pros**

| | |
|---|---|
| Distinguishes partial vs. full sync | `sync_health` row is written only after all sets are processed — `cached_at` rows may exist from a partial run |
| Clear observability | Health table is queryable for dashboards and alerting |
| Decoupled freshness signal | BFF does not need to interpret `cached_at` across multiple rows for multiple sets |

**Cons**

| | |
|---|---|
| New schema required | Migration to add `sync_health` table; Sync Lambda write path must be updated |
| Two writes for one sync run | Sync must write health record in addition to `draft_card_ratings` rows |
| Still no instant override | Same limitation as Option 2 for operational emergencies |
| Implementation complexity | Highest of the three options; requires coordinated DBA + backend + sync changes |

---

## Decision

**Adopt Option 2 (Staleness Threshold) as the primary mechanism, with Option 1's ENV var override as an escape hatch.**

The combined approach:

1. The BFF draft ratings handler reads `MAX(cached_at)` for the requested set/format from `draft_card_ratings`.
2. If `cached_at` is older than `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS` (default: 48), the BFF serves the data with response header `X-Cache-Degraded: true` and logs a structured warning.
3. The BFF never returns a 5xx error due to stale data alone — the draft UI must always receive a response.
4. An ENV var `DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK=true` provides the instant operational override for maintenance windows, disabling the threshold check entirely and still serving data (Option 1 escape hatch).
5. Option 3 (sync health table) is deferred. If partial sync runs become a production problem, a `sync_health` table can be added as an enhancement without invalidating this decision.

### Rationale

Option 2 is preferred over Option 1 alone because it is self-healing — no operator action is required when Sync recovers. The stale threshold approach is a well-understood degraded-mode pattern (used in distributed caches, CDNs, and read replicas). The `X-Cache-Degraded` header gives the frontend visibility to display a subtle "ratings may not be current" notice without blocking the user.

Option 1 alone is rejected as the primary mechanism because it requires manual operator intervention to re-enable after recovery, and it produces no observability signal. The ENV var is retained purely as an emergency override.

Option 3 is deferred because its correctness benefit (distinguishing partial vs. full syncs) is premature at current data volumes. If 17Lands ratings are fetched in a single HTTP call per set, the partial-sync gap does not exist in practice. The threshold approach covers the operational need today.

### Default configuration

| Parameter | Default | Notes |
|---|---|---|
| `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS` | `48` | 2× the daily sync cadence; allows one missed run without degraded mode |
| `DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK` | `false` | Set to `true` during Sync maintenance windows |

### Response contract

When serving ratings in degraded mode:

```
HTTP 200 OK
X-Cache-Degraded: true
X-Cache-Age-Hours: <integer>    # hours since last successful Sync write
Content-Type: application/json
```

When serving fresh ratings (normal mode):

```
HTTP 200 OK
Content-Type: application/json
```

The frontend should inspect `X-Cache-Degraded` and display a non-blocking notice when `true`.

---

## Implementation Plan

The following items need to be built. They are listed in dependency order.

1. **BFF: Add `cached_at` read to draft ratings query** — The existing `draft_card_ratings` table has `cached_at`. The BFF's future draft ratings repository method must return the max `cached_at` alongside the ratings rows so the handler can compute staleness.

2. **BFF: Add `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS` and `DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK` env vars to config** — Extend `services/bff/internal/config/` (or equivalent) to read and expose these two values. Include validation (threshold must be > 0).

3. **BFF: Implement draft ratings read handler** — New handler in `services/bff/internal/api/handlers/draft_ratings.go`. Handler queries `draft_card_ratings` by set code and draft format, computes staleness from `MAX(cached_at)`, sets `X-Cache-Degraded` and `X-Cache-Age-Hours` headers when applicable, and returns 200 in all cases where rows exist. Returns 404 if no rows found for the requested set/format.

4. **BFF: Register draft ratings route** — Wire the handler into the chi router under `/api/v1/draft-ratings/{setCode}/{format}`.

5. **Frontend: Consume `X-Cache-Degraded` header** — The draft ratings fetch adapter must inspect the response header and surface a degraded-mode notice in the draft UI when `X-Cache-Degraded: true` is present.

6. **BFF: Integration tests for staleness threshold logic** — Test that the handler returns `X-Cache-Degraded: true` when `cached_at` is older than the configured threshold, and does not when fresh. Test the bypass env var path. Test 404 for unknown set/format.

---

## Acceptance Criteria

- [ ] BFF draft ratings handler never returns 5xx due to stale data; always returns 200 when rows exist.
- [ ] `X-Cache-Degraded: true` header is present when `cached_at` is older than the configured threshold.
- [ ] `X-Cache-Age-Hours` header is present alongside `X-Cache-Degraded`.
- [ ] `DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK=true` disables the threshold check; handler serves data without degraded headers regardless of age.
- [ ] Default threshold is 48 hours and is configurable via `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS`.
- [ ] Frontend displays a non-blocking notice when `X-Cache-Degraded: true`.
- [ ] Integration tests cover: fresh data (no degraded header), stale data (degraded header), bypass mode, 404 for missing set/format.
- [ ] No new DB schema required (deferred to Option 3 if needed).

---

## Alternatives Considered

See Options 1 and 3 above. Option 1 alone was rejected as the primary mechanism because it is not self-healing and produces no observability signal. Option 3 was deferred as premature given current sync run characteristics (single HTTP call per set; partial run gap is theoretical, not observed).
