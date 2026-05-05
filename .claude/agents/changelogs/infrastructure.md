# Infrastructure Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-05 — Issue #1068: feat(infra): deploy React SPA to nginx on EC2 (ADR-001 frontend serving)
**PR**: #1184 (in RdHamilton/MTGA-Companion)
**Files changed**:
- `.github/workflows/frontend.yml` — new workflow: builds React/Vite SPA and deploys dist/ to EC2 nginx webroot via S3 + SSM with atomic staging-dir swap
**Summary**: Added the frontend deploy workflow that builds the SPA with VITE_BFF_URL=/api/v1 and atomically deploys it to /var/www/mtga-companion/ on EC2 via SSM RunShellScript, completing ADR-001 frontend serving without any nginx config changes.
