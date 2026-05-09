# v0.3.0 Post-Mortem — Infrastructure / CI/CD Findings

**Author**: Infrastructure Agent
**Date**: 2026-05-09
**Scope**: CI/CD failures that blocked the v0.3.0 release tag and delayed wave close

---

## Finding 1 — actionlint surfaced pre-existing shellcheck violations as hard failures

### What happened

The `lint-workflows` job was added to `ci.yml` as a hard-failure gate running actionlint (which calls shellcheck on embedded shell scripts). The job ran for the first time against pre-existing workflow files that already had shellcheck violations:

| File | Violation | Root cause |
|---|---|---|
| `release.yml` | SC2046 — unquoted command substitution in `git describe` | `PREV_TAG` and `COMPARE_BASE` assigned inline without variable intermediary |
| `rollback.yml` | SC2034×2 — unused loop variable `i` | Loop body never references `i` in "Restart BFF" and "Post-rollback health check" steps |
| `staging-deploy.yml` | SC2086×2 — unquoted `$STAGING_BUCKET` in `s3://` URI args | Variable expansion in S3 copy commands not double-quoted |

These violations existed before the lint job was introduced. They were latent — no tool had ever checked them.

### Why it blocked release

`lint-workflows` runs in CI on every push and is a required status check. When the job hard-failed, all PRs targeting main failed CI. The v0.3.0 release tag could not be cut while CI was red.

### Should the job have been added in warning-only mode first?

**Yes.** The correct rollout for any new linting job against an existing codebase is:

1. Run the linter in `--continue-on-error` / annotation-only mode first, on a PR that lists all existing violations.
2. Fix all existing violations in that same PR (or a companion PR) before the lint job becomes a hard gate.
3. Only then flip the job to hard-failure mode.

Adding a lint gate and violations in separate PRs guarantees that one of them will red-CI the other. This is what happened.

### Rules to add

- **RULE-INFRA-01**: Before adding any new CI lint job, run it locally or in a `continue-on-error: true` step against all existing workflow files. Fix all violations in the same PR that introduces the job. Never introduce a hard-failure lint gate against files that have not been cleaned first.
- **RULE-INFRA-02**: When actionlint is added, include a one-time `actionlint --shellcheck "shellcheck --severity=warning"` local pass in the PR description to prove zero violations before merge.

---

## Finding 2 — E2E shards failing 503 on GET /api/v1/events

### What happened

The CI E2E shards (4 parallel Playwright workers) hit `GET /api/v1/events` (the SSE endpoint) and received HTTP 503. The BFF's `BuildRouter` defaults to a 503 stub when `MTGA_ENV` is not explicitly `development` or `production`. In the CI environment:

- `DATABASE_URL` is not set (no RDS in CI — correct by design).
- `CLERK_SECRET_KEY` is not set (correct by design for E2E tests that use log-fixture mode).
- `MTGA_ENV` was not set in the E2E shard step env block, so `config.Load()` defaulted it to `"production"`, which requires both secrets — causing `BuildRouter` to fall into the `default` case and return 503 for all routes including `/api/v1/events`.

The frontend log-fixture E2E tests require the SSE endpoint to function. Without it, every shard that exercised SSE failed.

### What was missing

Two env vars were absent from the `ci.yml` E2E shard step:

```yaml
# Missing from E2E shard step — caused 503
MTGA_ENV: development
BFF_E2E_UNGUARDED_SSE: true
```

`BFF_E2E_UNGUARDED_SSE` additionally requires an application-side change: a `RouterDeps.E2EUnguardedSSE` flag that mounts the SSE route with a sentinel middleware injecting `userID=1` into context. That flag is readable only when `MTGA_ENV=development`, so staging/prod cannot be misconfigured.

### Why it was not caught earlier

The E2E shard setup was written before the BFF auth middleware was fully implemented. The SSE 503 path was only exercised under the full CI matrix (4 parallel shards hitting the real BFF), not in the standalone e2e-smoke job which previously ran against a different code path.

### Rules to add

- **RULE-INFRA-03**: Every CI job that starts the BFF process MUST explicitly set `MTGA_ENV` in its env block. Relying on `config.Load()` defaults is forbidden — the default is `production`, which is always wrong in CI.
- **RULE-INFRA-04**: When a new BFF route is added that is required by E2E tests, the CI shard env block must be updated in the same PR. The BFF engineer and infra must co-review any new env var required by the test harness.
- **RULE-INFRA-05**: The e2e-smoke.yml standalone job must use the same env vars as the CI shard step. A drift between the two jobs is what allowed this failure to go undetected. Add a comment block in both jobs listing all required BFF env vars and stating they must stay in sync.

---

## Finding 3 — Staging deploy was not run before release gate verification began

### What happened

The v0.3.0 release gate verification (E2E smoke, integration gate, then `Create Release`) started on the `v0.3.0` branch before staging had been deployed and verified. The staging deploy (`Staging Deploy` workflow run 25592372425) completed successfully on `main`, but this was a separate manually-triggered run — not a prerequisite enforced by the release workflow.

The release workflow (`release.yml`) begins with:

```
e2e-smoke-gate → integration-gate → release → deploy (BFF + SPA)
```

There is no step that confirms staging is healthy before the gate opens. The E2E smoke gate builds the BFF from source in a containerized environment — it does not validate against staging. A broken staging deploy would not block the release workflow.

### Why this matters

Active Directive #6 states: "Before cutting a release tag: (a) run staging deploy from scratch, (b) verify /healthz returns 200, (c) run Playwright staging smoke suite, (d) confirm all smoke tests pass." This is a manual human step with no enforcement in CI.

### Fix

Add a `staging-health-gate` job as the first job in `release.yml`, before `e2e-smoke-gate`:

```yaml
staging-health-gate:
  name: Verify Staging Health Before Release
  runs-on: ubuntu-latest
  steps:
    - name: Check staging /healthz
      run: |
        HTTP=$(curl -s -o /dev/null -w "%{http_code}" \
          --max-time 10 https://staging-api.vaultmtg.app/healthz)
        if [ "$HTTP" != "200" ]; then
          echo "Staging health check failed: HTTP $HTTP" >&2
          echo "Run the Staging Deploy workflow against main before releasing." >&2
          exit 1
        fi
        echo "Staging healthy: HTTP $HTTP"
```

This does not deploy staging — it only asserts that staging is already healthy. The human-triggered staging deploy remains a prerequisite, but the gate prevents releasing against an unhealthy staging environment.

### Rules to add

- **RULE-INFRA-06**: `release.yml` MUST include a `staging-health-gate` job as the first job in the DAG. It must `curl` `https://staging-api.vaultmtg.app/healthz` and fail if the response is not HTTP 200. All downstream jobs must `needs: [staging-health-gate]`.
- **RULE-INFRA-07**: The staging deploy workflow (`staging-deploy.yml`) must be run and pass on `main` HEAD before any release tag workflow is triggered. This is enforced by RULE-INFRA-06 at the CI level. Document this in `RELEASE_CHECKLIST.md`.

---

## Finding 4 — Sentry auth token was not in production SSM

### What happened

SSM parameter inventory as of 2026-05-09:

| Parameter | Present |
|---|---|
| `/vaultmtg/staging/sentry-auth-token` | Yes |
| `/vaultmtg/staging/sentry-bff-dsn` | Yes |
| `/vaultmtg/staging/sentry-spa-dsn` | Yes |
| `/vaultmtg/prod/sentry-bff-dsn` | Yes |
| `/vaultmtg/prod/sentry-spa-dsn` | Yes |
| `/vaultmtg/prod/sentry-auth-token` | **MISSING** |
| `/vaultmtg/production/sentry-auth-token` | **MISSING** |

The production Sentry auth token is absent from SSM. `deploy-spa.yml` passes `VITE_SENTRY_DSN` via a GitHub Actions variable but does not upload sourcemaps or create a Sentry release. When sourcemap upload is added (a likely future step), it will fail immediately because the auth token is not available.

No current release workflow step reads a Sentry auth token — the gap did not cause a hard failure in v0.3.0. However, it was flagged during release gate verification as a missing secret.

### Root cause

The staging Sentry auth token was provisioned when setting up the staging environment. Production Sentry credentials were provisioned piecemeal — DSN values were stored but the auth token (required for CLI operations: sourcemap upload, release tagging, deploy tracking) was skipped because no workflow step consumed it at the time.

### Fix

1. Provision immediately:
```bash
aws ssm put-parameter \
  --name /vaultmtg/prod/sentry-auth-token \
  --value "sntrys_..." \
  --type SecureString \
  --region us-east-1 \
  --profile personal
```

2. Add sourcemap upload step to `deploy-spa.yml` post-build:
```yaml
- name: Upload sourcemaps to Sentry
  run: |
    SENTRY_AUTH_TOKEN=$(aws ssm get-parameter \
      --name /vaultmtg/prod/sentry-auth-token \
      --with-decryption \
      --region us-east-1 \
      --query Parameter.Value \
      --output text)
    npx @sentry/cli releases \
      --auth-token "$SENTRY_AUTH_TOKEN" \
      --org vaultmtg \
      --project mtga-companion-spa \
      files "${{ env.RELEASE_TAG }}" upload-sourcemaps frontend/dist
```

### Rules to add

- **RULE-INFRA-08**: Before every release, run the following SSM secrets inventory check and confirm all required parameters exist. A missing parameter is a release blocker:

```bash
REQUIRED_PARAMS=(
  /vaultmtg/prod/sentry-auth-token
  /vaultmtg/prod/sentry-bff-dsn
  /vaultmtg/prod/sentry-spa-dsn
  /mtga-companion/production/CLERK_SECRET_KEY
  /mtga-companion/production/db-secret-arn
  /mtga-companion/production/db-endpoint
  /mtga-companion/production/db-name
  /vaultmtg/production/spa-bucket-name
  /vaultmtg/production/spa-distribution-id
)
for param in "${REQUIRED_PARAMS[@]}"; do
  aws ssm get-parameter --name "$param" --profile personal --region us-east-1 \
    --query Parameter.Name --output text 2>/dev/null \
    || echo "MISSING: $param"
done
```

- **RULE-INFRA-09**: When a new third-party service is integrated, provision BOTH staging and production SSM parameters in the same PR that adds the workflow step consuming them. A service that is only configured in staging is not production-ready.
- **RULE-INFRA-10**: Add the SSM inventory check script above as a step in `RELEASE_CHECKLIST.md`. Infrastructure must run it and post output to the release PR before PM issues GO.

---

## Finding 5 — CI/CD improvements to prevent these failures from blocking future releases

### 5.1 — Lint gate rollout process

New lint jobs must be introduced in two phases:
1. `continue-on-error: true` + annotation mode to enumerate existing violations.
2. All violations fixed, then mode flipped to hard-failure.

This is RULE-INFRA-01 above. Add it to `RELEASE_CHECKLIST.md` and the infra agent instructions.

### 5.2 — BFF env var contract

Create a file `services/bff/CI_ENV_CONTRACT.md` listing every env var the BFF requires per CI job type (unit, integration, E2E shard, e2e-smoke). When a new env var is added to the BFF, the contract file must be updated in the same PR. CI fails (via a simple `grep` check) if the contract file was not updated.

Alternatively: add a `validate-ci-env` job to `ci.yml` that runs `go run ./cmd/main.go --validate-env` (a dry-run flag that only checks config and exits 0/1) using only the env vars present in the CI env block.

### 5.3 — Staging gate before release

RULE-INFRA-06 above: add `staging-health-gate` as the first job in `release.yml`.

### 5.4 — Automated secrets inventory job in release.yml

Add a `secrets-inventory` job that runs before `e2e-smoke-gate` and checks that all required SSM parameters exist:

```yaml
secrets-inventory:
  name: Verify Production Secrets Inventory
  runs-on: ubuntu-latest
  environment: production
  steps:
    - name: Configure AWS credentials (OIDC)
      uses: aws-actions/configure-aws-credentials@v6
      with:
        role-to-assume: ${{ secrets.AWS_DEPLOY_ROLE_ARN }}
        aws-region: us-east-1
    - name: Assert required SSM parameters exist
      run: |
        MISSING=0
        for param in \
          /vaultmtg/prod/sentry-auth-token \
          /vaultmtg/prod/sentry-bff-dsn \
          /vaultmtg/prod/sentry-spa-dsn \
          /mtga-companion/production/CLERK_SECRET_KEY \
          /mtga-companion/production/db-secret-arn \
          /mtga-companion/production/db-endpoint \
          /mtga-companion/production/db-name \
          /vaultmtg/production/spa-bucket-name \
          /vaultmtg/production/spa-distribution-id; do
          aws ssm get-parameter --name "$param" --region us-east-1 \
            --query Parameter.Name --output text > /dev/null 2>&1 \
            || { echo "MISSING: $param"; MISSING=1; }
        done
        [ "$MISSING" -eq 0 ] || exit 1
```

### 5.5 — Release checklist enforcement

The existing `RELEASE_CHECKLIST.md` is manual. Gate it: add a required PR check (via a `release-checklist` job) that reads a checkbox file committed to the release branch and fails if any box is unchecked. Until this is automated, the checklist must be completed and linked in the release PR description — PM must verify before issuing GO.

### Summary of new rules

| Rule | One-line description |
|---|---|
| RULE-INFRA-01 | New lint jobs must fix all existing violations before flipping to hard-failure mode |
| RULE-INFRA-02 | PR introducing actionlint must include proof of zero violations across all workflows |
| RULE-INFRA-03 | Every CI job starting the BFF must explicitly set `MTGA_ENV` — never rely on default |
| RULE-INFRA-04 | New BFF env vars required by E2E tests must be added to CI in the same PR |
| RULE-INFRA-05 | E2E shard env block and e2e-smoke.yml env block must stay in sync; document in both |
| RULE-INFRA-06 | `release.yml` must include `staging-health-gate` job gating all downstream jobs |
| RULE-INFRA-07 | Staging deploy must pass before any release tag workflow is triggered |
| RULE-INFRA-08 | Run SSM secrets inventory check before every release; missing param = blocker |
| RULE-INFRA-09 | New third-party integrations must provision both staging and prod SSM in same PR |
| RULE-INFRA-10 | SSM inventory script must be in `RELEASE_CHECKLIST.md`; infra must post output to release PR |
