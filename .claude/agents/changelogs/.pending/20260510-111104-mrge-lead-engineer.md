target: lead-engineer
---
## 2026-05-10 — PR #1733: fix(ci): add set +e/set -e around SSM get-command-invocation polls in staging-deploy.yml

**Ticket(s)**: None (CI fix)

**Verdict**: APPROVED ✓

**Checks**: CLAUDE.md ✓ · Go skipped (workflow-only) · Frontend skipped (no UI)

**Discoveries**: 
- Correctly addresses bash `set -e` errexit bug in SSM polling loops
- Exit code 254 from `aws ssm get-command-invocation` now safely captured before errexit fires
- Applied consistently across all four polling loops (provision, stage-binary, migrations, restart)
- No over-engineering, no scope creep; minimal, focused fix
