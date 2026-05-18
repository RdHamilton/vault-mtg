# Project-Manager Instructions: Alignment Gap Corrective Actions

**Issued by**: Product Manager  
**Date**: 2026-05-09  
**Source**: `docs/product/milestones/v0.4.0/alignment-gap-report.md`  
**Board**: Board #30 (`PVT_kwHOABsZ684BW67K`), Milestone #68, Todo option: `cf9b61a3`

All items below are derived from the v0.4.0 alignment gap report. Execute in priority order. P0 items block Wave 0 start — do these first.

---

## P0 — Wave 0: Create Missing Tickets (BLOCKS WAVE 0)

Labels for all Wave 0 tickets: `architecture`, `v0.4.0`  
Milestone: #68  
Board: #30 (add to Todo column, option `cf9b61a3`)

### T1 — Extract `knownFormats` to `pkg/logparse`

**Title**: `refactor(bff): extract knownFormats map to handlers/formats.go`  
**Assign**: backend-engineer  
**Effort**: XS  
**Labels**: `architecture`, `v0.4.0`

**Description**:  
The `knownFormats` map is duplicated in `history.go` and `stats.go`. If a new format is added in one file but missed in the other, queries silently return wrong data. Extract it to a single `handlers/formats.go` file and have both handlers reference the package-level var.

**Acceptance Criteria**:
- [ ] `knownFormats` declared exactly once in `handlers/formats.go`
- [ ] Both `history.go` and `stats.go` reference the package-level var; no duplicate declarations remain
- [ ] Existing tests pass; no new test failures introduced
- [ ] Unit + integration tests present per CLAUDE.md

---

### T3 — Remove Projection Worker Contract Struct Redeclarations

**Title**: `refactor(projection): import vault-mtg/services/contract directly; remove redeclared structs`  
**Assign**: backend-engineer  
**Effort**: S  
**Labels**: `architecture`, `v0.4.0`

**Description**:  
The projection worker redeclares `GamePlayPayload`, `questCompletedPayload`, `inventoryUpdatedPayload`, `collectionUpdatedPayload`, and `deckUpdatedPayload` locally instead of importing them from `vault-mtg/services/contract`. Any new field added to the contract is silently lost on projection.

**Acceptance Criteria**:
- [ ] Projection worker `worker.go` imports `github.com/RdHamilton/vault-mtg/services/contract`
- [ ] Zero local struct redeclarations for `GamePlayPayload`, `questCompletedPayload`, `inventoryUpdatedPayload`, `collectionUpdatedPayload`, `deckUpdatedPayload`
- [ ] Compiler validates field names at build time
- [ ] Existing tests pass; unit + integration tests present per CLAUDE.md

---

### T4 — Add `projection_errors` Dead-Letter Table

**Title**: `feat(db+projection): add projection_errors dead-letter table for malformed events`  
**Assign**: backend-engineer + dba  
**Effort**: M  
**Labels**: `architecture`, `v0.4.0`

**Description**:  
The projection worker currently marks malformed rows as projected rather than quarantining them. Any wire format bug causes silent data loss with no recovery path. Add a `projection_errors` dead-letter table; projection worker writes a row on permanent failure (malformed JSON) and retries on transient failure (DB error).

**Acceptance Criteria**:
- [ ] New `projection_errors` table added with migration script (`up.sql`) and rollback script (`down.sql`)
- [ ] Projection worker writes a row to `projection_errors` on permanent failure (malformed JSON); does NOT mark the source row as projected
- [ ] Projection worker retries on transient failure (DB connection error, timeout)
- [ ] Integration test covers both paths: permanent failure writes dead-letter row; transient failure retries and succeeds
- [ ] DBA reviews migration before ticket moves to In Progress
- [ ] Unit + integration tests present per CLAUDE.md

---

### T5 — Filter Partial GRE Events from Aggregate Queries

**Title**: `fix(bff): filter partial GRE events from aggregate queries (WHERE partial = false)`  
**Assign**: backend-engineer  
**Effort**: S  
**Labels**: `architecture`, `v0.4.0`

**Description**:  
Partial GRE events are written with empty `match_id`/`game_number` and poison aggregate queries. All `game_plays` aggregate queries must include `WHERE partial = false`. Nil guards on `draftAnalytics` and `resultBreakdown` setters need TODO comments linking to the ticket that wires them.

**Acceptance Criteria**:
- [ ] All `game_plays` aggregate queries include `WHERE partial = false`
- [ ] Integration test: insert a row with `partial = true`; verify it is excluded from all stats endpoints
- [ ] Nil guards on `draftAnalytics` and `resultBreakdown` setters have `// TODO: remove nil guard after #NNNN wires this` comments
- [ ] Unit + integration tests present per CLAUDE.md

---

### Add Existing Tickets to Board #30

- [ ] Add **#1041** (Align daemon `models.go` structs with MTGA JSON keys) to Board #30, Milestone #68, status: Todo — this is T2
- [ ] Add **#1519** (GRE session flush threshold) to Board #30, Milestone #68, status: Todo — partial T5 implementation
- [ ] Add **#1520** (add partial column to game_plays) to Board #30, Milestone #68, status: Todo — partial T5 implementation

---

## P0 — Board Status Fixes: Move Business Track to Todo

Move the following tickets from NO STATUS → **Todo** on Board #30:

| Ticket | Title | Owner |
|--------|-------|-------|
| #1576 | Waitlist launch coordination — August 1 open date | growth-marketing |
| #1577 | Beta invite email copy and Clerk invite flow | growth-marketing |
| #1578 | NPS survey — design, tooling, distribution to internal testers | customer-success |
| #1579 | Beta FAQ / onboarding doc: daemon install, first draft, troubleshooting | customer-success |
| #1580 | PostHog activation funnel definition | business-analyst |
| #1581 | PostHog feature flag setup for closed-beta cohort | business-analyst |

> **URGENT**: #1576 (waitlist launch) has a hard deadline of August 1, 2026. Owner = growth-marketing agent (RdHamilton as proxy on GitHub). If no owner is actively driving it, escalate to Ray immediately.

---

## P0 — Add Missing Wave 1 Tickets to Board #30

Add the following existing GitHub issues to Board #30, Milestone #68, status: Todo, Wave 1:

- [ ] **#1398** — feat(frontend): daemon onboarding flow for new users (assign: front-engineer; note: blocked on OQ-1 Clerk Pro decision — add blocking dependency label)
- [ ] **#1541** — feat: free constructed win rate dashboard (assign: backend-engineer + front-engineer)
- [ ] **#1540** — feat: Brawl commander analytics (assign: backend-engineer + front-engineer)

---

## P0 — Create Beta Invite Flow Implementation Ticket

> Note: #1598 (beta invite flow architecture) already exists on Board #30 as Todo. This is a SEPARATE full implementation ticket.

**Title**: `feat: beta invite flow — waitlist to Clerk invite to sign-up`  
**Assign**: backend-engineer + front-engineer  
**Effort**: M  
**Labels**: `v0.4.0`, `beta`  
**Wave**: Wave 1  
**Milestone**: #68

**Description**:  
Implement the complete beta invite flow: waitlist email submission → operator sends Clerk invite → invitee receives email → invitee completes sign-up → arrives at onboarded dashboard. This is a required exit gate for v0.4.0. Architecture reference: #1598.

**Acceptance Criteria**:
- [ ] Waitlist form submits email and stores it (backend endpoint + frontend form)
- [ ] Operator can trigger a Clerk invite from the stored waitlist (admin action or script)
- [ ] Invitee receives a Clerk invitation email with a valid sign-up link
- [ ] Invitee completes sign-up via Clerk and lands on the authenticated dashboard
- [ ] Clerk webhook/callback confirms successful account creation and removes email from waitlist queue
- [ ] PostHog events: `beta_invite_sent`, `beta_signup_completed` fire at correct steps
- [ ] E2E Playwright test covers full flow: waitlist submission → invite → sign-up → dashboard load
- [ ] Unit + integration + component tests present per CLAUDE.md

---

## P0 — Create PostHog SPA Install Ticket

> This is a prerequisite to #1580 (PostHog activation funnel definition). Must be on Board #30 before #1580 can be worked.

**Title**: `feat(frontend): install and configure PostHog in the SPA`  
**Assign**: front-engineer  
**Effort**: S  
**Labels**: `v0.4.0`, `analytics`  
**Wave**: Wave 1  
**Milestone**: #68

**Description**:  
PostHog must be installed and configured in the React SPA before any activation funnel events can be instrumented. This ticket covers installation, initialization, and verified event emission. Prerequisite to #1580 (funnel definition) and #1581 (feature flags).

**Acceptance Criteria**:
- [ ] `posthog-js` installed and initialized in the SPA (`main.tsx` or equivalent)
- [ ] PostHog project key loaded from env var (`VITE_POSTHOG_KEY`) — not hardcoded
- [ ] Authenticated user identity sent to PostHog via `posthog.identify(clerkUserId, { email })` on sign-in
- [ ] `posthog.capture('signed_up')` fires on successful Clerk sign-up completion
- [ ] At least one test verifies PostHog `capture()` is called with correct event name on sign-up
- [ ] PostHog loads in staging environment and event appears in PostHog dashboard (screenshot in PR)
- [ ] No PostHog calls made for unauthenticated users (privacy — no anonymous tracking)
- [ ] Unit + component tests present per CLAUDE.md

---

## P1 — Create 5 Storybook/Chromatic Visual Testing Tickets

All tickets: assign front-engineer (except Chromatic CI: infrastructure), Wave 1, Milestone #68, Board #30 Todo.  
Labels: `v0.4.0`, `visual-testing`

ACs source: `docs/product/milestones/v0.4.0/kickoff.md` §5 Visual Testing section.

### SB-1 — Discovery Spike

**Title**: `spike(frontend): Storybook + Chromatic compatibility on React 19 + Vite`  
**Assign**: front-engineer  
**Effort**: XS (2 days)

**Acceptance Criteria**:
- [ ] Spike doc written in `docs/engineering/reference/storybook-spike.md`
- [ ] Confirms Storybook 8 + `@storybook/react-vite` builder works with React 19
- [ ] Documents any known incompatibilities
- [ ] GO/NO-GO recommendation included

---

### SB-2 — Install and Configure Storybook 8

**Title**: `feat(frontend): install and configure Storybook 8 with Vite builder`  
**Assign**: front-engineer  
**Effort**: S  
**Depends on**: SB-1 GO result

**Acceptance Criteria**:
- [ ] `npx storybook dev` runs without errors
- [ ] `.storybook/` config committed to repo
- [ ] Existing Vitest suite still passes
- [ ] TypeScript has no new errors (`npx tsc --noEmit` clean)

---

### SB-3 — Write Stories for All Existing UI Components

**Title**: `feat(frontend): write Storybook stories for all existing UI components`  
**Assign**: front-engineer  
**Effort**: M  
**Depends on**: SB-2

**Acceptance Criteria**:
- [ ] Every component in `frontend/src/components/` and `frontend/src/pages/` has at least one story covering its default state
- [ ] Components with multiple states (loading, empty, error) have a story per state
- [ ] Stories use MSW or static fixtures — no live API calls in stories

---

### SB-4 — Integrate Chromatic into CI Pipeline

**Title**: `feat(infra): integrate Chromatic into CI pipeline (PR visual diff gate)`  
**Assign**: infrastructure  
**Effort**: S  
**Depends on**: SB-2

**Acceptance Criteria**:
- [ ] Chromatic project linked in repo (API key in CI secrets)
- [ ] `chromatic --exit-zero-on-changes` runs on every PR in CI
- [ ] Chromatic build URL posted as a PR check
- [ ] Merge is blocked on unreviewed visual changes (Chromatic auto-accept only on main branch builds)

---

### SB-5 — Capture Chromatic Baseline Snapshots

**Title**: `feat(frontend): capture Chromatic baseline snapshots and approve initial set`  
**Assign**: front-engineer + ux-designer  
**Effort**: XS  
**Depends on**: SB-3 + SB-4

**Acceptance Criteria**:
- [ ] Chromatic baseline captured from main after Storybook install
- [ ] All stories reviewed and accepted by ux-designer before Wave 1 close

---

## P1 — Create BFF Hardening Tickets (Wave 1)

Labels: `v0.4.0`, `bff-hardening`  
Milestone: #68, Board #30 Todo, Wave 1

### T8 — Account Lookup Cache

**Title**: `feat(bff): add account lookup cache to reduce DB round-trips on authenticated requests`  
**Assign**: backend-engineer  
**Effort**: S

**Description**:  
Every authenticated BFF request performs a Clerk user ID → account ID lookup. A short-lived in-memory cache (or Redis, if available) reduces DB load and improves p99 latency. Reference: T8 in `docs/product/milestones/v0.4.0/arch-assessment.md`.

**Acceptance Criteria**:
- [ ] Account lookup result cached for a configurable TTL (default: 5 minutes)
- [ ] Cache invalidated on account deletion or update
- [ ] BFF p99 latency on authenticated endpoints measurably improved (Sentry performance baseline comparison in PR)
- [ ] Integration test: cache hit does not make a second DB call
- [ ] Unit + integration tests present per CLAUDE.md

---

### T9 — Request Timeout Middleware

**Title**: `feat(bff): add request timeout middleware to all BFF routes`  
**Assign**: backend-engineer  
**Effort**: S

**Description**:  
BFF has no global request timeout. A slow DB query or upstream call can hold a goroutine indefinitely. Add a middleware that cancels requests exceeding a configurable timeout (default: 10s) and returns 504.

**Acceptance Criteria**:
- [ ] Timeout middleware applied to all BFF routes (not just new ones)
- [ ] Timeout duration configurable via env var (`BFF_REQUEST_TIMEOUT_SECONDS`, default: 10)
- [ ] Slow requests return HTTP 504 with a structured error response after timeout
- [ ] Integration test: a handler that sleeps beyond timeout returns 504
- [ ] Unit + integration tests present per CLAUDE.md

---

## P1 — Wave 1 Board Addition: #1542

- [ ] Add **#1542** (feat: shareable player stats profile — public URL + OG preview) to Board #30, Milestone #68, status: Todo, Wave 1
  - Assign: front-engineer + backend-engineer
  - Note in ticket: "Moved from Wave 2 to Wave 1 per Ray decision 2026-05-09. Satisfies roadmap Growth Ready exit gate."

---

## P1 — Wave 2 Board Addition: #1543

- [ ] Add **#1543** (Collection log parsing spike — revised scope) to Board #30, Milestone #68, status: Todo, Wave 2
  - Assign: backend-engineer
  - Note: 3-day spike; GO/NO-GO for full collection-aware ML

---

## P2 — Board Hygiene (Lower Priority — Do After P0/P1)

### Remove Stripe/Tier Tickets from Board #30

Defer to GA. Remove these from Board #30 (do not close the issues; just remove from this board):

`#980`, `#982`, `#1259`, `#1306`, `#1381`, `#1382`, `#1383`, `#1384`, `#1385`, `#1386`

### Remove ML/17Lands Tickets from Board #30

Sidelined per reprioritization doc. Remove from Board #30:

`#1589`, `#1591`, `#1592`, `#1593`, `#1594`, `#1595`

---

## Completion Checklist

When all P0 items are done, notify product-manager so the Wave 0 kickoff checklist can be verified and Wave 0 can formally start.

| Priority | Item | Done? |
|----------|------|-------|
| P0 | Create T1 ticket + add to Board #30 | |
| P0 | Create T3 ticket + add to Board #30 | |
| P0 | Create T4 ticket + add to Board #30 | |
| P0 | Create T5 ticket + add to Board #30 | |
| P0 | Add #1041 to Board #30 | |
| P0 | Add #1519 to Board #30 | |
| P0 | Add #1520 to Board #30 | |
| P0 | Move #1576–#1581 to Todo | |
| P0 | Add #1398 to Board #30 | |
| P0 | Add #1541 to Board #30 | |
| P0 | Add #1540 to Board #30 | |
| P0 | Create beta invite flow implementation ticket | |
| P0 | Create PostHog SPA install ticket | |
| P1 | Create SB-1 Storybook spike ticket | |
| P1 | Create SB-2 Storybook install ticket | |
| P1 | Create SB-3 Write stories ticket | |
| P1 | Create SB-4 Chromatic CI ticket | |
| P1 | Create SB-5 Chromatic baseline ticket | |
| P1 | Create T8 account lookup cache ticket | |
| P1 | Create T9 request timeout middleware ticket | |
| P1 | Add #1542 to Board #30 Wave 1 | |
| P1 | Add #1543 to Board #30 Wave 2 | |
| P2 | Remove Stripe/tier tickets from Board #30 | |
| P2 | Remove ML/17Lands tickets from Board #30 | |
