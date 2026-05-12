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
| 3 | quests/* (4 paths)                    | 4     | ✅ **Merged** 2026-05-11 | PR #1874 |
| 4 | standard/* (6 paths)                  | 6     | ✅ **Merged** 2026-05-11 | PR #1875 |
| 5a | gameplays/* (6 endpoints — backed by game_plays, game_state_snapshots, opponent_cards_observed tables) | 6 | ✅ **Merged** 2026-05-11 | PR #1876 |
| 5b | meta/* (7 endpoints; 3 real reads from mtgzone_* + 4 shape-stubs pending ML/scrape infra) | 7 | ✅ **Merged** 2026-05-11 | PR #1877 |
| 6 | opponents/* + analytics + archetypes-expected (5 endpoints across 4 URL prefixes) | 5 | ✅ **Merged** 2026-05-11 | PR #1878 |
| 7 | notes/* + suggestions (10 endpoints across 3 URL prefixes; generate stubbed) | 10 | ✅ **Merged** 2026-05-11 | PR #1879 |
| 8 | cards/* (16 endpoints; refresh-ratings stubbed pending scrape pipeline) | 16 | ✅ **Merged** 2026-05-11 | PR #1880 |
| 9 | decks/* (~50 endpoints; CRUD + cards + tags + permutations + import/export real, deck-builder + recommendation stubs) | 50 | ✅ **Merged** 2026-05-11 | PR #1881 |
|10 | drafts/* — full module incl. `/decks/*` and `/feedback/*` strays (~38 endpoints; sessions + 17lands + community + trends real, grading/recs stubs) | 38 | ✅ **Merged** 2026-05-11 | PR #1882 |
|11 | mlSuggestions/* (11 endpoints — 3 alias to notes/, 8 net-new; process-history + play-patterns/update stubs) | 11 | ✅ **Merged** 2026-05-12 | PR #1883 |
|12 | settings/* — cloud-backed key/value (4 endpoints + new user_settings JSONB table) | 4 | ✅ **Merged** 2026-05-12 | PR #1884 |
|13 | system.ts cleanup — delete 6 dead wrappers + orphaned LLM/Clear UI (~1500 LOC) | -6 | ⏳ **In progress** | `feat/phase2-pr13-system-cleanup` |
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
- **2026-05-11** — PR #3 merged. Starting PR #4 (standard/*).
- **2026-05-11** — PR #4 (standard/*) built. Six endpoints under
  /api/v1/standard/* (sets, config, rotation, rotation/affected-decks,
  POST validate/{deckId}, cards/{arenaId}/legality). New
  StandardRepository covers global reads (sets, standard_config,
  cards.legalities) and account-scoped deck queries. validate
  parses the cards.legalities JSON inline; affected-decks iterates
  per-deck against the next rotation_date. standard.ts: import-only
  swap. Standard-format SPA tests fixed to mock apiClient.
- **2026-05-11** — PR #4 merged. Starting PR #5.
- **2026-05-11** — PR #5 plan was "gameplays + meta (2 paths)" but
  audit revealed 13 actual endpoints across the two modules.
  Splitting: PR #5a covers gameplays/* (6 endpoints), PR #5b will
  cover meta/* (7 endpoints) on its own branch.
- **2026-05-11** — PR #5a (gameplays/*) built. New GamePlaysRepository
  + GamePlaysHandler with 6 endpoints under /api/v1/matches/{id}/plays/*,
  /opponent-cards, /snapshots, plus /api/v1/gameplays/game/{id}.
  Backed by the existing game_plays / game_state_snapshots /
  opponent_cards_observed tables; scope enforced via matches join.
  gameplays.ts and gameplays.test.ts: import-only swap to apiClient.
- **2026-05-11** — PR #5a merged. Starting PR #5b (meta/*).
- **2026-05-11** — PR #5b (meta/*) built. New MetaRepository +
  MetaHandler with 7 endpoints. /meta/archetypes, /meta/tier,
  /meta/archetypes/cards are real reads from mtgzone_archetypes /
  mtgzone_archetype_cards. /meta/deck-analysis, /meta/identify-archetype,
  /meta/insights, /meta/refresh are shape-correct stubs documented
  inline pending the archetype-matching algorithm + scrape pipeline
  (separate follow-up PRs). meta.ts: import-only swap.
- **2026-05-11** — PR #5b merged. Starting PR #6 (opponents/*).
- **2026-05-11** — PR #6 (opponents/*) built. New OpponentsRepository
  + OpponentsHandler with 5 endpoints across 4 URL prefixes
  (/matches/{id}/opponent-analysis, /opponents/decks,
  /analytics/matchups, /analytics/opponent-history,
  /archetypes/{name}/expected-cards). Composite OpponentAnalysis
  endpoint stitches profile + observed cards + expected cards
  (with wasSeen flag) + matchup stats. StrategicInsights and
  MetaArchetype linkage emitted as empty/null pending follow-up.
  opponents.ts: import-only swap; MSW handlers for opponent routes
  repointed from API_BASE → BFF_BASE.
- **2026-05-11** — PR #6 merged. Starting PR #7 (notes/*).
- **2026-05-11** — PR #7 (notes/*) built. Plan said "7 paths"; actual
  is 10 endpoints across 3 URL prefixes. New NotesRepository +
  NotesHandler. Deck notes (CRUD), match notes (GET/PUT against
  matches.notes/rating columns), and ml_suggestions (list + dismiss).
  generate-suggestions stubbed pending the ML pipeline (returns
  existing list). priority derived from confidence (>=0.7 high,
  >=0.4 medium, else low); cardReferences encoded as JSON object
  for the SPA's parseEvidence helper. notes.ts + notes.test.ts:
  import-only swap.
- **2026-05-11** — PR #7 merged. Starting PR #8 (cards/*).
- **2026-05-11** — PR #8 (cards/*) built. Plan said "13 paths"; actual
  is 16 endpoints. New CardsRepository + CardsHandler covering
  search/lookup/sets, 17Lands ratings (with X-Cache-Age-Hours +
  X-Cache-Degraded headers from the staleness check), color ratings,
  and ChannelFireball ratings (CRUD + arena-id linking). Two
  account-scoped endpoints (collection-quantities,
  search-with-collection) join card_inventory. refresh-ratings is
  a documented stub: bumps cached_at on draft_card_ratings rows
  but doesn't actually scrape 17Lands (that lives in services/sync).
  Tier letter derived from gihwr (S/A/B/C/D/F bucketing). cards.ts
  + cards.test.ts + msw cards/sets handler: import-only swap to
  apiClient / BFF_BASE.
- **2026-05-11** — PR #8 merged. User pushed back on default-splitting
  for PR #9 (decks/*); confirmed one-shot is fine since decks.ts is
  one module (PR #5 was split because gameplays.ts/meta.ts were two
  files with unrelated concerns).
- **2026-05-11** — PR #9 (decks/*) built one-shot. ~50 endpoints
  across CRUD + per-deck stats/performance/validate/classify, deck
  cards add/remove, deck tags add/remove, library/by-tags/by-draft,
  permutations (list/get/current/diff/name/restore), import/parse/
  export, and the deck-builder + recommendations STUBs (build-around,
  generate, suggest, analyze, archetypes, card-performance, four
  recommendations endpoints). All real impls go through new
  DecksRepository (decks + deck_cards + deck_tags + deck_permutations
  + matches join for performance). Import accepts Arena-format text;
  export renders back to Arena format. STUBs documented inline pending
  the ML / archetype-matching pipeline. decks.ts + decks.test.ts +
  4 msw deck handlers: import-only swap to apiClient / BFF_BASE.
- **2026-05-11** — PR #9 merged. Starting PR #10 (drafts/*).
- **2026-05-11** — PR #10 (drafts/*) built one-shot. ~38 endpoints
  across draft sessions, picks, stats, formats, recent, exportable,
  17lands export (renders draft_picks → SPA's SeventeenLandsDraftExport),
  community comparison (single + all + by-format), temporal trends,
  learning curve, plus the /decks/* (recommendations,
  explain-recommendation, classify-draft-pool) and /feedback/*
  (recommendation, action, outcome, stats) strays from drafts.ts.
  Real impls back the session/picks/stats/community/trends queries
  via new DraftsRepository (draft_sessions + draft_picks +
  draft_temporal_trends + draft_community_comparison +
  recommendation_feedback). Grading + ML endpoints (grade-pick,
  win-probability, calculate-prediction, calculate-grade,
  current-pack, etc.) are documented STUBs pending the ML pipeline.
  drafts.ts + 2 drafts.test.ts files + 2 msw handlers: import-only
  swap to apiClient / BFF_BASE.
- **2026-05-12** — PR #12 merged (#1884). Starting PR #13 (system.ts
  cleanup). 6 dead wrappers deleted from system.ts: `clearAllData`
  (`/export/clear`), `checkOllamaStatus`, `getAvailableOllamaModels`,
  `pullOllamaModel`, `testLLMGeneration` (`/llm/*`), and
  `getFeedbackDashboardMetrics` (`/feedback/dashboard`). User approved
  full-cascade delete: removed `useMLSettings.ts` hook + test (Ollama-only),
  deleted unused `DataManagementSection.tsx` (legacy split-out), trimmed
  `useDataManagement.ts` to just `handleExportData`, stripped Ollama UI
  from `MLSettingsSection.tsx` (kept ML preferences + meta sources +
  weights + clear-learned-data), removed `Import from JSON` / `Import
  Single Log File` / `Clear All Data` from `ImportExportSection.tsx` +
  `DataRecoverySection.tsx`, removed `llmEnabled`/`ollamaEndpoint`/
  `ollamaModel` from `gui.AppSettings` + `useSettings.ts`, dropped
  related entries from `Settings.tsx` accordion + `apiMock.ts` legacy
  Wails bindings. Tests rewritten for each touched section. Net change:
  ~1500 LOC removed, 0 LOC of BFF/daemon work.
- **2026-05-12** — PR #11 merged (#1883). Starting PR #12 (settings/*).
  Audit: 4 endpoints in settings.ts (GET/PUT full + GET/PUT per-key).
  Old desktop-era `settings(key,value)` table is global + dead (no
  readers in current codebase). New migration 000076 adds
  account-scoped `user_settings(account_id, key, value JSONB)`. SPA's
  AppSettings constructor applies defaults for missing keys, so a
  brand-new account just gets `{}` on GET and renders zeros. PUT
  /settings (full replace) upserts each field as a row in a single
  transaction; PUT /settings/{key} stores {value} verbatim.
  settings.ts + new __tests__/settings.test.ts: import-only swap to
  apiClient. Settings.test.tsx + useSettings hook untouched (they
  mock the whole module via apiMock pattern).
- **2026-05-11** — PR #10 merged (#1882). Starting PR #11
  (mlSuggestions/*). Audit revealed 11 endpoints in mlSuggestions.ts,
  not 8 — three of them (list / generate / dismiss) overlapped with
  PR #7's /api/v1/decks/{id}/suggestions* surface. Resolved by
  mounting alias routes under /ml-suggestions/* that reuse the
  NotesRepository read+dismiss methods, paired with a new
  MLRepository for the 8 net-new endpoints (apply, synergy-report,
  card-synergies, combinations, process-history STUB,
  play-patterns read + STUB update, learned-data wipe). Aliases
  emit the richer MLSuggestion shape (confidence/cardId/swap fields/
  apply timestamps) so the SPA panel renders properly. learned-data
  DELETE is account-scoped: wipes the user's ml_suggestions +
  user_play_patterns only — global card_combination_stats survives.
  mlSuggestions.ts + mlSuggestions.test.ts: import-only swap to
  apiClient (no MSW handlers touched — component test mocks the
  whole module via apiMock).
