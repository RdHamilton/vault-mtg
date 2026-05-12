# Product Manager Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — [Initiative name]
**Triggered by**: [CS feedback / BA report / Finance alert / user request]
**Decision**: [what was prioritized and why]
**Output**: [PRD filename or ticket numbers created]
**RICE score**: [if applicable]
-->

## 2026-05-11 — Wave 9 pre-beta hardening: first two tickets filed
**Triggered by**: User request — file daemon uninstall + daemon bug reporting tickets ahead of Wave 9 kickoff
**Decision**: Created Wave 9 milestone (#72 — "Wave 9 — Pre-Beta Hardening") and new labels (`wave-9`, `pre-beta`, `observability`). Filed two tickets pre-kickoff:
  - #1831 — bundled daemon uninstall.sh for macOS .pkg + Windows NSIS verification (infrastructure-owned)
  - #1832 — Sentry crash capture in Go daemon + Copy Diagnostics button in SPA settings (backend + frontend co-owned)
Both decisions are locked: uninstall is script-only (no daemon subcommand, no SPA button); bug reporting is Sentry + clipboard JSON only (no in-app feedback form — Discord/email remains channel). Wave 9 kickoff doc + board placement deferred — tickets are tagged `wave-9` and will roll into the kickoff doc when it lands.
**Output**:
  - GitHub issues #1831, #1832 on milestone #72
  - docs/product/milestones/wave-9/project-manager-instructions.md (canonical spec for future kickoff doc)
  - New labels: wave-9, pre-beta, observability
**RICE score**: N/A — pre-beta operational hardening, sequenced by closed beta gate (2026-08-18), not RICE

## 2026-05-10 — v0.3.2 ticket list finalized after Ray's 6 decisions
**Triggered by**: User request — Ray resolved all 6 blocking decisions for the rename milestone
**Decision**: Updated ticket-list.md to reflect final scope: 34 tickets (was 31), spanning 8 waves (Wave 0, 0.5, 1, 2, 3, 4, 5, 6). Key changes: (1) Repo rename moved out of Wave 6 into a new Wave 0.5 sequenced before all downstream waves so docs/code/CI reference the canonical new URL from the start. (2) Companion repo tickets added: V32-W3-6 for `mtga-companion-infra` and V32-W5-4 for `mtga-companion-web`. (3) DB rename ACs trimmed — no maintenance window per Ray (no users yet). (4) 17lands ticket V32-W1-6 description clarified — rename only, no outreach. (5) rhamiltoneng CFN tag rename to `vaultmtg` made explicit in V32-W3-5 (no portfolio-specific alternative). (6) New ticket V32-W5-5 for PostHog event rename `mtga_companion.*` → `vaultmtg.*` with no continuity preservation. Wave 6 is now 3 tickets (DB rename, GONOSUMDB workflow update, final sweep) — repo rename gone from there.
**Output**: `docs/product/milestones/v0.3.2/ticket-list.md` (updated, 34 tickets), `docs/product/milestones/v0.3.2/project-manager-instructions.md` (updated handoff brief with the 6 decisions baked in)
**RICE score**: N/A — milestone scoping
**Handoff status**: PM Najah cannot dispatch the project-manager agent directly (no Agent/Task tool in this session's function list). Handoff brief at `docs/product/milestones/v0.3.2/project-manager-instructions.md` is ready — Ray (or main session) needs to invoke project-manager pointing to that brief.

## 2026-05-10 — v0.3.2 milestone setup (MTGA-Companion → VaultMTG rename)
**Triggered by**: User request — orchestrate full v0.3.2 milestone (project, tickets, arch review, PRD) from architect's rename audit + Ray's 4 decisions (VaultMTG casing, repo rename yes, DB rename yes, archive rewrite).
**Decision**: Created GitHub Project #34 ("v0.3.2 — mtga-companion rename", `PVT_kwHOABsZ684BXSA8`) and milestone #71 (v0.3.2). Synthesized 31 tickets across 6 waves from the 1,134-match audit. ADR-021 gates all work. Wave 1 (8 docs/strings) and Wave 2 (5 code) parallel-safe; Wave 3 (5 SSM/EC2) and Wave 4 (5 daemon — highest user risk) parallel-safe; Wave 5 (3 S3/Vercel) → Wave 6 (4 repo+DB+CI). Wrote arch-review.md flagging 7 risks: highest are (1) two daemons running simultaneously after macOS migration bug, (2) SSM cutover ordering, (3) Go module path mismatch with not-yet-renamed repo (recommended moving repo rename to Wave 0.5). PRD documents 6 user stories, 10 risks/mitigations, 3-week wall-clock estimate fitting before 2026-08-18 closed beta. Did NOT create GitHub issues directly (per rule #13) — handoff to project-manager prepared.
**Output**: docs/product/milestones/v0.3.2/ticket-list.md, arch-review.md, prd.md, project-manager-instructions.md. Project #34, milestone #71 created on GitHub.
**RICE score**: N/A — operational rename, not feature work

## 2026-05-09 — v0.3.1 docs PR + Project #33 ticket migration
**Triggered by**: User request — PR docs changes and move Storybook/Playwright tickets
**Decision**: Opened PR #1660 for docs/product/beta-roadmap.md v0.3.1 sequencing update (docs-only, no LE review). Moved Storybook issues #1621, #1622, #1625 from Project #30 to Project #33 (PVT_kwHOABsZ684BXMn-). No open issues with "playwright" in title found.
**Output**: PR #1660 opened; issues #1621, #1622, #1625 added to Project #33 and removed from Project #30
**RICE score**: N/A — operational/housekeeping

## 2026-05-09 — v0.4.0 Alignment Audit (Three-Source Gap Report)
**Triggered by**: User request — alignment audit across beta-roadmap.md, kickoff.md, Board #30
**Decision**: 9 gap categories identified. 4 Wave 0 tickets TBD + #1041 not on board; 8 Wave 1 eng items missing from board; all 5 Storybook tickets absent; 6 business-track tickets stranded at NO STATUS; 16 out-of-scope tickets polluting board; roadmap/kickoff contradiction on shareable stats exit gate.
**Output**: `docs/product/milestones/v0.4.0/alignment-gap-report.md` — prioritized corrective action list, P0–P3
**RICE score**: N/A — audit task

## 2026-05-08 — v0.3.0 Post-Mortem
**Triggered by**: User request — candid retrospective of v0.3.0 release failures
**Decision**: Wrote comprehensive post-mortem naming every failure honestly: 9/31 tickets open, 2/14 exit gates verified, CI red at release time, staging environment never validated end-to-end, rogue agent incident, wave-close report written prematurely. Identified 4 root cause categories (process, technical, agent, communication) and 18 numbered recommendations. 10 action items tied to specific owners and tickets.
**Output**: docs/reports/0.3.0-post-mortem.md
**RICE score**: N/A — retrospective document

## 2026-05-08 — Smart Craft Next ML feature research + PRD
**Triggered by**: User request — find best free-tier ML opportunity for v0.4.0 targeting 50K MAU
**Decision**: Recommended "Smart Craft Next" (personalized cards-to-craft ranking) over draft pick grades, post-match analysis, and opponent prediction. Draft grades are table stakes (17lands open-source tool already covers them). Smart Craft Next is the only feature that closes the match-data feedback loop with the user's own win history — a differentiation none of the competitors have in a free tier. Implementation is zero per-request inference (nightly Lambda batch + static table read). RICE score: 6,000.
**Output**: docs/prd/0005-smart-craft-next.md
**RICE score**: Reach 8K, Impact 3, Confidence 75%, Effort 3pw → Score 6,000

## 2026-05-09 — v0.3.0 Wave Close
**Triggered by**: Ray — all tickets merged, requested finalized wave-close report and GO/NO-GO
**Decision**: GO issued. All 10 required tickets merged, CI green on main. #1517 deferred to v0.4.0. Gates 11/12/14 require manual verification before beta invites.
**Output**: docs/prd/v0.3.0-wave-close.md created; docs/prd/v0.3.0-kickoff.md checkboxes ticked; PR #1611 opened to main

## 2026-05-09 — v0.3.0 Wave-Close Report (Draft)
**Triggered by**: Ray request — draft wave-close report while #1514 and CI fix (PR #1607) are still in flight
**Decision**: Produced draft wave-close report with all 8 completed tickets AC-verified, #1514 and #1607 marked PENDING, #1517 recorded as deferred to v0.4.0, soak gates 8 & 9 moved to beta monitoring period per 2026-05-09 decision. GO withheld pending #1514 merge and CI green on main.
**Output**: docs/prd/v0.3.0-wave-close-draft.md
**RICE score**: N/A — ceremony deliverable

## 2026-05-08 — v0.3.0 Post-Mortem
**Triggered by**: User request — candid retrospective of v0.3.0 release failures
**Decision**: Wrote comprehensive post-mortem naming every failure honestly: 9/31 tickets open, 2/14 exit gates verified, CI red at release time, staging environment never validated end-to-end, rogue agent incident, wave-close report written prematurely. Identified 4 root cause categories (process, technical, agent, communication) and 18 numbered recommendations. 10 action items tied to specific owners and tickets.
**Output**: docs/reports/0.3.0-post-mortem.md
**RICE score**: N/A — retrospective document

## 2026-05-08 — Smart Craft Next ML feature research + PRD
**Triggered by**: User request — find best free-tier ML opportunity for v0.4.0 targeting 50K MAU
**Decision**: Recommended "Smart Craft Next" (personalized cards-to-craft ranking) over draft pick grades, post-match analysis, and opponent prediction. Draft grades are table stakes (17lands open-source tool already covers them). Smart Craft Next is the only feature that closes the match-data feedback loop with the user's own win history — a differentiation none of the competitors have in a free tier. Implementation is zero per-request inference (nightly Lambda batch + static table read). RICE score: 6,000.
**Output**: docs/prd/0005-smart-craft-next.md
**RICE score**: Reach 8K, Impact 3, Confidence 75%, Effort 3pw → Score 6,000

## 2026-05-08 — v0.3.0 Close + v0.4.0 Kickoff
**Triggered by**: User request — formal wave close and next milestone kickoff
**Decision**: v0.3.0 formally closed (22/22 Wave 0-3 tickets Done). v0.3.0 release tag blocked — CI is red on main (Playwright AbortSignal / CI pipeline for pkg/logparse not yet fixed). Tag will be cut when #1524 lands. v0.4.0 kickoff issued: 12 engineering tickets + 6 business-track tickets on board #30. Critical path: #1524 → #1516 → #1513/#1514 → #1488 → release. Closed beta target: August 18, 2026.
**Output**: docs/prd/v0.4.0-kickoff.md. Board #30 populated with 23 total items. Board #32 (v0.5.0) created. Business-track issues #1576-#1581 created. Kickoff tracking issue #1582 posted.
**RICE score**: N/A — wave management

## 2026-05-08 — PM review of X beta announcement schedule
**Triggered by**: User request — review 3 X posts at docs/marketing/content/2026-05-beta-x-schedule.md
**Decision**: All 3 posts REQUEST CHANGES. Post 1 and Post 2 fabricated unshipped features (live pick grades, archetype analysis, win-rate-by-archetype). Post 3 invented "200+ early players" social proof and was scheduled 3 months before closed beta launch. Rewrote all 3 in place using only confirmed v0.3.0 features (SSE draft streaming, live draft view, pick history, real-time pack display). Rescheduled Post 3 to 2026-08-18 (closed beta launch day). Added PM review block at top of file.
**Output**: docs/marketing/content/2026-05-beta-x-schedule.md (revised in place with PM review block)
**RICE score**: N/A — content review

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
