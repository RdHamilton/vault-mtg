---
title: MTGA Companion — Next Sprint Plan
status: active
created: 2026-05-05
updated: 2026-05-05
owner: architect
companion-to: ~/.claude/plans/mtga-companion-aws-launch.md
---

## 0. Infrastructure Budget Update

**AWS Activate credits approved: $1,000** (as of 2026-05-05)
This removes cost as a blocker for infrastructure improvements. Candidates unlocked:
- ElastiCache (Redis) for BFF caching layer
- CloudFront CDN in front of Vercel/EC2
- SES for transactional email (monetization/auth flows)
- RDS storage scaling without concern
- Additional Lambda concurrency for sync jobs

## 1. What's Done (last sprint, merged to main)

### Frontend (Phase 1 close-out — milestone 55)
- #1134 SSE client replaced dead WebSocket — real-time events live (PR #1218)
- #1140 Color ratings render from BFF draft-ratings response (PR #1219)
- #1143 Replay UI stubs removed
- #1139 Authorization header injected at adapter layer (PR #1150)

### Backend / Sync (Phase 2 — milestone 56)
- #1122 Lambda RDS IAM token auth — static DATABASE_URL eliminated (PR #1216)
- #1128 Lambda DLQ + retries
- #1062 Lambda zip deploy replaces sync systemd unit (PR #1217)
- #1063 EventBridge Scheduler rules created
- #1064 Lambda VPC + RDS access wired
- #1126-A/B/C `daemon_events` migration + repo + ingest persistence (#1171/#1172/#1173 closed)
- #1141 SSE endpoint no longer falls back to unguarded path
- #1138 SSE heartbeat shipped

### Infrastructure
- #1179 Vercel deploys scoped to `frontend/` path changes
- #1181 ALLOWED_ORIGINS on EC2 (ADR-006)
- #1182 VITE_BFF_URL on Vercel (ADR-006)

### ADRs in flight / accepted
- ADR-001..006 accepted; **ADR-007 (frontend serving model)** still required before any further frontend deploy work — Vercel vs EC2 nginx conflict unresolved.

---

## 2. Open Issue Map (50 open)

### A. Phase 1 close-out (architecture foundation, milestone 55) — should be the next focus
- **#1126** parent (event persistence) — sub-tickets #1171/#1172/#1173 all closed; **parent issue can be closed by PM**.
- **#1099** Store iface for hash read/write
- **#1100** Lambda hash-check delta skip
- **#1101** Document delta sync in README
- **#1071** Daemon platform install scripts (PowerShell + launchd) — note: changelog says #1094 already shipped these as "daemon issue 1094"; **PM must verify and close #1071 if duplicate**.
- **#975** Daemon auto-update version check

### B. Phase 2 reliability gaps (AWS deployment, milestone 56)
- **#1127** ADR-005 delta sync (payload hashing) — open; depends on #1099/#1100/#1101
- **#1123** Lambda missing QuickDraft + Sealed coverage — Wave 3 entry says shipped; **verify and close**.
- **#1066** services/sync README rewrite for Lambda model
- **#979** Smoke test full stack at domain — blocked on Ray buying domain

### C. Daemon Player.log parsing (v2.0 milestone)
- **#1160** inventory.updated
- **#1161** quest.progress / quest.completed
- **#1162** collection.updated
- **#1163** deck.updated
- **#1164** match.game_started / match.game_ended

These are siblings, all daemon work, all P1+ for the v2.0 cloud SaaS data pipeline. Each is Sonnet-ready (single parser file + tests).

### D. No-milestone debt
- **#1211** infra: remove duplicate "Deploy React SPA to EC2" workflow — depends on ADR-007
- **#1169** prod EC2 missing DAEMON_JWT_SECRET env var — P0 blocker for daemon auth in prod
- **#1165** chore: add Task Completion Protocol + rules 11–13 to architect agent definition (already done in this repo's `.claude/agents/architect.md`; verify and close)
- **#1117** Holistic gap analysis — Sync + BFF (architect deliverable, parallel to #1116 + #1118 already closed)

### E. Pre-Phase / Phase 3+ (parked behind decisions)
- #968 AWS Activate credits (Ray manual)
- #967 landing page deploy
- #966 GitHub MCP / project-manager agent (already done — verify)
- #980–#985 monetization (auth, Stripe, gating, pricing) — gated on #980 (Ray decision)
- #989–#998 specialized agents + RAG (Phase 4–6, do not start yet)

### F. Old gameplay/UI bugs lingering (no milestone)
- #882 lift-based synergy scoring
- #902 Suggest Decks "no viable combinations"
- #904 Card Recommendations empty for 8-card constructed deck
- #907 17Lands identifier audit
- #921 grpId → card name mapping gaps
- #925 data-testid coverage on React components

These predate the SaaS pivot. They should be triaged: keep/defer/close based on whether the v2.0 frontend still uses these surfaces.

---

## 3. Architectural Recommendations

### Critical: ADR-007 (frontend serving model) is still unwritten
The Wave 4 plan in `~/.claude/plans/mtga-companion-aws-launch.md` flagged this 4 sessions ago. ADR-006 set up Vercel; PR #1184 deployed an EC2 nginx path; both now exist with different CORS configs. **#1211** (duplicate Deploy workflow) and **#1066** (sync README) cannot proceed cleanly until this is resolved.
- Recommendation: write ADR-007 picking **Vercel canonical** (frontend already deploys there, path-filtered, fast) and document EC2 nginx as preview-only. Closes #1211, unblocks future infra cleanup.

### Daemon JWT secret missing in prod (#1169) is a silent P0
Authenticated `/api/v1/events` ingestion will 500 in production until DAEMON_JWT_SECRET is provisioned on EC2. No tests catch this — production smoke test #979 is also blocked. **Action**: infrastructure agent ticket to provision the secret via SSM Parameter Store + CloudFormation, plus a healthcheck assertion that the secret is present at startup.

### Daemon log parsing (#1160–#1164) is the v2.0 critical path
The cloud SaaS value prop (collection/deck/match/quest sync) is bottlenecked entirely on these five parsers. Without them, the daemon broadcasts events but has nothing meaningful to ingest. Recommend bundling them into a v2.0 daemon parser sprint (one ticket per event type, each Sonnet-ready).

### Delta sync chain is a tight dependency
#1099 (Store iface) → #1100 (Lambda hash-check) → #1101 (README) → closes #1127. Should be sequenced as a 3-PR series, all backend-engineer.

### Phase 3 monetization is locked behind Ray decisions
#980 (free vs paid tier), domain purchase (#979), and auth provider choice (Clerk vs Supabase) all need Ray's input. Architect cannot unblock these — flag in standup.

---

## 4. Top 5 Prioritized Next Steps

| # | Action | Ticket(s) | Agent | Why now |
|---|--------|-----------|-------|---------|
| 1 | Provision DAEMON_JWT_SECRET on prod EC2 | #1169 | infrastructure | Silent prod P0 — daemon ingest 500s without it |
| 2 | Write ADR-007 frontend serving model | (new) | architect | Unblocks #1211, #1066, future deploy work |
| 3 | Daemon Player.log parsers — match + deck + collection | #1164, #1163, #1162 | backend-engineer | Critical path for v2.0 cloud SaaS data pipeline |
| 4 | Lambda delta sync chain (#1099 → #1100 → #1101) | #1099, #1100, #1101, closes #1127 | backend-engineer | Closes ADR-005 implementation gap; reduces RDS write load |
| 5 | PM hygiene pass — close duplicates + verify shipped work | #1126, #1123, #1071, #1165 | project-manager | Stale open tickets distort sprint planning |

---

## 5. Plan Sync & Ownership

- **Master roadmap**: `~/.claude/plans/mtga-companion-aws-launch.md` (Wave 5 should kick off after #1169 + ADR-007)
- **This file**: tactical next-sprint queue, refreshed each sprint by architect
- **Project board**: #27 — agents move tickets To Do → In Progress → Done; PM moves Done → Released after deploy verification
- **Milestones referenced**: 55 (Phase 1), 56 (Phase 2), v2.0 (daemon parsing)

### Architect follow-on tickets to coordinate with PM
1. New ADR-007 ticket (architecture label, architect agent)
2. New `feat(infra): provision DAEMON_JWT_SECRET via SSM` (closes #1169 properly with CFN diff)
3. PM cleanup: verify-and-close #1126, #1123, #1071, #1165 if confirmed shipped
4. Triage pass on no-milestone gameplay bugs (#882, #902, #904, #907, #921, #925) — either assign to v1.x maintenance milestone or close as deferred to post-v2.0
