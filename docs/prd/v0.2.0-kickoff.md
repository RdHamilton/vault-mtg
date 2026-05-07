# v0.2.0 Kickoff: "Foundation"

**Date**: 2026-05-06
**Theme**: Make the app useful for one user who installs the daemon today.
**Author**: Product Manager
**Last updated**: 2026-05-07 — completion status sync from board #28

---

## 1. P0 Backlog — Status Review

### Pre-conditions: Already Closed

The following tickets are CLOSED and represent completed foundation work. They are prerequisites, not open work items.

| Ticket | Title | Status |
|--------|-------|--------|
| #1257 | Clerk SDK spike — wire clerk-sdk-go v2 into BFF middleware | CLOSED |
| #1310 | users table schema for Clerk user sync | CLOSED |
| #1321 | SSM secrets for CLERK_SECRET_KEY and CLERK_PUBLISHABLE_KEY | CLOSED |

These close the prerequisite track. Auth implementation can begin immediately.

### Scope Clarification: #983

**#983 is NOT a v0.2.0 ticket.**

The current #983 body describes "gate paid features behind auth middleware" — i.e., tier enforcement (free vs. Pro). That is v0.4.0 scope per the beta roadmap. The v0.2.0 requirement is only that protected routes reject unauthenticated requests. That work lives entirely in #981 (ClerkAuthMiddleware). Do not pull #983 into this milestone.

The resolved Q1 answer in beta-roadmap.md describes a follow-on ticket sequence for #980 and #983 that belongs to v0.4.0.

---

## 2. Missing Ticket User Stories

### B3 — Event Projection Layer v1

**Status: DONE** — "feat(bff): event projection endpoints for match and draft history" is Done on board #28.

**As a** VaultMTG user who has connected the daemon,
**I want** my match history and draft sessions to appear in the app,
**So that** I can see my actual game data rather than an empty page.

**Acceptance Criteria:**
- [x] Given a row exists in `daemon_events` with `event_type = 'match.completed'`, when the projection worker runs, then a row is inserted or upserted into the `matches` table scoped to the correct `account_id`
- [x] Given a row exists in `daemon_events` with `event_type = 'draft.started'` or `'draft.pick'`, when the projection worker runs, then the corresponding `draft_sessions` row is created or updated
- [x] Given a `daemon_events` row has already been projected, when the worker runs again, then no duplicate row is created in `matches` or `draft_sessions` (idempotent upsert)
- [x] Given an event with a malformed or unrecognized `payload` JSONB, when projection is attempted, then the error is logged to Sentry and the row is skipped without crashing the worker
- [x] All projected rows include `account_id` sourced from the ingest path (not derived from payload content)
- [x] A `projected_at` timestamp is set on each `daemon_events` row after successful projection so the worker can use a cursor-based scan rather than a full-table scan on each run
- [x] Integration tests cover: happy path (match projection), happy path (draft projection), idempotency, malformed payload skip

**Notes:**
- This is the single most important v0.2.0 ticket. Match history and draft history endpoints are empty without it.
- The desktop app had equivalent parsing logic in the local DB layer — review before writing new code.
- Projection worker can be a BFF background goroutine for v0.2.0; a dedicated Lambda or async job is v0.3.0 scope.
- `matches` and `draft_sessions` tables must exist before this ticket can be implemented. The DBA must deliver schema migrations as a hard dependency.

**Effort:** L

**Depends on:** #981 (ClerkAuthMiddleware — establishes account_id context pattern), #1310 (users table — closed), schema migration ticket for `matches` and `draft_sessions` tables (new ticket, DBA owner)

---

### B5 — EmptyState UX

**Status: DONE** — "feat(frontend): EmptyState component for zero-data views" is Done on board #28.

**As a** VaultMTG user visiting any page in the app,
**I want** to see a clear, informative empty state when data is not available,
**So that** I know the page is functional and understand what action to take next, rather than seeing a stack trace or blank screen.

**Acceptance Criteria:**
- [x] Given a user visits any SPA route whose BFF endpoint returns a 404, 501, or empty array, when the page renders, then a styled EmptyState component is shown — not a stack trace, not a blank div, not a JavaScript error in the console
- [x] Given a user is not authenticated, when they visit a protected route, then they are redirected to the sign-in page (not shown an empty state or an error)
- [x] Given a user is authenticated but has no data yet (e.g., daemon not connected), when they visit Match History, then the EmptyState includes a call-to-action pointing to the daemon onboarding flow
- [x] Given the BFF returns a non-2xx response that is not a known empty-data case, when the SPA receives it, then the error is reported to Sentry and an EmptyState with a "something went wrong" message is shown
- [x] EmptyState component is shared and reusable across all SPA routes — not a one-off per page
- [x] A Vitest component test covers: renders with CTA, renders with error variant, renders with coming-soon variant
- [x] A Playwright smoke test verifies that navigating to at least three routes (Match History, Deck Builder, Collection) does not show a stack trace or console error for an authenticated user with no data

**Notes:**
- The "coming soon" variant (for features not yet implemented) is different from the "no data yet" variant (feature implemented, user just has no records). Both must be designed. The CTA differs: coming soon has no action; no data yet points to onboarding.
- Audit all current SPA routes to enumerate which ones need which variant. This audit is part of the ticket scope.

**Effort:** S

**Depends on:** None (can start immediately — purely frontend work, does not require BFF endpoints to exist)

---

### B7 — Daemon Onboarding Flow

**Status: IN SCOPE — not yet started** (Wave 3 dependency on #1314 API key UX, which is still open)

**As a** new VaultMTG user,
**I want** a clear, in-app path from sign-up through API key generation to daemon installation and first connection,
**So that** I can get my match data flowing without asking a developer for help.

**Acceptance Criteria:**
- [ ] Given a user completes Clerk sign-up for the first time, when they land in the SPA, then they are shown an onboarding checklist or wizard with steps: (1) account created, (2) copy API key, (3) download and install daemon, (4) first connection confirmed
- [ ] Given a user is on the API key step, when they click "Generate API key," then a key is created and displayed with a one-click copy button; the step is marked complete
- [ ] Given a user has generated an API key, when the daemon is configured with that key and makes its first successful ingest call, then step 4 of the onboarding checklist is automatically marked complete (the SPA polls or receives a push update)
- [ ] Given a user has completed all four steps, when they visit the onboarding page again, then they see a "You're connected" confirmation with a link to Match History
- [ ] Given a user has not completed onboarding, when they visit any data page (Match History, Drafts), then a banner or prompt directs them to complete setup
- [ ] Daemon download link points to the correct installer artifact for the user's platform (Windows/Mac). If cross-platform detection is not implemented in v0.2.0, show both download links
- [ ] The onboarding page includes a "fresh start" notice: "VaultMTG starts tracking from the moment you connect — historical data from a prior desktop installation is not imported"
- [ ] A Playwright E2E test covers the full onboarding flow for an authenticated user: arrive at onboarding → generate key → (simulate daemon connection) → see completion state

**Notes:**
- Q2 is resolved: no desktop-to-cloud migration. The "fresh start" messaging is required to set correct user expectations.
- Step 4 completion detection (daemon first connection) requires a BFF endpoint or webhook that the SPA can query. Simplest v0.2.0 approach: poll `GET /api/v1/daemon/status` every 10 seconds during onboarding. The daemon health indicator ticket (see below) likely shares this endpoint.
- The daemon installer artifact must already exist and be downloadable. If it is not publicly hosted, this is a blocker that must be resolved before this ticket ships.

**Effort:** M

**Depends on:** #981 (ClerkAuthMiddleware), #1314 (API key UX — the key generation step is this ticket's core mechanism)

---

### Health Indicator — Daemon Connection Status in SPA Header

**Status: DONE** — "feat(frontend): daemon health indicator in navigation" is Done on board #28.

**As a** VaultMTG user,
**I want** to see the current daemon connection status on every page,
**So that** I know immediately whether my game data is being captured without navigating to a settings page.

**Acceptance Criteria:**
- [x] Given a user is authenticated and the daemon is actively sending events, when any SPA page renders, then the header shows a "Connected" indicator (green dot or equivalent)
- [x] Given the daemon has not sent an event in the last 5 minutes (configurable), when the SPA polls the status endpoint, then the header shows a "Disconnected" or "Not running" indicator
- [x] Given the daemon is connected but an ingest error was recorded in the last 5 minutes, when the header renders, then it shows a "Sync error" indicator with a link to a support or troubleshooting page
- [x] Given a user is not authenticated or has no API key, when the header renders, then the daemon status indicator is hidden (not shown as disconnected)
- [x] The status is fetched from `GET /api/v1/daemon/status` (new BFF endpoint) and polled every 30 seconds; the response shape includes `{ status: "connected" | "disconnected" | "error", last_seen_at: ISO8601 | null }`
- [x] Status indicator is accessible: includes an `aria-label` describing the current state
- [x] A Vitest component test covers all three status variants (connected, disconnected, error)
- [x] The BFF endpoint returns the correct status when called with a valid Clerk JWT

**Notes:**
- This is CS gate 5. It is elevated to P0 for v0.2.0 because it directly affects the exit gate ("Daemon health indicator is visible on every page").
- The `GET /api/v1/daemon/status` endpoint is shared with the B7 onboarding flow (step 4 polling). Coordinate scope between these two tickets — the BFF endpoint should be designed to serve both consumers.
- "Last seen" should be based on the most recent `daemon_events` row for this `account_id`. No separate heartbeat table needed in v0.2.0.

**Effort:** S

**Depends on:** #981 (ClerkAuthMiddleware — endpoint must be protected), B3 schema work (last_seen derived from daemon_events)

---

### Sentry — Error Monitoring in BFF and SPA

**Status: DONE** — "feat(observability): Sentry error monitoring — Go BFF + React SPA" is Done on board #28.

**As a** developer operating VaultMTG in production,
**I want** errors from the BFF (Go) and SPA (React) to be captured automatically in Sentry,
**So that** I can detect and diagnose user-facing failures before users report them.

**Acceptance Criteria:**
- [x] Given any unhandled panic or 5xx error occurs in the BFF, when it is triggered, then an error event appears in the Sentry project within 60 seconds with stack trace, request path, and Clerk user ID (if authenticated)
- [x] Given any unhandled JavaScript exception or React error boundary triggers in the SPA, when it occurs, then a Sentry error event is captured with component stack and user context
- [x] Given a Sentry DSN is configured, when the BFF starts, then it initializes the Sentry Go SDK with environment tag (`production` / `staging`), release version, and sample rate
- [x] Given a Sentry DSN is configured, when the SPA loads, then it initializes the Sentry React SDK with the same environment and release tags
- [x] Sentry DSN values are stored in SSM and injected at runtime — never hardcoded in source or committed to the repo
- [x] A test Sentry event can be triggered manually (e.g., `GET /api/v1/debug/sentry-test` in non-production) and verified to appear in the dashboard
- [x] No PII (email addresses, full names) is included in Sentry payloads. The Clerk user ID (opaque string) is acceptable as user context.
- [x] Sentry is live and capturing events in the production environment before the v0.2.0 exit gate is declared

**Notes:**
- Sentry project creation (account setup, DSN generation) must happen before this ticket is implemented. If no Sentry account exists, that is a prerequisite action item.
- BFF release version should be injected at build time (ldflags or env var from CI/CD) so Sentry can tie errors to specific deploys.
- Source maps must be uploaded for the SPA build so Sentry shows readable stack traces, not minified output.

**Effort:** S

**Depends on:** #1321 (SSM pattern established — closed), CI/CD pipeline (for release version injection and source map upload)

---

### Schema Migration — matches and draft_sessions Tables

**Status: DONE** — "chore(dba): schema migration — matches and draft_sessions projection tables" is Done on board #28.

**As a** backend engineer implementing the event projection layer,
**I want** the `matches` and `draft_sessions` tables to exist in the production database with correct schema and indexes,
**So that** projected events have a destination and queries are performant.

**Acceptance Criteria:**
- [x] `matches` table created with at minimum: `id`, `account_id`, `played_at`, `format`, `result` (`win`/`loss`/`draw`), `opponent_rank`, `deck_id` (nullable), `raw_event_id` (FK to daemon_events), `created_at`
- [x] `draft_sessions` table created with at minimum: `id`, `account_id`, `started_at`, `completed_at` (nullable), `set_code`, `picks` JSONB, `raw_event_ids` JSONB array, `created_at`
- [x] Both tables include `account_id TEXT NOT NULL` with a non-null constraint and an index on `(account_id, played_at DESC)` / `(account_id, started_at DESC)` for paginated list queries
- [x] `daemon_events` table gains a `projected_at TIMESTAMPTZ` nullable column (used by the projection worker cursor)
- [x] Migrations are numbered sequentially and reversible (down migration exists)
- [x] Migrations are applied and verified in the staging environment before the projection layer ticket is unblocked
- [x] A DBA review is completed on index strategy before migration is merged

**Notes:**
- This ticket is a hard blocker for B3 (event projection layer). It should be the first ticket assigned to the DBA in v0.2.0.
- `account_id` type should match the `users` table established in #1310 (CLOSED). Confirm type consistency.

**Effort:** S

**Depends on:** #1310 (users table — closed), #1321 (SSM — closed for migration runner secrets)

---

### BFF MatchHistory Endpoint

**Status: DONE** — "feat(frontend): wire Match History and Draft History pages to cloud BFF history endpoints" is Done on board #28. BFF endpoint shipped as part of B3 projection work.

**As a** VaultMTG user,
**I want** to retrieve a paginated list of my match history from the BFF,
**So that** the Match History page can display my real game data.

**Acceptance Criteria:**
- [x] `GET /api/v1/matches` returns a paginated list of matches for the authenticated user, scoped by `account_id`
- [x] Response shape: `{ data: Match[], pagination: { cursor: string | null, has_more: bool, total: int } }`
- [x] Supports `limit` query param (default 20, max 100) and `cursor` query param for cursor-based pagination
- [x] Supports `format` query param to filter by format (e.g., `?format=draft`)
- [x] Returns HTTP 200 with empty `data: []` when the user has no matches — never a 404
- [x] Returns HTTP 401 when called without a valid Clerk JWT
- [x] Rows are ordered by `played_at DESC`
- [x] Integration test covers: authenticated request returns correct data, unauthenticated request returns 401, empty result returns 200 with empty array, pagination cursor works correctly

**Notes:**
- This endpoint only works after the schema migration and projection layer (B3) are complete.
- For v0.2.0, the 30-day window enforcement (free tier) is NOT applied — that is v0.4.0 scope (#983 follow-ons). Return all matches.

**Effort:** S

**Depends on:** #981 (ClerkAuthMiddleware), B3 (event projection layer), schema migration ticket

---

### BFF DraftHistory Endpoint

**Status: DONE** — shipped with MatchHistory endpoint (same Done ticket above).

**As a** VaultMTG user,
**I want** to retrieve a paginated list of my draft sessions from the BFF,
**So that** the Draft History page can display my real draft data.

**Acceptance Criteria:**
- [x] `GET /api/v1/drafts` returns a paginated list of draft sessions for the authenticated user, scoped by `account_id`
- [x] Response shape: `{ data: DraftSession[], pagination: { cursor: string | null, has_more: bool, total: int } }`
- [x] Supports `limit` (default 20, max 100) and `cursor` query params
- [x] Supports `set_code` query param to filter by set (e.g., `?set_code=EOE`)
- [x] Returns HTTP 200 with empty `data: []` when the user has no drafts
- [x] Returns HTTP 401 when called without a valid Clerk JWT
- [x] Rows are ordered by `started_at DESC`
- [x] Integration test covers: authenticated request, unauthenticated request, empty result, cursor pagination

**Notes:**
- Shares the same dependency chain as MatchHistory. Can be implemented in the same PR or back-to-back by the same engineer.

**Effort:** S

**Depends on:** #981 (ClerkAuthMiddleware), B3 (event projection layer), schema migration ticket

---

## 3. Confirmed P0 List — Dependency Order

The following is the authoritative execution sequence for v0.2.0. Items at the same wave can be parallelized.

### Wave 0 — Complete

| # | Ticket | Note |
|---|--------|------|
| 1 | #1257 | Clerk SDK spike — CLOSED |
| 2 | #1310 | users table schema — CLOSED |
| 3 | #1321 | SSM secrets — CLOSED |

### Wave 1 — Complete

| # | Ticket | Owner | Status |
|---|--------|-------|--------|
| 4 | #981 | backend-engineer | DONE — "Integrate Clerk or Supabase Auth for user accounts" |
| 5 | Schema migration (matches + draft_sessions) | dba | DONE — "chore(dba): schema migration" |
| 6 | B5 EmptyState UX | front-engineer | DONE — "feat(frontend): EmptyState component" |
| 7 | Sentry (BFF + SPA) | backend-engineer + front-engineer | DONE — "feat(observability): Sentry error monitoring" |
| 8 | #1344 Singleton guard | backend-engineer | OPEN — still in Todo on board |

### Wave 2 — Complete (except #1314)

| # | Ticket | Blocked by | Status |
|---|--------|-----------|--------|
| 9 | #1314 API key UX | #981 | OPEN — deferred; Clerk Pro decision pending 2026-05-09 |
| 10 | B3 Event projection layer v1 | #981, schema migration | DONE — "feat(bff): event projection endpoints" |
| 11 | Daemon health indicator | #981, schema migration | DONE — "feat(frontend): daemon health indicator" |

### Wave 3 — In Progress (Batch 1)

| # | Ticket | Blocked by | Status |
|---|--------|-----------|--------|
| 12 | B7 Daemon onboarding flow | #1314, health indicator | BLOCKED — waiting on #1314 |
| 13 | BFF MatchHistory endpoint | B3 | DONE — wired in SPA |
| 14 | BFF DraftHistory endpoint | B3 | DONE — wired in SPA |
| — | #1433 test(bff): add repository integration tests | Wave 2 | IN PROGRESS |
| — | #1442 test(e2e): add authenticated smoke tests for history routes | Wave 2 | IN PROGRESS |
| — | #1458 fix(ci): Frontend E2E job fails — ../bin/apiserver not found | CI blocker | IN PROGRESS |
| — | #1445 [ARCH-1] Staging: update CLAUDE_CODE_GUIDE.md | Staging track | IN PROGRESS |

### Wave 4 — Integration and Exit Gate Verification

| # | Work item | Status |
|---|-----------|--------|
| 15 | Wire SPA Match History page to BFF MatchHistory endpoint | DONE — same ticket as above |
| 16 | Wire SPA Draft History page to BFF DraftHistory endpoint | DONE — same ticket as above |
| 17 | Run full onboarding flow end-to-end (sign-up → key → daemon → first match visible) | PENDING — blocked on B7 / #1314 |

---

## 4. Scope Concerns and Calls

### #983 — Confirm out of v0.2.0

**Decision**: #983 ("Gate paid features behind auth middleware") is v0.4.0 scope. It must NOT be pulled into v0.2.0. The current #983 body describes tier enforcement logic that depends on the Stripe subscription state — Stripe is not in scope until v0.4.0.

The v0.2.0 auth gating requirement is fully covered by #981: protect routes with ClerkAuthMiddleware, return 401 for unauthenticated requests. That is the complete v0.2.0 auth story.

**Action for project-manager**: Confirm #983 is NOT on the v0.2.0 board. Move it to the v0.4.0 milestone if it is currently unassigned.

### Daemon health indicator — Elevate to P0

The beta-roadmap.md lists the daemon health indicator as P1. That is wrong. It is directly named in the v0.2.0 exit gate ("Daemon health indicator is visible on every page"). Elevate to P0 now.

**Update 2026-05-07**: This is now DONE.

### B7 Daemon onboarding — Size risk

B7 is sized M but contains a hidden dependency: the daemon installer must be publicly downloadable before the onboarding flow can direct users to it. If the daemon installer is not hosted (no download URL exists), that is a blocker that must be resolved before B7 can ship. This needs a confirm from Ray before engineering starts B7.

**Update 2026-05-07**: Daemon download page shipped ("feat(marketing-site): daemon download page at vaultmtg.app/download" — Done). Download URL blocker is resolved. B7 is now gated only on #1314.

**Action**: Ray to confirm daemon installer download URL exists and is production-ready. RESOLVED.

### Sentry — Account setup prerequisite

Sentry requires an account and project to be created before the implementation ticket can be worked. If no Sentry account exists, Ray must create it (takes ~15 minutes) and provide the DSN values before the engineering ticket is unblocked.

**Update 2026-05-07**: Sentry is DONE.

### Schema migration is a missing ticket

The PRD lists B3 as a new ticket but does not explicitly call out the database schema migration as its own ticket. The projection layer cannot be implemented before the destination tables exist. A separate DBA ticket is required.

**Update 2026-05-07**: Schema migration is DONE.

### v0.2.0 board state

The current v0.2.0 board (#28) has 30 items but most appear to be carry-over or CI/CD work not related to the Foundation theme.

**Update 2026-05-07**: Board now has 27 Done, 4 In Progress, 8 PR Review, 11 Todo.

---

## 5. v0.2.0 Exit Gate Checklist

All six must be true before the milestone is declared complete:

- [ ] A new user can complete the full onboarding flow (sign-up → API key → daemon install → first match visible) without developer assistance — **BLOCKED on B7 / #1314**
- [x] Match History page renders real data for a connected user (at least one real match row from projection layer) — **DONE**
- [x] Draft History page renders real data for a connected user — **DONE**
- [x] Every other SPA route shows EmptyState — no stack traces, no blank screens, no console errors — **DONE**
- [x] Sentry is capturing errors in both BFF and SPA (verified by triggering a test event) — **DONE**
- [x] Daemon health indicator is visible on every page of the SPA for an authenticated user — **DONE**

**Exit gate status as of 2026-05-07: 5 of 6 satisfied. Blocked on #1314 (API key UX / Clerk Pro decision, expected 2026-05-09).**
