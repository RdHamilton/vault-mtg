# ADR-015: Go Lambda Batch Pattern for Personalized Pre-computation

**Date**: 2026-05-08
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-003 (sync service deployment), PRD-0005 (Smart Craft Next)

---

## Context

Smart Craft Next (PRD-0005, milestone v0.4.0) recommends wildcard
spends to each user based on their personal play history, the current
metagame, and archetype-level pick proxies. The recommendation
algorithm reads tens of thousands of historical match rows per user,
joins against archetype tables, and applies a scoring model. Running
this at request time would push the BFF p95 well past acceptable
limits and would re-do identical work every time the user opens the
craft page.

The natural shape of the work is a **nightly per-user batch**:
recommendations only need to refresh once a day, the inputs (match
history, archetype data) settle on a daily cadence, and the output is
small (a ranked list of ~20 cards per user per format).

VaultMTG already runs one Go Lambda on EventBridge cron — the set
data sync at `services/sync/cmd/lambda/main.go` (ADR-003). It pulls
from the Scryfall API, transforms, and writes to RDS. This is the
nearest precedent for the Smart Craft Next batch and worth
generalizing into a documented pattern before adding the second
instance, because future features (opponent prediction, deck
scoring, personalized format meta-reports) will follow the same
shape.

Three placements were considered for the new batch: (A) extend the
BFF process with a goroutine on a ticker; (B) stand up an ECS Fargate
task with its own scheduler; (C) write a second Go Lambda following
the ADR-003 pattern.

---

## Decision

**All personalized pre-computation features run as Go Lambdas
triggered by EventBridge cron, following the pattern established by
`services/sync/cmd/lambda/main.go`. Smart Craft Next's batch is the
second instance of this pattern. Subsequent pre-computation features
adopt the same shape unless explicitly justified otherwise in their
own ADR.**

### Specifics

1. **Runtime**: Go on AWS Lambda (`provided.al2023`), built with the
   monorepo's existing Lambda toolchain.
2. **Trigger**: EventBridge scheduled rule. Smart Craft Next runs
   nightly at 03:00 UTC. Future features pick non-overlapping windows
   to avoid RDS contention.
3. **Database access**: **RDS IAM authentication only.** The Lambda
   execution role is granted `rds-db:connect` on the BFF database
   user. No hardcoded passwords, no SSM `SecureString` for DB creds.
   This matches the policy applied to the sync Lambda and is enforced
   by the CloudFormation template's IAM policy block.
4. **Idempotency**: The output table for each batch feature includes
   a natural idempotency key. For Smart Craft Next, that is
   `(account_id, format, computed_at::date)` on the
   `craft_recommendations` table, written via `INSERT ... ON CONFLICT
   ... DO UPDATE`. Re-running the Lambda for the same date is safe
   and produces identical output.
5. **Timeout**: Lambda timeout capped at 15 minutes (the AWS hard
   limit). The batch must fit. Smart Craft Next is sized for ~10k
   accounts in well under that envelope; a single invocation
   processes all accounts in one run, paginating through them via
   keyset pagination on `account_id`.
6. **Per-account error handling**: Errors processing one account are
   logged with the `account_id` and skipped. **The batch never aborts
   the full run because of a single bad account.** A summary log line
   at the end of the invocation reports total processed, total
   skipped, and the first three error signatures.
7. **Observability**: Each invocation emits a structured log line at
   start and end (CloudWatch Logs). A CloudWatch metric
   `BatchAccountsProcessed` and `BatchAccountsErrored` is published
   per invocation per feature (dimension: `Feature=smart-craft-next`).
   Alarms on `BatchAccountsErrored > 5%` of total accounts in two
   consecutive runs.
8. **CloudFormation**: Each batch Lambda lives in its own CF stack
   under `infrastructure/cloudformation/`. The stack defines the
   Lambda function, EventBridge rule, IAM role with RDS IAM auth
   permissions, CloudWatch log group, and alarms. **A reusable
   template is created as part of this ADR's implementation work**
   (the directory does not exist today and must be added — see
   Implementation Tickets).
9. **Code layout**: Batch Lambdas live under
   `services/<feature>/cmd/lambda/main.go`. Smart Craft Next's lives
   at `services/craft/cmd/lambda/main.go`. Shared helpers (RDS IAM
   token generation, structured logging, account pagination) lift to
   `pkg/batch/` once the second Lambda lands so we do not have two
   copies of the same boilerplate.

### What this changes

- Establishes the Lambda batch pattern as the **default** for
  personalized pre-computation across the v0.4.0 roadmap and beyond.
- Adds a new `infrastructure/cloudformation/` directory in the
  primary repo (today CF templates live in `vault-mtg-infra` —
  see Consequences for the split rationale).
- Adds `services/craft/` as the second batch service in the monorepo,
  alongside `services/sync/`.
- Lifts shared batch boilerplate to `pkg/batch/` after the second
  Lambda is in flight.

### What this does not change

- The set data sync Lambda (`services/sync/cmd/lambda/main.go`)
  continues to run unchanged. This ADR codifies the pattern it
  already follows; it does not require a rewrite.
- The BFF's request-time path. Reads from `craft_recommendations`
  remain a simple indexed `SELECT` from the user-facing API — the
  Lambda only writes to that table.

---

## Consequences

### Positive

- **One pattern, many features.** Smart Craft Next, opponent
  prediction, deck scoring, and any future personalized
  pre-computation share the same runtime, deploy story, IAM model,
  observability shape, and idempotency contract. Engineers only learn
  this once.
- **Cheap.** Lambda cold-start cost is negligible for a once-a-night
  invocation, and Lambda's per-millisecond billing is well below
  Fargate or always-on EC2 for batch workloads of this size.
- **No new infra primitive.** The team already operates Lambdas,
  EventBridge rules, CloudWatch alarms, and RDS IAM auth. Nothing
  new to learn or harden.
- **Per-account isolation by default.** Errors stay scoped to one
  account. A bad row in one user's match history cannot block another
  user's recommendations.

### Negative

- **15-minute hard cap.** If any future batch feature outgrows 15
  minutes, it will need either to shard across multiple Lambda
  invocations (fan-out via EventBridge or SQS) or to migrate to ECS
  Fargate. Smart Craft Next is comfortably inside the envelope today;
  re-evaluate at the 10x user-base mark (~100k accounts).
- **Cold start on the first invocation of the day.** Adds a few
  hundred milliseconds. Acceptable for a nightly cron; would not be
  for a request-time path.
- **CloudFormation templates currently live in a sibling repo.**
  `vault-mtg-infra` owns the existing CF stacks. The Smart Craft
  Next stack lands there too, **not** in the primary monorepo, to
  preserve the single-source-of-truth boundary. The
  `infrastructure/cloudformation/` reference in earlier drafts of
  this ADR is corrected: all CF templates live in the infra repo.
  See Implementation Tickets.

### Deferred

- **Sharded fan-out.** When any batch crosses the 15-minute timeout,
  introduce a "splitter" Lambda that lists accounts and emits one SQS
  message per shard, with worker Lambdas consuming the queue. Not a
  v0.4.0 concern.
- **Cross-feature scheduling coordination.** Today each batch picks
  its own cron window manually. Once we have ~5 batch features,
  introduce a shared schedule registry to prevent two heavy batches
  from colliding on RDS at the same minute.
- **Backfill tooling.** A one-shot CLI invocation of the same Lambda
  handler (for re-computing historical days) is useful but not on
  the v0.4.0 critical path. Add when the first batch needs a backfill
  in production.

---

## Implementation Tickets

These tickets land in milestone v0.4.0 on project board #29. Project
Manager owns final ticket creation, milestone assignment, and
sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Add `services/craft/cmd/lambda/main.go` skeleton: handler signature, RDS IAM auth, account pagination loop, structured logging | backend-engineer |
| **TBD-B** | Implement Smart Craft Next scoring algorithm inside the handler; idempotent upsert into `craft_recommendations` keyed on `(account_id, format, computed_at::date)` | backend-engineer |
| **TBD-C** | Schema migration: create `craft_recommendations` table with the idempotency key as a unique constraint | dba |
| **TBD-D** | CloudFormation: add Smart Craft Next batch stack to `vault-mtg-infra` (Lambda, EventBridge rule, IAM role with RDS IAM auth, log group, alarms) | infrastructure |
| **TBD-E** | After TBD-A merges, lift shared batch boilerplate (RDS IAM token gen, account pagination, log helpers) to `pkg/batch/` and refactor both Lambdas to use it | backend-engineer |
| **TBD-F** | Observability: CloudWatch metrics + alarm on `BatchAccountsErrored > 5%` for two consecutive runs | infrastructure |
| **TBD-G** | Docs: pattern reference doc at `docs/patterns/batch-lambda.md` summarizing this ADR for future feature authors | architect |

Each ticket gets acceptance criteria when the Project Manager files
it.

---

## Alternatives Considered

### A. Goroutine on a ticker inside the BFF process

**Rejected.** Couples batch work to BFF process lifecycle. A BFF
restart mid-batch leaves work half-done with no clean recovery.
Scaling the BFF horizontally would either run the batch N times in
parallel (data races on the output table) or require a leader-election
mechanism we do not currently have. Lambda + EventBridge gives us
exactly-once cron semantics for free.

### B. ECS Fargate scheduled task

**Rejected for v0.4.0.** Fargate is the right answer **if** the batch
outgrows 15 minutes or needs >10 GB memory. Smart Craft Next needs
neither. Fargate adds a container build step, an ECR repo, a task
definition, a cluster, and a more complex IAM story — none of which
the team is currently operating for any other workload. Lambda is
strictly simpler.

### C. Second Go Lambda following the ADR-003 pattern

**Accepted.** See Decision section above.

---

## References

- ADR-003 — Sync service deployment strategy. Establishes the
  Lambda+EventBridge+RDS-IAM pattern this ADR generalizes.
- PRD-0005 — Smart Craft Next. The first consumer of this pattern
  beyond the set data sync.
- `services/sync/cmd/lambda/main.go` — reference implementation of
  the pattern.
- AWS Lambda quotas — 15-minute execution timeout, 10 GB memory
  ceiling, 250 MB unzipped deployment package limit.
- ADR-016 — External data dependency (17lands bulk CSV). The Smart
  Craft Next Lambda is also responsible for refreshing the 17lands
  S3 cache as part of its nightly run.
