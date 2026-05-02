# ADR-001: Service Split Approaches for Cloud SaaS Migration

**Date**: 2026-05-02
**Status**: Proposed
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

MTGA Companion currently ships as a monorepo Go binary (`cmd/mtga-companion`) that co-locates:

- **Log reader / daemon** (`internal/daemon`, `internal/mtga/logreader`) — polls `Player.log` on the user's machine via fsnotify or a 2 s ticker, parses JSON events, and broadcasts domain events over a local WebSocket (port 9999)
- **REST API + WebSocket BFF** (`internal/api`, `internal/gui` facade layer) — HTTP handlers (matches, drafts, cards, decks, collection, ML suggestions, etc.) and bridges daemon events to the browser via `DaemonEventForwarder → Hub`
- **Data sync / poller** (`internal/mtga/cards/refresh/scheduler.go`, `internal/mtga/cards/seventeenlands`, `internal/mtga/cards/draftdata`) — daily/weekly scheduler that refreshes 17Lands ratings and card metadata from external sources
- **Frontend SPA** (`frontend/src`) — React + TypeScript app served by the Go binary in production or a Vite dev server locally

The goal is to split this into 4 independently deployable services for a cloud SaaS offering.

### Target Service Boundaries

| Service | Deployment target | Responsibility |
|---|---|---|
| **Daemon** | User's machine (local binary) | Read `Player.log`, POST events to BFF |
| **BFF** | EC2 | REST API + WebSocket, owns all DB writes |
| **Sync/Poller** | EC2 or Lambda | 17Lands ratings, card metadata on schedule |
| **Frontend** | EC2/nginx or S3+CloudFront | React SPA |

### Hard Constraints

- Daemon **must** stay local — `Player.log` is only accessible on the user's machine
- BFF **must** expose a WebSocket endpoint — live draft pick recommendations require it
- All services **must** be deployable via the existing GitHub Actions CI pipeline
- Module path: `github.com/ramonehamilton/MTGA-Companion`

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
  mtga-companion/      → split into Daemon binary + BFF binary
  apiserver/           → absorbed into BFF
  log-analyzer/        → Daemon dev tool (keep as-is)

frontend/src/          → Frontend service
```

---

## Approach A: Monorepo with Build-Tag Service Isolation

### Overview

All code stays in the **single `github.com/ramonehamilton/MTGA-Companion` monorepo**. Each service gets a new `cmd/` entrypoint. Go `depguard` linter rules (enforced in CI) prevent cross-service package imports.

### What Moves Where

```
cmd/
  daemon/        ← NEW entrypoint: imports internal/daemon + internal/mtga/logreader only
  bff/           ← RENAME from cmd/mtga-companion: imports internal/api + internal/gui + internal/storage + internal/ml
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
MTGA-Companion/
  cmd/daemon/   cmd/bff/   cmd/sync/
  internal/
    daemon/  api/  gui/  storage/  ml/  mtga/  shared/
  frontend/
  .github/workflows/
    daemon.yml    ← cross-compile win/mac, attach to GH Release
    bff.yml       ← Docker build, push ECR, deploy EC2
    sync.yml      ← Docker or Lambda zip deploy
    frontend.yml  ← npm build, S3 upload or nginx deploy
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
MTGA-Companion/
  go.work
  services/
    contract/            ← module: github.com/ramonehamilton/mtga-contract
      go.mod
      events.go          ← DaemonEvent, SyncEvent, shared DTOs
    daemon/              ← module: github.com/ramonehamilton/mtga-daemon
      go.mod
      cmd/main.go
      internal/
        daemon/          ← from internal/daemon/
        logreader/       ← from internal/mtga/logreader/
    bff/                 ← module: github.com/ramonehamilton/mtga-bff
      go.mod
      cmd/main.go
      internal/
        api/             ← from internal/api/
        gui/             ← from internal/gui/
        storage/         ← from internal/storage/ (migrations stay here)
        ml/              ← from internal/ml/
        mtga/            ← analysis, recommendations, deckexport, deckimport
    sync/                ← module: github.com/ramonehamilton/mtga-sync
      go.mod
      cmd/main.go
      internal/
        mtga/cards/      ← seventeenlands, draftdata, refresh, datasets, mtgjson, mtgazone, cfb
  frontend/              ← unchanged
  .github/workflows/
    daemon.yml  bff.yml  sync.yml  frontend.yml
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
4. Refactor `cmd/mtga-companion/main.go` — daemon startup → `services/daemon/cmd/main.go`, BFF startup → `services/bff/cmd/main.go`
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

- **Daemon**: cross-compiled (Windows + macOS) and attached to GitHub Releases; users download alongside MTGA. Authentication via per-install JWT issued at first registration with BFF.
- **BFF**: Dockerized, deployed to EC2. Shares a single Postgres instance with Sync. WebSocket hub remains in `internal/api/websocket/` — no change to the existing `Hub + DaemonEventForwarder` pattern, only the transport (daemon now POSTs over HTTP instead of calling in-process).
- **Sync**: runs as a long-lived EC2 process using the existing `ticker`-based `refresh/scheduler.go`, or wrapped as a Lambda handler triggered by EventBridge cron with minimal changes to the scheduler interface.
- **Frontend**: static build uploaded to S3 + CloudFront (or served from EC2 nginx). No code changes required; the API base URL becomes an environment variable at build time.
- **Database migrations**: the 30+ SQL migration files stay with `services/bff`. Sync uses a Postgres role scoped to `card_metadata`, `draft_ratings`, and related tables only, enforced via `GRANT` statements added to the migration that initializes the sync role.
