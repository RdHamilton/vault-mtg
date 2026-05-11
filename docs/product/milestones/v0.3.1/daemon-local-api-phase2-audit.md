# Phase 2 — Full Migration & API Normalization

**Status:** in progress (2026-05-11)
**Parent plan:** [daemon-local-api-plan.md](./daemon-local-api-plan.md)
**Architectural direction confirmed by Ray:** no shortcuts. All 60 paths get a real implementation. Naming gets normalized. No daemon-proxy shim layer.

## Architecture rules (lock-in)

### 1. Where each path lives

The 60 `daemonClient` paths split cleanly into two categories:

- **Cloud data** — anything the BFF can answer from its database. Lives on the BFF under `/api/v1/...`. The SPA calls these via `apiClient`.
- **Live local state** — anything that requires the running daemon's in-memory MTGA log state (current draft pool, in-progress match, etc). Lives on the daemon's localapi under `/api/v1/...`. The SPA calls these via `daemonClient`.

Result: `daemonClient` shrinks from 60 paths to ~5. The remaining 55 paths move to `apiClient`.

The daemon never proxies BFF reads. A user without the daemon installed can still browse their match history, decks, etc. — they just can't grade live picks.

### 2. JSON naming convention

**All new and migrated endpoints use camelCase JSON keys.** Snake_case is out, PascalCase (Wails-era) is out.

- BFF Go responses: `json:"matchId"`, `json:"durationSeconds"`, `json:"deckName"`
- Daemon localapi responses: same
- SPA TypeScript types: `matchId: string`, `durationSeconds?: number`, `deckName?: string`

Existing snake_case BFF responses (`history/matches`, `history/drafts`, `stats/*`) get migrated as part of their respective feature-area PRs. Existing Wails-generated PascalCase SPA types get regenerated as TS interfaces with camelCase fields.

### 3. Path layout

Every path lives under `/api/v1/{feature}/{action}`. No more bare `/matches` or `/decks` — those collide with BFF auth-key paths.

- Old (daemonClient): `POST /matches` → New: `GET /api/v1/matches`
- Old: `POST /matches/stats` → New: `POST /api/v1/matches/stats`
- Old: `GET /collection` → New: `GET /api/v1/collection`

### 4. Auth model

- **BFF cloud-data paths:** `DaemonAPIKeyAuth` middleware (Bearer daemon api_key from keychain) → resolves to `users.id`
- **Daemon localapi paths:** loopback-only, no auth (the firewall is the security boundary)

## Path catalogue with disposition (60 paths)

### Cloud data → BFF (migrate from daemonClient → apiClient)

#### matches/* (12 paths) — PR #1
| SPA call (today) | New BFF endpoint | Notes |
|---|---|---|
| `POST /matches` | `GET /api/v1/matches?format=&page=&limit=` | Replaces filter-body with query params for caching |
| `GET /matches/:id` | `GET /api/v1/matches/:id` | Single match detail |
| `GET /matches/:id/games` | `GET /api/v1/matches/:id/games` | Per-game breakdown |
| `POST /matches/stats` | `POST /api/v1/matches/stats` | Aggregate metrics |
| `GET /matches/formats` | `GET /api/v1/matches/formats` | Distinct formats |
| `GET /matches/archetypes` | `GET /api/v1/matches/archetypes` | Distinct archetypes |
| `POST /matches/format-distribution` | `POST /api/v1/matches/format-distribution` | Per-format stats |
| `POST /matches/performance-by-hour` | `POST /api/v1/matches/performance-by-hour` | Hour-of-day perf |
| `POST /matches/matchup-matrix` | `POST /api/v1/matches/matchup-matrix` | Archetype×Archetype |
| `POST /matches/performance` | `POST /api/v1/matches/performance` | Performance metrics |
| `GET /matches/rank-progression/:fmt` | `GET /api/v1/matches/rank-progression/:fmt` | Rank tier history |
| `POST /matches/compare(/decks|/formats|/time-periods)` | matching POST under `/api/v1/matches/compare*` | Comparison views |

#### decks/* (14 paths) — PR #2
| SPA call | New BFF endpoint |
|---|---|
| `GET /decks` | `GET /api/v1/decks` |
| `POST /decks/analyze` | `POST /api/v1/decks/analyze` |
| `GET /decks/archetypes` | `GET /api/v1/decks/archetypes` |
| `POST /decks/build-around` | (Bucket C — see daemon paths) |
| `POST /decks/build-around/suggest-next` | (Bucket C — see daemon paths) |
| `GET /decks/by-tags` | `GET /api/v1/decks/by-tags` |
| `POST /decks/classify-draft-pool` | (Bucket C — see daemon paths) |
| `POST /decks/explain-recommendation` | `POST /api/v1/decks/explain-recommendation` |
| `POST /decks/generate` | `POST /api/v1/decks/generate` |
| `POST /decks/import` | `POST /api/v1/decks/import` |
| `GET /decks/library` | `GET /api/v1/decks/library` |
| `POST /decks/parse` | `POST /api/v1/decks/parse` |
| `POST /decks/recommendations` | `POST /api/v1/decks/recommendations` |
| `POST /decks/suggest` | `POST /api/v1/decks/suggest` |
| `POST /decks/suggested/export-content` | `POST /api/v1/decks/suggested/export-content` |

#### drafts/* (10 paths, minus 3 Bucket C) — PR #3
| SPA call | New BFF endpoint |
|---|---|
| `GET /drafts` | `GET /api/v1/drafts` |
| `POST /drafts/community-comparison` | `POST /api/v1/drafts/community-comparison` |
| `GET /drafts/formats` | `GET /api/v1/drafts/formats` |
| `POST /drafts/grade-pick` | (Bucket C — daemon) |
| `POST /drafts/insights` | `POST /api/v1/drafts/insights` |
| `POST /drafts/recalculate-set-grades` | `POST /api/v1/drafts/recalculate-set-grades` |
| `POST /drafts/stats` | `POST /api/v1/drafts/stats` |
| `POST /drafts/trends` | `POST /api/v1/drafts/trends` |
| `GET /drafts/win-probability` | (Bucket C — daemon) |

#### collection/* (4 paths) — PR #4
| SPA call | New BFF endpoint |
|---|---|
| `GET /collection` | `GET /api/v1/collection` |
| `GET /collection/sets` | `GET /api/v1/collection/sets` |
| `GET /collection/stats` | `GET /api/v1/collection/stats` |
| `GET /collection/value` | `GET /api/v1/collection/value` |

#### cards/* (4 paths) — PR #5
| SPA call | New BFF endpoint |
|---|---|
| `POST /cards/cfb/import` | `POST /api/v1/cards/cfb/import` |
| `GET /cards/collection-quantities` | `GET /api/v1/cards/collection-quantities` |
| `POST /cards/search-with-collection` | `POST /api/v1/cards/search-with-collection` |
| `GET /cards/sets` | `GET /api/v1/cards/sets` |

#### quests/* (3 paths) — PR #6
| SPA call | New BFF endpoint |
|---|---|
| `GET /quests/active` | `GET /api/v1/quests/active` |
| `GET /quests/wins/daily` | `GET /api/v1/quests/wins/daily` |
| `GET /quests/wins/weekly` | `GET /api/v1/quests/wins/weekly` |

#### standard/* (4 paths) — PR #7
| SPA call | New BFF endpoint |
|---|---|
| `GET /standard/config` | `GET /api/v1/standard/config` |
| `GET /standard/rotation` | `GET /api/v1/standard/rotation` |
| `GET /standard/rotation/affected-decks` | `GET /api/v1/standard/rotation/affected-decks` |
| `GET /standard/sets` | `GET /api/v1/standard/sets` |

#### feedback/* (3 paths) — PR #8
| SPA call | New BFF endpoint |
|---|---|
| `GET /feedback/dashboard` | `GET /api/v1/feedback/dashboard` |
| `POST /feedback/recommendation` | `POST /api/v1/feedback/recommendation` |
| `GET /feedback/stats` | `GET /api/v1/feedback/stats` |

#### misc (4 paths) — PR #9
| SPA call | New BFF endpoint |
|---|---|
| `POST /meta/identify-archetype` | `POST /api/v1/meta/identify-archetype` |
| `GET /ml/learned-data` | `GET /api/v1/ml/learned-data` |
| `POST /llm/status` | `POST /api/v1/llm/status` |
| `GET /settings` / `POST /settings` | `GET|POST /api/v1/settings` |

### Live local state → Daemon localapi (stay on daemonClient)

#### drafts (Bucket C) — PR #10
| SPA call | Daemon endpoint | State source |
|---|---|---|
| `POST /drafts/grade-pick` | `POST /api/v1/drafts/grade-pick` | Current draft pack + pool from log |
| `GET /drafts/win-probability` | `GET /api/v1/drafts/win-probability` | In-progress match state |

#### decks (Bucket C) — PR #11
| SPA call | Daemon endpoint | State source |
|---|---|---|
| `POST /decks/build-around` | `POST /api/v1/decks/build-around` | Current draft pool (optional input) |
| `POST /decks/build-around/suggest-next` | `POST /api/v1/decks/build-around/suggest-next` | Current pack state |
| `POST /decks/classify-draft-pool` | `POST /api/v1/decks/classify-draft-pool` | Live draft pool |

## Execution order

PRs land in this order, smallest-area-first to keep review velocity high:

1. **Naming/architecture foundation** — codify camelCase contract, regenerate any SPA types affected by Phase 1
2. **system/* migration polish** (already merged — Phase 1)
3. **matches/* (PR #1)** — core feature, blocks closed-beta testing
4. **collection/* (PR #4)** — small, high-value
5. **decks/* cloud paths (PR #2)** — large, important
6. **drafts/* cloud paths (PR #3)**
7. **cards/* (PR #5)**
8. **quests/standard/feedback/misc (PRs #6–9)**
9. **Bucket C daemon endpoints (PRs #10–11)**
10. **Frontend cleanup** — delete unused `daemonClient` paths

Each PR is self-contained: BFF schema migration (if needed) + handler + repo + tests + SPA migration + SPA tests.
