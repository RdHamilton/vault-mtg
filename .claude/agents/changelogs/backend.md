# Backend Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file.go` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-03 — Issue #1011: fix UpsertRatings storing 0 rows and missing standard-legal sets
**PR**: #1043
**Files changed**:
- `services/sync/internal/datasets/postgres_store.go` — replace ON CONFLICT upsert with DELETE + batch INSERT to fix arena_id=0 collision; add inserted-row log after commit
- `services/sync/internal/datasets/postgres_store_test.go` — add TestMockStore_SecondUpsertReplacesAllCards to verify DELETE+INSERT semantics across two consecutive calls
- `services/sync/internal/refresh/scheduler.go` — add WARNING log when 0 cards returned from 17Lands (set code mismatch indicator)
- `services/bff/internal/storage/migrations/postgres/000058_fix_standard_legal_sets.up.sql` — seed all 11 current standard-legal sets with ON CONFLICT upsert
- `services/bff/internal/storage/migrations/postgres/000058_fix_standard_legal_sets.down.sql` — revert is_standard_legal for the 11 sets
**Summary**: Fixed two bugs: UpsertRatings was collapsing all cards to a single row due to arena_id=0 ON CONFLICT; the sets table only had 3 of 11 standard-legal sets seeded. Also added a 0-card warning to surface 17Lands set-code mismatches (AED/BIG).

## 2026-05-03 — Issue #1011: scaffold services/sync Go module for 17Lands and card data polling (ADR-001 Approach B)
**PR**: #1043
**Files changed**:
- `services/sync/go.mod` — new Go module (github.com/ramonehamilton/mtga-sync)
- `services/sync/cmd/main.go` — entry point: pgxpool wiring, graceful shutdown
- `services/sync/internal/seventeenlands/client.go` — HTTP client for 17Lands card ratings API
- `services/sync/internal/seventeenlands/rating.go` — CardRating domain struct
- `services/sync/internal/seventeenlands/client_test.go` — httptest-based unit tests
- `services/sync/internal/draftdata/models.go` — SetRatings aggregate model
- `services/sync/internal/datasets/store.go` — Store interface (GetActiveSets, UpsertRatings, GetRatings)
- `services/sync/internal/datasets/postgres_store.go` — pgxpool implementation; queries sets.is_standard_legal for active sets
- `services/sync/internal/datasets/postgres_store_test.go` — mock round-trip and interface compile-time assertion
- `services/sync/internal/refresh/scheduler.go` — daily scheduler; queries DB for active sets, SYNC_ACTIVE_SETS env overrides
- `services/sync/internal/refresh/scheduler_test.go` — startup fetch, DB-sourced sets, and no-sets skip tests
- `services/bff/internal/storage/migrations/postgres/000057_create_sync_user_grants.up.sql` — mtga_sync Postgres role scoped to card/ratings tables
- `services/bff/internal/storage/migrations/postgres/000057_create_sync_user_grants.down.sql` — drop mtga_sync role
- `.github/workflows/sync.yml` — path-filtered CI (build, test, vet)
- `go.work` — added services/sync module
**Summary**: Scaffolded the sync service as an independent Go module per ADR-001 Approach B; active sets are resolved dynamically from sets.is_standard_legal rather than a static env var, with SYNC_ACTIVE_SETS retained as a local override.
