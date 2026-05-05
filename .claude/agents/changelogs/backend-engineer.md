# Backend Engineer Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file.go` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-05 — [backend-engineer] Issue #1130: fix(sync): expand Scryfall filter to include alchemy, masters, and draft_innovation sets
**PR**: #1189
**Files changed**:
- `services/bff/internal/storage/migrations/postgres/000062_add_is_draft_active.down.sql` — migration rollback: drop is_draft_active column
- `services/bff/internal/storage/migrations/postgres/000062_add_is_draft_active.up.sql` — migration: add is_draft_active column to sets table with backfill from is_standard_legal
- `services/sync/internal/datasets/postgres_store.go` — updated UpsertSets to set is_draft_active=TRUE; updated GetActiveSets to query is_draft_active=TRUE
- `services/sync/internal/datasets/store.go` — updated Store interface signatures for is_draft_active column
- `services/sync/internal/scryfall/client.go` — added isDraftableSetType helper covering expansion, core, masters, draft_innovation, alchemy
- `services/sync/internal/scryfall/client_test.go` — updated and added tests for all 5 draftable set types and excluded types
**Summary**: Expanded the Scryfall set filter to cover all Arena-draftable set types (masters, draft_innovation, alchemy) and added a dedicated is_draft_active column to separate draft availability from Standard legality.

## 2026-05-04 — [backend-engineer] Issue #1135: fix(sync): wire color ratings fetch into scheduler and Lambda handler
**PR**: #1191
**Files changed**:
- `services/sync/internal/seventeenlands/rating.go` — added ColorRating struct
- `services/sync/internal/seventeenlands/client.go` — added FetchColorRatings method
- `services/sync/internal/seventeenlands/client_test.go` — unit tests for FetchColorRatings
- `services/sync/internal/datasets/store.go` — added UpsertColorRatings to Store interface
- `services/sync/internal/datasets/postgres_store.go` — implemented UpsertColorRatings with DELETE + batch INSERT
- `services/sync/internal/datasets/postgres_store_test.go` — updated stubs and added tests
- `services/sync/internal/refresh/scheduler.go` — wired color ratings fetch after card ratings per set/format
- `services/sync/internal/refresh/scheduler_test.go` — scheduler color ratings tests
- `services/sync/internal/handler/lambda.go` — wired color ratings fetch into Lambda handler
- `services/sync/internal/handler/lambda_test.go` — Lambda color ratings tests
**Summary**: Wired the color ratings fetch (17Lands /color_ratings/data) into both the scheduler and Lambda handler so draft_color_ratings are populated after each card ratings sync run for every active set and format.

## 2026-05-04 — [bff] Issue #1173: wire IngestHandler to persist events before broadcast
**PR**: #1178
**Files changed**:
- `services/bff/internal/storage/repository/daemon_events_repo.go` — DaemonEventsRepository with Insert and ListByUserID; DaemonEventRow struct; all queries scoped by user_id
- `services/bff/internal/api/handlers/ingest.go` — DaemonEventInserter interface; repo field on IngestHandler; WithRepository setter; persistence call before broadcast with error-logged-but-non-fatal failure mode
- `services/bff/internal/api/handlers/ingest_test.go` — mockDaemonEventsRepo stub; three new tests (persist-when-wired, broadcast-even-on-insert-failure, nil-repo-broadcast-only)
- `services/bff/cmd/main.go` — wire NewDaemonEventsRepository and ingestHandler.WithRepository inside cfg.DatabaseURL != "" block
**Summary**: Wired DaemonEventsRepository to IngestHandler so daemon events are persisted to the database before broadcasting; persistence failures are logged but never drop the live SSE event, completing sub-task C of the #1126 ingest pipeline decomposition.

## 2026-05-04 — [bff] Issue #1172: implement DaemonEventsRepository [#1126-B]
**PR**: #1176
**Files changed**:
- `services/bff/internal/storage/repository/daemon_events_repo.go` — DaemonEventsRepository with Insert and ListByUserID; DaemonEventRow struct; all queries scoped by user_id
- `services/bff/internal/storage/migrations/postgres/` — migration adding daemon_events table
- `services/bff/internal/storage/repository/daemon_events_repo_test.go` — integration tests for Insert and ListByUserID
**Summary**: Implemented DaemonEventsRepository as the data-access layer for persisting daemon ingest events, completing sub-task B of the #1126 ingest pipeline decomposition.

## 2026-05-04 — [daemon] Issue #1094: feat(daemon): install scripts (PowerShell + launchd)
**PR**: #TBD
**Files changed**:
- `services/daemon/install/macos/install.sh` — new: detects arch, downloads binary from GitHub Releases, installs to /usr/local/bin, writes launchd plist, loads with launchctl
- `services/daemon/install/macos/uninstall.sh` — new: unloads launchd job, removes plist and binary
- `services/daemon/install/windows/install.ps1` — new: downloads Windows amd64 binary, writes daemon.yaml config, registers AtLogon Task Scheduler task (no UAC required)
- `services/daemon/install/windows/uninstall.ps1` — new: stops and removes scheduled task and binary
- `services/daemon/install/README.md` — new: one-liner install instructions for macOS and Windows
**Summary**: Added platform install scripts so users can install and autostart the daemon on macOS (via launchd) and Windows (via Task Scheduler) without admin elevation on Windows; binary is downloaded from GitHub Releases with auto-detection of the latest daemon/* tag.

## 2026-05-04 — [daemon] Issue #1131: fix(daemon): JWT mid-session expiry refresh + CI and binary naming cleanup
**PR**: #1175
**Files changed**:
- `services/daemon/internal/dispatcher/dispatcher.go` — added 401 detection and JWT refresh logic for mid-session token expiry
- `services/daemon/internal/dispatcher/dispatcher_test.go` — unit tests for 401 refresh path
- `.github/workflows/release.yml` — consolidated dual CI workflow confusion; standardized binary naming
**Summary**: Fixed mid-session JWT expiry by adding 401-triggered refresh in the dispatcher, cleaned up dual CI workflow confusion, and standardized daemon binary naming across platforms.
**Merged**: 2026-05-04 — PR #1175 merged into main.

## 2026-05-03 — [bff] Issue #1045: sync service — fetch active standard sets from Scryfall
**PR**: #1051
**Files changed**:
- `services/sync/internal/scryfall/client.go` — `Client.FetchSets` with 30s timeout, `net/url` construction, filters to digital expansion/core sets only
- `services/sync/internal/scryfall/set.go` — `ScryfallSet` domain struct
- `services/sync/internal/scryfall/client_test.go` — httptest-based unit tests
- `services/sync/internal/datasets/store.go` — added `UpsertSets(ctx, []scryfall.ScryfallSet) error` to `Store` interface
- `services/sync/internal/datasets/postgres_store.go` — implemented `UpsertSets` with `ON CONFLICT (code) DO UPDATE SET is_standard_legal=TRUE`
- `services/sync/internal/refresh/scheduler.go` — Scryfall set sync runs before 17Lands ratings fetch on each daily run
- `services/sync/cmd/main.go` — wired `scryfall.NewClient()` as the set fetcher
**Summary**: Eliminated manual BFF migrations to seed `sets.is_standard_legal` — the sync service now fetches active standard sets from Scryfall daily and upserts them into the sets table automatically, keeping standard-legal state current without operator intervention.

## 2026-05-03 — [bff] PR review fixes: feat/sync-service-scaffold
**Files changed**:
- `services/bff/internal/storage/migrations/postgres/000057_create_sync_user_grants.up.sql` — remove hardcoded password, add DELETE grant on draft_card_ratings, replace hardcoded DB name with current_database()
- `services/bff/internal/storage/migrations/postgres/000058_fix_standard_legal_sets.down.sql` — restore rotation_date values in down migration
- `services/bff/internal/storage/migrations/postgres/000058_fix_standard_legal_sets.up.sql` — ON CONFLICT clause now updates name and released_at
- `services/sync/go.mod` / `go.sum` — upgrade pgx v5.7.5 → v5.9.2
- `services/sync/internal/refresh/scheduler.go` — warn on invalid SYNC_REFRESH_HOUR; check ctx.Err() inside set loop
- `services/sync/internal/seventeenlands/client.go` — 30s HTTP timeout; URL-encode query params via net/url
- `.github/workflows/sync.yml` — enable race detector on test step
- `services/sync/internal/datasets/postgres_store_integration_test.go` — new integration test (build tag: integration)
**Summary**: Address all PR review comments on the sync service scaffold: security hardening (no hardcoded password, proper DB grant scoping), correctness fixes (down migration, ON CONFLICT clause), robustness improvements (HTTP timeout, URL encoding, ctx cancellation, invalid env var warning), and a race detector in CI.

## 2026-05-03 — [bff] Issue #1011: fix UpsertRatings storing 0 rows and missing standard-legal sets
**PR**: #1043
**Files changed**:
- `services/sync/internal/datasets/postgres_store.go` — replace ON CONFLICT upsert with DELETE + batch INSERT to fix arena_id=0 collision; add inserted-row log after commit
- `services/sync/internal/datasets/postgres_store_test.go` — add TestMockStore_SecondUpsertReplacesAllCards to verify DELETE+INSERT semantics across two consecutive calls
- `services/sync/internal/refresh/scheduler.go` — add WARNING log when 0 cards returned from 17Lands (set code mismatch indicator)
- `services/bff/internal/storage/migrations/postgres/000058_fix_standard_legal_sets.up.sql` — seed all 11 current standard-legal sets with ON CONFLICT upsert
- `services/bff/internal/storage/migrations/postgres/000058_fix_standard_legal_sets.down.sql` — revert is_standard_legal for the 11 sets
**Summary**: Fixed two bugs: UpsertRatings was collapsing all cards to a single row due to arena_id=0 ON CONFLICT; the sets table only had 3 of 11 standard-legal sets seeded. Also added a 0-card warning to surface 17Lands set-code mismatches (AED/BIG).

## 2026-05-03 — [bff] Issue #1011: scaffold services/sync Go module for 17Lands and card data polling (ADR-001 Approach B)
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
**Merged**: 2026-05-03 — PR #1043 merged and verified.

## 2026-05-03 — [daemon] Issue #1014: daemon: investigate log preservation and MTGA log overwrite on startup
**PR**: #1042
**Files changed**:
- `services/daemon/internal/logreader/preservation.go` — new: `Snapshot`, `ListSnapshots`, `PruneSnapshots`, `copyFile`
- `services/daemon/internal/logreader/preservation_test.go` — new: 7 unit tests for all preservation functions
- `services/daemon/internal/logreader/poller.go` — drain bug fix: call `checkForUpdates()` before position reset on Remove/Rename fsnotify events
- `services/daemon/internal/logreader/reader_test.go` — added `TestPollerHandlesRotationDrain` integration test
- `services/daemon/internal/config/config.go` — added `LogArchiveDir`, `LogArchiveMaxAge` (7d default), `LogPreserveOnStart` (true default) with env var `MTGA_DAEMON_LOG_ARCHIVE_DIR`
- `services/daemon/internal/config/config_test.go` — 4 new config field tests
- `services/daemon/internal/daemon/service.go` — wired `Snapshot` + `PruneSnapshots` into `Run()` before poller starts
**Summary**: Fixed log preservation so the daemon snapshots Player.log on startup before MTGA can overwrite it; also fixed a bug where unread bytes in the old log were discarded when a rotation was detected via fsnotify. Filed follow-on issue #1041 for dead `models.go` structs that need alignment with actual MTGA JSON keys.
**Merged**: 2026-05-03 — PR #1042 merged into main.
