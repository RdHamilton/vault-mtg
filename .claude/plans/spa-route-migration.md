# SPA Route Migration Plan (Phase 2)

**Started:** 2026-05-11
**Last updated:** 2026-05-11

Tracking the migration of every `frontend/src/services/api/*.ts` module
off of `daemonClient` (port 9001, daemon localapi) onto `apiClient`
(port 8080, BFF). Live-state-only paths stay on the daemon.

> **Why this exists:** Phase 1 unified ports and stripped the daemon
> down to `/health` + `/api/v1/system/*`. Every SPA module that still
> targets `daemonClient` for cloud-data routes (matches, decks, drafts,
> collection, cards, …) is hitting a daemon that doesn't serve those
> routes. They've been silently 404'ing since Phase 1 landed.

Companion reference doc (full path catalogue): `docs/product/milestones/
v0.3.1/daemon-local-api-phase2-audit.md` (currently on branch
`feat/phase2-audit-and-bucket-a`, not yet PR'd).

---

## Current state — SPA module ↔ client ↔ paths

| Module             | Client       | Paths called | Disposition |
|--------------------|--------------|--------------|-------------|
| `matches.ts`       | apiClient ✅  | 17           | **In progress** — PR #1872 wired SPA to BFF but only 3 of 17 routes mounted; PR #2 needed for the rest |
| `system.ts`        | daemonClient | 14           | 8 paths (`/system/*`) work today via daemon; 6 dead paths (`/llm/*`, `/feedback/dashboard`, `/export/clear`) need triage |
| `decks.ts`         | daemonClient | 33           | Migrate to BFF (cloud) — needs handlers + repo + repoint |
| `drafts.ts`        | daemonClient | 31           | Migrate to BFF (cloud) — split out `/decks/classify-draft-pool`, `/decks/explain-recommendation`, `/decks/recommendations` (belong in decks PR) and `/feedback/*` (belong in feedback PR) |
| `cards.ts`         | daemonClient | 13           | Migrate to BFF |
| `collection.ts`    | daemonClient | 8            | BFF already has `GET /api/v1/collection`; SPA needs to repoint and add 7 missing handlers |
| `mlSuggestions.ts` | daemonClient | 8            | Migrate to BFF |
| `notes.ts`         | daemonClient | 7            | Migrate to BFF |
| `standard.ts`      | daemonClient | 6            | Migrate to BFF |
| `quests.ts`        | daemonClient | 4            | Migrate to BFF |
| `opponents.ts`     | daemonClient | 3            | Migrate to BFF; some overlap with BFF `/api/v1/stats/*` |
| `gameplays.ts`     | daemonClient | 1            | Migrate to BFF |
| `meta.ts`          | daemonClient | 1            | Migrate to BFF |
| `settings.ts`      | daemonClient | 1            | Decide: cloud (BFF) vs local-only (daemon) |

**Total non-`matches`, non-`system` paths to migrate: ~116.**

What the BFF actually serves today (post-PR #1872):
```
/api/v1/collection                        (GET)
/api/v1/daemon/version                    (GET)
/api/v1/draft-ratings/{set}/{format}      (GET)
/api/v1/events                            (GET, SSE)
/api/v1/health/daemon                     (GET)
/api/v1/history/{matches,drafts}          (GET)         legacy
/api/v2/{matches,drafts,decks,collection} (GET)
/api/v1/ingest/events                     (POST)
/api/v1/stats/{deck-performance,win-rate-trend,format-distribution,
              draft-analytics,rank-progression,result-breakdown}
                                          (GET)
/api/v1/matches                           (POST list)   ← PR #1872
/api/v1/matches/formats                   (GET)         ← PR #1872
/api/v1/matches/{matchId}                 (GET)         ← PR #1872
```

What the daemon localapi serves today (post-Phase 1):
```
/health
/api/v1/system/status
/api/v1/system/health
/api/v1/system/version
/api/v1/system/account
/api/v1/system/database/path
/api/v1/system/daemon/{status,connect,disconnect}
```

---

## Execution order

Smallest-area-first to keep review velocity high. Each PR is
self-contained: BFF handlers + repo + tests + SPA migration + SPA
tests + lint/format gates.

| # | PR scope                              | Paths | Status        | Branch / PR |
|---|---------------------------------------|-------|---------------|-------------|
| 1 | matches/* — full surface (17 paths)   | 17    | ✅ **Merged** 2026-05-11 | PR #1872 |
| 2 | collection/* (8 paths)                | 8     | ✅ **Merged** 2026-05-11 | PR #1873 |
| 3 | quests/* (4 paths)                    | 4     | ⏳ **In progress** | `feat/phase2-pr3-quests` |
| 4 | standard/* (6 paths)                  | 6     | Pending       | — |
| 5 | gameplays/* + meta/* (2 paths)        | 2     | Pending       | — |
| 6 | opponents/* + analytics overlap       | 3     | Pending       | — |
| 7 | notes/* (7 paths)                     | 7     | Pending       | — |
| 8 | cards/* (13 paths)                    | 13    | Pending       | — |
| 9 | decks/* cloud paths (33 paths)        | 33    | Pending       | — |
|10 | drafts/* — full module incl. `/decks/*` and `/feedback/*` strays (31 paths, minus Bucket C) | 28 | Pending | — |
|11 | mlSuggestions/* (8 paths)             | 8     | Pending       | — |
|12 | settings/* — implement cloud-backed settings on BFF | 1+ | Pending | — |
|13 | system.ts cleanup — delete dead paths (`/llm/*`, `/feedback/dashboard`, `/export/clear`) | — | Pending | — |
|14 | drafts/* Bucket C — live-state stays on daemon (current-pack, in-flight grading) | ~3 | Pending | — |
|15 | Frontend cleanup — drop unused `daemonClient` exports if empty | — | Pending | — |
|16 | Audit doc PR — land `feat/phase2-audit-and-bucket-a` on main for reviewers | — | Pending | branch already pushed |

---

## Per-PR template

For each PR, fill in:

```
### PR #N — <area>/* (<count> paths)

**Scope:**
- BFF handlers: <list>
- BFF repo methods: <list>
- SPA module(s): <file(s)>

**Paths in this PR:**
- POST /<area>/...
- GET  /<area>/<id>
- ...

**Repoint-only paths (already on BFF, just change import):**
- ...

**Out of scope (deferred to PR #?):**
- ...

**Tests:**
- [ ] BFF: handler unit tests
- [ ] BFF: repo integration tests
- [ ] SPA: vitest module tests updated to mock apiClient
- [ ] SPA: component/integration tests still green
- [ ] Pre-PR: gofumpt, go vet, tsc --noEmit, eslint, vitest run

**Risks / open questions:**
- ...
```

---

## Decisions (locked 2026-05-11)

1. **drafts.ts scope** — `/decks/classify-draft-pool`,
   `/decks/explain-recommendation`, `/decks/recommendations`,
   `/feedback/*` stay inside the drafts PR (#11). Don't split them
   out into the decks/feedback PRs. Keeps each SPA module touched
   exactly once.
2. **system.ts dead paths** — `/llm/status`, `/llm/test`,
   `/llm/models...`, `/llm/models/pull`, `/feedback/dashboard`,
   `/export/clear` get **removed** from system.ts. Folded into PR
   #13 as a deletion-only task — no BFF/daemon work needed.
3. **settings.ts disposition** — **cloud (BFF)**. Per-account
   storage in the BFF DB, no daemon involvement. PR #14 implements.
4. **PR #1872 mount gaps** — **oversight**. Fix on the open PR
   instead of cutting a separate PR #2. Add the missing 14 handlers
   + routes + tests to `feat/phase2-pr1-matches` before merging.

---

## Pre-PR gates (every PR)

1. `gofumpt -l services/bff services/daemon services/sync services/contract pkg/logparse` → empty
2. `cd services/bff && go vet ./... && go test -race ./...`
3. `cd frontend && npx tsc --noEmit && npm run lint && npm run test:run`
4. PR body has zero references to Claude Code (per CLAUDE.md)
5. Branch follows `feat/phase2-pr<N>-<area>` naming

---

## PR #1 (matches) — locked scope

Expanded on the open PR #1872 branch `feat/phase2-pr1-matches`.

**Delete from `frontend/src/services/api/matches.ts` (zero consumers):**
- `getWinRateOverTime`
- `getPerformanceMetrics`

**Repoint to existing BFF endpoint:**
- `getRankProgression(format)` → `GET /api/v1/stats/rank-progression?format={format}`
  Requires `DaemonAPIKeyAuth` to also protect this route (currently
  Clerk-only). Add an alternative middleware mount alongside the
  existing Clerk one.

**New BFF handlers + routes + repo methods + tests:**
- `GET  /api/v1/matches/{matchId}/games`
- `POST /api/v1/matches/stats`
- `POST /api/v1/matches/trends`
- `GET  /api/v1/matches/archetypes`
- `POST /api/v1/matches/format-distribution` (filter-aware)
- `POST /api/v1/matches/performance-by-hour`
- `POST /api/v1/matches/matchup-matrix`
- `GET  /api/v1/matches/rank-progression-timeline`
- `GET  /api/v1/matches/export`
- `POST /api/v1/matches/compare`
- `POST /api/v1/matches/compare/formats`
- `POST /api/v1/matches/compare/decks`
- `POST /api/v1/matches/compare/time-periods`

All new routes guarded by `DaemonAPIKeyAuthMiddl`, scoped to the
authenticated user's accounts. camelCase JSON wire format.

## Status log

- **2026-05-11** — Plan created. PR #1872 opened (matches PR #1).
  Found PR #1872 only mounted 3 of 17 matches routes — decided to fix
  the gap on the open PR rather than cut a separate follow-up. Locked
  scope decisions for drafts (no split), system.ts dead paths
  (delete), and settings (cloud).
- **2026-05-11** — Audited matches.ts call sites. 2 functions
  (`getWinRateOverTime`, `getPerformanceMetrics`) are dead wrappers
  with no consumers — deleting in PR #1. 1 function repoints to
  existing `/api/v1/stats/rank-progression`. 12 functions need
  brand-new BFF handlers + repo + tests. Authenticated user picked
  "do it all on PR #1872" (~1500 lines of Go).
- **2026-05-11** — PR #1872 merged. Full /api/v1/matches/* surface
  shipped (17 routes), envelope + casing contract bugs fixed.
  Discovered while building: the original "repoint to /stats/rank-
  progression" plan was wrong — that endpoint is a timeline, not a
  current-rank summary. RankProgression became a 13th /matches/*
  handler. Starting PR #2 (collection/*).
- **2026-05-11** — PR #2 (collection/*) built. Audited consumers and
  found 7 of 12 collection.ts wrappers were dead Wails-era code with
  no real callers (getMissingCardsForSet, getMissingCards,
  getCollectionBySet, getCollectionByRarity, getRecentChanges,
  getMissingCardsForDeck, getDeckValue) — deleted them. Real BFF
  surface: 4 endpoints (POST /collection, GET /collection/stats,
  /collection/sets, /collection/value). Branch
  `feat/phase2-pr2-collection` ready to open as PR.
- **2026-05-11** — PR #2 merged. Starting PR #3 (quests/*).
- **2026-05-11** — PR #3 (quests/*) built. Five endpoints under
  /api/v1/quests/* (active/history/wins/daily, wins/weekly, stats).
  Read-side methods added to existing QuestRepository (was
  write-only). Daily/weekly wins computed from matches table;
  TotalGoldEarned in stats stubbed at 0 pending a richer rewards
  schema. quests.ts: import-only swap (URLs unchanged).
