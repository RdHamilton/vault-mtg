# Sync Lambda <-> BFF Holistic Gap Analysis

**Date**: 2026-05-06
**Author**: Architect Agent
**Issue**: #1117
**Status**: Findings (no ADRs written yet — recommendations only)
**Scope**: `services/sync/` (AWS Lambda) and `services/bff/` (api.vaultmtg.app)
**Phase context**: Phase 4 Clerk auth in flight; north star 50K MAU; pricing $6.99/mo Pro

---

## Executive summary

The Sync Lambda and the BFF do **not** talk to each other. They share a single
PostgreSQL database (RDS) and communicate exclusively through three reference
tables (`sets`, `draft_card_ratings`, `draft_color_ratings`) plus one
state table (`sync_hashes`). All sync data is **global / read-only / non-PII**;
no per-user information flows through the Lambda today.

This is a deliberately narrow integration surface and is the right call for the
"global card ratings" use case. It is **not** ready for the workloads Phase 3+
implies:

1. **Per-user sync triggers** (e.g. on-demand collection import, post-draft
   ratings refresh) have no architectural seam at all today.
2. **Idempotency** of the Lambda's writes is correct for the global case
   (DELETE+INSERT in a transaction per set/format) but a partial failure mid-run
   leaves a noticeably-stale snapshot for the unprocessed sets/formats with no
   retry signal back to the BFF.
3. **Observability** of sync failures is log-only. The BFF surfaces freshness
   via `X-Cache-Degraded`, but there is no end-to-end alert when 17Lands or
   Scryfall is broken; a silent Lambda failure presents to users as a stale
   ratings page.
4. **Auth and IAM scoping** is mostly correct — the Lambda has its own
   constrained Postgres role (`mtga_sync`), uses RDS IAM auth, and writes only
   to ratings/sets/sync_hashes tables. The remaining gap is the **lack of a
   BFF-side auth contract** for any future "per-user-sync-trigger" path; today
   that path doesn't exist, but the Clerk migration is the right moment to
   pre-declare it before someone wires it up ad hoc.
5. **Multi-tenancy readiness** is the largest gap. The Lambda is single-tenant
   by design — there is no `account_id` in any ratings table and no per-user
   data path. As VaultMTG monetizes Pro features (per-user collection sync,
   per-user 17Lands token sync, per-user ML pick history), the current
   "scheduled global sync" architecture cannot extend; a separate per-user
   sync surface is required.

The good news: the current design is **clean**. It is not a tangle that has to
be unwound. The recommendations below are forward-looking, not remediations.

---

## 1. Data flow (today)

```
EventBridge Scheduler  ──cron──>  mtga-sync Lambda
                                         |
                                         |  (1) Scryfall /sets        --> sets (UPSERT)
                                         |  (2) 17Lands /card_ratings --> draft_card_ratings (DELETE+INSERT)
                                         |  (3) 17Lands /color_ratings --> draft_color_ratings (DELETE+INSERT)
                                         |  (4) sync_hashes (delta-skip state)
                                         v
                                    RDS PostgreSQL
                                         ^
                                         |
                                    BFF (api.vaultmtg.app)
                                         |
                                    GET /api/v1/draft-ratings/{setCode}/{format}
                                         |
                                    SPA / Daemon
```

### Key facts

- **No HTTP between Sync and BFF.** The Lambda never calls the BFF, and the BFF
  never calls the Lambda. They are coupled only at the schema level.
- **No SQS / EventBridge between Sync and BFF.** No message queue exists for
  cross-service events. The only EventBridge integration is the Scheduler that
  triggers the Lambda itself.
- **Idempotency model**:
  - Per `(set_code, draft_format)`, the Lambda runs `DELETE` + `INSERT` inside a
    single Postgres transaction. The transaction makes a single set/format
    update atomic from a reader's perspective.
  - Hash-skip (ADR-005): the Lambda hashes the sorted payload and writes only
    when the hash differs from `sync_hashes`. If hashes match, no write
    happens.
  - Sets table is `INSERT … ON CONFLICT (code) DO UPDATE` — also idempotent.
- **Race conditions**:
  - The Lambda runs once per day at 02:00 UTC (per ADR-003). Concurrent
    invocations would race on the same `(set_code, draft_format)` rows; the
    transaction would serialize them but the second writer's `DELETE` would
    blow away the first writer's freshly inserted rows. EventBridge Scheduler
    does not deliver more-than-once by default, but Lambda retry on error can
    cause double-execution. **No advisory lock or invocation singleton check
    exists**. See Recommendation R1.
  - Reader race: a SPA request that lands while the Lambda is mid-transaction
    on a given set/format will see the *previous* committed state (read
    committed isolation). Once the transaction commits, all subsequent reads
    see the new state. This is correct behaviour for the global ratings
    use case.

---

## 2. Auth gaps (Clerk transition)

### Current BFF auth model (per `services/bff/cmd/main.go`)

| Route | Auth required | Mechanism |
|---|---|---|
| `GET /health` | none | public |
| `GET /api/v1/daemon/version` | none | public |
| `GET /api/v1/draft-ratings/{set}/{format}` | **none** | public read of global data |
| `GET /api/v1/events` (SSE) | yes | Clerk JWT (preferred) -> API key fallback -> 503 |
| `POST /api/keys` | yes | Daemon JWT (HMAC) |
| `POST /api/daemon/register` | yes | API key (bcrypt-compared) |
| `POST /v1/ingest/events` | yes | Daemon JWT (HMAC) -> API key fallback -> **unguarded in dev** |

### Sync Lambda's effect on this surface

**The Sync Lambda touches none of these BFF routes.** It is a sibling
service that writes directly to the shared DB with its own Postgres role
(`mtga_sync`), authenticated via RDS IAM. Migration `000057_create_sync_user_grants`
constrains the role to `SELECT/INSERT/UPDATE/DELETE` on ratings tables and
explicitly `REVOKE INSERT, UPDATE, DELETE ON matches/draft_sessions/collection`
from the role.

**This is the correct posture today.** The Lambda IAM scope is appropriately
narrow:

- IAM policy: `rds-db:connect` for `dbuser/mtga_sync` only.
- Postgres role: write access to `sets`, `draft_card_ratings`,
  `draft_color_ratings`, `sync_hashes`. Read-only on `set_cards`. No access to
  user-owned tables.
- Lambda is not in the `account_id`-scoped query path because it doesn't write
  any rows that have an `account_id`.

### Gaps revealed by Clerk

1. **No service-to-service auth contract for `Lambda -> BFF` ever.** Today
   that's fine because no such call exists. But per Recommendation R3 below,
   we will likely want a per-user sync-trigger path in Phase 4+, and we
   should pre-declare how a Lambda authenticates to the BFF before it
   exists. ADR-009 added Clerk for user auth but did not address service-to-
   service auth. The cleanest answer is **internal HMAC-signed requests with
   a separate `INTERNAL_SVC_SECRET`** (not the daemon HMAC, not Clerk) — see
   recommended ADR R-A below.

2. **`POST /v1/ingest/events` falls back to "unguarded" in dev mode.** This
   is intentional (`cfg.DaemonJWTSecret == ""` in development) but the same
   code path runs in production. The `config.Load()` function correctly
   refuses to start when `MTGA_ENV=production` and `DAEMON_JWT_SECRET` is
   unset, which closes the loophole. **No action needed**; documented here
   for completeness.

3. **`GET /api/v1/draft-ratings/...` is unauthenticated.** This is correct —
   the data is global and non-PII. Worth re-confirming during the Clerk
   rollout that we do **not** accidentally gate it behind Clerk and break
   logged-out previews. Recommend explicit comment in `main.go` that this
   route is permanently public.

4. **Per-user 17Lands token sync (Pro feature, future).** A Pro user could
   plausibly want their personal 17Lands data folded into their pick advisor
   ratings. There is no architectural seam for this today. The Lambda is
   global-only. See R3.

---

## 3. Error handling

### What the Lambda does on failure

Inside a single invocation, per `services/sync/internal/handler/lambda.go`:

- `FetchCardRatings` failure: `log.Printf` and `continue` to the next set/
  format. No failure aggregation; no SQS DLQ message. Other set/format
  pairs still process.
- `UpsertRatings` failure (after a successful fetch): `log.Printf` and
  `continue`. Transaction rolls back; the previous good rows for that set/
  format remain (because of DELETE+INSERT in a single tx).
- `FetchColorRatings`/`UpsertColorRatings` failure: best-effort, non-fatal.
- The Lambda returns `nil` when it finishes the loop, even if individual
  sets/formats failed.

### What EventBridge / Lambda does

- EventBridge Scheduler retries failed invocations per the "retry policy"
  attribute (default: 0 retries when target is Lambda + a DLQ is not
  configured).
- Lambda has its own async-invoke retry behaviour, but EventBridge Scheduler
  uses **synchronous** invocation, so Lambda async retries don't apply.
- **There is no DLQ configured today.** A complete invocation failure (panic,
  cold-start crash, IAM token failure) is logged in CloudWatch and silently
  dropped.

### What the BFF does

- BFF reads `cached_at` from `draft_card_ratings.cached_at`. If older than
  48h (per ADR-004), it returns `X-Cache-Degraded: true` with the rows.
- BFF returns 404 if there are no rows for the requested set/format.
- BFF has no awareness of Lambda invocation status.

### Gaps

1. **No DLQ on the Lambda.** A complete failure (e.g. RDS unreachable, IAM
   token generation failure, Scryfall down for the entire window) results in
   a CloudWatch error log and **no alarm**. The next user-facing signal is
   `X-Cache-Degraded: true` ~24h later when the freshness threshold trips.
   See R2.

2. **No per-set/format retry.** A transient 17Lands 5xx skips that set for
   24 hours. The right answer is in-Lambda retry with backoff for transient
   HTTP failures (the existing code does not retry).

3. **No alerting on `cached_at` regression.** If 17Lands silently changes
   their data shape and we begin storing partial rows or skipping all sets,
   `cached_at` keeps advancing on the few sets that succeed but goes stale
   on the rest. There is no metric for "fraction of expected sets/formats
   refreshed in last run". See R2.

4. **Hash-skip can mask bugs.** A bug in our hash normalization that
   accidentally produces a stable hash on bad data would result in
   indefinite skips. Today this is mitigated only by the BFF's freshness
   check on `cached_at` — but skips intentionally do **not** advance
   `cached_at`. **The freshness threshold is the only safety net.**
   Recommend adding a "max consecutive skip days" guard (R2).

---

## 4. Multi-tenancy readiness

### Today

- All ratings data is **global**. `draft_card_ratings` has no `user_id` /
  `account_id`. The same row is served to every user.
- Daemon-derived data (matches, draft sessions, collection) is correctly
  scoped to `user_id` / `account_id` in the BFF (per ADR commitments and
  enforced by repository-layer queries).
- The Lambda has no per-user data path.

### What 50K MAU will require

The product roadmap suggests at least three per-user sync-shaped workloads:

1. **Personal 17Lands ingestion (Pro feature).** A Pro user with a 17Lands
   account could authorize VaultMTG to pull their personal pick history /
   draft logs via the 17Lands API. This is per-user, so the global scheduled
   Lambda is the wrong vehicle.
2. **Collection diff sync.** Daemon already streams collection events via
   `/v1/ingest/events`. A scheduled "reconcile collection against MTGA
   inventory snapshot" job would benefit from being per-user and triggered
   on-demand (e.g. when the daemon comes online after a long offline period).
3. **ML pick-quality batch backfill.** When a user upgrades to Pro, we may
   want to reprocess their last N draft sessions through ML scoring. This is
   per-user, infrequent, and ideal for SQS-fanout-to-Lambda.

None of these fit the current `mtga-sync` Lambda model. They will each need:

- **Trigger surface**: BFF endpoint that enqueues an SQS message keyed by
  `user_id`, OR a Clerk-webhook-driven Lambda that fires on user events.
- **Auth model**: the Lambda must somehow re-authenticate as that user
  (or carry a delegated `user_id` in its payload, with HMAC-signed proof from
  the BFF that the user authorized the action). Today there is no such
  pattern.
- **Multi-tenant Postgres role**: the existing `mtga_sync` role is hard-
  REVOKED from `matches`, `draft_sessions`, `collection`. A per-user Lambda
  would need a different role with `account_id`-scoped access — best modeled
  as a separate Lambda function with its own IAM execution role and Postgres
  role (e.g. `mtga_user_sync`).

### Recommendations

See R3, R4, R5.

---

## 5. Missing pieces (theoretical integration points not yet built)

| Integration point | Status | Notes |
|---|---|---|
| Per-user sync trigger via BFF | **Missing** | No endpoint, no SQS, no Lambda. R3 proposes the seam. |
| DLQ on `mtga-sync` Lambda | **Missing** | EventBridge target has no DLQ ARN. R2. |
| CloudWatch alarms for sync failures | **Missing** | Only logs exist. R2. |
| Service-to-service auth (Lambda -> BFF) | **Missing** | No call exists today, but R-A pre-declares the pattern. |
| `users` table linking Clerk -> account_id | **Pending** | ADR-009 ticket TBD-C tracks this. Affects per-user sync paths in R3. |
| Sync metrics emission | **Missing** | No CloudWatch custom metrics published. R2. |
| Per-set/format retry with backoff | **Missing** | Single attempt per invocation. R2. |
| Hash-skip ceiling ("force refresh after N skips") | **Missing** | Recommend adding. R2. |
| `draft_card_ratings.cached_at` index for freshness scans | **Present implicitly** | Primary key covers most queries; not a gap. |
| Internal/admin BFF route to "force a sync now" | **Missing** | Only manual `aws lambda invoke` works. Useful for incident response. R6. |

---

## Recommendations

These are forward-looking. Each is a candidate ADR. **No ADRs written yet** —
the architect (this agent) and the user will decide which to file as ADRs and
which to ticket directly.

### R1 — Add a Lambda invocation singleton guard (low cost, high value)

**Problem**: EventBridge Scheduler retries on transient failures can result in
overlapping invocations that race on the same set/format DELETE+INSERT.

**Proposal**: Acquire a Postgres advisory lock at Lambda startup, keyed by a
constant (e.g. `pg_try_advisory_lock(hashtext('mtga-sync-lambda'))`). Bail out
with a clean log line if the lock is already held.

**Effort**: ~1 hour. One file, ~20 lines. Sonnet-ready.

**Candidate ADR**: No — too small. File as ticket directly.

### R2 — Sync observability bundle (DLQ, alarms, metrics, retry)

**Problem**: A failed sync run is invisible to operators until the freshness
threshold trips ~24h later.

**Proposal** (multi-ticket bundle):

1. Add SQS DLQ to the `mtga-sync` Lambda's EventBridge target.
2. CloudWatch alarm on Lambda errors > 0 in 1h.
3. Custom metric `MtgaSync/SetsRefreshed` (gauge) + alarm when value drops
   below expected count.
4. Per-set/format retry-with-backoff inside the handler (3 attempts, 500ms
   backoff) — same pattern the daemon dispatcher already uses.
5. "Max consecutive skips" guard: if `sync_hashes.updated_at` for a key has
   not advanced in 7 days **and** the hash matched again, force a refresh
   anyway (write `cached_at` even if data unchanged) so the freshness
   threshold doesn't false-positive on stable sets.

**Effort**: 2-3 tickets, each 1-2 hours. Sonnet-ready individually.

**Candidate ADR**: Yes — observability strategy for the sync surface.
Recommend filing **ADR-010 — Sync Observability and Failure Modes**.

### R3 — Per-user sync surface (the big one)

**Problem**: Per-user sync workloads (Pro 17Lands ingestion, collection
reconcile, ML backfill) have no architectural home.

**Proposal**: Introduce a separate Lambda — provisional name `mtga-user-sync`
— with its own IAM role, its own Postgres role (`mtga_user_sync` with
`account_id`-scoped grants on `matches`, `draft_sessions`, `collection`), and
its own SQS queue.

- Trigger surface: a new BFF route `POST /api/v1/sync/{kind}` that:
  1. Verifies the user's Clerk session.
  2. Resolves `user_id -> account_id` via the `users` table (ADR-009).
  3. Enqueues an SQS message `{ user_id, account_id, kind, ts }`,
     HMAC-signed with `INTERNAL_SVC_SECRET` (see R-A).
  4. Returns 202 with a job ID the SPA can poll.
- Lambda processes one message at a time, performs the per-user work, writes
  results to the appropriate `account_id`-scoped tables.
- DLQ on the queue captures permanent failures.

**Effort**: This is a multi-ticket epic. Architect work to scope; multiple
sub-agent tickets to implement. **Not Sonnet-ready as a single task.**

**Candidate ADR**: Yes — definitively. Recommend filing **ADR-011 — Per-user
Sync Service (mtga-user-sync)** before any implementation begins. The ADR
should also resolve the "Clerk webhook vs. SPA-triggered enqueue" question
for first-sign-in provisioning.

### R4 — Decouple `mtga-sync` from per-user concerns by name

**Problem**: The current Lambda is named `mtga-sync` and lives in
`services/sync/`. If R3 lands, we'll have two sync services and the names
will be confusing.

**Proposal**: Rename in-place is too disruptive at this stage; instead,
**document** in `services/sync/README.md` and the proposed ADR-011 that
`mtga-sync` is the **global reference data** Lambda and the new service is
the **per-user** Lambda. Strict naming discipline going forward.

**Effort**: <30 min. Doc-only.

**Candidate ADR**: No — handled inside ADR-011.

### R5 — Pre-declare the contract for "Sync provider" pluggability

**Problem**: 17Lands is the only data provider for ratings. CFB ratings
infrastructure exists in the schema (migration 000037) but is unused. If we
add CFB ratings, MTGGoldfish, or our own ML-derived ratings, today's
single-provider Lambda needs structural changes.

**Proposal**: Refactor `services/sync/internal/seventeenlands/` callers to
accept a `RatingsProvider` interface, then document in a brief ADR that new
providers are added by implementing the interface and registering them in
the handler — not by adding new Lambdas.

**Effort**: ~3-4 hours for a careful refactor + tests. **Borderline
Sonnet-ready** — could be done by a backend-engineer with a clear ticket.

**Candidate ADR**: Maybe — small enough to defer until a second provider is
real. Note in the changelog as "deferred until needed."

### R6 — Admin "force sync" endpoint on the BFF

**Problem**: Today, recovering from a sync failure requires shell access:
`aws lambda invoke --function-name mtga-sync ...`. During an incident this
is friction.

**Proposal**: `POST /admin/sync/force` on the BFF, gated by Clerk role
`admin` (see ADR-009 tier/role gating). The handler invokes the Lambda
synchronously via the AWS SDK and returns the resulting log lines.

**Effort**: ~2 hours. Sonnet-ready once R-A (service-to-service auth) is
defined.

**Candidate ADR**: No — small enough to ticket directly under ADR-011's
umbrella.

### R-A — Service-to-service auth pattern (HMAC, separate from daemon HMAC)

**Problem**: ADR-009 specifies user auth (Clerk) and pre-existing daemon
auth (HMAC `DAEMON_JWT_SECRET`, slated for removal). It does **not** define
how internal services (BFF -> Lambda, Lambda -> BFF, future jobs -> BFF)
authenticate.

**Proposal**: A single `INTERNAL_SVC_SECRET` HMAC key, stored in SSM and
mounted into every internal service's environment. Internal-only routes on
the BFF (e.g. `POST /internal/...`) require an `X-Internal-Sig: <hmac>`
header. The HMAC is computed over `method + path + body + nonce + ts` to
prevent replay.

This is **not** Clerk and **not** the daemon HMAC. It is a third, separate
trust boundary used only for internal pluming.

**Effort**: ~1 day to design + implement + document. Not Sonnet-ready as a
single task; needs an ADR.

**Candidate ADR**: Yes — recommend filing **ADR-012 — Internal
Service-to-Service Auth**. Should be filed **before** R3/R6 implementation
because both depend on it.

---

## Recommended ADRs (summary)

| ID | Title | Trigger | Priority |
|---|---|---|---|
| ADR-010 | Sync Observability and Failure Modes | R2 | High — operational gap |
| ADR-011 | Per-user Sync Service (`mtga-user-sync`) | R3 | High — Phase 4 dependency |
| ADR-012 | Internal Service-to-Service Auth | R-A | High — must precede ADR-011 |

The architect (and user) should sequence these as ADR-012 -> ADR-011 -> ADR-010
during Phase 4 planning, with ADR-010 fillable in parallel.

---

## Appendix A — Files reviewed

- `services/sync/cmd/lambda/main.go`
- `services/sync/internal/handler/lambda.go`
- `services/sync/internal/refresh/scheduler.go`
- `services/sync/internal/datasets/store.go`
- `services/sync/internal/datasets/postgres_store.go`
- `services/sync/internal/dbconn/rds_iam.go`
- `services/sync/internal/scryfall/client.go`
- `services/sync/README.md`
- `services/bff/cmd/main.go`
- `services/bff/internal/config/config.go`
- `services/bff/internal/api/handlers/ingest.go`
- `services/bff/internal/api/handlers/draft_ratings.go`
- `services/bff/internal/api/middleware/auth.go`
- `services/bff/internal/api/middleware/clerk_auth.go`
- `services/bff/internal/api/middleware/daemon_jwt.go`
- `services/bff/internal/storage/repository/api_key_repo.go`
- `services/bff/internal/storage/repository/daemon_events_repo.go`
- `services/bff/internal/storage/repository/draft_ratings_repo.go`
- `services/bff/internal/storage/migrations/postgres/000057_create_sync_user_grants.up.sql`
- `services/bff/internal/storage/migrations/postgres/000059_grant_sync_sets_write.up.sql`
- `services/bff/internal/storage/migrations/postgres/000061_add_daemon_events.up.sql`
- `.github/workflows/sync.yml`
- `docs/adr/003-sync-service-deployment-strategy.md`
- `docs/adr/005-sync-delta-strategy.md`
- `docs/adr/ADR-009-user-auth-provider-clerk.md`

## Appendix B — What was checked and is fine

- `mtga_sync` Postgres role grants (correctly narrow).
- RDS IAM auth path (correct per ADR-003).
- DELETE+INSERT transaction boundary in `UpsertRatings` (correct).
- Hash normalization (sort by `MtgaID` before SHA-256 — stable per ADR-005).
- `account_id` enforcement on user-data tables in BFF repositories (sync
  Lambda has zero involvement; correctly scoped at the BFF layer).
- CORS allow-list on the BFF (sync Lambda is not a CORS-relevant origin —
  it doesn't talk to the BFF).
- `DATABASE_URL` / `DAEMON_JWT_SECRET` / `CLERK_SECRET_KEY` startup guards
  (`config.Load()` correctly fails fast in production when any are unset).
