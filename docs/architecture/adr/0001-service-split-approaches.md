# ADR-001: Service Split Approaches for Cloud SaaS Migration

**Date**: 2026-05-02
**Status**: Proposed
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

VaultMTG currently ships as a monorepo Go binary (`cmd/vaultmtg`) that co-locates:

- **Log reader / daemon** (`internal/daemon`, `internal/mtga/logreader`) — polls `Player.log` on the user's machine via fsnotify or a 2 s ticker, parses JSON events, and broadcasts domain events over a local WebSocket (port 9999)
- **REST API + WebSocket BFF** (`internal/api`, `internal/gui` facade layer) — HTTP handlers (matches, drafts, cards, decks, collection, ML suggestions, etc.) and bridges daemon events to the browser via `DaemonEventForwarder → Hub`
- **Data sync / poller** (`internal/mtga/cards/refresh/scheduler.go`, `internal/mtga/cards/seventeenlands`, `internal/mtga/cards/draftdata`) — daily/weekly scheduler that refreshes 17Lands ratings and card metadata from external sources
- **Frontend SPA** (`frontend/src`) — React + TypeScript app served by the Go binary in production or a Vite dev server locally

The goal is to split this into 4 independently deployable services for a cloud SaaS offering.

### Target Service Boundaries

| Service | Deployment target | Responsibility |
|---|---|---|
| **Daemon** | User's machine (local binary) | Read `Player.log`, POST events to BFF |
| **BFF** | EC2 | REST API + SSE (or WebSocket), owns all DB writes |
| **Sync/Poller** | **Lambda + EventBridge Scheduler** | 17Lands ratings, card metadata on schedule |
| **Frontend** | **EC2/nginx** | React SPA served from existing EC2 instance |

### Hard Constraints

- Daemon **must** stay local — `Player.log` is only accessible on the user's machine
- BFF **must** expose a real-time push endpoint for draft pick updates — protocol TBD (see BFF section below)
- All services **must** be deployable via the existing GitHub Actions CI pipeline
- Module path: `github.com/RdHamilton/vault-mtg`

### Known Risks

**Log loss — MTGA overwrites `Player.log` on startup.** If the daemon was not running when MTGA started, all draft/match events written since the last daemon run are lost. A log preservation mechanism was attempted but is not functioning correctly. The data model may also not accurately represent the draft log format, requiring a longer investigation and refinement phase before the feature is reliable. This is flagged for the **daemon agent** (fix log preservation) and the **DBA agent** (review whether the schema fits the draft log event structure). See GitHub issue: `daemon: investigate log preservation and MTGA log overwrite on startup`.

---

## Current Package Inventory and Service Mapping

```
internal/
  daemon/              → Daemon  (service.go, websocket.go, flight_recorder, replay_engine)
  mtga/logreader/      → Daemon  (poller, poller_manager, parser, deck, draft_picks, quests)
  mtga/analysis/       → BFF     (opponent_analyzer, play_analyzer, ml_engine)
  mtga/recommendations/→ BFF
  mtga/deckexport/     → BFF
  mtga/deckimport/     → BFF
  mtga/cards/
    seventeenlands/    → Sync    (ratings fetcher, HTTP client)
    draftdata/         → Sync    (updater)
    refresh/           → Sync    (scheduler.go, staleness tracker)
    setcache/          → Sync + BFF read path
    mtgjson/           → Sync    (client, models)
    mtgazone/          → Sync    (scraper)
    cfb/               → Sync    (importer)
    datasets/          → Sync    (downloader, parser)
    scryfall.go        → Sync
  api/                 → BFF     (server.go, router.go, handlers/, websocket/)
  gui/                 → BFF     (facade layer: card, collection, deck, draft, match, meta, ml, settings)
  storage/             → BFF     (db.go, migrate.go, scheduler.go, 30+ migration files)
  storage/repository/  → BFF     (AccountRepository, MatchRepository, DraftRepository, ...)
  ml/                  → BFF     (engine, model, pipeline, personal, meta_weighting)
  archetype/           → BFF
  metrics/             → BFF
  commands/            → BFF     (startup_command, replay_command)

cmd/
  vaultmtg/            → split into Daemon binary + BFF binary
  apiserver/           → absorbed into BFF
  log-analyzer/        → Daemon dev tool (keep as-is)

frontend/src/          → Frontend service
```

---

## BFF Real-Time Protocol: SSE vs WebSocket

### Background

ADR-001 originally listed "BFF must expose a WebSocket endpoint" as a hard constraint. That assumption was made in the context of a local app where the BFF and browser are on the same machine. In the cloud context the tradeoff changes.

### Comparison

| | Server-Sent Events (SSE) | WebSocket |
|---|---|---|
| Direction | Server → client only | Bidirectional |
| Protocol | Standard HTTP/1.1 or HTTP/2 | HTTP Upgrade to `ws://` / `wss://` |
| nginx config | No special config needed | Requires `Upgrade` and `Connection` headers |
| Reconnection | Built-in (`EventSource` API) | Manual reconnect logic |
| Proxying | Transparent through any HTTP proxy | Requires nginx `proxy_http_version 1.1` + upgrade headers |
| Browser support | All modern browsers | All modern browsers |
| Use in draft UI | Sufficient — BFF pushes pick events; browser sends picks via REST | Overcomplicated — bidirectional not needed |

### Decision

**Use SSE** for BFF → browser push of draft pick events. The browser sends pick confirmations as regular REST `POST` requests; SSE delivers real-time recommendations and draft state updates back to the browser. This is strictly server-to-client, which is all the draft UI requires.

WebSocket should be revisited **only if** a future feature requires client-initiated server push (e.g., multiplayer draft) that cannot be handled by REST polling or long-poll. If that arises, upgrade to WebSocket at that point.

The existing `Hub + DaemonEventForwarder` pattern in `internal/api/websocket/` will be refactored to use SSE during the BFF migration.

---

## Sync/Poller: Lambda + EventBridge (Not EC2 Process)

Card metadata updates occur every few months; 17Lands ratings refresh daily. These are batch jobs with bounded runtimes — not long-running services.

### Decision

**Deploy Sync as Lambda functions triggered by EventBridge Scheduler**, not as a persistent EC2 process.

| Factor | EC2 process | Lambda + EventBridge |
|---|---|---|
| Idle cost | EC2 always running | Zero idle cost |
| Retries | Manual | Built-in Lambda retry + DLQ |
| Max runtime | Unlimited | 15 min (not a constraint for daily batch) |
| Process management | systemd service | None |
| Cold start | None | Acceptable for scheduled jobs |

The existing `refresh/scheduler.go` ticker loop is replaced with a Lambda handler that runs the same fetch+store logic on schedule. The `setcache` package moves to `services/sync` (see SetCache ownership below).

---

## Frontend Serving: EC2/nginx (Not S3+CloudFront)

### Decision

**Serve the React static build from nginx on the existing EC2 instance.**

EC2 is already deployed and paid for. Adding nginx as a static file server on that instance has zero marginal cost. S3+CloudFront adds per-request and data-transfer fees with no performance or reliability benefit at current traffic levels.

**Migration path**: If traffic grows to a level where CloudFront CDN caching or geographic distribution provides meaningful benefit, the static files can be moved to S3+CloudFront at that point. The React build process is identical in both cases — only the deploy step changes.

---

## SetCache Ownership and Sync/BFF Contract

`internal/mtga/cards/setcache` is read by the BFF (draft ratings lookups) and written by Sync (card data refresh). Ownership rules:

1. **Sync owns writes.** Set cache data is the authoritative source for future deck-building models; only the Sync service may update it.
2. **BFF reads via `DraftRatingsRepository`.** The BFF queries Postgres directly — it does not import the `setcache` package after the module split.
3. **Draft UI must never block on Sync.** If the Sync Lambda is running, failing, or delayed, the BFF must fall back to the last known good cache data in Postgres. A feature flag or fallback mechanism must be designed so that cache ownership can be flipped (BFF serves stale data from its own read path) without operator intervention.

The feature flag / fallback design is tracked in GitHub issue: `arch: design SetCache ownership flip mechanism for sync/BFF`.

---

## Approach A: Monorepo with Build-Tag Service Isolation

### Overview

All code stays in the **single `github.com/RdHamilton/vault-mtg` monorepo**. Each service gets a new `cmd/` entrypoint. Go `depguard` linter rules (enforced in CI) prevent cross-service package imports.

### What Moves Where

```
cmd/
  daemon/        ← NEW entrypoint: imports internal/daemon + internal/mtga/logreader only
  bff/           ← RENAME from cmd/vaultmtg: imports internal/api + internal/gui + internal/storage + internal/ml
  sync/          ← NEW entrypoint: imports internal/mtga/cards/refresh + external fetchers
  log-analyzer/  ← unchanged (dev tool)
frontend/        ← unchanged; nginx-served in production
```

A new `internal/shared/` package holds wire types crossing service boundaries:

```
internal/shared/
  events/    ← DaemonEvent structs (Daemon POSTs; BFF deserializes)
  contract/  ← HTTP request/response DTOs
  auth/      ← JWT validation helpers
```

### Communication Contracts

```
Daemon  ──POST /v1/ingest/events──▶  BFF
        { type, account_id, session_id, occurred_at, payload }
        Auth: Bearer <daemon-jwt>

Browser ──WSS /ws─────────────────▶  BFF  (live event fan-out)
Browser ──REST /api/v1/────────────▶  BFF

Sync    ──writes card/ratings data─▶  shared Postgres (different table namespace)
```

### Repo Structure

```
vault-mtg/
  cmd/daemon/   cmd/bff/   cmd/sync/
  internal/
    daemon/  api/  gui/  storage/  ml/  mtga/  shared/
  frontend/
  .github/workflows/
    daemon.yml    ← cross-compile win/mac, attach to GH Release
    bff.yml       ← Docker build, push ECR, deploy EC2
    sync.yml      ← Lambda zip deploy (EventBridge Scheduler)
    frontend.yml  ← npm build, nginx deploy on EC2
```

Workflows use `paths` filters so only changed services rebuild.

### Migration Complexity

**Low.** No directory restructuring.

1. Add three new `cmd/` entrypoints factoring out existing `main()` logic
2. Extract `internal/shared/events` (the current `DaemonEventForwarder` already uses reflection to avoid cycles — replace with a real contract type)
3. Configure `depguard` in `.golangci.yml`
4. Add four workflow files with `paths` triggers

Estimated: 1–2 days.

### Tradeoffs

| Pros | Cons |
|---|---|
| Minimal disruption; tests and tooling unchanged | Import boundary is soft — `depguard` is advisory, not enforced by the Go toolchain |
| Single PR review cycle for cross-cutting changes | All four binaries build on every CI run without careful path filtering |
| Compile-checked shared types | Harder to open-source only the daemon later |

---

## Approach B: Go Workspace Multi-Module Split (Recommended)

### Overview

The monorepo is restructured as a **Go workspace** (`go.work`) containing four Go modules, each in a subdirectory. Module boundaries are enforced by the Go toolchain — the daemon module literally cannot import the BFF's storage package.

### What Moves Where

```
vault-mtg/
  go.work
  services/
    contract/            ← module: github.com/RdHamilton/vault-mtg/services/contract
      go.mod
      events.go          ← DaemonEvent, SyncEvent, shared DTOs
    daemon/              ← module: github.com/RdHamilton/vault-mtg/services/daemon
      go.mod
      cmd/main.go
      internal/
        daemon/          ← from internal/daemon/
        logreader/       ← from internal/mtga/logreader/
    bff/                 ← module: github.com/RdHamilton/vault-mtg/services/bff
      go.mod
      cmd/main.go
      internal/
        api/             ← from internal/api/
        gui/             ← from internal/gui/
        storage/         ← from internal/storage/ (migrations stay here)
        ml/              ← from internal/ml/
        mtga/            ← analysis, recommendations, deckexport, deckimport
    sync/                ← module: github.com/RdHamilton/vault-mtg/services/sync
      go.mod
      cmd/main.go
      internal/
        mtga/cards/      ← seventeenlands, draftdata, refresh, datasets, mtgjson, mtgazone, cfb
  frontend/              ← unchanged
  .github/workflows/
    daemon.yml  bff.yml  sync.yml  frontend.yml  # frontend: nginx deploy; sync: Lambda zip
```

### Shared Code Strategy

`services/contract` is the single source of truth for all wire types. During local development, `go.work` `replace` directives point to the local path. In CI, each service pins a tagged release (`v0.x.y`) of `mtga-contract`. No shared business logic crosses module boundaries — only JSON-serializable structs.

The `internal/mtga/cards/setcache` package is used by both BFF (cache reads) and Sync (writes). Resolution: Sync owns the write path; BFF queries Postgres directly via its `DraftRatingsRepository`. The `setcache` package moves to `services/sync`.

### Communication Contracts

```go
// services/contract/events.go
package contract

type DaemonEvent struct {
    Type       string          `json:"type"`        // "MATCH_COMPLETED", "DRAFT_PICK", ...
    AccountID  string          `json:"account_id"`  // multi-tenant identifier
    SessionID  string          `json:"session_id"`
    OccurredAt time.Time       `json:"occurred_at"`
    Payload    json.RawMessage `json:"payload"`
}
```

```
Daemon  ──POST /v1/ingest/events──▶  BFF
        Body: contract.DaemonEvent
        Auth: Bearer <daemon-jwt>

BFF     ──broadcasts DaemonEvent──▶  Browser (WSS /ws)

Sync    ──PUT /v1/sync/ratings────▶  BFF   (push model, preferred)
        OR writes directly to shared Postgres (same DSN, scoped user)

Browser ──REST /api/v1/────────────▶  BFF
```

### Repo Structure and CI

```yaml
# .github/workflows/bff.yml (excerpt)
on:
  push:
    paths: ['services/bff/**', 'services/contract/**']
jobs:
  build:
    defaults:
      run:
        working-directory: services/bff
    steps:
      - uses: actions/checkout@v4
      - run: go build ./cmd/...
      - run: go test ./...
```

Daemon workflow cross-compiles for `GOOS=windows GOARCH=amd64` and `GOOS=darwin GOARCH=arm64`, then attaches binaries to a GitHub Release via `softprops/action-gh-release` (already configured in the infra repo).

### Migration Complexity

**Medium.** Estimated 3–5 days.

1. `go work init` + create four `go.mod` files
2. `git mv` packages into service subdirectories; update import paths (`sed` + `goimports`)
3. Extract `DaemonEvent` into `services/contract/`
4. Refactor `cmd/vaultmtg/main.go` — daemon startup → `services/daemon/cmd/main.go`, BFF startup → `services/bff/cmd/main.go`
5. Existing tests stay in place; `go test ./...` from root via `go.work` runs all of them

### Tradeoffs

| Pros | Cons |
|---|---|
| Hard module boundary enforced by Go toolchain | More upfront restructuring than Approach A |
| Each service evolves its own `go.mod` dependencies independently | `go.work replace` directives require discipline to sync with tagged releases |
| `go test ./...` from root still runs everything via `go.work` | `setcache` package shared between BFF and Sync needs explicit ownership decision |
| CI builds only changed services (path-filtered workflows) | |
| Contract module can be versioned, documented, and open-sourced separately | |

---

## Approach C: Polyrepo — Separate Git Repositories

### Overview

Four independent Git repositories, each a standalone Go module. The shared contract types live in a fifth repo (`mtga-contract`) published as a proper Go module.

```
github.com/RdHamilton/mtga-daemon     ← local binary
github.com/RdHamilton/mtga-bff        ← EC2 service
github.com/RdHamilton/mtga-sync       ← EC2 / Lambda
github.com/RdHamilton/mtga-frontend   ← React SPA
github.com/RdHamilton/mtga-contract   ← shared wire types
```

### Shared Code Strategy

`mtga-contract` is published via GitHub tags. Consumer repos use `go get github.com/RdHamilton/mtga-contract@v0.x.y`. Schema changes require a PR to `mtga-contract`, a new release tag, then dependency-bump PRs in all consumers.

### Migration Complexity

**High.** All work from Approach B plus:
- Four new GitHub repos with independent CI pipelines and secrets
- Managing `mtga-contract` as a real published module (semantic versioning, Dependabot)
- Cross-cutting changes (e.g., adding a new daemon event type) span multiple PRs
- No shared `go test ./...` — integration tests must be explicit or run via a dedicated test harness repo

### Tradeoffs

| Pros | Cons |
|---|---|
| Maximum isolation; services can be independently open-sourced or handed to contractors | Cross-cutting changes require N PRs and N CI runs |
| Each service has a clean independent PR history | `mtga-contract` versioning becomes a coordination bottleneck for a small team |
| | Highest migration effort with no functional benefit over Approach B for a team of 1–3 |
| | Shared tooling (linters, test helpers) must be duplicated or extracted to yet another repo |

---

## Decision

**Recommended: Approach B — Go Workspace Multi-Module Split**

Approach B provides hard, toolchain-enforced module isolation (the daemon binary genuinely cannot import BFF storage internals) while retaining mono-repo developer ergonomics: one `git clone`, one `go test ./...` from root, and one `go.work` for local cross-service development. The `services/contract` module provides a versioned, compile-checked schema for all cross-service events, replacing the current `DaemonEventForwarder` reflection hack with a real type.

Approach A is viable if velocity is the immediate priority — it can land in a single PR — but leaves import boundaries advisory rather than enforced. Approach C adds polyrepo coordination overhead that is not justified until the team grows beyond 3–4 engineers or a service must be independently open-sourced.

### Implementation Sequence

1. Create `services/contract` module — extract `DaemonEvent` and sync DTOs
2. Scaffold `services/daemon` — migrate `internal/daemon` and `internal/mtga/logreader`; update `go.work`
3. Scaffold `services/sync` — migrate `internal/mtga/cards/refresh` and all external data fetchers
4. Rename remaining code root to `services/bff`; all existing packages stay in place initially
5. Add four GitHub Actions workflow files with `paths` filters
6. Enforce boundaries in CI via `go vet` + `depguard` per-module config

---

## Consequences

- **Daemon**: cross-compiled (Windows + macOS) and attached to GitHub Releases; users download alongside MTGA. Authentication via per-install JWT issued at first registration with BFF. Log preservation must be fixed — MTGA overwrites `Player.log` on startup and the current preservation mechanism is broken (see Known Risks above).
- **BFF**: Dockerized, deployed to EC2. Shares a single Postgres instance with Sync. The existing `Hub + DaemonEventForwarder` WebSocket pattern is **replaced with SSE** — the `internal/api/websocket/` package is refactored to use `text/event-stream` responses. Daemon POSTs events over HTTP; BFF fans them out to browsers via SSE.
- **Sync**: deployed as **Lambda functions triggered by EventBridge Scheduler** (daily for ratings, on-demand for card metadata). The `ticker`-based `refresh/scheduler.go` is replaced by a Lambda handler calling the same fetch+store logic. `setcache` package moves to `services/sync`. Sync owns all writes; BFF reads via `DraftRatingsRepository`. A fallback/feature-flag mechanism ensures draft UI never blocks if Sync is unavailable.
- **Frontend**: static build served from **nginx on the existing EC2 instance** — zero extra cost. Can be migrated to S3+CloudFront if traffic justifies CDN distribution. No code changes required; the API base URL is an environment variable at build time.
- **Database migrations**: the 30+ SQL migration files stay with `services/bff`. Sync uses a Postgres role scoped to `card_metadata`, `draft_ratings`, and related tables only, enforced via `GRANT` statements added to the migration that initializes the sync role.
