# CI/CD Remediation Plan

**Date:** 2026-05-06  
**Source audit:** `~/.claude/plans/cicd-audit.md`  
**Repos in scope:** VaultMTG, vault-mtg-web, vaultmtg-web

---

## Already Shipped (do not re-plan)

| PR | What it fixed | Audit IDs |
|----|--------------|-----------|
| #1359 | ALLOWED_ORIGINS + DAEMON_JWT_SECRET SSM quoting | Q1, Q2 |
| #1360 | DATABASE_URL SSM quoting + `VITE_CLERK_PUBLISHABLE_KEY` in deploy-spa.yml | Q3 partial, E13 |

---

## Already in Working Tree (uncommitted — need PR)

These changes are staged/modified in the current branch (`fix/clerk-provider-invalid-props`) and are ready to commit as one or more PRs:

| File | Change | Audit ID |
|------|--------|----------|
| `ci.yml` | `-race` added to BFF unit tests | E8 |
| `ci.yml` | `go-lint` job: gofumpt + go vet across all modules | E9 |
| `ci.yml` | `frontend-e2e` job (Playwright, path-filtered) | E15 |
| `ci.yml` | `sync-unit-tests` job + `sync` detect-changes output | E16 |
| `release.yml` | Concurrency group on deploy job | E10 |
| `release.yml` | DAEMON_JWT_SECRET provision step removed | O1 |
| `release.yml` | CLERK_SECRET_KEY provision step added | E14 |
| `release.yml` | Deploy split: Stage (60×10s) + Restart (12×10s) | SSM poll timeout |
| `release.yml` | Post-deploy `/healthz` check via SSM | E3 |
| `e2e-smoke.yml` | Build path fixed: `services/bff/cmd/main.go` | E1, O6 |
| `e2e-smoke.yml` | GONOSUMDB/GOPRIVATE env added to build step | E2 |
| `deploy-spa.yml` | Post-deploy SPA smoke check (`curl …/app.vaultmtg.app/`) | E4 |
| `rollback.yml` | New rollback workflow (untracked) | E5 |

**Action:** Open a PR from `fix/clerk-provider-invalid-props` that covers all of the above. Owner: **infrastructure**.

---

## Wave 0 — P0 Critical (block release until done)

### 0-A: Commit working-tree fixes as PR

**Files:** `.github/workflows/ci.yml`, `release.yml`, `e2e-smoke.yml`, `deploy-spa.yml`, `rollback.yml`  
**Owner:** infrastructure  
**Scope:** S (already written, just needs PR)  
**AWS IAM required:** No (no new IAM policies; CLERK_SECRET_KEY SSM param must exist — see prerequisites below)  
**Depends on:** Nothing

**Prerequisites before merge:**
1. Create SSM SecureString for Clerk secret key:
   ```bash
   aws ssm put-parameter \
     --name /vaultmtg/production/CLERK_SECRET_KEY \
     --value "sk_live_..." \
     --type SecureString \
     --region us-east-1 \
     --profile personal
   ```
2. Verify EC2 instance role has `ssm:GetParameter` + `kms:Decrypt` on that parameter path (should already be covered by the existing `ssm:GetParameter /vaultmtg/production/*` policy).

### 0-B: Decide vaultmtg-web repo fate

**Files:** GitHub repo settings (no workflow files)  
**Owner:** Ray (product decision)  
**Scope:** S  
**AWS IAM required:** No  
**Depends on:** Nothing

The Vercel project for `vaultmtg-web` was deleted 2026-05-06. Any push to main is a silent no-op. Options:
- **Archive** the repo on GitHub (recommended if rhamiltoneng.com content has moved to vault-mtg-web).
- **Rewire** to a new Vercel project or S3+CloudFront if it still serves content.

Until resolved, document the status in the repo's README so no developer thinks they're shipping when they push.

---

## Wave 1 — P1 High (this milestone)

### 1-A: Remove deploy-sync-lambda from release.yml

**Files:** `.github/workflows/release.yml`  
**Owner:** infrastructure  
**Scope:** S  
**AWS IAM required:** No  
**Depends on:** Wave 0-A merged

`sync.yml` deploys the `mtga-sync` Lambda on every push to `main` that touches `services/sync/**`. `release.yml` also has a `deploy-sync-lambda` job that races `sync.yml` — whichever finishes second wins. Remove the job from `release.yml` and let `sync.yml` be the sole Lambda deploy owner.

**YAML diff:**
```yaml
# DELETE from release.yml (lines 376-412 in HEAD):
  deploy-sync-lambda:
    name: Deploy Sync Lambda
    needs: [release]
    if: always() && needs.release.result == 'success'
    ...
```

After removal, the `deploy` and `deploy-sync-lambda` parallelism is gone; `sync.yml` handles Lambda on its own cadence.

### 1-B: Wire integration.yml into release.yml as Tier-2 gate

**Files:** `.github/workflows/release.yml`  
**Owner:** infrastructure  
**Scope:** M  
**AWS IAM required:** No  
**Depends on:** Wave 0-A merged

`integration.yml` has integration tests but is never called by any workflow. Add it as a gate between e2e-smoke and the release job:

**YAML diff in release.yml:**
```yaml
jobs:
  e2e-smoke-gate:
    # ... existing ...

  integration-gate:
    name: Integration Tests
    needs: [e2e-smoke-gate]
    if: always() && (needs.e2e-smoke-gate.result == 'success' || needs.e2e-smoke-gate.result == 'skipped')
    uses: ./.github/workflows/integration.yml
    secrets: inherit

  release:
    needs: [e2e-smoke-gate, integration-gate]
    if: always() && (needs.e2e-smoke-gate.result == 'success' || needs.e2e-smoke-gate.result == 'skipped') && needs.integration-gate.result == 'success'
    # ... rest unchanged ...
```

**Note:** Verify `integration.yml` is `workflow_call`-compatible (has `on: workflow_call:` trigger). If not, add it.

### 1-C: vault-mtg-web CI workflow

**Files:** `RdHamilton/vault-mtg-web/.github/workflows/ci.yml` (new file)  
**Owner:** front-engineer  
**Scope:** S  
**AWS IAM required:** No  
**Depends on:** Nothing (separate repo)

Create `.github/workflows/ci.yml` in vault-mtg-web:

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  ci:
    name: Type Check, Lint, Build
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v6

    - uses: actions/setup-node@v4
      with:
        node-version: '20'
        cache: 'npm'

    - name: Install dependencies
      run: npm ci

    - name: Type check
      run: npx tsc --noEmit

    - name: Lint
      run: npm run lint

    - name: Build
      run: npm run build
```

### 1-D: vault-mtg-web Playwright smoke tests

**Files:** `RdHamilton/vault-mtg-web/tests/smoke.spec.ts` (new), `playwright.config.ts` (new/update)  
**Owner:** front-engineer  
**Scope:** M  
**AWS IAM required:** No  
**Depends on:** 1-C

Add a minimal Playwright config and smoke test:

```typescript
// tests/smoke.spec.ts
import { test, expect } from '@playwright/test';

test('homepage loads', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/VaultMTG/i);
});

test('key pages return 200', async ({ page }) => {
  for (const path of ['/', '/about', '/pricing']) {
    const response = await page.goto(path);
    expect(response?.status()).toBeLessThan(400);
  }
});
```

Add `frontend-e2e` job to vault-mtg-web `ci.yml`:

```yaml
  e2e:
    name: Playwright Smoke
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v6
    - uses: actions/setup-node@v4
      with:
        node-version: '20'
        cache: 'npm'
    - run: npm ci
    - run: npx playwright install chromium --with-deps
    - run: npm run test:e2e
      env:
        CI: true
```

---

## Wave 2 — P2 Medium

### 2-A: Refactor SSM RunCommand steps to shell scripts

**Files:** `.github/workflows/release.yml`, `.github/workflows/frontend.yml`, `scripts/deploy/provision-db-url.sh` (new), `scripts/deploy/provision-env.sh` (new)  
**Owner:** infrastructure  
**Scope:** M  
**AWS IAM required:** No  
**Depends on:** Wave 0-A merged

The nested-escape patterns in DATABASE_URL provisioning (Q3) and `frontend.yml` (Q5) are fragile. The pattern:
```bash
SHELL_CMD="SECRET=\$(aws secretsmanager ... python3 -c 'import sys,json,urllib.parse; ...')"
```
breaks if any value contains `"`, `\`, or `$`.

**Fix:** Ship provisioning logic as shell scripts in `scripts/deploy/`, upload to the instance via S3 or inline base64, execute via SSM RunCommand:

```bash
# In CI:
aws s3 cp scripts/deploy/provision-db-url.sh s3://$DEPLOY_BUCKET/scripts/provision-db-url.sh
COMMAND_ID=$(aws ssm send-command \
  --instance-ids "$EC2_INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters 'commands=["aws s3 cp s3://'$DEPLOY_BUCKET'/scripts/provision-db-url.sh /tmp/provision-db-url.sh && chmod +x /tmp/provision-db-url.sh && /tmp/provision-db-url.sh"]' \
  ...)
```

Also fixes Q4 (bare `commands=[...]` JSON interpolation of `$DEPLOY_BUCKET` and `$GITHUB_SHA`).

### 2-B: Update frontend.yml header comment

**Files:** `.github/workflows/frontend.yml`  
**Owner:** infrastructure  
**Scope:** S (one line)  
**AWS IAM required:** No  
**Depends on:** Nothing

```yaml
# Before:
# Production frontend is served by Vercel.

# After:
# Production frontend is served by S3+CloudFront (see deploy-spa.yml).
# This workflow is preview/DR only — see ADR-007.
```

### 2-C: vault-mtg-web scheduled production smoke ping

**Files:** `RdHamilton/vault-mtg-web/.github/workflows/smoke-ping.yml` (new)  
**Owner:** front-engineer  
**Scope:** S  
**AWS IAM required:** No  
**Depends on:** 1-C

```yaml
name: Production Smoke Ping

on:
  schedule:
    - cron: '*/15 * * * *'   # every 15 min

jobs:
  ping:
    runs-on: ubuntu-latest
    steps:
    - name: Check vaultmtg.app loads
      run: |
        curl -fsSL https://vaultmtg.app/ | grep -q 'VaultMTG' || exit 1
```

---

## Wave 3 — P3 Low

### 3-A: Fix changelog git describe to filter BFF tags only

**Files:** `.github/workflows/release.yml`  
**Owner:** infrastructure  
**Scope:** S  
**Depends on:** Nothing

```yaml
# Before (release.yml ~line 93):
PREV_TAG=$(git describe --tags --abbrev=0 HEAD^ 2>/dev/null || echo "")

# After:
PREV_TAG=$(git describe --tags --abbrev=0 --match 'v*' HEAD^ 2>/dev/null || echo "")
```

Prevents daemon tags (`daemon/v*`) from polluting the BFF changelog range.

### 3-B: Confirm / document daemon Linux exclusion

**Files:** `.github/workflows/daemon-release.yml` (comment update)  
**Owner:** infrastructure  
**Scope:** S  
**Depends on:** Nothing

`daemon-release.yml` builds Windows/amd64 and Darwin (amd64+arm64) only. If this is intentional (consumer app, no Linux desktop), add a comment:

```yaml
# Targets: Windows (amd64), macOS (amd64 + arm64).
# Linux desktop daemon is out of scope — the daemon is a consumer app.
# If Linux support is needed, add linux/amd64 to the matrix.
```

If Linux support is actually needed, add `{goos: linux, goarch: amd64}` to the matrix.

### 3-C: Remove skip_e2e input after E2E gate is healthy

**Files:** `.github/workflows/release.yml`  
**Owner:** infrastructure  
**Scope:** S  
**Depends on:** Wave 0-A (e2e-smoke fix must be live and validated in prod)

Once the e2e-smoke fix (E1) has survived a full release, remove `skip_e2e` or change its default to `'false'` with a required justification comment:

```yaml
# Before:
inputs:
  skip_e2e:
    description: 'Skip E2E smoke gate'
    type: boolean
    default: false

# After (remove entirely or):
inputs:
  skip_e2e:
    description: 'Emergency override — requires reason in workflow run name'
    type: boolean
    default: false  # keep but enforce in release trigger migration below
```

### 3-D: Audit set-vercel-env.yml for deleted Vercel project

**Files:** `vaultmtg-infra/.github/workflows/set-vercel-env.yml`  
**Owner:** infrastructure  
**Scope:** S  
**Depends on:** Wave 0-B (repo fate decided)

The deleted `vaultmtg` Vercel project's ID may be referenced. Remove stale project references.

### 3-E: Enable Dependabot for vault-mtg-web

**Files:** `RdHamilton/vault-mtg-web/.github/dependabot.yml` (new)  
**Owner:** infrastructure  
**Scope:** S  
**Depends on:** Nothing

```yaml
version: 2
updates:
  - package-ecosystem: npm
    directory: /
    schedule:
      interval: weekly
```

---

## Release Trigger Migration Design

### Goal

Migrate `release.yml` from `workflow_dispatch` (manual) to `release: types: [published]` (fires when a GitHub Release is published). This enables a one-click release flow from the GitHub UI and eliminates the manual dispatch step.

### Current dispatch inputs

| Input | Type | Used for |
|-------|------|----------|
| `tag` | string | Git tag to create |
| `prerelease` | boolean | Mark GH Release as prerelease |
| `skip_e2e` | boolean | Skip e2e-smoke gate |

### Migration design

**Phase 1 — dual trigger (implement now, validate for 2 releases)**

Add `release: types: [published]` alongside the existing `workflow_dispatch`. Both paths stay active. Use `env` to normalize inputs:

```yaml
on:
  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag to release (e.g. v1.5.0)'
        required: true
        type: string
      prerelease:
        description: 'Mark as prerelease'
        type: boolean
        default: false
      skip_e2e:
        description: 'Skip E2E smoke gate'
        type: boolean
        default: false

  release:
    types: [published]

env:
  RELEASE_TAG: ${{ github.event_name == 'release' && github.event.release.tag_name || inputs.tag }}
  IS_PRERELEASE: ${{ github.event_name == 'release' && github.event.release.prerelease || inputs.prerelease }}
  SKIP_E2E: ${{ github.event_name == 'release' && contains(github.event.release.name, '[noe2e]') || inputs.skip_e2e }}
```

Replace all `${{ inputs.tag }}`, `${{ inputs.prerelease }}`, `${{ inputs.skip_e2e }}` references in the workflow with `${{ env.RELEASE_TAG }}`, `${{ env.IS_PRERELEASE }}`, `${{ env.SKIP_E2E }}`.

**skip_e2e convention:** When triggering via GitHub Release UI, append `[noe2e]` to the release title (not the tag) to skip E2E. Example: `v1.5.0 [noe2e]`. This is checked via `contains(github.event.release.name, '[noe2e]')`.

**Phase 2 — remove workflow_dispatch (after 2 validated releases)**

Remove the `workflow_dispatch` trigger block. The `env` normalization collapses to direct `github.event.release.*` reads.

### SPA deploy auto-trigger evaluation

**Current problem:** `deploy-spa.yml` triggers on `v*` tag push, but when `release.yml` pushes the tag using `GITHUB_TOKEN`, GitHub suppresses re-triggering other workflows — so `deploy-spa.yml` never fires automatically from a release.

**After the migration:** When a GitHub Release is published via the UI, the tag already exists at publish time. The `release: [published]` event fires `release.yml`, which no longer needs to push the tag. The `deploy-spa.yml` `push: tags: [v*]` trigger becomes the problem again — the tag was pushed by the developer before creating the release, so it should already have fired `deploy-spa.yml` at tag push time.

**Recommended SPA deploy strategy post-migration:**

Remove `push: tags: [v*]` from `deploy-spa.yml` and instead call it from `release.yml` as a reusable workflow or add an explicit `workflow_dispatch` step:

```yaml
# In release.yml, add job after release:
  deploy-spa:
    name: Deploy SPA
    needs: [release]
    if: needs.release.result == 'success'
    uses: ./.github/workflows/deploy-spa.yml
    secrets: inherit
```

Make `deploy-spa.yml` callable via `on: workflow_call:` in addition to `workflow_dispatch`. This gives deterministic sequencing (SPA deploys after BFF release) and removes the GITHUB_TOKEN suppression issue entirely.

### AWS IAM note

No new IAM changes required for the trigger migration itself. The OIDC role trust policy already allows `repo:RdHamilton/vault-mtg:*` — the `release` event fires from the same repo context.

---

## SSM Poll Timeout Fix (in working tree — see Wave 0-A)

**Recommendation: Option (b) — split step (already implemented in working tree)**

The original "Deploy to EC2 via SSM" was a single step polling 30×10s (5 min) for `s3 cp + chmod + systemctl restart`. A 16.7 MB binary download from S3 can take longer than 5 min under load.

**Working tree splits it into three steps:**

| Step | Poll budget | What it does |
|------|-------------|--------------|
| Stage binary | 60×10s (10 min) | `s3 cp` + `chmod` + atomic `mv` to final path |
| Restart BFF service | 12×10s (2 min) | `systemctl restart mtga-bff` |
| Post-deploy health check | 15×10s with `curl` retries | `curl http://127.0.0.1:8080/healthz` |

Total max: ~24 min. The `/healthz` step runs on-instance (no extra SSM invocation overhead).

**Why not option (a)?** Increasing from 30 to 60 iterations is simpler but conflates download time with service restart time — a slow download masks a broken start. Splitting gives better failure attribution.

**Why not option (c)?** `--timeout-seconds` is a server-side SSM document timeout, not a per-poll timeout. It would extend the server window but still requires the client poll to match, and the Go SDK behavior under long polls is less predictable than a simple shell loop.

---

## Ordering Summary

```
Wave 0-A (CI + release fixes) ─┬─> Wave 1-A (remove deploy-sync-lambda)
                                ├─> Wave 1-B (wire integration.yml)
                                └─> Wave 3-C (remove skip_e2e — after 2 releases)

Wave 0-B (repo fate) ──────────> Wave 3-D (audit set-vercel-env.yml)

Wave 1-C (vault-mtg-web CI) ───> Wave 1-D (Playwright smoke)
                                └─> Wave 2-C (smoke ping)

Wave 2-A (shell scripts) ──────> independent but reads same files as Wave 0-A,
                                 so sequence after Wave 0-A PR merges.

Release trigger migration ──────> Phase 1 after Wave 0-A; Phase 2 after 2 releases validated.
```

## AWS IAM Changes Required

| Item | IAM change | Notes |
|------|-----------|-------|
| CLERK_SECRET_KEY SSM param | **No new policy needed** — existing `ssm:GetParameter /vaultmtg/production/*` covers it | Must create the SSM parameter manually before Wave 0-A merge |
| Release trigger migration | **None** | OIDC trust already covers `repo:RdHamilton/vault-mtg:*` |
| Shell scripts in S3 | **May need** `s3:GetObject` on `$DEPLOY_BUCKET/scripts/*` for the EC2 instance role | Verify EC2 instance role policy; current policy likely covers the whole bucket |

---

## Implementation Checklist

- [x] **Wave 0-A** — #1365 merged. E2E fix, Clerk wiring, -race, lint, split deploy, healthz, rollback.
- [x] **Wave 0-B** — vaultmtg-web Vercel project deleted; repo fate decided.
- [x] **Wave 1-A** — #1367 merged. deploy-sync-lambda removed from release.yml.
- [x] **Wave 1-B** — #1368 merged. integration-gate wired into release.yml.
- [x] **Wave 1-C/D** — vault-mtg-web PR #3 merged. CI workflow + Dependabot. Playwright deferred (not installed in repo).
- [x] **Wave 2-A** — #1370 merged. SSM steps refactored to shell scripts in `scripts/deploy/`.
- [x] **Wave 2-B** — #1369 merged. Stale Vercel comment in frontend.yml updated.
- [x] **Wave 2-C** — vault-mtg-web PR #9 merged. smoke-ping.yml running every 15 min.
- [x] **Wave 3-A** — #1371 merged. `--match 'v*'` added to changelog git describe.
- [x] **Wave 3-B** — #1371 merged. Daemon Linux exclusion documented in daemon-release.yml.
- [x] **Wave 3-C** — #1371 merged. skip_e2e tightened to boolean emergency override.
- [x] **Wave 3-D** — vaultmtg-infra #24 merged. set-vercel-env.yml deleted; stale VERCEL_* secrets to be removed from Actions environment.
- [x] **Wave 3-E** — vault-mtg-web PR #3 merged. dependabot.yml created.
- [x] **Release trigger migration Phase 1** — #1372 merged. Dual trigger + env normalization + SPA workflow_call fix.
- [ ] **Release trigger migration Phase 2** — #1373 (open). Remove workflow_dispatch after 2 validated releases via release event.
