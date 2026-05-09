# ADR-003: Sync Service Deployment Strategy — Lambda vs EC2

**Date**: 2026-05-03
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

`services/sync` is the primary data ingestion layer for the AI/ML pipeline. It polls 17Lands for card ratings and syncs card metadata from Scryfall on schedule. It is a first-class service — not a secondary concern — because the quality of all draft recommendations and future ML models depends on the freshness and correctness of its data.

During the v2.0 implementation cycle, a critical deployment drift was discovered (reported in issue #1050):

- **ADR-001 specifies**: "Deploy Sync as Lambda functions triggered by EventBridge Scheduler — the ticker-based `refresh/scheduler.go` is replaced by a Lambda handler."
- **What was actually built** (merged to `main` before the drift was caught):
  - `services/sync/internal/scheduler/scheduler.go` — ticker loop, not a Lambda handler
  - `.github/deploy/mtga-sync.service` — systemd unit for EC2 process management
  - `.github/workflows/release.yml` — EC2 SSM deploy step added for the sync service (PR #1048)
  - `services/sync/README.md` — documents env vars assuming EC2/systemd deployment
  - `services/sync/cmd/main.go` — long-running process startup with DB ping log

The EC2 deploy step and systemd service were merged (PRs #1048, #1049, #1053) before this drift was identified. Those PRs must be considered for revision or replacement.

### Why this decision matters now

The `mtga_sync` Postgres credential strategy is blocked on this decision (issue #1054). The correct auth approach depends on the runtime:

- **Lambda**: IAM execution role authenticates to RDS via IAM token — no static password, zero credential management overhead.
- **EC2 process**: IAM instance profile can also authenticate via token, but the systemd unit reads `DATABASE_URL` from SSM — that parameter must be populated manually or via IaC.

A decision here unblocks both the deploy pipeline and the credential strategy.

### Sync job characteristics

| Property | Value |
|---|---|
| 17Lands ratings refresh | Daily (bounded runtime, ~seconds to minutes) |
| Scryfall card metadata sync | On new set release (~monthly, also bounded) |
| Writer to global reference tables | Yes — `sets`, `cards`, `draft_ratings`, `card_ratings` |
| Long-running process required? | No |
| Real-time response required? | No |
| Current ticker loop max runtime | Unbounded (runs forever, polls on schedule) |

Batch jobs with bounded runtimes and infrequent schedules are the canonical Lambda use case.

---

## Decision

**Revert to ADR-001: deploy `services/sync` as AWS Lambda functions triggered by EventBridge Scheduler.**

The EC2/systemd approach that was inadvertently merged is superseded by this decision. The following previously-merged work must be revised:

| Artifact | Required change |
|---|---|
| `.github/workflows/release.yml` (sync EC2 deploy step) | Replace with Lambda zip deploy step |
| `.github/deploy/mtga-sync.service` | Delete — not applicable to Lambda runtime |
| `services/sync/cmd/main.go` | Refactor to Lambda handler entrypoint |
| `services/sync/internal/scheduler/scheduler.go` | Delete ticker loop — scheduling is EventBridge's job |
| `services/sync/README.md` | Rewrite for Lambda deployment model |
| `.github/workflows/sync.yml` (if exists) | Produce Lambda zip artifact instead of binary |

### Lambda handler design

```
services/sync/
  cmd/
    lambda/
      main.go        — Lambda handler entrypoint (replaces cmd/main.go)
  internal/
    handler/
      handler.go     — implements lambda.Handler; dispatches by event type
    fetcher/         — 17Lands, Scryfall, existing fetch logic unchanged
    storage/         — DB write path; unchanged
```

`cmd/lambda/main.go` calls `lambda.Start(handler.New(...))`. The handler receives an EventBridge scheduled event payload and dispatches to the appropriate fetcher (ratings vs. card metadata). No ticker, no long-running process.

### EventBridge Scheduler configuration

Two rules:

| Rule name | Schedule | Target | Description |
|---|---|---|---|
| `mtga-sync-ratings-daily` | `rate(1 day)` | sync Lambda | Refresh 17Lands card ratings |
| `mtga-sync-cards-manual` | On-demand / `workflow_dispatch` | sync Lambda | Trigger Scryfall card metadata sync for new sets |

The ratings rule runs daily. The card sync rule is triggered manually when a new MTGA set releases (correlates with the existing `Set Data Sync` workflow already documented in project memory).

### RDS connectivity and credential strategy (resolves #1054)

**Use RDS IAM authentication via Lambda execution role.**

- Lambda execution role is granted `rds-db:connect` permission in IAM.
- `mtga_sync` Postgres role is configured with `rds_iam` attribute (migration supplement required).
- The Lambda function generates an IAM auth token at connect time using the AWS SDK — no static password stored anywhere.
- `DATABASE_URL` in SSM is replaced by individual SSM parameters (`DB_HOST`, `DB_NAME`, `DB_USER`) with no password field. The Go client assembles the DSN and requests the token at startup.

This eliminates the credential gap documented in issue #1054 and migration 000057 (`CREATE ROLE mtga_sync LOGIN` — no password was intentional but left auth undefined).

**Option B (IaC-managed password) is rejected** for the Lambda runtime because IAM auth is the AWS-native pattern for Lambda→RDS and requires no password rotation infrastructure.

### Impact on already-merged work

- **Migration 000059** (`grant_sync_sets_write.sql`): Safe to keep as-is. The `GRANT INSERT, UPDATE ON sets TO mtga_sync` statement is runtime-independent — the role and grants apply regardless of whether sync runs as Lambda or EC2.
- **Migration 000057** (`create_sync_role.sql`): Must be supplemented. Add `ALTER ROLE mtga_sync WITH rds_iam;` in migration 000060 or a patch migration alongside the Lambda refactor.
- **`services/sync/README.md`**: Must be rewritten. Env vars become Lambda environment variables; systemd `EnvironmentFile` references are removed.
- **EC2 deploy step in `release.yml`** (PR #1048): Must be replaced with a Lambda zip build and deploy step.

---

## Consequences

**Easier**
- Zero idle cost for sync — Lambda charges only for execution time.
- No credential management: IAM token auth eliminates the `DATABASE_URL` password problem entirely.
- Built-in retry and dead-letter queue on EventBridge rule failures.
- No systemd process management on EC2 — fewer moving parts in the EC2 instance.
- Card metadata sync is triggered cleanly for new set releases without SSH/SSM access.

**Harder**
- Lambda cold starts add ~200–500 ms to the first invocation of each schedule window. Acceptable for daily batch jobs.
- 15-minute max runtime. Bounded by current data volumes — if a full Scryfall sync takes longer than 15 minutes in the future, the handler must be split into paginated steps or moved to Step Functions. Not a current constraint.
- VPC configuration required for Lambda to reach RDS in the private subnet. Lambda must be deployed into the same VPC as RDS with a security group allowing port 5432 egress.
- `go.work` must include `services/sync` and its Lambda dependencies; the CI artifact is a `bootstrap` binary (Lambda custom runtime) rather than a standard Go binary.

---

## Alternatives Considered

### A — Keep EC2/systemd (what was accidentally merged)

Rejected. Contradicts ADR-001. Requires systemd process management on EC2, a `DATABASE_URL` SSM parameter containing a password (credential management overhead), and costs the EC2 instance resources even when idle. The only advantage is that the work is already merged — but the credential gap (issue #1054) means it cannot be deployed to production as-is regardless.

### B — ECS Fargate scheduled task

Rejected at current scale. More expensive than Lambda for short scheduled jobs, adds ECR image management overhead, and provides no benefit over Lambda until container size or startup requirements exceed Lambda limits.

### C — Keep ticker loop, run in Lambda with `context.Done()` termination

Rejected. A ticker loop in Lambda is a misuse of the runtime model. EventBridge provides the scheduling guarantee; the Lambda handler should execute once and return. A looping Lambda would waste execution time and obscure errors in the retry model.
