# Architect Agent Changelog

This is the system-wide record of all changes made across the project. Every agent (daemon, backend, frontend, architect) appends here on task completion. The architect reads this file to maintain full context of what has been built.

<!-- Entry format — prefix the agent name in brackets:
## YYYY-MM-DD — [<agent>] Issue #NNN: <title>
**PR**: #NNN (or "N/A — ADR only")
**ADR**: docs/adr/NNN-title.md (if applicable, architect entries only)
**Files changed**:
- `path/to/file` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-16 — [architect] Bug: daemon postinstall multi-instance on reinstall
**PR**: #2154
**ADR**: N/A
**Files changed**:
- `services/daemon/install/macos/pkg/postinstall` — expanded stop/cleanup block: label-based bootout, pkill fallback, sleep 1 before bootstrap
- `services/daemon/install/macos/pkg/postinstall_test.bats` — added test #8 verifying bootout precedes bootstrap on reinstall
**Summary**: Fixed a bug where reinstalling the daemon pkg could spawn multiple vaultmtg-daemon processes because postinstall loaded the new LaunchAgent without first stopping the old one via label-based bootout and pkill fallback.

## 2026-05-17 — [architect] RC24 tag cut
**PR**: N/A — release tag
**ADR**: N/A
**Summary**: Cut daemon/v0.3.1-rc24 from main to trigger RC24 installer build; includes PRs #2150 and #2152 (keychain reinstall fix + retry/tray-error-state).

## 2026-05-17 — [architect] Issue #2151: feat(daemon): keychain retry loop, StatusKeychainError tray state, BFF dispatch gating
**PR**: #2152
**ADR**: N/A
**Summary**: Added 3-attempt exponential-backoff retry loop when keychain.Get() fails at startup; StatusKeychainError tray status + "Try Again" menu item; BFF event dispatch gated behind successful keychain resolution; headless exit path when all retries exhausted; 4 new unit tests covering all retry paths.

## 2026-05-17 — [architect] Issue #2136: fix(daemon): NeedsFirstRunAuth verifies keychain entry before trusting sentinel
**PR**: #2150
**ADR**: N/A
**Summary**: Fixed silent reinstall bug where NeedsFirstRunAuth returned false when keychain:true without verifying the OS keychain entry actually exists; refactored to accept keychainGetter func() (string, error) parameter to avoid import cycle.

## 2026-05-09 — [architect] v0.3.1 Wave 0 architectural review and gate
**PR**: #1673
**ADR**: N/A — review note only; flags need for ADR-020 addendum (keychain naming, contracts section) and potential ADR-021 (daemon_api_keys lifecycle)
**Summary**: Delivered mandatory Wave 0 Architectural Implications Note for v0.3.1 Packaging. Verdict: APPROVED WITH CONDITIONS — Wave 1 and Wave 2 unblocked immediately; Wave 4 gated on 8 pre-conditions covering keychain naming resolution, PKCE callback port decision (OQ-5/6/7), Clerk redirect URI registration, missing daemon_api_keys migration, and POST /v1/daemon/register contract documentation.

## 2026-05-08 — [architect] ADR audit for v0.4.0 + ADR-017, ADR-018
**PR**: N/A — ADRs committed directly to current branch (fix/ci-e2e-bff-dev-mode)
**ADR**: docs/adr/017-bff-precomputed-read-contract.md, docs/adr/018-list-endpoint-pagination-standard.md
**Files changed**:
- `docs/adr/017-bff-precomputed-read-contract.md` — read envelope for /v1/user/craft-next and future precomputed endpoints; status enum (ok / not_yet_computed / stale / partial_fallback / format_unsupported); 200-on-empty contract; no request-time computation; account scoped from Clerk context
- `docs/adr/018-list-endpoint-pagination-standard.md` — keyset pagination standard for #1516; shared listing envelope; typed filter allowlist per handler; no offset; covering-index DBA review per endpoint; /v1 → /v2 migration plan with one-release deprecation window
**Summary**: Audited existing 16 ADRs against v0.4.0 / Smart Craft Next scope; identified two critical-path gaps (BFF read contract for precomputed reads, list pagination/filter standard) and authored both. Remaining gaps (data retention/GDPR, PostHog event taxonomy, beta invite flow) flagged for follow-up tickets but deferred — not on Wave 4 critical path.

## 2026-05-08 — [architect] prod fixes: nginx welcome page + IAM PutBucketVersioning gap
**PR**: RdHamilton/mtga-companion-infra#29 (CFN); nginx hot-patch applied directly to EC2 i-065351fbb99da2d22 via SSM
**Files changed**:
- (infra) cloudformation/deploy-artifacts.yml — add s3:PutBucketVersioning + s3:GetBucketVersioning to StagingBucketAccess; add StagingArtifactsBucketName param; reference staging bucket by ARN; drop StagingDeployArtifactsBucket resource (bucket managed out-of-band)
- (infra) nginx/api.vaultmtg.app.conf — add location = / 302 to app.vaultmtg.app; add catch-all JSON 404 (committed locally; pending PR for source-of-truth)
- (ec2) /etc/nginx/conf.d/api.vaultmtg.app.conf — same content patched in-place via SSM, nginx reloaded; old version backed up as .bak.<ts>
- (aws) stack mtga-companion-deploy-artifacts — UPDATE_COMPLETE with new IAM permissions
**Summary**: Fixed two prod-blocking issues. (1) nginx returned the default welcome page on https://api.vaultmtg.app/ because the api.vaultmtg.app server block had no location / handler — added a 302 redirect to app.vaultmtg.app and a JSON 404 catch-all. (2) staging-deploy GitHub Actions workflow was failing AccessDenied on put-bucket-versioning — added s3:PutBucketVersioning and s3:GetBucketVersioning to the GitHubActionsDeployRole and deployed the stack. Discovered: BFF runs as bare process with no systemd unit — filed as follow-on for PM/infrastructure.

## 2026-05-06 — [architect] Issue #1117: holistic gap analysis — Sync Lambda and BFF
**PR**: (this PR)
**ADR**: N/A — gap analysis doc; recommends three new ADRs (010, 011, 012)
**Files changed**:
- `docs/architecture/1117-sync-lambda-bff-gap-analysis.md` — new gap analysis covering data flow, auth, error handling, multi-tenancy, missing pieces, and six numbered recommendations
**Summary**: Documented that Sync Lambda and BFF share only the Postgres schema (no HTTP / SQS link) and that the largest forward gap is the lack of a per-user sync surface for Phase 4+ Pro features; recommended ADR-010 (sync observability), ADR-011 (per-user sync service), and ADR-012 (internal service-to-service auth) with ADR-012 sequenced before ADR-011.

## 2026-05-06 — [architect] ADR-009: User Auth Provider = Clerk
**PR**: N/A — ADR only
**ADR**: docs/adr/ADR-009-user-auth-provider-clerk.md
**Files changed**:
- `docs/adr/ADR-009-user-auth-provider-clerk.md` — new ADR selecting Clerk over Supabase Auth, with explicit forbidden patterns for the React+Vite integration (`@clerk/react@latest`, `VITE_CLERK_PUBLISHABLE_KEY`, `<ClerkProvider afterSignOutUrl="/">` in `main.tsx` only, `<Show when="signed-in">`, no `frontendApi`/`publishableKey` props, no `<SignedIn>`/`<SignedOut>`)
**Summary**: Recorded the Phase 3 user-auth decision (Clerk) with concrete frontend/backend integration patterns, daemon API-key migration plan, and a list of nine implementation tickets the Project Manager must file.

## 2026-05-05 — [architect] ADR-007: Frontend Serving Model
**PR**: #1221
**ADR**: docs/adr/007-frontend-serving-model.md
**Files changed**:
- `docs/adr/007-frontend-serving-model.md` — new ADR
- `.claude/plans/adr-007-tickets.md` — six Sonnet-ready implementation tickets for the PM
**Summary**: Resolved Vercel-vs-EC2 frontend serving conflict (introduced by PR #1184) by declaring Vercel canonical and demoting the EC2 nginx path to manual-dispatch DR/preview-only; unblocks #1211 and #1066.

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

## 2026-05-05 — [architect] PR #1200: fix: replace integer 0/1 with FALSE/TRUE in E2E fixtures
**PR**: #1200
**Files changed**:
- `frontend/tests/e2e/fixtures/test-data.sql` — replaced integer 0/1 literals with FALSE/TRUE for boolean columns (is_standard_legal, rotation_enabled, from_draft_pick, completed, can_swap) to fix SQLSTATE 42804 crash on PostgreSQL 16
**Summary**: Fixed E2E smoke test startup crash caused by PostgreSQL 16 rejecting integer literals for boolean columns; updated all affected boolean columns in the test fixture SQL while intentionally leaving accounts.is_default as integer (non-boolean column).

## 2026-05-05 — [infrastructure] Issue #1068: feat(infra): deploy React SPA to nginx on EC2 (ADR-001 frontend serving)
**PR**: #1184 (in RdHamilton/MTGA-Companion)
**Files changed**:
- `.github/workflows/frontend.yml` — new workflow: builds React/Vite SPA and deploys dist/ to EC2 nginx webroot via S3 + SSM with atomic staging-dir swap
**Summary**: Added the frontend deploy workflow that builds the SPA with VITE_BFF_URL=/api/v1 and atomically deploys it to /var/www/mtga-companion/ on EC2 via SSM RunShellScript, completing ADR-001 frontend serving without any nginx config changes.

## 2026-05-04 — [architect] Issue #1025: Vercel→BFF connectivity ADR and CORS update
**PR**: #1174 (merged)
**ADR**: N/A
**Summary**: PR #1174 merged covering ticket #1025 (Vercel→BFF connectivity ADR and CORS update); ticket moved to Done on project board #27.

## 2026-05-04 — [backend] Issue #1172: implement DaemonEventsRepository [#1126-B]
**PR**: #1176
**Files changed**:
- `services/bff/internal/storage/repository/daemon_events_repo.go` — DaemonEventsRepository with Insert and ListByUserID; all queries scoped by user_id
- `services/bff/internal/storage/migrations/postgres/` — migration adding daemon_events table
- `services/bff/internal/storage/repository/daemon_events_repo_test.go` — integration tests
**Summary**: Implemented DaemonEventsRepository as the data-access layer for persisting daemon ingest events, completing sub-task B of the #1126 ingest pipeline decomposition.

## 2026-05-04 — [backend] Issue #1173: wire IngestHandler to persist events before broadcast [#1126-C]
**PR**: #1178
**Files changed**:
- `services/bff/internal/storage/repository/daemon_events_repo.go` — DaemonEventsRepository with Insert and ListByUserID; DaemonEventRow struct; all queries scoped by user_id
- `services/bff/internal/api/handlers/ingest.go` — DaemonEventInserter interface; repo field on IngestHandler; WithRepository setter; persistence before broadcast with non-fatal error logging
- `services/bff/internal/api/handlers/ingest_test.go` — mockDaemonEventsRepo; three new tests covering persist-when-wired, broadcast-on-failure, nil-repo modes
- `services/bff/cmd/main.go` — wire DaemonEventsRepository inside cfg.DatabaseURL != "" block
**Summary**: Wired DaemonEventsRepository to IngestHandler so daemon events are persisted before broadcasting; persistence failures are logged but never drop live SSE events, completing sub-task C of the #1126 ingest pipeline.

## 2026-05-04 — [frontend] Issue #1136 #1142: fix(frontend): add BFF draft-ratings and API key adapters
**PR**: #1177
**Files changed**:
- `frontend/src/services/api/bffDraftRatings.ts` — new adapter: getDraftRatings() targeting GET /api/v1/draft-ratings/{setCode}/{format} with cache-degraded header support
- `frontend/src/services/api/bffDraftRatings.test.ts` — 10 MSW tests covering URL, response shape, header parsing, URL encoding, error handling
- `frontend/src/services/api/bffAuth.ts` — new adapter: createAPIKey() targeting POST /api/keys with daemon JWT auth
- `frontend/src/services/api/bffAuth.test.ts` — 9 MSW tests covering URL, Authorization header, response shape, error handling
- `frontend/src/services/api/index.ts` — exported both new modules and their TypeScript types
**Summary**: Added two BFF-only adapter modules for the draft-ratings and API key endpoints; both use direct fetch (not apiClient wrappers) because the BFF returns raw JSON rather than the data-wrapped envelope shape.

## 2026-05-03 — [architect] Issue #1050: architecture review — sync service deployment drift
**PR**: N/A — ADR only
**ADR**: docs/adr/003-sync-service-deployment-strategy.md
**Files changed**:
- `docs/adr/003-sync-service-deployment-strategy.md` — new: ADR-003 documents Lambda deployment for sync, RDS IAM auth decision, impact on merged PRs #1048/#1049/#1053
- `docs/architecture/CHANGELOG.md` — new: architect changelog tracking major findings and plan sync events
- `~/.claude/plans/mtga-companion-aws-launch.md` — updated: marked sync scaffold complete, flagged EC2 deploy step as needing replacement, documented Lambda next steps
**Summary**: Discovered and documented that services/sync drifted from ADR-001 (Lambda+EventBridge) when EC2/systemd artifacts were merged in PRs #1048-#1053; wrote ADR-003 to formally re-affirm the Lambda deployment decision and resolve the credential strategy gap (issue #1054) via RDS IAM auth on Lambda execution role.

## 2026-05-03 — [architect] Issue #1016: arch: design SetCache ownership flip mechanism for sync/BFF
**PR**: #1081
**ADR**: docs/adr/004-setcache-ownership-flip.md
**Files changed**:
- `docs/adr/004-setcache-ownership-flip.md` — new: ADR-004 documents staleness threshold mechanism for BFF draft ratings handler; Option 1 (feature flag) kept as ENV var escape hatch; Option 3 (sync health table) deferred
**Summary**: Designed the SetCache ownership flip mechanism; chose staleness threshold on existing `cached_at` column as primary approach with `DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK` env var as emergency override; decision ensures BFF never returns 5xx on stale data and is self-healing when Sync recovers. 5 implementation tickets created (#1082–#1086).

## 2026-05-04 — [architect] fix: release workflow deploy skipped on skip_e2e=true; E2E webServer exit code 1
**PR**: #TBD
**Files changed**:
- `.github/workflows/release.yml` — added `if: always() && needs.release.result == 'success'` to `deploy` job so it runs when e2e-smoke is skipped
- `.github/workflows/e2e-smoke.yml` — switched runner to `ubuntu-latest`, added postgres:16 service container, set `DATABASE_URL` env var on smoke test step
- `frontend/playwright.config.ts` — removed invalid `--db-path` flag from CI webServer command (flag not defined in cmd/apiserver/main.go; caused immediate exit code 2)
**Summary**: Fixed two release workflow bugs: (1) `deploy` job was skipped when `skip_e2e=true` because GitHub Actions propagates skip through dependency chain without an explicit `if` condition; (2) E2E webServer exited code 1 because `--db-path` is not a recognized flag in the apiserver binary and `DATABASE_URL` was never set in CI — resolved by removing the invalid flag and providing a postgres service container.

## 2026-05-04 — [architect] chore: enforce GONOSUMDB/GOPRIVATE on all Go CI workflow steps
**PR**: #1087
**Summary**: Audited all .github/workflows/ — patched integration.yml and release.yml Go steps that were missing GONOSUMDB/GOPRIVATE env vars. Codified the rule in backend.md, daemon.md, and architect.md agent definitions to prevent recurrence.

## 2026-05-04 — [daemon] Issue #1094: feat(daemon): install scripts (PowerShell + launchd)
**PR**: #TBD
**Files changed**:
- `services/daemon/install/macos/install.sh` — macOS launchd installer with arch detection
- `services/daemon/install/macos/uninstall.sh` — macOS launchd uninstaller
- `services/daemon/install/windows/install.ps1` — Windows Task Scheduler installer (no UAC)
- `services/daemon/install/windows/uninstall.ps1` — Windows Task Scheduler uninstaller
- `services/daemon/install/README.md` — install documentation
**Summary**: Platform install scripts for the daemon; macOS uses launchd, Windows uses Task Scheduler (AtLogon, no UAC elevation); binary sourced from GitHub Releases with automatic latest-tag resolution.

## 2026-05-04 — [daemon] Issue #1131: fix(daemon): JWT mid-session expiry refresh + CI and binary naming cleanup
**PR**: #1175
**Files changed**:
- `services/daemon/internal/dispatcher/dispatcher.go` — added 401 detection and JWT refresh logic for mid-session token expiry
- `services/daemon/internal/dispatcher/dispatcher_test.go` — unit tests for 401 refresh path
- `.github/workflows/release.yml` — consolidated dual CI workflow confusion; standardized binary naming
**Summary**: Fixed mid-session JWT expiry by adding 401-triggered refresh in the dispatcher, cleaned up dual CI workflow confusion, and standardized daemon binary naming across platforms.
**Merged**: 2026-05-04 — PR #1175 merged into main.

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
**Summary**: Fixed log preservation so the daemon snapshots Player.log on startup before MTGA can overwrite it; fixed a drain bug where unread bytes were discarded on fsnotify rotation events; filed follow-on issue #1041 for dead `models.go` structs needing alignment with actual MTGA JSON keys.
**Merged**: 2026-05-03 — PR #1042 merged into main.

## 2026-05-04 — [frontend] Issue #1139: feat(frontend): add Authorization header to all BFF requests
**PR**: #1150
**Files changed**:
- `frontend/src/adapters/` — added Authorization header injection to all BFF fetch calls via the REST API adapter layer
**Summary**: Wired the auth token into every outbound BFF request so authenticated endpoints receive the Authorization header; implemented at the adapter layer to keep components free of auth concerns.
