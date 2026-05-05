# Release Recommendation

**Version**: v2.0.0.5-alpha
**Status**: CONDITIONAL
**Date**: 2026-05-05
**Author**: architect agent

## Recommendation: CONDITIONAL — cut release after closing 2 blockers

Cut a new alpha release after the following blockers close. Both are short-fuse and actionable today.

## Blockers (must close before tag)

1. **#1169** — `Fix: add DAEMON_JWT_SECRET to production EC2 environment`
   - Open. P0 production gap. Daemon registration endpoint disabled in prod.
   - Owner: infrastructure agent.
   - Note: #1170 (the EC2-bootstrap automation) is already CLOSED. #1169 is the operational follow-through (provision the actual secret to the running EC2 box and confirm BFF logs no longer error).

2. **PR #1221** — `ADR-007 Frontend Serving Model`
   - Open. Doc-only PR, but currently CHANGES_REQUESTED by CodeRabbit (reference to `.claude/plans/adr-007-tickets.md` — file already exists in repo, so this comment can be resolved/dismissed, then merged).
   - Owner: architect (this agent).

If either cannot close in this window, defer the release.

## Rationale

- 94 commits and ~50 closed issues since `v2.0.0.3-alpha` (last tagged release on `main`). Significant scope: SSE migration, daemon JWT auth + JWT mid-session refresh, daemon binaries published, daemon_events table + repo + ingest persistence, sync Lambda migration (RDS IAM auth, multi-format ratings, alchemy/anthology sets, color ratings, FetchedAt fix), BFF draft-ratings adapter wired, Vercel path-scoped deploys, ADR-006 + ADR-007.
- Project board #27: 72 in Done, 25 in Released, 0 In Progress. Healthy backlog of completed-but-untagged work warranting a cut.
- No open issues with `p0` / `priority: critical-path` / `critical-path` labels. Open `bug`-labeled tickets (#1128 DLQ, #904, #902) are non-blocking enhancements / pre-existing v1 issues.
- Recent CI on `main`: CI green; "Deploy Frontend to EC2" failures are expected — ADR-007 demotes that workflow to manual-dispatch only and the failures don't represent a regression.
- v2.0 milestone is open with 15 issues remaining (Clerk auth migration, daemon log parsing extensions). None are release blockers; they're the scope for the next milestone.

## Issues to move Done → Released (48 from board)

Wave 1 / Phase 1+2 work:
- #972, #973, #974, #1011, #1014, #1016, #1024, #1025
- #1036, #1037, #1038, #1041, #1045, #1048, #1049, #1050
- #1054, #1055, #1056, #1057, #1058, #1059, #1060, #1061, #1062, #1063, #1064, #1065, #1067, #1068, #1069, #1070
- #1072, #1073, #1074, #1082, #1083, #1084, #1085, #1086
- #1089, #1090, #1091, #1092, #1094, #1113, #1117, #1119, #1120, #1121, #1122, #1123, #1125, #1128, #1130, #1131, #1132, #1133, #1134, #1135, #1136, #1138, #1139, #1140, #1141, #1142, #1143
- Wave 2/3: #1170, #1171, #1172, #1173, #1179, #1181, #1182, #1183

(Final list to be verified by PM at release-cut time. Full Done snapshot captured in this plan; PM should run `gh project item-list 27` again immediately before transition to catch any new entries.)

## Release notes summary

v2.0.0.5-alpha completes the foundational cloud SaaS architecture: daemon→BFF JWT auth with mid-session refresh, SSE replaces WebSocket for browser push, sync service migrated to Lambda with RDS IAM auth and multi-format ratings coverage (PremierDraft/QuickDraft/Sealed/alchemy/anthology), and Vercel-canonical frontend with EC2 nginx demoted to DR-only (ADR-006, ADR-007). Daemon binaries (Win/macOS) and install scripts ship via release workflow. Production secret provisioning (#1169) closes the last P0 ops gap.

## Post-release actions for PM

1. Tag `v2.0.0.5-alpha` from `main` after both blockers close.
2. Create GitHub Release with the notes above; attach daemon binaries via release workflow.
3. Move all 48 Done items to Released on project board #27.
4. Verify production smoke: daemon registers + receives JWT, ingest persists, SSE delivers per-user, draft-ratings UI populates.
5. Open release-verification ticket per `feedback_release_workflow.md`.
