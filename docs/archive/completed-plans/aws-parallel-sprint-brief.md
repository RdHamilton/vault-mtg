# AWS Parallel Sprint — Architectural Brief

**Date**: 2026-05-03
**Author**: Architect Agent
**Status**: Reference — do not edit during sprint

---

## Sprint Scope

Four tickets executing in parallel as part of the AWS/Lambda deployment push:

| Ticket | Title | Agent |
|--------|-------|-------|
| #1061 | refactor(sync): replace ticker loop with Lambda handler entrypoint | backend |
| #1069 | go.work production readiness — remove local replace directives, tag mtga-contract | backend |
| #1065 | feat(db): migration 000060 — add rds_iam attribute to mtga_sync role | dba |
| #1041 | daemon: align draft log data model with actual MTGA JSON keys | daemon |

---

## 1. How services/sync Fits into the AWS Architecture

### Current state

`services/sync/cmd/main.go` runs a blocking ticker loop (`refresh.Scheduler.Start(ctx)`). The
scheduler fires once immediately on startup, then every hour, executing `runFetch` when the
configured UTC hour matches. It connects via `pgxpool` using a `DATABASE_URL` environment variable.

### Target state (ADR-001, referenced as ADR-003 in issues)

Sync deploys as a **Lambda function triggered by EventBridge Scheduler**. The ticker loop is
deleted. `cmd/lambda/main.go` calls `lambda.Start(handler)` where the handler invokes the same
`runFetch` logic that already exists in `refresh/scheduler.go`.

### Architectural role

```
EventBridge Scheduler (daily, e.g. 02:00 UTC)
    │
    └─► Lambda (services/sync)
            │  IAM role → RDS IAM auth → mtga_sync Postgres role
            │
            ├─ reads:  sets WHERE is_standard_legal = TRUE
            ├─ writes: draft_card_ratings (DELETE+INSERT per set/format in a single tx)
            ├─ writes: draft_color_ratings
            └─ writes: dataset_metadata

BFF (EC2)
    └─ reads draft_card_ratings via DraftRatingsRepository (never imports services/sync)
```

Sync and BFF share one RDS instance. They use separate Postgres roles: `mtga_sync` (scoped to
card/ratings tables, granted in migration 000057) and the BFF's application role. There is no
HTTP contract between Sync and BFF — they communicate exclusively through the shared database.

### What the backend agent must know for #1061

- The `runFetch` logic in `refresh/scheduler.go` is already self-contained and side-effect-free
  (takes `ctx`, uses `Fetcher` and `Store` interfaces). The Lambda handler should instantiate the
  same `PostgresStore` + `seventeenlands.Client`, then call `runFetch` directly. No scheduler loop
  is needed.
- The Lambda must obtain its DB connection string via **IAM token**, not a static password. This
  requires `rds_iam` on the `mtga_sync` role — which is the subject of #1065. See ordering
  dependencies below.
- `SYNC_ACTIVE_SETS` override env var should be preserved so the Lambda can be invoked manually
  for a specific set without touching the DB.
- The `DATABASE_URL` env var used today must be replaced or supplemented with IAM token
  generation (using `aws/rds-auth-token`) before Lambda deployment. Do not merge #1061 with a
  static-password connection — that would not work in the Lambda execution environment on RDS.

---

## 2. go.work Structure and Production Readiness (#1069)

### Current state

`go.work` (repo root) lists four modules with `use` directives and **no `replace` directives**:

```
go 1.25.0
toolchain go1.25.9
use (
    .
    ./services/bff
    ./services/contract
    ./services/daemon
    ./services/sync
)
```

The root module (`.`) is the legacy monorepo `go.mod`. It is not tested in CI per ADR-002.

### What "production readiness" means for module boundaries

Per ADR-001 Go Workspace Rules:

1. `go.work` on `main` must contain zero `replace` directives pointing to local filesystem paths.
   The current `go.work` already satisfies this — **no replace directives present**.
2. Each service `go.mod` must import `github.com/ramonehamilton/mtga-contract` at a published
   tagged version, not via `go.work` resolution.
3. CI builds each service independently (no `go.work` in CI) — each module resolves contract
   types from the published module path.

### Current gap

`services/sync/go.mod` does **not** import `github.com/ramonehamilton/mtga-contract` at all.
`services/bff/go.mod` imports it (`github.com/ramonehamilton/mtga-contract` is present in
`services/bff/cmd/main.go` and `services/bff/internal/api/handlers/ingest.go`), but the version
in `go.mod` is not visible here — the backend agent must verify the exact version pinned.

The `services/contract` module at `./services/contract` has not yet been published with a tag.
The go.work `use` directive makes it available locally, but CI service builds will fail until a
`v0.x.y` tag is pushed to the `services/contract` subtree and consumer `go.mod` files are updated.

### What the backend agent must do for #1069

1. Publish `services/contract` as `github.com/ramonehamilton/mtga-contract@v0.1.0` (or the
   current appropriate version) by pushing a tag scoped to that subdirectory path.
2. Update `services/bff/go.mod` and `services/daemon/go.mod` to pin the published version.
3. `services/sync/go.mod` currently has no contract dependency — add it only if #1061 introduces
   a contract type import; otherwise leave it out.
4. Add a CI step that greps `go.work` for `replace` and fails the build if found on PRs targeting
   `main`. A simple `grep -E '^\s*replace' go.work && exit 1 || exit 0` is sufficient.
5. `go test ./...` from repo root (via `go.work`) must continue to pass after pinning.

---

## 3. RDS IAM Auth — Migration 000060 (#1065)

### Context chain

- Migration 000057: creates `mtga_sync` role with `LOGIN`, grants access to card/ratings tables,
  explicitly revokes writes on user-facing tables.
- Migration 000058: data-only seed of standard-legal sets (not role-related).
- Migration 000059: not yet written (next slot). The dba agent must check whether 000059 already
  exists in a pending branch before writing 000060.
- Migration 000060 (#1065): `ALTER ROLE mtga_sync WITH rds_iam;`

### What RDS IAM auth requires

For a Lambda to connect to RDS using IAM authentication:

1. The Postgres role must have `rds_iam` attribute — this is what 000060 adds.
2. The Lambda execution role must have `rds-db:connect` IAM permission for the specific RDS
   resource ARN and database user (`mtga_sync`).
3. The Lambda code must call the RDS auth token API to generate a short-lived password token and
   use it as the Postgres password in the connection string. This is a backend/Lambda concern, not
   a migration concern.
4. SSL must be enforced on the connection (`sslmode=require` or `verify-full`).

### What the dba agent must know for #1065

- The migration belongs in `services/bff/internal/storage/migrations/postgres/` — this is the
  single migration directory for the shared RDS instance. Sync does not own its own migrations.
- The migration is additive (`ALTER ROLE ... WITH rds_iam`). It does not change any grants.
  The down migration should be `ALTER ROLE mtga_sync WITH NOLOGIN` or simply a no-op comment,
  since removing `rds_iam` without also revoking grants would leave the role in an inconsistent
  state. Recommended down: `ALTER ROLE mtga_sync WITH PASSWORD NULL;` — this disables
  password-based login and is reversible.
- The `DO $$ IF NOT EXISTS` guard pattern used in 000057 is not applicable here — `ALTER ROLE`
  is idempotent (re-running with the same attribute is a no-op on RDS).
- Test against RDS, not local Postgres. The `rds_iam` attribute is RDS-specific and has no
  meaning or effect on a local Postgres instance. The acceptance criterion "Migration tested
  against RDS" is non-negotiable before merging.
- Migration 000059 must be confirmed absent before writing 000060. If another agent writes a
  000059 migration concurrently, there will be a sequence collision. Coordinate with the backend
  agent before filing the PR.

---

## 4. Daemon Draft Log Model and BFF/Sync Contract Impact (#1041)

### Current state

`services/daemon/internal/logreader/models.go` contains `DraftEvent`, `DraftHistory`, `DraftDeck`,
`DeckCard` structs that are never unmarshalled into. The daemon's `classifyEntry()` performs raw
`entry.JSON["draftPack"]` / `entry.JSON["pickedCards"]` key presence checks and does not use these
types.

The daemon POSTs `contract.DaemonEvent` to the BFF. The `Payload` field is `json.RawMessage` —
meaning the BFF currently receives draft events as opaque blobs and does not validate or
deserialize the payload.

### Impact on BFF

The BFF `IngestHandler` receives `DaemonEvent` and broadcasts it over WebSocket. It does **not**
persist draft events to the database. There is no BFF handler that parses `Payload` as a draft
pick event. Until #1041 establishes the correct MTGA JSON key names, the BFF is not blocked — it
passes through whatever the daemon sends.

However: once #1041 is resolved and the correct `DraftPick` payload shape is known, the
`services/contract` module must be updated to add a typed `DraftPickPayload` struct. This is a
contract change that must be tagged before BFF or daemon can compile against it. The backend
agent working on #1069 must not finalize the contract tag until #1041 has at least documented
the correct field names — otherwise the v0.1.0 contract tag will be missing the draft pick type.

### Impact on Sync

Sync (`services/sync`) does not consume daemon events and does not import `mtga-contract`. It
reads from 17Lands and writes to Postgres. #1041 has no impact on Sync.

### Impact on DBA

The existing `draft_card_ratings` schema (written by Sync) is unrelated to the draft pick event
schema (written by BFF when it persists daemon events). #1041 affects the BFF-side draft storage
tables (`draft_sessions`, `draft_picks` — migrations 000005, 000014). The dba agent should not
run any draft_sessions or draft_picks schema changes until #1041 is resolved and the correct
field names are confirmed.

### Concrete risk

If the dba agent designs a `draft_picks` column layout based on the current speculative model
(e.g. `draft_pack`, `picked_cards`) and the actual MTGA JSON uses different key names, the
column names in the migration will be wrong. Wait for #1041's key name resolution before any
`draft_picks` schema work.

---

## 5. Ordering Dependencies

### Hard dependencies (must complete before)

```
#1065 (rds_iam migration) ──must merge before──► #1061 (Lambda handler) is deployed to RDS
```

The Lambda entrypoint in #1061 can be written and merged to `main` without #1065 — the code can
be written to use IAM auth and fall back to a `DATABASE_URL` for local/test runs. But the Lambda
**cannot be deployed to production RDS** until the `mtga_sync` role has `rds_iam` applied.

```
#1041 (correct JSON key names) ──should complete before──► contract v0.1.0 tag is finalized
```

If `DraftPickPayload` is to be included in the initial contract tag, #1041 must resolve the field
names first. If the contract tag ships without draft pick types (reasonable for v0.1.0 — the BFF
uses `json.RawMessage` today), then #1069 can proceed independently.

```
#1069 (contract tag + go.mod updates) ──must merge before──► any CI service build can pass
```

Until `services/contract` is published with a tag and consumer `go.mod` files are updated, CI
builds for `services/bff` and `services/daemon` will fail when run in isolation (without go.work).

### Safe to execute in parallel

- #1065 and #1069: no shared files. Different concerns (SQL migration vs Go module metadata).
- #1065 and #1041: no shared files. Different layers (DB role vs daemon parser).
- #1041 and #1069: no shared files unless #1041 decides to add a `DraftPickPayload` to
  `services/contract` — in that case coordinate with the backend agent before tagging.
- #1061 and #1041: no shared files. Sync does not import daemon code or contract types.
- #1061 and #1065: share the same RDS instance and `mtga_sync` role, but at different layers.
  Safe to develop in parallel; deployment is ordered (see above).

---

## 6. Shared Files / Conflict Risks

| File/Path | Touched by | Risk |
|-----------|-----------|------|
| `services/bff/internal/storage/migrations/postgres/` | #1065 (dba) | Migration sequence number collision if another agent is writing a 000059 migration. Verify 000059 does not exist before writing 000060. |
| `services/contract/` | #1069 (backend) + potentially #1041 (daemon) | If daemon agent adds `DraftPickPayload` to contract as part of #1041, that is a conflict with #1069 tagging. Coordinate via PR review. |
| `services/sync/cmd/main.go` | #1061 (backend) | No other ticket touches this file. |
| `go.work` | #1069 (backend) | Only #1069 modifies go.work. No conflict. |

---

## 7. ADR Gap: ADR-003 Does Not Exist

Both #1061 and #1065 reference `docs/adr/003-sync-service-deployment-strategy.md`. This file
does not exist. The relevant decisions (Lambda deployment for Sync, EventBridge Scheduler, IAM
auth) are documented in ADR-001 (`docs/adr/001-service-split-approaches.md`) under the sections
"Sync/Poller: Lambda + EventBridge (Not EC2 Process)" and the Consequences section.

The architect agent should write ADR-003 to formalize the sync deployment strategy, referencing
ADR-001 as predecessor. Until ADR-003 exists, implementation agents should treat ADR-001's Sync
section as authoritative.

---

## 8. Summary: Is Each Task Safe to Execute in Isolation?

| Ticket | Safe in isolation? | Notes |
|--------|-------------------|-------|
| #1061 | Yes (code only) | Can be written and merged. Cannot be deployed to RDS until #1065 merges. |
| #1069 | Yes | Depends only on a `services/contract` tag being created, which this ticket itself creates. No other ticket blocks it, unless #1041 adds contract types (coordinate). |
| #1065 | Yes | Pure SQL migration. Verify 000059 slot is free first. Must be tested on RDS. |
| #1041 | Yes | Daemon-internal refactor. No BFF or Sync code changes required. Flag any new `DraftPickPayload` additions to the backend agent before they are tagged in contract. |
