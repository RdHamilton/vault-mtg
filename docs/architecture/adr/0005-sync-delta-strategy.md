# ADR-005: Delta Sync Strategy for services/sync

**Date**: 2026-05-04
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

Issue #1079 asks whether the sync Lambda can be changed from a full sync (fetch and upsert all card ratings for all active sets on every invocation) to a delta sync (only fetch and persist data that has changed since the last run).

At the time of this ADR, the sync Lambda:

- Calls `GET /card_ratings/data?expansion=<set>&format=PremierDraft` on 17Lands once per active set per daily invocation.
- Deletes all rows for the set/format and re-inserts the full payload (delete-then-insert strategy in `UpsertRatings`).
- Stores `cached_at` (the `FetchedAt` timestamp from `draftdata.SetRatings`) on every row.
- Calls Scryfall `GET /sets` and upserts all Arena expansion/core sets.

The issue also asks whether match and draft data should be ingested from 17Lands datasets. This is addressed separately below.

### 17Lands API: delta capabilities

The public 17Lands card ratings API (`/card_ratings/data`) does not expose any cursor, `since`, `etag`, `If-Modified-Since`, or `last-updated-at` query parameter. Every call returns the full current snapshot for the requested set/format. There is no server-side mechanism to request only records that changed since a prior call.

17Lands does publish bulk dataset files (CSV/JSON) at `https://17lands.com/public_datasets` — these are snapshots of aggregate pick and game data, updated periodically. They are not real-time and are not served via the API endpoint the sync service currently calls.

**Conclusion**: The 17Lands card ratings API does not support server-side delta fetching.

### Feasibility of client-side delta via payload hashing

Because the API always returns a full snapshot, a client-side approach is possible:

1. Fetch the full payload for a set/format.
2. Compute a hash (e.g., SHA-256) of the JSON body or the sorted set of `(arena_id, gihwr, ohwr, alsa, ata, gih_count)` tuples.
3. Compare to the hash stored from the previous successful sync.
4. If hashes match, skip the database write entirely.

This avoids unnecessary DB writes when 17Lands data has not changed (which is common — ratings update at most once per day during peak season, often less frequently for older sets).

### Existing schema assets

The `dataset_metadata` table already exists (migration 000054, initial schema):

```sql
CREATE TABLE IF NOT EXISTS dataset_metadata (
    id              BIGSERIAL PRIMARY KEY,
    set_code        TEXT NOT NULL,
    draft_format    TEXT NOT NULL,
    data_source     TEXT NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total_cards     INTEGER,
    total_games     INTEGER,
    dataset_version TEXT,           -- currently unused
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format)
);
```

The `dataset_version` column is unused and can serve as the payload hash. No new table is needed.

### Match and draft data from 17Lands datasets

The issue also asks whether match and draft data should be ingested from 17Lands public datasets (bulk CSV/JSON files, not the API). Assessment:

- 17Lands public datasets contain aggregate draft pick data and game win rates — community-level statistics, not per-user match or draft records. They cannot be mapped to individual MTGA player accounts.
- VaultMTG already collects match and draft data directly from `Player.log` via the daemon. This is the correct authoritative source for per-user data.
- Ingesting 17Lands bulk dataset files would only be useful to supplement community-level aggregate statistics (e.g., color win rates, archetype win rates), which is a separate feature with its own scope. That is out of scope for the sync service's current role.

**Conclusion**: The sync service should not ingest 17Lands bulk dataset files as a substitute for or supplement to daemon-collected match/draft data. If community aggregate ingestion is desired in the future, it warrants its own ADR and service boundary decision.

---

## Decision

**Adopt client-side payload hashing as the delta sync mechanism for 17Lands card ratings.**

1. After fetching the card ratings payload for a set/format, the sync Lambda computes a SHA-256 hash of the normalized payload (sorted by `arena_id`).
2. The Lambda reads the stored hash from `dataset_metadata.dataset_version` for that `(set_code, draft_format)` row.
3. If the hashes match, the Lambda skips `UpsertRatings` for that set/format and logs a skip event. No DB write occurs.
4. If the hashes differ (or no prior hash exists), the Lambda proceeds with the existing delete-then-insert `UpsertRatings` path and writes the new hash to `dataset_metadata.dataset_version`.
5. `dataset_metadata.last_updated_at` is updated only when a new upsert is performed (hash changed). This provides an accurate "last real change" timestamp distinct from `cached_at` in `draft_card_ratings`.

### What does not change

- The 17Lands API call still happens on every Lambda invocation per active set. We cannot skip the fetch — the API provides no staleness signal.
- The delete-then-insert strategy in `UpsertRatings` is unchanged for cases where data has changed.
- The Scryfall set sync path is unchanged. Scryfall does not provide ETags or delta endpoints either; the existing upsert-by-conflict approach is already idempotent and cheap.
- ADR-004 (staleness threshold via `cached_at`) is unchanged. The BFF reads `cached_at` from `draft_card_ratings` rows, which still reflects the `FetchedAt` of the last upsert. When data is unchanged and we skip the upsert, `cached_at` does not advance — this is correct, because the data was not re-fetched from a write perspective. The BFF staleness threshold (48 hours default) must be calibrated to account for sets where data is stable and skips occur across multiple daily runs. The threshold is still measured against the last time data was actually written, not the last invocation.

### Hash normalization

To produce a stable, order-independent hash:

- Sort the `[]CardRating` slice by `MtgaID` ascending before hashing.
- Marshal to JSON and compute SHA-256.
- Store the hex-encoded digest in `dataset_metadata.dataset_version`.

This is deterministic regardless of the order 17Lands returns cards.

### Handling the BFF staleness threshold with skip runs

When the hash matches and a write is skipped, `draft_card_ratings.cached_at` does not advance. Over multiple daily Lambda invocations where 17Lands data has not changed, `cached_at` will age. The BFF staleness threshold (ADR-004, default 48 hours) will eventually trigger `X-Cache-Degraded: true` even though the data is correct — it is just stable.

Resolution: the `DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS` env var should be set to a value that accounts for expected skip runs. For sets in active draft rotation, 17Lands updates ratings frequently; threshold of 48 hours remains appropriate. For sets that have rotated out of standard, the sync service should stop including them in `GetActiveSets` (governed by `is_standard_legal = TRUE` already). No threshold change is required as a result of this ADR.

---

## Consequences

**Easier**

- Eliminates unnecessary DB writes on days when 17Lands data has not changed, reducing write amplification on `draft_card_ratings`.
- `dataset_metadata.last_updated_at` becomes a meaningful "data changed at" timestamp, not just "Lambda ran at".
- No new table or migration required — `dataset_metadata.dataset_version` is already present.

**Harder**

- The sync Lambda now has a two-phase flow per set: fetch → hash-check → conditional upsert. This adds one DB read per set per run (read current hash from `dataset_metadata`).
- Hash normalization (sort + marshal) must be tested explicitly to ensure stability across Go versions and map iteration order changes.
- The `dataset_metadata` write path must be added to the `Store` interface and `PostgresStore`.

---

## Alternatives Considered

### A — Full sync on every invocation (status quo)

Acceptable at current scale (2-4 active sets, daily cadence). Rejected because the issue specifically asks for efficiency improvements and the fix is low-cost.

### B — ETag / If-Modified-Since at the HTTP layer

Not possible. The 17Lands API does not return `ETag` or `Last-Modified` headers. Rejected.

### C — Skip the fetch entirely and rely on a "ratings update schedule" heuristic

Rejected. We have no reliable signal for when 17Lands publishes updated ratings. Fetching and checking is the only correct approach.

### D — Ingest 17Lands bulk dataset files for community aggregate data

Deferred to a future ADR. Out of scope for the current sync service role, which focuses on per-set card ratings for the draft pick advisor.
