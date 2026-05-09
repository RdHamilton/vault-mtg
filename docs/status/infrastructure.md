# Infrastructure Agent -- Current Task Status
**Updated**: 2026-05-09T06:15 UTC
**Task**: #1696 — Fix nginx/CloudFront 503+no-CORS on unhandled routes → return 404+CORS
**Status**: Complete

## Root Cause

The nginx config (`infrastructure/nginx/api.vaultmtg.app.conf`) had two problems:

1. **No CORS headers in nginx** — CORS headers were only added by the Go BFF application. When nginx returned an error response (502/503 -- Go unreachable), no CORS headers were present. The browser blocked the error response entirely.

2. **No catch-all for unknown routes** -- nginx had no `location /` fallback. Any request that did not match `/api/v1/*`, `/health`, or `/healthz` received nginx's default 404 without any CORS headers.

Together these meant: an unknown `/api/v1/*` route either (a) reached Go, which returned 503 (its framework default for unhandled routes) without CORS headers, or (b) nginx itself returned 404 without CORS headers. Either way, the browser could not read the error.

## Fix

1. Added CORS headers (`add_header ... always`) to the `location /api/v1/` proxy block -- these apply to all responses including 502/503.
2. Added OPTIONS preflight handler inside `location /api/v1/` -- short-circuits before the proxy_pass, returns 200+CORS for preflight on any `/api/v1/*` path.
3. Added `location /` catch-all that returns `{"error":"not_found"}` with 404 status and full CORS headers for any unmatched path.

## Progress
- [x] Read agent instructions and changelog
- [x] Analyzed existing nginx config
- [x] Identified root cause
- [x] Fixed nginx config -- CORS headers on proxy block + catch-all 404
- [x] OPTIONS preflight handled at nginx layer
- [x] Committed and PR opened

## Blockers
None

## ETA
Complete
