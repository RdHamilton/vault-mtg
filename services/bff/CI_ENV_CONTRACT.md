# BFF CI Environment Contract

**RULE-INFRA-03**: Every CI job that starts the BFF process MUST explicitly set `MTGA_ENV`
in its `env` block. Relying on `config.Load()` defaults is forbidden — the default is
`"production"`, which is always wrong in CI.

## Required env vars per job type

| Job type | MTGA_ENV | Notes |
|---|---|---|
| E2E shard (ci.yml `frontend-e2e`) | `development` | Allows BFF to start without a real DB or Clerk key |
| E2E smoke (e2e-smoke.yml `e2e-smoke`) | `development` | Same rationale; must stay in sync with shards |
| Integration tests (integration.yml) | Not started — tests call the BFF via `go test` | No BFF process launched |
| Release deploy (release.yml `deploy`) | Set on EC2 via SSM — not in the workflow env | Production value provisioned by provision-env.sh |
| Staging deploy (staging-deploy.yml) | Set on EC2 via SSM | Staging value provisioned by provision-env.sh |

## Jobs that start the BFF process

The following CI jobs launch a BFF binary and therefore MUST set `MTGA_ENV`:

- `ci.yml` → `frontend-e2e` (run step: `bin/mtga-bff &`)
- `e2e-smoke.yml` → `e2e-smoke` (run step: `bin/mtga-bff &`)

If you add a new job that starts the BFF, add it to this list and set `MTGA_ENV` explicitly.

## Enforcement

- RULE-INFRA-03 is enforced by code review. Any PR that adds or modifies a CI job
  starting the BFF without an explicit `MTGA_ENV` must be rejected.
- This file is the canonical reference for the contract. Keep it up to date.

## Related rules (see docs/product/milestones/v0.3.0/post-mortem-infra.md)

- **RULE-INFRA-01**: New lint jobs must be introduced in two phases (annotation-only then hard-failure).
- **RULE-INFRA-02**: Prove zero actionlint violations before merge.
- **RULE-INFRA-03**: (this rule) Explicit MTGA_ENV in every BFF-starting job.
- **RULE-INFRA-04**: New BFF routes required by E2E must update the shard env block in the same PR.
- **RULE-INFRA-05**: e2e-smoke.yml and ci.yml shard env blocks must stay in sync.
