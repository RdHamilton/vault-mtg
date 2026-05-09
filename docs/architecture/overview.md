# VaultMTG — Cloud Architecture Overview

**Date**: 2026-05-09
**Status**: Current (post v0.3.0)
**Supersedes**: `docs/archive/desktop-era/ARCHITECTURE.md`

This document describes the live VaultMTG cloud architecture. For the legacy desktop-era architecture, see `docs/archive/desktop-era/ARCHITECTURE.md`.

---

## Top-level diagram

```
+-------------------+        +------------------+         +----------------------+
| MTGA Player.log   |  tail  |  daemon (Go)     |  HTTPS  |  BFF (Go) on ECS     |
| (player desktop)  +------->|  macOS/Windows   +-------->|  api.vaultmtg.app    |
+-------------------+        +------------------+         +----------+-----------+
                                                                     |
                                                          (RDS PostgreSQL)
                                                                     |
        +-----------------+        Clerk JWT       +-----------------+----------+
        | React 19 SPA    | <--- (auth)             | Projection worker        |
        | Vite, S3+CF     | --- HTTPS API --------->| (in-process w/ BFF)      |
        | vaultmtg.app    | <--- SSE -------------- |                          |
        +-----------------+                         +--------------------------+

        +-----------------+        scheduled        +--------------------------+
        | Sync Lambda     | <--- EventBridge cron --| AWS Lambda (Go)          |
        | (Scryfall, set  |                         | delta sync, idempotent   |
        | catalog ingest) |                         |                          |
        +-----------------+                         +--------------------------+
```

---

## Components

### `services/daemon` (Go, macOS / Windows)

**Role**: Local watcher on the player's machine. Tails MTGA's `Player.log` file, parses log lines into typed events (matches, drafts, inventory, quests, deck updates, gameplay), and posts them to the BFF ingest endpoint.

**Distribution**: Signed installers (ADR-0011). macOS uses notarized .pkg; Windows uses signed .msi via Squirrel auto-update.

**Auth**: Per-installation API key issued via the daemon registrar flow; long-lived JWT signed by the BFF (`DAEMON_JWT_SECRET`). This is M2M-only — never read by user-facing code.

**Key files**: `services/daemon/internal/logreader/` (parsers), `services/daemon/internal/dispatch/` (HTTP poster), `services/daemon/internal/registrar/` (registration handshake).

### `services/bff` (Go, ECS / RDS PostgreSQL)

**Role**: Backend-for-frontend. Three responsibilities:
1. **Ingest API** — accepts daemon event posts, writes raw events to `daemon_events` table, returns 202.
2. **Projection worker** — in-process goroutine that polls `daemon_events`, deserializes payloads, and writes typed rows to per-domain tables (matches, draft_sessions, card_inventory, inventory, quests, decks, game_plays). 30s tick interval today; planned NOTIFY/LISTEN upgrade in Wave 2 (T7).
3. **Read API** — Clerk-authenticated REST endpoints serving the SPA (history, stats, decks, drafts) plus an SSE broker for live updates.

**Auth**:
- Daemon endpoints: HMAC-style JWT verified against `DAEMON_JWT_SECRET`.
- User endpoints: Clerk JWT verified by `clerk-sdk-go v2` middleware (ADR-0009).

**Database**: RDS PostgreSQL. Schema migrations in `services/bff/migrations/` applied via the BFF binary on startup.

**Production URL**: `https://api.vaultmtg.app`
**Staging URL**: `https://staging-api.vaultmtg.app` (ADR-0019)

### `services/sync` (Go, AWS Lambda)

**Role**: Pulls external data (Scryfall card metadata, set catalog) on a cron schedule and writes to the BFF read paths. Stateless and idempotent — uses delta sync (ADR-0005) over full re-fetch.

**Trigger**: EventBridge scheduled rule (default: hourly).

### `frontend/` (React 19, Vite)

**Role**: User-facing SPA. Built with Vite, served as static assets from S3 behind CloudFront via Route 53 (ADR-0008).

**Auth**: Clerk React SDK only — never custom JWT handling, never `localStorage` session caching.

**Production URL**: `https://vaultmtg.app` (S3 + CloudFront)
**Staging URL**: `https://staging.vaultmtg.app`
**PR previews**: Vercel — preview environments only, never production (per repo memory).

---

## Environments

| Environment | Frontend | BFF | Database | Daemon target |
|-------------|----------|-----|----------|---------------|
| Local dev | `localhost:5173` (Vite) | `localhost:8080` | local Docker postgres | `localhost:8080` |
| Staging | `staging.vaultmtg.app` | `staging-api.vaultmtg.app` | RDS staging | `staging-api.vaultmtg.app` |
| Production | `vaultmtg.app` | `api.vaultmtg.app` | RDS production | `api.vaultmtg.app` |

---

## Cross-cutting concerns

- **Authentication**: Clerk for users (ADR-0009), HMAC JWT for daemon. Frontend uses `useAuth()` / `useUser()` hooks; BFF uses `ClerkAuthMiddleware` route group plus `auth.UserIDFromContext(ctx)` helper.
- **Multi-tenancy**: Clerk user_id resolves to an `accounts.id` row server-side. Every user-data query scopes by `account_id`.
- **Observability**: Sentry for errors and performance, PostHog for product analytics. Activation funnel tracked via PostHog feature flags + cohort filters.
- **Pagination**: All list endpoints use cursor pagination per ADR-0018. Cursors are base64-encoded opaque tokens.
- **CI/CD**: Tiered test strategy (ADR-0002) — unit on every push, integration on PR, full E2E nightly. Deploy via GitHub Actions to ECS + S3.

---

## Where to look in the repo

| Component | Path |
|-----------|------|
| Daemon | `services/daemon/` |
| BFF | `services/bff/` |
| Sync Lambda | `services/sync/` |
| Frontend SPA | `frontend/` |
| Shared contract types | `services/contract/` |
| Shared log parser | `pkg/logparse/` |
| ADRs | `docs/architecture/adr/` |
| Engineering reference docs | `docs/engineering/reference/` |

---

## Related docs

- ADRs: `docs/architecture/adr/`
- Deployment runbooks: `docs/engineering/deployment.md`
- Release checklist: `docs/engineering/release-checklist.md`
- Active milestone PRD: `docs/product/milestones/v0.4.0/kickoff.md`
- Latest architecture assessment: `docs/product/milestones/v0.4.0/arch-assessment.md`
