# Infrastructure Agent -- Current Task Status
**Updated**: 2026-05-09
**Task**: PR #1706 — Address CodeRabbit CHANGES_REQUESTED (CORS on /api/v1/events, proxy_hide_header)
**Status**: Complete

## Root Cause (CodeRabbit findings)

1. **Missing CORS on `/api/v1/events`** — The SSE location block was added without the OPTIONS preflight handler and CORS `add_header` directives that were added to the `/api/v1/` block in this PR. A browser preflight to `/api/v1/events` would fail.

2. **Duplicate CORS headers from upstream** — The Go BFF already emits CORS headers. Without `proxy_hide_header`, nginx layered its own `add_header` directives on top, producing duplicate `Access-Control-Allow-*` headers that browsers reject.

## Fix

1. Added OPTIONS preflight handler + CORS `add_header` directives to `location /api/v1/events` (mirrors `/api/v1/` block).
2. Added `proxy_hide_header Access-Control-Allow-Origin/Methods/Headers` to both `/api/v1/events` and `/api/v1/` proxy blocks to strip upstream BFF CORS headers before nginx adds its own.

## Progress
- [x] Read agent instructions and broadcast
- [x] Checked out PR #1706 branch (worktree-agent-af3f5d25c8fa7139a)
- [x] Applied Finding 1: OPTIONS preflight + CORS headers on `/api/v1/events`
- [x] Applied Finding 2: `proxy_hide_header` on both proxy location blocks
- [x] Wrote status checkpoint
- [x] Committed and pushed to PR branch

## Blockers
None

## ETA
Complete
