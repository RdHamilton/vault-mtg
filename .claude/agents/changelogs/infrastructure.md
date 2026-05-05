# Infrastructure Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file` — short description
**Summary**: One sentence summary of what was done and why.
-->

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
