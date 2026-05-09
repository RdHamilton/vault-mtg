# PRD: VaultMTG Beta Roadmap

**Status**: Active — v0.2.0 closed, v0.3.0 in progress
**Author**: Product Manager
**Date**: 2026-05-06
**Last updated**: 2026-05-08
**Scope**: v0.2.0 → v0.4.0 (Beta Launch)

## Closed Beta Launch Date

| Milestone | Date | Notes |
|---|---|---|
| Waitlist opens | **2026-06-02** | Aligned with Marvel Super Heroes Arena set release |
| Internal stretch target | **2026-06-26** | Full v0.4.0 exit gates green; first batch invited if on track |
| **Public closed beta launch** | **2026-08-18** | Official public commitment — all six CS gates + growth infrastructure required |

These dates were decided 2026-05-08 after consulting BA, CS, Finance, and Growth Marketing. June 2 is a hard external anchor (set release). June 26 is an internal stretch goal that does not shift the August 18 commitment. August 18 is the public commitment and must not move without a full business review.

---

## 1. Executive Summary

VaultMTG is a Magic: The Gathering Arena companion web app targeting 50,000 MAU. Today the cloud product is a working daemon ingestion pipe with Clerk auth plumbing — every user-facing page beyond Draft Ratings either errors or renders empty. Three milestones bridge the gap from "ingestion shell" to public beta. v0.2.0 ("Foundation") wires up auth, API keys, and the minimum read endpoints users need to see their own data. v0.3.0 ("Telemetry Parity") backports the desktop log parser into the daemon, fills in the full domain schema, and delivers live overlays that work reliably. v0.4.0 ("Beta Launch") adds the growth and monetization infrastructure required to acquire and convert users at scale. Beta is declared at v0.4.0 when all six CS beta gates pass, the free/paid tier is live, and the acquisition funnel (email capture, PostHog, shareable stats) is instrumented. Success at beta = 500 MAU within 30 days, 40%+ activation rate, D7 retention ≥ 25%.

---

## 2. Current State Assessment

The cloud infrastructure launched on 2026-05-05 at an AWS baseline of ~$38/mo. Auth (Clerk), daemon event ingestion, and CI/CD pipelines are partially wired. The BFF exposes roughly 5 endpoints against a SPA that expects 70+. Every major feature — match history, deck builder, collection, drafts, quests, metagame — fails silently or with a stack trace in production. The daemon emits 7 event types but the projection layer that turns raw JSON in `daemon_events` into structured domain tables (matches, decks, draft sessions, collection) does not exist in the cloud. The desktop app had this logic; it has not been ported.

The user-facing onboarding path is incomplete in two ways. First, there is no documented or UI-supported flow from Clerk sign-up to API key issuance to daemon installation — a user cannot successfully connect the daemon without manual intervention. Second, when the daemon is not connected there is no visible indicator in the SPA; the app is silently empty. These two gaps together guarantee that early adopters who find the app before beta will bounce with no path to success.

The $1,000 AWS Activate credit approved 2026-05-05 provides runway. At pre-beta traffic (≤1,000 MAU) AWS costs will not exceed $44/mo, meaning credits cover infrastructure through at least May 2027. The financial constraint is not money — it is engineering bandwidth. Ray is a single full-stack engineer. Every decision on scope must be evaluated through that lens. The AI agents, MCP server, and RAG infrastructure on the v0.2.0 board are post-beta and must be deferred without exception.

---

## 3. Milestones

### Milestone 1 — v0.2.0 "Foundation" — CLOSED

**Status**: CLOSED — 2026-05-07
**Close notes**: All six exit gates satisfied. Match History and Draft History render real data. EmptyState UX live on all routes. Sentry capturing errors in BFF and SPA. Daemon health indicator live on every page. API key UX (#1314) and daemon onboarding flow (#1398) deferred to v0.4.0 pending Clerk Pro decision (2026-05-09); onboarding flow replaced by manual instructions for alpha testers.

**Theme**: Make the app useful for one user who installs the daemon today.

**Duration**: 5–6 weeks (actual)

**What it accomplishes**: A user can sign up via Clerk, receive an API key through the UI, install the daemon, and see their real match history and draft history populated from daemon events. Every other page shows a proper EmptyState rather than an error. Error monitoring is live. Auth and API key UX are complete.

**What it does not accomplish**: Live overlays, full telemetry parity with desktop, monetization, shareable stats, or growth acquisition infrastructure.

#### Key Deliverables

| # | Deliverable | Existing ticket |
|---|---|---|
| 1 | Clerk auth fully wired: users table, JWT middleware on all protected routes, session management | #1257, #1310, #981, #983 |
| 2 | API key issuance, list, revoke, and rotate UI in SPA — complete daemon onboarding flow | #1314 |
| 3 | Daemon onboarding documented path: Clerk sign-up → API key copy → daemon installer | B7 (new ticket needed) |
| 4 | Event projection layer v1: parse `daemon_events` into matches and draft_sessions tables | B3 (new ticket needed) |
| 5 | BFF read endpoints: MatchHistory, DeckList, DraftHistory (paginated, scoped by account_id) | B1 (new ticket needed) |
| 6 | EmptyState UX for all SPA routes that lack a working endpoint — no more stack traces | B5 (new ticket needed) |
| 7 | Sentry wired in BFF and SPA — error tracking live before any user traffic | B5, Growth P0 |
| 8 | Daemon singleton guard + sync resilience | #1344 |
| 9 | SSM secrets management for Clerk keys | #1321 |
| 10 | Live daemon connection health indicator in SPA header (Connected / Syncing / Error) | CS gate 5 |

#### Exit Gate (all must be true)

- [ ] A new user can complete the full onboarding flow (sign up → get API key → connect daemon → see first match) without developer assistance
- [ ] Match history page renders real data for a connected user
- [ ] Draft history page renders real data for a connected user
- [ ] Every other SPA route shows EmptyState, not an error or stack trace
- [ ] Sentry is capturing errors in both BFF and SPA
- [ ] Daemon health indicator is visible on every page

#### Dependencies and Blockers

- Clerk SDK spike (#1257) must complete before any auth tickets can start — it is the Phase 1 critical path gate
- Event projection layer (B3) depends on schema migrations for matches and draft_sessions with account_id scope (B4, partial)
- API key UX (#1314) blocked by auth middleware (#981)

---

### Milestone 2 — v0.3.0 "Telemetry Parity" — ACTIVE

**Status**: ACTIVE — started 2026-05-07
**Board**: #29 — 31 tickets, all Todo
**Kickoff doc**: `docs/prd/v0.3.0-kickoff.md`
**Required reading**: ADR-012 (gameplay event correlation), ADR-013 (daemon event ordering), ADR-014 (legacy parser extraction) — all in `docs/adr/`

**Spike finding (2026-05-07)**: The daemon log parsers already exist in `internal/mtga/logreader/`. The log parsing work is a refactor + extension, not a rewrite. Revised estimate: **3–4 weeks**, not the >5 weeks originally flagged as a risk in the beta roadmap. The spike (#1501) will confirm no format drift before classifiers begin.

**Theme**: Match the desktop app's data depth in the cloud.

**Duration**: 4 weeks estimated (3–4 with spike confirmation)

**What it accomplishes**: The daemon captures all event types the desktop parser handled — gameplay, deck deltas, collection deltas, quest events. The full domain schema (all tables that require account_id scoping) is migrated. Live draft overlay works with SSE. Win rate views, deck performance, and format stats are live.

**What it does not accomplish**: Monetization, shareable stats pages, SEO content, or any AI features.

#### Key Deliverables

| # | Deliverable | Existing ticket |
|---|---|---|
| 1 | Backport desktop log parser into daemon — all event types captured | #1160–#1164, H7 |
| 2 | Full domain schema migration: matches, decks, draft_picks, card_inventory, game_plays, opponents, notes, quests all scoped by account_id | B4 (new tickets needed) |
| 3 | Event projection layer v2: project all event types into structured tables | B3 expansion |
| 4 | BFF endpoints: DeckPerformance, WinRateTrend, FormatDistribution, DraftAnalytics, RankProgression, ResultBreakdown, Collection | B1 expansion |
| 5 | SSE consumer wired to live draft overlay and match-in-progress views | H1 |
| 6 | BFF pagination, filtering, and sorting standard applied across all list endpoints | H2 |
| 7 | Draft overlay set data current for Edge of Eternities | CS gate 3 |
| 8 | Daemon stale-event tolerance — durable retry queue, no silent drops | H4 |
| 9 | CSP and security headers on CloudFront | H5 |
| 10 | Settings page (user profile, preferences) | PM gap |

#### Exit Gate (all must be true)

- [ ] All 6 CS beta gates pass in internal testing
- [ ] Win rate by deck and format renders correctly for a user with 20+ matches
- [ ] Live draft overlay shows correct card grades for current set without crashing Arena (measured by CS testers)
- [ ] No daemon event drops observed over a 48-hour soak test (gap detection PostHog event count = 0 under normal conditions)
- [ ] All BFF endpoints listed above return data for a connected account
- [ ] CSP headers live on CloudFront production domain

#### v0.3.0-lite Bailout Scope

If game-play projector (#1512), collection delta projector (#1511), and quest projector (#1510 quest branch) are not green by end of Week 2 (approximately 2026-05-21), v0.3.0 scopes down to **v0.3.0-lite**:

- **In scope**: match history, win rates (deck + format), draft analytics, live draft overlay, Settings page, CSP headers
- **Deferred to v0.3.1**: game-play life total history, collection delta projection, quest tracking, RankProgression and ResultBreakdown endpoints

v0.3.0-lite still satisfies the alpha invite gate — testers can validate match accuracy, win rates, and live draft view. Full parity deferred to v0.3.1.

**Decision deadline**: End of Week 2. Ray makes the call.

#### Dependencies and Blockers

- v0.2.0 is CLOSED — all required auth middleware and projection layer v1 are live
- Parser extraction (ADR-014): parsers exist in `internal/mtga/logreader/` — extraction to `pkg/logparse` is 3–4 days of refactor, not a rewrite
- Three ADRs (012, 013, 014) written and accepted — required reading before implementation
- Edge of Eternities set data must be available before draft overlay can be tested against live drafts

---

### Milestone 3 — v0.4.0 "Beta Launch"

**Theme**: Acquire, convert, and retain the first 500 real users.

**Target launch date**: **2026-08-18** (public closed beta — official commitment)
**Waitlist opens**: 2026-06-02 (aligned with Marvel Super Heroes Arena set release)
**Internal stretch target**: 2026-06-26 (all v0.4.0 exit gates green; first waitlist batch released if on track)

**Duration**: 3–4 weeks

**What it accomplishes**: The growth infrastructure is live. Email waitlist is replaced with real sign-up. Shareable stats pages enable organic viral loops. The launch sequence runs.

**What it does not accomplish**: AI draft picks, collection vs. meta gap analysis, deck export (these are Pro features that can ship post-beta as they become feasible for a solo dev), any AI agents/RAG work, or Stripe billing (deferred to GA — beta is free/invite-only).

> **Decision 2026-05-06**: Beta will be free and invite-only. Stripe integration, Stripe Tax, and PostHog revenue events are deferred to post-beta GA. Revisit when preparing for paid launch. Ticket #982 moved to Post-Beta board.

#### Key Deliverables

| # | Deliverable | Existing ticket |
|---|---|---|
| 1 | Email capture / waitlist → real Clerk sign-up flow on vaultmtg.app | #985, Growth P0 |
| 2 | PostHog in SPA — funnels, retention, activation tracking wired | Growth P0 (new ticket) |
| 3 | GA4 verified tracking on vaultmtg.app | Growth P0 (new ticket) |
| 4 | OpenGraph meta tags on vaultmtg.app | Growth P0 (new ticket) |
| 5 | Shareable stats pages (vaultmtg.app/stats/[username]) with OG image preview | Growth P1 (new ticket) |
| 6 | Discord server live with #support and #bugs channels | Growth P0 (no-code) |
| 7 | VaultMTG X account created | Growth P0 (no-code) |
| 8 | In-app bug report link including app version + OS | CS gate 5 |
| 9 | EOE draft tier list article on vaultmtg.app | Growth P1 (new ticket) |
| 10 | sitemap.xml submitted to Google Search Console | Growth P1 |

**Removed from v0.4.0 scope (2026-05-06):**
- Stripe billing integration (#982) — deferred to post-beta GA; beta is free/invite-only
- Stripe Tax — deferred to post-beta GA
- PostHog revenue events — deferred to post-beta GA
- Free vs. paid tier enforcement (#980, #985 billing component) — deferred to post-beta GA

#### Exit Gate (all must be true)

- [ ] 50+ beta testers have connected daemon and tracked at least one match
- [ ] Activation rate (first match tracked within 7 days) ≥ 40% across testers
- [ ] PostHog funnel instrumented and showing data
- [ ] Discord server has at least 1 moderator and a pinned known issues post
- [ ] Shareable stats URL works and renders OG preview on Discord/Reddit

> **Stripe exit gate removed 2026-05-06**: Beta is free/invite-only. Stripe flow will be an exit gate for the GA launch milestone, not v0.4.0 beta.

---

## 4. Beta Gate Definition

**v0.3.0 completion is the internal beta gate. v0.4.0 completion is the public beta launch.**

The distinction matters. Do not invite external users at the end of v0.3.0 — the growth infrastructure is not ready and you will burn your one shot at a first impression with the r/MagicArena community. The right model is:

- End of v0.3.0: invite 10–20 trusted testers (existing contacts, Discord contacts, friends who play Arena) to validate the six CS gates and the draft overlay. This is an alpha validation, not a beta.
- End of v0.4.0: public beta announcement via Reddit/Discord/X.

### Technically Ready (v0.3.0 exit)

All six CS beta gates must pass:

| Gate | Signal |
|---|---|
| Match data accuracy | 0 discrepancies in a 20-match sample vs. Arena client log |
| Account persistence | Data present after re-login on a different device/session |
| Draft overlay | Correct set grades, no Arena crash, works on current set |
| Win rate views | Deck and format win rates render correctly |
| Visible status + bug report | Daemon health indicator visible; bug report link works |
| Arena performance | No measurable FPS degradation with daemon running |

Additional technical gates:
- Sentry capturing errors in both BFF and SPA
- CSP headers live on CloudFront
- Zero P0 open bugs in Sentry

### Financially Ready (v0.4.0 exit)

> **Updated 2026-05-06**: Beta is free/invite-only. Stripe integration deferred to post-beta GA. The financially ready gate for v0.4.0 is simplified to AWS runway only.

| Requirement | Target |
|---|---|
| AWS runway | Credits cover ≥ 6 months at projected load |

**Deferred to GA launch milestone:**
- Stripe integration (Pro tier $6.99/mo) — ticket #982 on Post-Beta board
- Free tier limits enforcement (#980) — depends on Stripe
- Break-even / MRR tracking — revisit when preparing paid launch

Do NOT introduce paid features before 1,000 MAU. Launch Stripe at GA, not beta.

### Growth Ready (v0.4.0 exit)

| Requirement | Target |
|---|---|
| Email / sign-up capture | Clerk sign-up live on landing page |
| Product analytics | PostHog funnel instrumented, activation visible |
| Social infrastructure | Discord + X accounts live, OG tags on all pages |
| Shareable loop | stats/[username] URLs functional with OG preview |
| Launch content | EOE draft tier list article published or scheduled |
| Launch sequence | 2-week post schedule drafted and ready to execute |

---

## 5. v0.2.0 Prioritized Backlog

### P0 — Beta Blockers (nothing ships without these)

These block the v0.2.0 exit gate. If any of these slip, the milestone slips.

| Priority | Item | Ticket | Notes |
|---|---|---|---|
| P0 | Clerk SDK spike + integration plan | #1257 | Unblocks entire auth track |
| P0 | Users table schema + migration | #1310 | Required for account-scoped data |
| P0 | SSM secrets for Clerk keys | #1321 | Required before any Clerk call in BFF |
| P0 | BFF auth middleware (ClerkAuthMiddleware) | #981 | Required before any protected endpoint |
| P0 | Feature gating skeleton | #983 | Required for paid/free enforcement later |
| P0 | API key issuance + revoke + rotate UI | #1314 | Required for daemon onboarding |
| P0 | Daemon onboarding flow (Clerk sign-up → key copy → installer) | NEW | B7 — no existing ticket |
| P0 | Event projection layer v1 (daemon_events → matches + draft_sessions) | NEW | B3 — no existing ticket |
| P0 | BFF MatchHistory endpoint (paginated, account-scoped) | NEW | B1 — no existing ticket |
| P0 | BFF DraftHistory endpoint (paginated, account-scoped) | NEW | B1 — no existing ticket |
| P0 | EmptyState UX for all broken SPA routes | NEW | B5 — no existing ticket |
| P0 | Daemon singleton guard + sync resilience | #1344 | Already on board, parallelize |
| P0 | Sentry in BFF and SPA | NEW | Required before any user-facing traffic |

### P1 — Beta Required (beta quality needs these)

These must ship in v0.2.0 or early v0.3.0 or the product does not meet CS beta gate standards.

| Priority | Item | Ticket | Notes |
|---|---|---|---|
| P1 | Daemon health indicator in SPA (Connected / Syncing / Error) | NEW | CS gate 5 |
| P1 | Daemon log parsing improvements (all event types) | #1160–#1164 | Scope for v0.3.0 |
| P1 | Full domain schema migrations (account_id scoped) | NEW | B4 — multiple tickets needed |
| P1 | BFF pagination + filtering + sorting pattern | NEW | H2 — no existing ticket |
| P1 | SSE consumer wired to draft overlay | NEW | H1 — no existing ticket |
| P1 | WinRateTrend, DeckPerformance, FormatDistribution endpoints | NEW | B1 expansion |
| P1 | Settings / user profile page | NEW | PM gap — no existing ticket |
| P1 | CSP / security headers on CloudFront | NEW | H5 — no existing ticket |
| P1 | Daemon stale-event tolerance / durable retry | NEW | H4 — no existing ticket |

### P2 — Beta Nice-to-Have (ship if time allows in v0.3.0 or v0.4.0)

| Priority | Item | Ticket | Notes |
|---|---|---|---|
| P2 | Email capture / waitlist on vaultmtg.app | #985 | Move to v0.4.0 if needed |
| P2 | GA4 verified tracking | NEW | Growth P0 — move to v0.4.0 |
| P2 | PostHog in SPA | NEW | Growth P0 — move to v0.4.0 |
| P2 | OpenGraph meta tags | NEW | Growth P0 — move to v0.4.0 |
| P2 | Shareable stats pages with OG preview | NEW | Growth P1 — move to v0.4.0 |
| P2 | In-app bug report link | NEW | CS gate 5 complement |
| DEFERRED | Stripe billing ($6.99/mo Pro) | #982 | Post-beta GA — beta is free/invite-only |
| DEFERRED | Free vs. paid tier enforcement | #980 | Post-beta GA — depends on Stripe |
| P2 | EOE draft tier list article | NEW | v0.4.0 |

### Post-Beta (explicitly deferred — do not schedule before 1K MAU confirmed)

| Item | Ticket | Reason deferred |
|---|---|---|
| AI agents infrastructure | #987–#994 | Single engineer; no ROI at <1K MAU |
| MCP server | #995–#998 | Same as above |
| RAG / vector store | RAG tickets | Same as above |
| CI/CD Node.js 24 upgrades | #1296–#1354 | Does not affect user experience |
| AI draft pick recommendations | NEW | Finance says do not build Pro features pre-1K MAU |
| Collection vs. meta gap analysis | NEW | Same |
| Deck export | NEW | Same |
| Price tracking with alerts | NEW | Same |
| Lifetime deal (beta exit) | NEW | Launch at v0.4.0 public beta exit, cap 500 seats |
| Performance monitoring (APM) | NEW | Add after v0.4.0 when load justifies it |
| Load testing (50K MAU capacity) | NEW | Needed before Scale Threshold milestone |
| Security audit / pen test | NEW | Required before paid features go live at scale |
| GDPR / CCPA compliance docs | NEW | Required before EU users invited |
| Account deletion / GDPR export | NEW | Required before GDPR exposure |
| Email verification + password reset | NEW | Clerk handles most of this; validate gaps |
| Public roadmap / status page | NEW | Nice to have at beta; not blocking |
| Incident response playbook | NEW | Write after first production incident |

### New Tickets Flagged (not on v0.2.0 board, must be created)

| Ticket Name | Milestone | Agent Source |
|---|---|---|
| Daemon onboarding end-to-end flow (sign-up → key → installer) | v0.2.0 | Architect B7 |
| Event projection layer v1 (daemon_events → matches, draft_sessions) | v0.2.0 | Architect B3 |
| BFF MatchHistory read endpoint | v0.2.0 | Architect B1 |
| BFF DraftHistory read endpoint | v0.2.0 | Architect B1 |
| EmptyState UX for all broken SPA routes | v0.2.0 | Architect B5, CS |
| Sentry integration (BFF + SPA) | v0.2.0 | Architect, Growth P0 |
| Daemon health indicator in SPA | v0.2.0 | CS gate 5 |
| Full domain schema migrations with account_id scope | v0.3.0 | Architect B4 |
| BFF pagination / filtering / sorting standard | v0.3.0 | Architect H2 |
| SSE consumer wired to draft overlay + live pages | v0.3.0 | Architect H1 |
| BFF WinRateTrend endpoint | v0.3.0 | Architect B1 |
| BFF DeckPerformance endpoint | v0.3.0 | Architect B1 |
| BFF FormatDistribution endpoint | v0.3.0 | Architect B1 |
| BFF DraftAnalytics endpoint | v0.3.0 | Architect B1 |
| BFF RankProgression endpoint | v0.3.0 | Architect B1 |
| BFF ResultBreakdown endpoint | v0.3.0 | Architect B1 |
| BFF Collection endpoint | v0.3.0 | Architect B1 |
| Daemon stale-event tolerance / durable retry queue | v0.3.0 | Architect H4 |
| CSP / security headers on CloudFront | v0.3.0 | Architect H5 |
| Settings / user profile page | v0.3.0 | PM gap |
| Event projection layer v2 (full domain events) | v0.3.0 | Architect B3 |
| PostHog in SPA | v0.4.0 | Growth P0 |
| GA4 verified tracking | v0.4.0 | Growth P0 |
| OpenGraph meta tags on vaultmtg.app | v0.4.0 | Growth P0 |
| Shareable stats pages with OG image (stats/[username]) | v0.4.0 | Growth P1 |
| In-app bug report link (version + OS included) | v0.4.0 | CS gate 5 |
| EOE draft tier list article | v0.4.0 | Growth P1 |
| sitemap.xml submitted to GSC | v0.4.0 | Growth P1 |

---

## 6. Key Risks & Mitigations

| # | Risk | Likelihood | Impact | Mitigation | Owner |
|---|---|---|---|---|---|
| 1 | **Event projection complexity** — **RISK REDUCED (2026-05-07)**. Spike finding: parsers already exist in `internal/mtga/logreader/`. Work is refactor + extension, estimate 3–4 weeks not >5. v0.3.0-lite bailout defined: if game-play/collection/quest projectors not green by end of Week 2 (~2026-05-21), scope down to match history + win rates + draft analytics only; defer remainder to v0.3.1. | ~~High~~ Low | Medium | Bailout trigger and scope documented in `docs/prd/v0.3.0-kickoff.md`. | Ray (eng) |
| 2 | **Single engineer burnout / velocity collapse** — 14–15 weeks of sustained solo work with no slack will degrade quality and increase bug density. | Medium | High | v0.2.0 scope is the most critical to protect. If v0.2.0 slips, cut P1 items before adding scope. Build in 1 week of buffer at the end of each milestone. Do not start v0.4.0 growth work until v0.3.0 exit gates are met. | Ray (PM + eng) |
| 3 | **Draft overlay causes Arena performance issues** — CS flagged this as a beta failure mode. If the daemon or SPA overlay introduces FPS drops, draft users (the highest-value segment) will uninstall immediately. | Medium | High | Add an explicit performance gate to v0.3.0 exit criteria. Run a 48-hour soak test with Arena + daemon before inviting any external testers. Disable overlay at first sign of FPS impact. | Ray (eng) |
| 4 | **Competitor pre-empts beta positioning** — Untapped.gg or a new entrant launches an "all-in-one" MTGA companion during the build window. | Low | Medium | Do not delay beta to add features that compete with existing Untapped functionality. Ship with the draft-grinder niche (narrower than Untapped's full suite) and iterate. Our differentiation is integrated lifecycle, not data volume. | PM |
| 5 | **AWS Activate credits expire before monetization traction** — credits expire May 2027. If 1,000 MAU and Stripe are not live by then, Ray pays ~$44–$96/mo out of pocket. | Low | Low | Credits at ≤1K MAU burn rate easily cover 12 months. Risk only materializes if beta is delayed past May 2027 or scale reaches 5K MAU before monetization. Monitor monthly; trigger Stripe setup alert at 800 MAU if not already live. | Finance |

---

## 7. Open Questions

These need a decision before engineering starts the relevant phase. No ticket should be opened for a blocked area until the question is resolved.

| # | Question | Blocks | Deadline | Status |
|---|---|---|---|---|
| 1 | **Free tier limits**: What exactly is gated behind Pro on day 1 of beta? The Finance report proposes limits (30-day history, 10 deck slots, weekly meta snapshot) but these need engineering sign-off on feasibility of enforcement at the BFF level before #980 and #983 are scoped. | v0.4.0 Stripe, #980, #983 | Before v0.3.0 starts | ✅ Resolved |
| 2 | **Desktop-to-cloud data migration**: Do early beta users who ran the desktop app expect their historical data to appear in the cloud product? | v0.2.0 onboarding flow | Before v0.2.0 starts | ✅ Resolved |
| 3 | **Draft overlay architecture**: Does the live draft overlay run as a browser extension, an Electron shell, or a desktop overlay injected by the daemon? This decision affects SSE-to-overlay path complexity and Arena performance risk. | v0.3.0 H1, SSE work | Before v0.3.0 starts | ✅ Resolved |
| 4 | **Beta access model**: Invite-only, open waitlist drain, or fully open? CS recommends gating on onboarding success rate rather than feature count. | v0.4.0 launch sequence | Before v0.4.0 starts | ✅ Resolved |
| 5 | **Email provider confirmation**: Growth Marketing recommends Resend (3K emails/mo free) after SendGrid killed their free tier in May 2025. | v0.4.0 email capture | Before v0.4.0 starts | ✅ Resolved |

### Resolved Answers

**Q1 — Free tier limits** *(resolved 2026-05-06 by architect)*

All three proposed limits are **feasible at the BFF layer**. Engineering sign-off granted with the following verdicts:

| Limit | Verdict | Effort | Mechanism |
|---|---|---|---|
| 30-day match history | **CONDITIONAL YES** | S | Query-layer `WHERE played_at > NOW() - 30 days`; `tier.IsPro(ctx)` injects nil window for Pro |
| 10 deck slot cap | **CONDITIONAL YES** | S | Pre-insert `COUNT` check in handler; wrap in `SERIALIZABLE` tx to prevent TOCTOU race |
| Weekly meta snapshot | **YES** | XS | Two cache rows per snapshot (`tier='free'` weekly, `tier='pro'` daily); handler picks by caller tier |

The "conditional" on limits 1 and 2 means both depend on `#980` establishing the tier source. **Recommendation: `users.tier TEXT DEFAULT 'free'`** column populated by a Clerk webhook — not Clerk metadata on every request (adds a roundtrip per read).

**Foundational work required before `#983` can ship:**
1. Add `users.tier` column + Clerk webhook at `POST /api/v1/clerk/webhook` that updates tier on subscription events (part of `#980`)
2. Extend `ClerkUserResolver` to load tier in the same upsert query → `TierFromContext(ctx)` helper
3. `#983` ships two primitives: `RequireTier("pro")` middleware (whole-endpoint gate) + `tier.IsPro(ctx)` in-handler helper (response-shaping gate) — **not** the per-feature enforcement
4. Per-feature enforcement is 3 separate S-sized follow-on tickets after `#983` lands

**Key gotchas:**
- Match aggregates (win rate, streak stats) must use the same 30-day window as the list — or free users see a 60% win rate "over 200 matches" with only 30 visible
- Daemon ingest is **never gated** — free users accumulate all matches, they just can't see >30d ones (preserves the "upgrade to unlock 247 hidden matches" upsell)
- Deck cap: use HTTP 402 with stable `error_code: "deck_limit_reached"` so the frontend shows an upgrade modal, not a generic error
- Pro→Free downgrade: never auto-delete data; block new inserts only and show a "N decks hidden" banner

**Revised ticket breakdown for `#980` + `#983` + 4 follow-ons:**
- `#980`: users.tier column, Clerk webhook handler, ClerkUserResolver tier load, `GET /api/v1/me` returning tier + limits object
- `#983`: tier package (`IsPro`, `Limits`, `RequireTier`), 402 error envelope with `upgrade_url`
- Follow-on 1: Enforce 30-day match history window (backend-engineer, S)
- Follow-on 2: Enforce 10-deck cap with SERIALIZABLE guard (backend-engineer, S)
- Follow-on 3: Per-tier meta snapshot generation (backend-engineer, S — touches sync + BFF)
- Follow-on 4: Frontend tier-aware upgrade prompts on matches/decks/meta (front-engineer, M)

**Q2 — Desktop-to-cloud data migration** *(resolved 2026-05-06)*
The desktop app had only one user (Ray Hamilton). No user data migration is needed. The v0.2.0 onboarding flow must include a clear "fresh start" message so any future early tester understands VaultMTG is a new account with data going forward from installation. No migration ticket required.

**Q5 — Email provider** *(resolved 2026-05-06)*
**Resend** is confirmed as the transactional email provider. 3,000 emails/month free tier, modern API with React Email support, SPF/DKIM setup required on `vaultmtg.app`. SendGrid is explicitly ruled out (killed free tier May 2025). Scope the waitlist/transactional email ticket against Resend.

**Q3 — Draft overlay architecture** *(resolved 2026-05-06 by architect)*
**Decision: Option C — Daemon + SPA in a browser window. No overlay injection.**

Reasoning: Option A (browser extension) is dead — Arena has no web client. Option B (Electron overlay) is disqualifying for a solo dev: always-on-top transparent windows over fullscreen DirectX/Metal cause FPS drops, focus-stealing, anti-cheat false positives, and Mac Metal compositor crashes — exactly the CS failure mode. It also requires code-signing certs ($300+/yr Apple Developer + Windows EV), a second installer, and a second auto-updater. Option C reuses 100% of the existing stack with zero Arena performance risk.

The "draft overlay" gate in the CS report becomes "live draft view" at `app.vaultmtg.app/draft/live`. This is the same UX pattern 17Lands LiveTracker uses (browser tab alongside Arena). Data accuracy and grade-correctness remain fully testable.

**v0.3.0 implementation tickets required (to be created by project-manager):**
- F1: `useDraftEventStream` SSE consumer hook with reconnect/backoff
- F2: `/draft/live` page rendering current pack with card grades and pick recommendation
- F3: EventSource auth spike — cookie-based Clerk session on `/api/v1/events` (EventSource cannot send `Authorization` header)
- F4: Draft session state machine (pack/pick number, picked cards, reset on `draft.started`/`draft.ended`)
- B1: Verify `IngestHandler` publishes `draft.pack`/`draft.pick` to SSE broker per-user
- OPS1: Verify `proxy_read_timeout >= 60s` on nginx for `/api/v1/events`
- DOC1: User docs — "Open /draft/live in a second window while drafting"

ADR-010 to be written by architect agent.

**Q4 — Beta access model** *(resolved 2026-05-06 by CS + Growth Marketing)*
**Decision: Option B — Open waitlist drain, paced by onboarding success rate.**

CS says invite-only (25 first batch, 80% gate). Growth says waitlist drain (50/batch, 65% gate). Synthesis: **waitlist drain with CS's tighter gate**.

- Waitlist form live on `vaultmtg.app` from v0.4.0 launch day — email capture before Clerk sign-up opens
- Visible position counter ("X people ahead of you") to generate demand signal
- Target 500 waitlist signups before draining begins
- First batch: 25–50 users, weighted toward draft players and competitive ladder players
- Drain gate: 80% of prior batch must have a confirmed daemon connection + at least 1 match logged before next batch is released
- Critical prerequisite: **daemon telemetry must emit a "first successful connection" ping** — without it the 80% gate cannot be measured
- Pause drain and diagnose if success rate drops below 80%
- Do **not** build a custom invite system — manual Clerk invitations + a "you're in" Resend email is sufficient until this process is fully automated

Invite-only is rejected: MTG Arena community expects free/immediately-available tools; invite friction generates negative Reddit posts. Fully open is rejected: solo founder + broken daemon onboarding + Reddit memory = reputation risk that is hard to recover from.

---

## Appendix: RICE Scores for Milestone Ordering

| Initiative | Reach (users/qtr) | Impact | Confidence | Effort (pw) | Score | Decision |
|---|---|---|---|---|---|---|
| Auth + API key onboarding | 500 | 3 (enabling) | 90% | 3 | 450 | P0, v0.2.0 |
| Event projection layer | 500 | 3 (enabling) | 70% | 4 | 263 | P0, v0.2.0 |
| EmptyState / error UX | 500 | 2 | 95% | 0.5 | 1900 | P0, v0.2.0 (low effort, high impact ratio) |
| Sentry error monitoring | 500 | 2 | 99% | 0.5 | 1980 | P0, v0.2.0 |
| Full telemetry parity (v0.3.0) | 2000 | 3 | 65% | 6 | 650 | v0.3.0 |
| Shareable stats pages | 2000 | 2 | 75% | 2 | 1500 | v0.4.0 |
| Stripe / paid tier | 1000 | 2 | 85% | 3 | 567 | v0.4.0 |
| AI agents / RAG | 5000 | 2 | 40% | 12 | 333 | Post-beta |

Note: Auth and projection layer scores are suppressed by confidence and effort relative to quick wins (Sentry, EmptyState). They ship first because they are enabling — nothing else works without them. The RICE scores for quick wins are inflated by low effort; their high scores reflect correct sequencing (ship them fast alongside the blockers).
