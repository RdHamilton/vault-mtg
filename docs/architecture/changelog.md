# Architecture Changelog

This file records major architectural findings, decisions, and plan sync events. It is maintained by the architect agent.

---

## 2026-05-03 — Issue #1050 Analysis and Architecture Review

### Issue #1050 Summary

Issue #1050 is a documentation and gap-analysis task that triggered a critical architectural finding: **the sync service drifted from ADR-001 during the v2.0 implementation cycle**.

ADR-001 specifies that `services/sync` must be deployed as Lambda functions triggered by EventBridge Scheduler. However, before the drift was identified, the following EC2/systemd artifacts were merged to `main`:

- PR #1048 — EC2 SSM deploy step added to `release.yml` for the sync service
- PR #1049 — sync service env vars verified for EC2/systemd deployment
- PR #1053 — `services/sync/README.md` and `cmd/main.go` landing a startup DB ping; `services/bff` migration 000059 granting `INSERT, UPDATE ON sets TO mtga_sync`

The deployment gap was flagged in issue #1050 (comment by RdHamilton, 2026-05-04) and is now resolved by ADR-003.

### New Architectural Direction

**`services/sync` must be rewritten and redeployed as a Lambda function.** Key decisions recorded in ADR-003:

1. Lambda handler entrypoint replaces the ticker loop in `cmd/main.go` and `internal/scheduler/`.
2. EventBridge Scheduler drives two rules: daily ratings refresh and on-demand card metadata sync.
3. RDS connectivity uses IAM authentication via Lambda execution role — no static password, resolves credential gap in issue #1054.
4. Migration 000059 (grants) is safe to keep. Migration 000057 must be supplemented to add `rds_iam` attribute to the `mtga_sync` role.
5. EC2 deploy step from PR #1048 must be replaced with a Lambda zip build and deploy.

### Plan Sync Status

The active plan at `~/.claude/plans/vaultmtg-aws-launch.md` was updated to reflect:

- Phase 1 Track C: `services/sync` scaffold is complete (PR #1043 merged), but deployment is **blocked** pending Lambda refactor.
- EC2 deploy step for sync (PR #1048) is merged but must be **replaced** — it is not the correct deployment model per ADR-001.
- Issue #1054 (credential strategy) is **unblocked** by ADR-003: use RDS IAM auth via Lambda execution role.
- New "Next Up" steps documented: Lambda refactor for sync, EventBridge rules, migration supplement for `mtga_sync` role.

### ADRs Written

- `docs/adr/003-sync-service-deployment-strategy.md` — Accepted. Sync deploys as Lambda + EventBridge. RDS IAM auth via execution role.

### Tickets Recommended for PM Creation

See the architect's final response for the full list with descriptions.

---

## 2026-05-02 — ADR-001: Service Split Decision (Approach B)

ADR-001 written and accepted: Go workspace multi-module monorepo. Four services: Daemon, BFF, Sync, Frontend. Lambda + EventBridge for Sync. EC2+nginx for Frontend. SSE for BFF→browser push.

Implementation tickets cut: #1009 (daemon), #1010 (bff), #1011 (sync), #1012 (contract), #1014 (log preservation), #1015 (DB optimization), #1016 (SetCache flip).

---

## 2026-05-03 — ADR-002: CI/CD Tiered Test Strategy (Accepted)

ADR-002 written and accepted: path-filtered workflows, two-tier testing (PR = unit tests only, main push = integration tests). Reduced CI minute usage ~60–70%.

Implementation tickets: #1037 (path-filtered CI), #1038 (tiered test strategy). Both merged via PRs #1039 and #1040.
