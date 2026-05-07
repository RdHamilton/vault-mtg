# Product Manager Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — [Initiative name]
**Triggered by**: [CS feedback / BA report / Finance alert / user request]
**Decision**: [what was prioritized and why]
**Output**: [PRD filename or ticket numbers created]
**RICE score**: [if applicable]
-->

## 2026-05-07 — v0.3.0 ADR gap analysis, kickoff doc, and PRD update
**Triggered by**: User request — three-task v0.3.0 launch sequence
**Decision**: (1) Read ADRs 012/013/014 and existing tickets #1501-#1517. Found 6 uncovered ADR requirements: GRE flush threshold config, game_plays partial column, daemon_events sequence column, BFF gap detection, desktop import path update, CI pipeline update. Created tickets #1519-#1524 and added to board #29. (2) Wrote v0.3.0 kickoff doc covering all 31 tickets in 4 waves with user stories, ACs, exit gates, Week 2 bailout trigger, and v0.3.0-lite scope. (3) Updated beta-roadmap.md: v0.2.0 closed 2026-05-07, v0.3.0 active with 31 tickets, spike estimate revised to 3-4 weeks, ADR references added, v0.3.0-lite bailout scope added, Stripe deferral confirmed.
**Output**: docs/prd/v0.3.0-kickoff.md, docs/prd/beta-roadmap.md (updated). GitHub issues #1519-#1524 on board #29. PR #1526.
**RICE score**: N/A — planning/documentation work

## 2026-05-07 — v0.3.0 Telemetry Parity full backlog creation
**Triggered by**: User request — create ~15 missing tickets from spike report and add to board #29
**Decision**: Created 17 tickets covering the full v0.3.0 scope: log sample spike, parser extraction (ADR-014), account_id schema retrofit (5 tables), 5 daemon classifiers (inventory/quest/collection/deck/match), sequence contract field (ADR-013), 4 BFF projection layer v2 tickets, 2 analytics endpoint groups, Settings page frontend, pagination standard, and CloudFront security headers.
**Output**: GitHub issues #1501–#1517, all added to board #29 as Todo with milestone v0.3.0 (#69)
**RICE score**: N/A — full milestone backlog build

## 2026-05-07 — v0.2.0 Close and v0.3.0 Kickoff
**Triggered by**: User request — sequence of three tasks: deferred onboarding tickets, close v0.2.0, launch v0.3.0
**Decision**: Moved #1398 (daemon onboarding flow) and #1314 (API key UX) from v0.2.0 to v0.4.0 milestone — both require Clerk Pro ($25/mo) decision deferred to 2026-05-09. With these two deferred, gates 2-6 of v0.2.0 are all satisfied. Declared v0.2.0 DONE. Closed v0.2.0 milestone #67. Created v0.3.0 milestone #69 (Telemetry Parity). Board #29 already existed with 7 SSE/live-draft tickets; assigned v0.3.0 milestone to all 7. Also moved #1495 (staging EnvBadge test) from v0.2.0 to v0.3.0.
**Output**: #1398 and #1314 moved to v0.4.0 milestone and added to board #30. Milestone #67 (v0.2.0) closed. Milestone #69 (v0.3.0) created. Board #29 (v0.3.0) confirmed with 7 tickets assigned v0.3.0 milestone.
**RICE score**: N/A — wave management

## 2026-05-07 — v0.2.0 Board #28 Cleanup
**Triggered by**: User request — board had 19 no-status items creating noise
**Decision**: Classified all 19 no-status items by milestone and state. Closed issues from old Phase 3/4 milestones moved to Done (they were complete work). Open issues from wrong milestones (Pre-Phase, Phase 2, Phase 5, v2.0, no milestone) removed from board entirely. Also moved 7 closed-issue PR Review items to Done, plus #1459 which was missed by a prior pipeline run.
**Output**: 14 items removed, 13 items moved to Done (5 no-status closed + 7 PR Review closed + 1 from prior pipeline). Board reduced from 69 to 55 items. Final composition: Done 46, Todo 5, In Progress 2, PR Review 2. Zero no-status items remain.
**RICE score**: N/A — board hygiene

## 2026-05-07 — v0.2.0 Board Re-verification
**Triggered by**: User request — previous status check was stale (65% with Clerk Pro blocker)
**Decision**: Re-queried board #28 directly. Confirmed #1314 and #1398 are NOT on the board (moved to v0.4.0 as user stated). Real completion is 54% excluding No Status noise (27/50), or 66-70% once 6 merged-but-stuck PR Review tickets are moved to Done. All remaining work is unblocked: 6 staging tail tickets + 4 testing debt tickets + 2 open PRs (#1354, #1410). No decision gates remain.
**Output**: Status rollup delivered in conversation. Board hygiene actions identified (move 6 merged PRs to Done, triage 19 No Status items).
**RICE score**: N/A — status report

## 2026-05-07 — Pre-beta security audit gate issue
**Triggered by**: User request — hard release gate required before v0.4.0
**Decision**: Created blocking security audit ticket covering 6 scope areas (PostHog PII, Clerk integration, M2M daemon auth, BFF route exposure, frontend bundle audit, CVE scan). Zero High/Critical findings required to release.
**Output**: GitHub issue #1488 — added to board #30 (v0.4.0) as Todo; labels: security, v0.4.0, blocked-release; milestone: v0.4.0
**RICE score**: N/A — mandatory gate, not a prioritized feature

## 2026-05-07 — v0.2.0 Status Report
**Triggered by**: User request — milestone status update
**Decision**: Synthesized issue labels, PR merge history, and kickoff doc. 3 of 6 exit gates satisfied. Release blocked on Clerk Pro decision (2026-05-09) → #1314 → #1398 onboarding flow. Testing debt in #1459, #1460, #1458 is non-blocking. #1410 needs manual close (PR merged, issue still open).
**Output**: Status report delivered in conversation. No files written.
**RICE score**: N/A — status report

## 2026-05-07 — Wave Status Rollup (v0.2.0, v0.4.0, Post-Beta)
**Triggered by**: User request — full wave status rollup across boards 28, 30, 31
**Decision**: v0.2.0 wave close is NO-GO — 10 Todo tickets remain (testing, docs, CI bug #1458, PostHog funnel #1410). v0.4.0 has no in-progress tickets; critical path gated on Clerk Pro decision (2026-05-09) and pricing tier sign-off. Installer Waves A-D are unblocked and should start. #1467-#1473 and #1465 need v0.4.0 milestone assignment.
**Output**: Status rollup delivered in conversation. No files written.
**RICE score**: N/A — rollup task

## 2026-05-06 — Pre-Beta Tooling Checklist
**Triggered by**: User request — synthesize tooling needs from all four business agents
**Decision**: Triaged all BA, CS, Finance, and Growth Marketing tooling needs into Must-Have Day 1 vs Nice-to-Have. Created 5 engineering tickets (#1477-#1481) and 1 master checklist issue (#1482). Stripe integration blocked on Ray's paid/free beta decision — no Stripe ticket created. All issues added to v0.4.0 milestone (#68). Board placement pending GraphQL rate limit reset.
**Output**: GitHub issues #1477 (PostHog event schema), #1478 (GA4+OG tags), #1479 (GSC verification), #1480 (Sentry User Feedback), #1481 (Session Replay), #1482 (tooling checklist). All milestone v0.4.0.
**RICE score**: N/A — pre-beta operational readiness, sequenced by launch dependency not RICE

## 2026-05-06 — Beta monetization deferral: Stripe deferred to GA
**Triggered by**: User decision — beta will be free/invite-only
**Decision**: Stripe integration (#982), Stripe Tax, free vs. paid tier enforcement (#980), and PostHog revenue events removed from all beta milestones. Beta (v0.2.0–v0.4.0) is free and invite-only. Stripe deferred to post-beta GA milestone, to be revisited when 1,000 MAU is confirmed.
**Output**: Updated docs/prd/beta-roadmap.md — removed Stripe from v0.4.0 deliverables, exit gates, and financially ready section; added deferral notes. Comments posted on #982 and #980. Tickets being moved to Post-Beta board #31.
**RICE score**: N/A — policy decision

## 2026-05-06 — v0.2.0 Business Track issues + status rollup
**Triggered by**: User request
**Decision**: Created 4 business-track tickets covering waitlist signup (#1409), PostHog activation funnel (#1410), support infrastructure (#1411), and AWS cost modelling (#1412). All added to board #28 as Todo. Also produced v0.2.0 status rollup.
**Output**: GitHub issues #1409, #1410, #1411, #1412 on board #28 (Todo)
**RICE score**: N/A — operational/GTM work mandated by beta readiness requirements

## 2026-05-06 — v0.2.0 Kickoff: P0 backlog review and user stories
**Triggered by**: User request — v0.2.0 sprint start
**Decision**: Confirmed P0 backlog in dependency wave order. Elevated daemon health indicator from P1 to P0 (named in exit gate). Confirmed #983 is out of v0.2.0 scope (tier enforcement is v0.4.0). Identified schema migration as a missing ticket that is a hard blocker for B3. Noted three PM action items: (1) Ray confirms daemon installer is publicly hosted, (2) Ray creates Sentry account and stores DSN in SSM, (3) project-manager confirms board #28 composition.
**Output**: docs/prd/v0.2.0-kickoff.md — full user stories with ACs for B3, B5, B7, health indicator, Sentry, schema migration, MatchHistory endpoint, DraftHistory endpoint. Confirmed 4-wave execution order.
**RICE score**: Health indicator: P0 (exit gate); EmptyState: 1900; Sentry: 1980; Projection layer: 263 (enabling)

## 2026-05-06 — Beta Roadmap PRD
**Triggered by**: Synthesis of 6 specialist agent reports (Architect, PM, CS, BA, Finance, Growth Marketing)
**Decision**: Defined 3-milestone roadmap (v0.2.0 Foundation → v0.3.0 Telemetry Parity → v0.4.0 Beta Launch). v0.3.0 is the internal beta gate; v0.4.0 is public beta. AI agents and RAG infrastructure explicitly deferred post-beta. Do not introduce Stripe before 1,000 MAU.
**Output**: docs/prd/beta-roadmap.md
**RICE score**: Auth+onboarding: 450 | EmptyState: 1900 | Sentry: 1980 | Full telemetry: 650 | Shareable stats: 1500 | AI agents: 333 (deferred)
