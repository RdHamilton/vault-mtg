# Architect Agent Changelog

This is the system-wide record of all changes made across the project. Every agent (daemon, backend, frontend, architect) appends here on task completion. The architect reads this file to maintain full context of what has been built.

<!-- Entry format — prefix the agent name in brackets:
## YYYY-MM-DD — [<agent>] Issue #NNN: <title>
**PR**: #NNN (or "N/A — ADR only")
**ADR**: docs/adr/NNN-title.md (if applicable, architect entries only)
**Files changed**:
- `path/to/file` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-03 — [architect] Issue #1050: architecture review — sync service deployment drift
**PR**: N/A — ADR only
**ADR**: docs/adr/003-sync-service-deployment-strategy.md
**Files changed**:
- `docs/adr/003-sync-service-deployment-strategy.md` — new: ADR-003 documents Lambda deployment for sync, RDS IAM auth decision, impact on merged PRs #1048/#1049/#1053
- `docs/architecture/CHANGELOG.md` — new: architect changelog tracking major findings and plan sync events
- `~/.claude/plans/mtga-companion-aws-launch.md` — updated: marked sync scaffold complete, flagged EC2 deploy step as needing replacement, documented Lambda next steps
**Summary**: Discovered and documented that services/sync drifted from ADR-001 (Lambda+EventBridge) when EC2/systemd artifacts were merged in PRs #1048-#1053; wrote ADR-003 to formally re-affirm the Lambda deployment decision and resolve the credential strategy gap (issue #1054) via RDS IAM auth on Lambda execution role.

## 2026-05-03 — [daemon] Issue #1014: daemon: investigate log preservation and MTGA log overwrite on startup
**PR**: #1042
**Files changed**:
- `services/daemon/internal/logreader/preservation.go` — new: `Snapshot`, `ListSnapshots`, `PruneSnapshots`, `copyFile`
- `services/daemon/internal/logreader/preservation_test.go` — new: 7 unit tests for all preservation functions
- `services/daemon/internal/logreader/poller.go` — drain bug fix: call `checkForUpdates()` before position reset on Remove/Rename fsnotify events
- `services/daemon/internal/logreader/reader_test.go` — added `TestPollerHandlesRotationDrain` integration test
- `services/daemon/internal/config/config.go` — added `LogArchiveDir`, `LogArchiveMaxAge` (7d default), `LogPreserveOnStart` (true default) with env var `MTGA_DAEMON_LOG_ARCHIVE_DIR`
- `services/daemon/internal/config/config_test.go` — 4 new config field tests
- `services/daemon/internal/daemon/service.go` — wired `Snapshot` + `PruneSnapshots` into `Run()` before poller starts
**Summary**: Fixed log preservation so the daemon snapshots Player.log on startup before MTGA can overwrite it; fixed a drain bug where unread bytes were discarded on fsnotify rotation events; filed follow-on issue #1041 for dead `models.go` structs needing alignment with actual MTGA JSON keys.
**Merged**: 2026-05-03 — PR #1042 merged into main.
