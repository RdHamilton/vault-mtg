# v0.4.0 Kickoff: "Beta Launch"

**Date**: 2026-05-09
**Theme**: Stabilize architecture, ship CSP (#1517), onboard beta testers, and address bugs found during v0.3.0 manual testing.
**Author**: Product Manager
**Milestone**: v0.4.0 — Board #30 (`PVT_kwHOABsZ684BW67K`), Milestone #68
**Beta target**: Closed beta opens August 18, 2026. Internal stretch goal: June 26, 2026.
**Post-mortem source**: `docs/product/milestones/v0.3.0/post-mortem.md` (PC-1–PC-11, A1–A10)
**Architect assessment source**: `docs/product/milestones/v0.4.0/arch-assessment.md` (T1–T5 blocking, T6–T17 medium/low)

> **Active board**: Board #30 is the authoritative board for this milestone. Do NOT use Board #27 (PM backlog) or Board #29 (v0.3.0). See BROADCAST.md `## Current Wave` for the live board ID. (PC-6)

---

## 1. Purpose

v0.4.0 delivers the closed beta under the theme "Beta Launch." It has two equally weighted objectives:

1. **Architecture stabilization** — five architect-flagged brittleness issues in the v0.3.0 codebase are silent data-loss or data-corruption risks. They must be resolved before any new feature work begins. Engineering does not start new feature work until Wave 0 (brittleness fixes) is complete.

2. **Beta readiness** — CSP/security headers (#1517), beta invite flow, analytics surface, daemon stability, and business-track instrumentation must all ship before the August 18, 2026 closed beta opens.

v0.3.0 delivered the telemetry pipeline. v0.4.0 builds on it safely — no new features on an unstable foundation.

---

## 2. Architect Review (Required — BROADCAST Active Directive 2)

The v0.3.0 Architecture Assessment (`docs/product/milestones/v0.4.0/arch-assessment.md`, dated 2026-05-09) was completed by the Architect Agent before this kickoff. Key findings:

**Five blocking issues (Wave 0 scope — must resolve before new feature work):**

| ID | Finding | Risk |
|----|---------|------|
| T1 | `knownFormats` map duplicated in `history.go` and `stats.go` | Data correctness — new format added in one file, missed in other |
| T2 | `#1041`: daemon `models.go` structs not aligned with MTGA JSON keys | Silent data corruption — daemon produces empty/default payloads |
| T3 | Projection worker redeclares all contract payload structs | Schema drift — new contract fields silently lost on projection |
| T4 | Projection marks malformed rows as projected (no dead-letter mechanism) | Data loss on any wire format bug |
| T5 | Partial GRE events written with empty `match_id`/`game_number` | Analytics query correctness — partial rows poison aggregate queries |

**Medium-priority issues:** T8 (account lookup cache) and T9 (request timeout middleware) are **Wave 1** scope (BFF hardening track). T6 (daemon local event queue), T7 (NOTIFY/LISTEN projection), T10 (SSE slow-client metrics), T11 (typed stats envelope), T12 (SSE cookie name constant) are **Wave 2** scope.

**Architectural decisions confirmed correct:** SSE (ADR-001), Clerk auth (ADR-009), cursor pagination (ADR-018), event ordering (ADR-013), game-play correlation (ADR-012). Do not revisit.

**Architect sign-off**: Architect has reviewed and issued GO for Wave 0 start pending T1–T5 ticket creation by project-manager.

---

## 3. Pre-Wave-0: Process Work Required Before Engineering Starts (A1–A10)

The following action items from the v0.3.0 post-mortem are process prerequisites. They are PM-owned unless noted. Engineering does not start Wave 0 tickets until all items marked **[GATE]** are complete.

| # | Action | Owner | Status | Notes |
|---|--------|-------|--------|-------|
| A1 | Update PM agent instructions board ID; document PC-6 rule | PM | **[GATE]** — done in this doc; BROADCAST.md Current Wave updated to Board #30 | |
| A2 | Add agent permission audit step to RELEASE_CHECKLIST.md §0 | PM | **[GATE]** — to be verified before Wave 0 starts | Verify `bash` permission present in all agent configs |
| A3 | Add PC-4 (branch cleanliness) to BROADCAST.md Standing Orders | PM | **[GATE]** — to be added to BROADCAST.md at kickoff | Every agent runs `git status && git log --oneline -5` before any PR |
| A4 | Add PC-9 (agent invocation mode) to BROADCAST.md Standing Orders | PM | **[GATE]** — to be added to BROADCAST.md at kickoff | Sync for output-producing tasks; background for state-update tasks |
| A5 | Add PR template with required-tests checklist to repo | LE | Before Wave 0 first PR | Checklist: required tests present, branch clean, scope matches ticket, no TODO stubs |
| A6 | Audit v0.4.0 exit gates for feasibility (PC-10) | PM | **[GATE]** — completed in this doc; see Section 4 | No soak gates without explicit measurement plan |
| A7 | Verify all PM action items in this kickoff are live, not stale (PC-11) | PM | **[GATE]** — completed in this doc; see Section 7 | Stale items pre-marked `[x]` or removed |
| A8 | Add rebase SLA (PC-7) to engineering conventions or CLAUDE.md | LE | Before Wave 0 first PR | PRs open >24h must rebase daily during release periods |
| A9 | Confirm #1517 (CSP/security headers) is on Board #30 | PM → project-manager | **[GATE]** — verified: #1517 is on Board #30 (Milestone #68) | |
| A10 | Move soak gate measurement to PostHog/Sentry continuous monitoring | PM | **[GATE]** — completed in this doc; see Section 4 | No "48-hour soak" blocking exit gates |

**Gate check**: All **[GATE]** items must be confirmed complete before Wave 0 tickets are moved to In Progress on Board #30.

---

## 4. Exit Gates

**PC-10 applied**: All exit gates have been audited for feasibility. Soak requirements have been moved to continuous beta monitoring via PostHog and Sentry — they are not release blockers (per PC-10 from post-mortem).

The following must all be true before v0.4.0 ships:

1. CI is green on main (hard gate — no exceptions; BROADCAST Active Directive 5)
2. All Wave 0 brittleness tickets are Done (T1–T5 resolved)
3. All Wave 1 engineering tickets are Done
4. Security audit (#1488) passes — zero High/Critical findings
5. CSP/security headers (#1517) deployed to CloudFront and verified
6. Beta invite flow is end-to-end working: waitlist → invite → sign-up → daemon install
7. PostHog activation funnel instrumented and emitting real events from at least one complete internal test session
8. Crisp widget live in SPA (#1573) — CS tooling on day 1 of beta
9. NPS survey distributed to internal testers; first results reviewed before public open
10. All business-track tickets are Done on Board #30

**Continuous beta monitoring (not blocking gates):**
- Daemon event drop rate (PostHog `daemon_event_gap_detected`) — alert threshold: >2% of sessions
- BFF p99 response time (Sentry performance) — alert threshold: >2s on any authenticated endpoint
- No High/Critical Sentry errors in 72-hour window post-deploy

---

## 5. Wave Structure

v0.4.0 runs in three sequential waves. Wave 1 does not start until Wave 0 is Done. Wave 2 does not start until Wave 1 is Done.

---

### Wave 0 — Architecture Brittleness Fixes (REQUIRED FIRST)

**Rationale**: T1–T5 are silent data-loss or data-correctness risks. Building new features on top of a projection worker that silently discards malformed events, or a `knownFormats` map that will be updated in only one of two files, is not acceptable for a beta product. These fix-it tickets are small-to-medium effort and must ship first.

**Exit gate for Wave 0**: All five tickets Done; CI green; no new test failures introduced.

| Ticket | Title | Owner | Effort | Finding |
|--------|-------|-------|--------|---------|
| TBD | Extract `knownFormats` to `handlers/formats.go` | backend-engineer | XS | T1 |
| #1041 | Align daemon `models.go` structs with MTGA JSON keys | backend-engineer | M | T2 |
| TBD | Remove projection worker contract struct redeclarations; import `vault-mtg/services/contract` directly | backend-engineer | S | T3 |
| TBD | Add `projection_errors` dead-letter table; separate transient from permanent projection failures | backend-engineer + dba | M | T4 |
| TBD | Filter partial GRE events from aggregate queries (`WHERE partial = false`); add TODO comment on nil guard | backend-engineer | S | T5 |

> **Ticket creation**: project-manager creates GitHub issues for TBD rows with full ACs, labels `architecture` + `v0.4.0`, and Milestone #68. Assign to backend-engineer and dba as appropriate. Add to Board #30.

**ACs for Wave 0 tickets (engineering must verify all before PR):**

- **T1 (knownFormats)**: `knownFormats` declared exactly once in `handlers/formats.go`; both `history.go` and `stats.go` reference the package-level var; no duplicate declarations; existing tests pass.
- **T2 (#1041)**: Every field in daemon `models.go` has a unit test asserting correct JSON key round-trip against a sample MTGA log payload; zero fields produce empty/default output on a valid log line.
- **T3 (contract import)**: Projection worker `worker.go` imports `github.com/RdHamilton/vault-mtg/services/contract`; zero local struct redeclarations for `GamePlayPayload`, `questCompletedPayload`, `inventoryUpdatedPayload`, `collectionUpdatedPayload`, `deckUpdatedPayload`; compiler validates field names at build time.
- **T4 (dead-letter)**: New `projection_errors` table with migration + rollback script; projection worker writes a row on permanent failure (malformed JSON) and retries on transient failure (DB error); integration test covers both paths.
- **T5 (partial events)**: All `game_plays` aggregate queries include `WHERE partial = false`; integration test with a partial=true row verifies it is excluded from stats endpoints; nil guards on `draftAnalytics` and `resultBreakdown` setters have `// TODO: remove nil guard after #NNNN wires this` comments.

---

### Wave 1 — Beta Readiness (Feature + Hardening)

**Rationale**: These tickets deliver the beta product. Wave 1 runs after Wave 0 completes.

**Exit gate for Wave 1**: All tickets Done; security audit passed; CI green; beta invite flow tested end-to-end.

#### Engineering Track

| Ticket | Title | Owner | Notes |
|--------|-------|-------|-------|
| #1517 | feat(infra): CSP and security headers on CloudFront | infrastructure | Security gate — required before public beta |
| #1488 | security: pre-beta security audit | lead-engineer | Hard release gate — zero High/Critical findings |
| #1398 | feat(frontend): daemon onboarding flow for new users | front-engineer | Gated on Clerk Pro decision (see Section 7 Open Questions) |
| #1541 | feat: free constructed win rate dashboard | backend-engineer + front-engineer | Extends match tracking to Standard/Explorer/Historic Brawl |
| #1540 | feat: Brawl commander analytics | backend-engineer + front-engineer | Tags commander identity from match logs; parallel with #1541 |
| TBD | feat: beta invite flow (waitlist → Clerk invite → sign-up) | backend-engineer + front-engineer | Required exit gate; project-manager creates ticket |
| #1573 | feat(frontend): Crisp chat widget + `setup_idle_90s` event | front-engineer | CS tooling on day 1 |
| TBD | Medium-priority BFF hardening (T8: account lookup cache, T9: request timeout middleware) | backend-engineer | Two small tickets; project-manager creates |
| #1542 | feat: shareable player stats profile — public URL + OG preview | front-engineer + backend-engineer | Moved from Wave 2 per Ray decision 2026-05-09; satisfies roadmap Growth Ready exit gate |

**Critical path**: Wave 0 Done → #1517 → #1488 → beta invite flow → exit gate.

### Visual Testing — Storybook + Chromatic

Beta users will see the UI before any other external audience, so visual regressions caught pre-beta are significantly cheaper to fix than those surfaced during or after the closed beta. Storybook also serves as living component documentation that enables UX reviews without requiring a running environment. Adding this track now — with roughly three months before the August 18 open beta — gives the team enough runway to build a full story library and establish a Chromatic baseline before beta traffic begins.

| Ticket | Title | Owner | Effort | Wave |
|--------|-------|-------|--------|------|
| TBD | Discovery spike: Storybook + Chromatic setup on React 19 + Vite | front-engineer | XS (2 days) | Wave 1 |
| TBD | feat(frontend): install and configure Storybook 8 with Vite builder | front-engineer | S | Wave 1 |
| TBD | feat(frontend): write stories for all existing UI components | front-engineer | M | Wave 1 |
| TBD | feat(infra): integrate Chromatic into CI pipeline (PR visual diff gate) | infrastructure | S | Wave 1 |
| TBD | feat(frontend): capture Chromatic baseline snapshots + approve initial set | front-engineer + ux-designer | XS | Wave 1 |

**ACs for Visual Testing tickets:**

- **Discovery spike**: Spike doc written in `docs/engineering/reference/storybook-spike.md`; confirms Storybook 8 + `@storybook/react-vite` builder works with React 19; documents any known incompatibilities; GO/NO-GO recommendation included.
- **Install + configure**: `npx storybook dev` runs without errors; `.storybook/` config committed; existing Vitest suite still passes; TypeScript has no new errors.
- **Write stories**: Every component in `frontend/src/components/` and `frontend/src/pages/` has at least one story covering its default state; components with multiple states (e.g. loading, empty, error) have a story per state; stories use MSW or static fixtures (no live API calls).
- **Chromatic CI**: Chromatic project linked in repo; `chromatic --exit-zero-on-changes` runs on every PR in CI; Chromatic build URL posted as a PR check; merge is blocked on unreviewed visual changes (Chromatic auto-accept only on main branch builds).
- **Baseline**: Chromatic baseline captured from main after Storybook install; all stories reviewed and accepted by ux-designer before Wave 1 close.

#### User Stories — Wave 1

1. As a beta tester, I want to see my constructed win rate per deck so that I can track improvement over time.
   **ACs:**
   - [ ] Given I have ≥1 Standard, Explorer, or Historic Brawl match logged, when I visit the Analytics page, then I see a win rate dashboard per deck and per format
   - [ ] Dashboard shows win/loss/draw counts and win rate percentage per deck
   - [ ] Data is scoped to my account only
   - [ ] PostHog event `analytics_win_rate_viewed` fires on first load per session
   - [ ] Unit + integration + component tests present per CLAUDE.md

2. As a Brawl player, I want to see my commander win rates and matchup records so that I can evaluate my commander choices.
   **ACs:**
   - [ ] Given I have ≥1 Brawl match logged, when I visit the Brawl Analytics tab, then I see per-commander win rates and opponent commander matchup records
   - [ ] Commander identity is tagged from match logs (not manual input)
   - [ ] PostHog event `analytics_brawl_viewed` fires on first load per session
   - [ ] Unit + integration + component tests present

3. As a new user, I want a guided daemon installation flow so that I can connect my Arena account without reading a manual.
   **ACs (conditional on Clerk Pro approval):**
   - [ ] Given I sign up and have no daemon connected, when I land on the dashboard, then I see the onboarding flow
   - [ ] Flow covers: download daemon, install, start, verify connection
   - [ ] PostHog event `daemon_onboarding_started` and `daemon_onboarding_completed` fire at correct steps
   - [ ] Playwright E2E test covers the full onboarding flow

4. As the operator, I want CSP and security headers on CloudFront so that the SPA passes a basic security scan before the public beta.
   **ACs:**
   - [ ] `Content-Security-Policy` restricts `script-src`, `connect-src`, `img-src` to known domains
   - [ ] `X-Frame-Options: DENY` set
   - [ ] `X-Content-Type-Options: nosniff` set
   - [ ] `Referrer-Policy: strict-origin-when-cross-origin` set
   - [ ] Headers present on all CloudFront responses (HTML, JS, CSS)
   - [ ] Verified with `curl -I` on production CloudFront URL

5. As a beta tester, I want in-app chat support so that I can get help without leaving VaultMTG.
   **ACs:**
   - [ ] Crisp widget loads for authenticated users
   - [ ] Crisp identity set from Clerk user (email, name)
   - [ ] `setup_idle_90s` event fires when user is idle 90s without completing setup
   - [ ] Playwright E2E smoke test verifies widget loads

#### Business Track — Wave 1

| Owner | Ticket | Notes |
|-------|--------|-------|
| growth-marketing | #1576: Waitlist launch coordination — August 1 open date | Board #30 |
| growth-marketing | #1577: Beta invite email copy and Clerk invite flow | Board #30 |
| customer-success | #1578: NPS survey — design, tooling, distribution to internal testers | Board #30 |
| customer-success | #1579: Beta FAQ / onboarding doc: daemon install, first draft, troubleshooting | Board #30 |
| business-analyst | #1580: PostHog activation funnel definition (4 events: `signed_up`, `daemon_connected`, `first_draft_started`, `first_draft_complete`) | Board #30 |
| business-analyst | #1581: PostHog feature flag setup for closed-beta cohort | Board #30 |

---

### Wave 2 — Architecture Investment + ML Foundation (Post-Beta Launch)

Wave 2 starts after the closed beta is open (August 18, 2026). These are medium-priority architectural improvements and the ML foundation.

| Ticket | Title | Owner | Finding/Notes |
|--------|-------|-------|---------------|
| TBD | Add daemon local SQLite write-ahead event queue (T6) | architect design + backend-engineer | Crash-safe event delivery; not Sonnet-ready — architect designs first |
| TBD | NOTIFY/LISTEN projection worker (T7) | architect design + backend-engineer | Reduces projection lag from 30s to <100ms; architect designs first |
| TBD | ML deck building — degraded mode (no collection data) | backend-engineer + front-engineer | Repositioned ML vision per reprioritization doc |
| #1543 | Collection log parsing spike (revised scope) | backend-engineer | 3-day spike; go/no-go for full collection-aware ML |

> **Note**: T6 and T7 are architect-designed tickets. The architect must produce a design note before backend-engineer starts implementation. project-manager creates the tickets with placeholder ACs; architect fills in the design ACs before Wave 2 starts.

---

## 6. Metrics

- **Primary**: All Wave 0 + Wave 1 engineering tickets Done; CI green; security audit passed with zero High/Critical findings
- **Secondary**: PostHog activation funnel emitting real events from ≥1 complete internal test session by June 26, 2026
- **Lagging**: ≥10 internal testers onboarded by June 26; ≥50 by August 18

---

## 7. Open Questions — Action Required Before Wave 0 Starts

**PC-11 applied**: All items below have been verified as actually open (not stale) as of 2026-05-09.

| # | Question | Owner | Gate? |
|---|----------|-------|-------|
| OQ-1 | **Clerk Pro decision** (#1398 daemon onboarding, #1314 API key UX): Is Clerk Pro approved? If yes, #1398 moves to Wave 1. If no, #1398 stays out of scope. Deferred 2026-05-09 per reprioritization doc. | Ray | Yes — #1398 ACs depend on this |
| OQ-2 | **Opponent archetype breakdown scope** (#1541): Does the Arena log reliably expose opponent deck composition? If not, `win rate by format + deck only` in Wave 1 (no opponent archetype). BA spike (3 days) answers this. Should it run before or in parallel with Wave 1 development? | Ray | Yes — #1541 scope depends on this |
| OQ-3 | **Collection investigation trigger** (#1543): If log parsing spike returns no-go, is ML full mode parked post-GA, or is it a must-ship for v0.4.0? | Ray | No (Wave 2 only) |
| OQ-4 | **Wave 2 architecture tickets** (T6/T7): Architect has flagged these as "not Sonnet-ready" — they require architect design passes before implementation starts. Confirm Wave 2 architect availability before Wave 1 closes. | PM + architect | No (Wave 2 only) |

> Items from the v0.3.0-era stale list (e.g., "confirm EOE set data registered in BFF") have been removed — they were already resolved before v0.3.0 shipped. (PC-11)

---

## 8. Out of Scope for v0.4.0

| Item | Reason |
|------|--------|
| Stripe billing and tier enforcement (#982, #980, #1381–#1386) | Deferred to GA |
| API key UX (#1314) | Pending Clerk Pro decision (OQ-1) |
| Draft ML — win-rate pick advisor (17Lands-style) | Sidelined — data volume does not support a competitive model at pre-50K MAU scale. See reprioritization doc. |
| 17Lands bulk CSV ingestion (#1592) | Sidelined with win-rate model |
| Overwolf GEP integration | Post-GA, Windows lock-in, architecture cost not justified at beta |
| Full standalone deck builder | Moxfield dominates; wrong investment at this stage |
| Alchemy-specific features | ~8–12% format share and declining; divisive |
| Shareable player stats season card (extended scope) | Wave 1 ships public URL + OG preview (#1542); full season card deferred to Wave 2 if time allows |
| ML full mode (collection-aware) | Wave 3 conditional — gated on collection spike GO |
| ADR-015, ADR-016 tickets (#1589, #1590) | Lambda batch pattern and 17Lands data source — sidelined with ML reprioritization |

---

## 9. Dependencies

| Dependency | Status | Impact |
|-----------|--------|--------|
| Wave 0 complete | REQUIRED before Wave 1 starts | Blocking |
| ADR-008 (CloudFront) | Deployed | #1517 implements security header requirement |
| ADR-009 (Clerk auth) | Deployed | Settings page, beta invite flow |
| ADR-013 (event ordering) | Deployed | Projection correctness (T2–T5 fix this layer) |
| PostHog (free tier, 1M events/mo) | Active | All features instrument PostHog events as ACs |
| Clerk Pro decision | Pending (OQ-1) | #1398 daemon onboarding blocked until resolved |
| Architect availability for T6/T7 design | Needed before Wave 2 | Wave 2 start gated on design note |

---

## 10. Risks

| Risk | Likelihood | Mitigation |
|------|-----------|-----------|
| T2 (#1041 daemon models) reveals additional JSON key mismatches beyond current scope | Medium | T2 scoped as open-ended investigation; backend-engineer flags any expansion before starting Wave 1 |
| T4 (projection dead-letter) schema migration is more complex than XS estimate | Low | DBA reviews before ticket moves to In Progress; architect consulted if down.sql has nontrivial rollback |
| Clerk Pro decision not made before Wave 1 starts | Medium | #1398 is in-scope conditional — stays parked if decision not made; Wave 1 proceeds without it |
| Security audit (#1488) finds a High finding that requires a full Wave 1.5 fix cycle | Low | Audit scheduled at Wave 1 start, not end; fixes counted as Wave 1 scope |
| Beta invite flow scope underestimated (Clerk invite webhooks + UI) | Medium | project-manager creates detailed ticket with ACs before Wave 1 starts; estimate revisited at ticket creation |

---

## 11. Wave Close Checklist (v0.4.0 Full)

**Wave 0 close (required before Wave 1 starts):**
- [ ] T1–T5 tickets Done on Board #30
- [ ] CI green on main
- [ ] No new test failures introduced by brittleness fixes
- [ ] Wave 0 close report written and GO/NO-GO issued by PM + LE co-sign

**Wave 1 close (required before beta opens):**
- [ ] All Wave 1 engineering tickets Done on Board #30
- [ ] Security audit #1488 — zero High/Critical findings documented
- [ ] CSP headers #1517 deployed and verified on production CloudFront
- [ ] Beta invite flow tested end-to-end (waitlist → invite → sign-up → daemon install)
- [ ] PostHog activation funnel emitting events from ≥1 complete internal test session
- [ ] Crisp widget live in staging and production
- [ ] All business-track tickets Done: NPS survey distributed, invite email copy approved, activation funnel defined
- [ ] RELEASE_CHECKLIST.md §0 staging gate completed (infrastructure + PM)
- [ ] RELEASE_CHECKLIST.md §1 all pre-deploy gates green
- [ ] CI green on main — verified at time of wave-close report (not at PR merge time)
- [ ] Visual testing gate: Chromatic baseline captured and all stories accepted by ux-designer before Wave 1 closes.
- [ ] Wave 1 close report written and GO/NO-GO issued by PM + LE co-sign

---

## 12. Post-Mortem Process Changes In Effect for v0.4.0

These changes from the v0.3.0 post-mortem are active and enforced for this milestone. See the post-mortem for root-cause detail.

| Change | What it means for v0.4.0 |
|--------|-------------------------|
| PC-1: Exit gate pre-flight at start of final wave | PM runs exit gate table at Wave 1 start, not at release time |
| PC-2: CI is a hard gate | Wave 0 and Wave 1 close with CI red = NO-GO, no exceptions |
| PC-3: Agent permission audit in release pre-flight | PM verifies `bash` permission in all agent configs before releasing |
| PC-4: Branch cleanliness before any PR | All agents run `git status && git log --oneline -5` first; dirty tree = STOP |
| PC-5: Required test types are a hard LE block | Missing unit/integration/component/E2E tests = hard BLOCK at LE review |
| PC-6: Board ID from BROADCAST.md only | Board #30 is in BROADCAST.md; agent files must not hardcode board IDs |
| PC-7: PR rebase SLA during release periods | PRs open >24h during release week must rebase against main daily |
| PC-8: PR self-review checklist before LE submission | LE PR template with four checkboxes; PRs without it returned immediately |
| PC-9: Agent invocation mode clarity | Sync for output-producing tasks; `run_in_background: true` for state updates only |
| PC-10: Soak gates in beta monitoring, not exit criteria | No "48-hour soak" exit gate; continuous monitoring via PostHog/Sentry |
| PC-11: PM action items verified live before publishing | All Section 7 items verified open as of 2026-05-09; stale items removed |
