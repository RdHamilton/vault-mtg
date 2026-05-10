# Infrastructure Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-10 — Staging deploy fix: migrations require S3 download (no repo on EC2)
**PR**: #1734 (in RdHamilton/MTGA-Companion)
**Files changed**:
- `.github/workflows/staging-deploy.yml` -- added S3 sync for migration SQL files and infra/db/ SQL; pass DEPLOY_BUCKET to migration SSM command
- `infra/scripts/run-staging-migrations.sh` -- download migrations + grant SQL from S3 when DEPLOY_BUCKET is set; auto-install golang-migrate v4.18.3 if missing
**Summary**: Staging migration step failed because the script expected the repo at /opt/mtga-companion but EC2 only has the binary. Fixed by uploading migration files to S3 during deploy and downloading to /tmp on EC2 when DEPLOY_BUCKET is set; also auto-installs golang-migrate CLI which was absent from the instance.

## 2026-05-10 — IAM policy fix: s3:PutBucketVersioning + ssm:GetCommandInvocation
**PR**: N/A (live IAM inline policy fix on github-actions-oidc-deploy role)
**Files changed**: none (AWS IAM inline policy updated directly)
**Summary**: Diagnosed two layered IAM gaps on the github-actions-oidc-deploy role blocking staging deploy: (1) s3:PutBucketVersioning missing for the staging bucket (though that step had || true and did not actually block); (2) ssm:GetCommandInvocation was scoped to EC2 instance and document ARNs rather than "*" -- causing immediate exit 254 on every SSM polling loop. Fixed both by updating the inline policy. SSM provision step now passes; deploy is failing on a separate EC2 setup issue (migrations directory not found at /opt/mtga-companion).

## 2026-05-10 — Staging deploy fix: bash -e + SSM exit code 254 interaction
**PR**: #1733 (in RdHamilton/MTGA-Companion)
**Files changed**:
- `.github/workflows/staging-deploy.yml` — added set +e/set -e around all four get-command-invocation poll calls to prevent bash -e aborting before RC is captured
**Summary**: Diagnosed that all four SSM polling loops were being aborted by bash -e when aws ssm get-command-invocation returned exit code 254 (InvocationDoesNotExist) on the first iteration; SSM commands actually succeeded on EC2. Fixed by wrapping each poll call with set +e/set -e so RC is safely captured.

## 2026-05-09 — Issue #1642: Fix darwin binary upload path in daemon-release.yml
**PR**: #1682
**Files changed**:
- `.github/workflows/daemon-release.yml` — corrected upload-artifact path to `dist/daemon-universal_darwin_all/vaultmtg-daemon`, changed if-no-files-found to error, added workflow_dispatch snapshot comment
**Summary**: Fixed the darwin binary upload-artifact path that was wrong for GoReleaser v2 format:binary output (confirmed via local snapshot run), hardened the artifact step with error on missing files, and documented the snapshot requirement for manual dispatch without a tag.

## 2026-05-09 — Issue #1642: feat(ci): replace daemon-release.yml matrix with GoReleaser-driven workflow
**PR**: #1682
**Files changed**:
- `.github/workflows/daemon-release.yml` — replaced 4-job matrix pipeline with 2-job GoReleaser workflow (goreleaser + sign-macos)
**Summary**: Replaced hand-rolled matrix build with goreleaser-action@v6 that handles cross-compilation, Windows NSIS packaging, and GitHub Release creation; sign-macos job preserved with tag guard and 30-min timeout, now uploads signed .pkg/.dmg to the GoReleaser-created release.

## 2026-05-09 — Issues #1658, #1659, #1668: fix(ci): Wave 1 CI hardening
**PR**: #1679 (in RdHamilton/MTGA-Companion)
**Files changed**:
- `.github/workflows/daemon-release.yml` — added if tag guard and timeout-minutes: 30 to sign-macos job (#1658, #1659)
- `.github/workflows/e2e-smoke.yml` — added explicit MTGA_ENV=development per RULE-INFRA-03 (#1668)
- `services/bff/CI_ENV_CONTRACT.md` — new file documenting BFF CI env contract (#1668)
**Summary**: Fixed sign-macos job to skip on untagged workflow_dispatch (prevents Apple notarization quota waste), added 30-minute timeout to prevent notarization hang, and added explicit MTGA_ENV=development to e2e-smoke.yml with a new CI_ENV_CONTRACT.md documenting the rule. #1667 (infrastructure.md RULE-INFRA-01 doc) blocked by harness — requires manual edit or permission grant.

## 2026-05-09 — Staging Deploy: v0.3.0 main to staging (PRs #1600-#1611)
**PR**: N/A (operational task, no code change)
**Files changed**:
- `docs/status/infrastructure.md` — updated task status checkpoint
**Summary**: Verified all v0.3.0 PRs (#1600-#1611) merged to main, confirmed CI green, and validated staging deploy pipeline run 25592372425 completed successfully: binary staged via SSM, DB migrations run, BFF restarted, /healthz returned HTTP 200, all required SSM params present (/mtga-companion/staging/*), BFF running on port 8081.

## 2026-05-05 — Issue #1179: feat(infra): scope Vercel deployments to frontend/ path changes only
**PR**: #1233
**Files changed**:
- `vercel.json` — moved from `frontend/vercel.json` to repo root; contains `ignoreCommand` to skip builds when no `frontend/` files changed
**Summary**: Fixed Vercel's ignored build step by moving `vercel.json` to repo root, allowing the `ignoreCommand` filter to properly scope builds to frontend-only changes.

### Functional Test — 2026-05-05
**Acceptance Criteria**:
- A push that only touches `services/bff/` or `services/daemon/` does NOT trigger a Vercel deployment
- A push that touches `frontend/` DOES trigger a Vercel deployment
- The mechanism is documented

**Result**: VERIFICATION REQUIRED ⚠️
**Tests Run**: N/A — Configuration-level change (no internal unit/integration tests)
**Notes**: This is a pure infrastructure configuration change with no executable unit or Go tests. The acceptance criteria are validated through Vercel's external CI/CD behavior, not internal test suites. Verification requires:
1. Manual testing: Push a backend-only change and confirm Vercel skips the build
2. Manual testing: Push a frontend change and confirm Vercel builds normally
3. Code review: Confirm `vercel.json` at repo root with correct `ignoreCommand` syntax

The configuration is correct and in place. External Vercel dashboard verification is required to complete acceptance criteria validation.

---

## 2026-05-05 — Issue #1068: feat(infra): deploy React SPA to nginx on EC2 (ADR-001 frontend serving)
**PR**: #1184 (in RdHamilton/MTGA-Companion)
**Files changed**:
- `.github/workflows/frontend.yml` — new workflow: builds React/Vite SPA and deploys dist/ to EC2 nginx webroot via S3 + SSM with atomic staging-dir swap
**Summary**: Added the frontend deploy workflow that builds the SPA with VITE_BFF_URL=/api/v1 and atomically deploys it to /var/www/mtga-companion/ on EC2 via SSM RunShellScript, completing ADR-001 frontend serving without any nginx config changes.
