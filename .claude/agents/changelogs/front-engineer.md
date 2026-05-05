# Frontend Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file.tsx` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-04 — Issue #1136 #1142: fix(frontend): add BFF draft-ratings and API key adapters
**PR**: #1177
**Files changed**:
- `frontend/src/services/api/bffDraftRatings.ts` — new adapter: getDraftRatings() targeting GET /api/v1/draft-ratings/{setCode}/{format} with cache-degraded header support
- `frontend/src/services/api/bffDraftRatings.test.ts` — 10 MSW tests covering URL, response shape, header parsing, URL encoding, error handling
- `frontend/src/services/api/bffAuth.ts` — new adapter: createAPIKey() targeting POST /api/keys with daemon JWT auth
- `frontend/src/services/api/bffAuth.test.ts` — 9 MSW tests covering URL, Authorization header, response shape, error handling
- `frontend/src/services/api/index.ts` — exported both new modules and their TypeScript types
**Summary**: Added two BFF-only adapter modules for the draft-ratings and API key endpoints; both use direct fetch (not apiClient wrappers) because the BFF returns raw JSON rather than the data-wrapped envelope shape.

## 2026-05-04 — Issue #1139: feat(frontend): add Authorization header to all BFF requests
**PR**: #1150
**Files changed**:
- `frontend/src/adapters/` — added Authorization header injection to all BFF fetch calls via the REST API adapter layer
**Summary**: Wired the auth token into every outbound BFF request so authenticated endpoints receive the Authorization header; implemented at the adapter layer to keep components free of auth concerns.
