# ADR-006: Vercel→BFF Connectivity

**Date**: 2026-05-04
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

ADR-001 specified that the React frontend would be served from nginx on the EC2 instance
alongside the BFF. Vercel has since been selected as the frontend hosting platform instead,
trading per-request nginx ops for Vercel's global CDN, atomic preview deploys, and zero-config
TLS. This changes the connectivity model: the browser now originates from a `*.vercel.app`
domain and must reach the BFF over the public internet rather than over loopback.

### Current state

| Concern | Current value |
|---|---|
| Frontend base URL | `http://localhost:8080/api/v1` (hardcoded in `apiClient.ts`) |
| BFF CORS allowed origins | `http://localhost:*`, `http://127.0.0.1:*` |
| Auth header | `Authorization: Bearer <api-key>` (already correct) |
| TLS termination | nginx on EC2 port 443 → BFF port 8080 (planned; #977) |
| `ALLOWED_ORIGINS` env var | absent from BFF config |

### Constraints

- The BFF is a single instance on EC2; there is no staging environment yet.
- Preview deploys (per-PR Vercel builds) must reach the same prod BFF for now.
- The SSE transport uses a `?token=<api-key>` query parameter because `EventSource` does
  not support custom headers — the BFF SSE handler must continue to accept that form.

---

## Decision

### 1. BFF URL strategy — `VITE_BFF_URL` environment variable

The frontend resolves the BFF base URL from `import.meta.env.VITE_BFF_URL` at build time.
When the variable is absent the client falls back to `http://localhost:8080/api/v1` so local
development requires no env file change.

```
# Vercel dashboard → project → Settings → Environment Variables
VITE_BFF_URL = https://api.vaultmtg.app/api/v1   # Production
VITE_BFF_URL = https://api.vaultmtg.app/api/v1   # Preview (same BFF for now)
# Development: unset — falls back to http://localhost:8080/api/v1
```

`apiClient.ts` is updated to read the variable:

```ts
let config: ApiConfig = {
  baseUrl: import.meta.env.VITE_BFF_URL ?? 'http://localhost:8080/api/v1',
  timeout: 30000,
};
```

### 2. CORS policy — `ALLOWED_ORIGINS` environment variable

The BFF reads a comma-separated `ALLOWED_ORIGINS` env var and parses it at startup.
When the var is absent the allowed-origins list defaults to `["http://localhost:*",
"http://127.0.0.1:*"]` — identical to the previous behaviour so local dev is unaffected.

Production EC2 must set:

```
ALLOWED_ORIGINS=https://vaultmtg.vercel.app,https://*.vercel.app
```

The wildcard `https://*.vercel.app` covers every Vercel preview deploy URL without
requiring per-PR configuration. The risk (any Vercel-hosted app could call the BFF) is
accepted for now; the API key auth middleware on every guarded endpoint is the primary
security control, not CORS.

The allowed methods remain `GET, POST, OPTIONS`; the allowed headers remain
`Authorization, Content-Type, X-Request-ID`. No additional headers are introduced.

### 3. Auth token propagation

No change required. The existing adapter pattern in `apiClient.ts` reads the API key from
`localStorage` and injects `Authorization: Bearer <key>` on every request. For SSE, the
key is passed as a `?token=<key>` query parameter because the `EventSource` API does not
support custom request headers.

The BFF middleware validates the bearer token / query-parameter token against the
`api_keys` table (when `DATABASE_URL` is set). This is unchanged.

### 4. Preview deploy isolation

Vercel preview deploys (created for every PR) will connect to the production BFF and the
production database for the duration of Phase 2. This is an accepted risk:

- Preview deploys are accessible only via obscure per-build URLs, not linked from any
  public page.
- All mutating endpoints require a valid API key; there is no anonymous write path.
- A staging environment (separate EC2 + RDS) is the long-term solution; it is out of
  scope for Phase 2.

A follow-on ticket must be created to provision a staging BFF + RDS and to configure
Vercel preview environments to point at it.

> **[Resolved — 2026-05-07 — see [Staging Environment ADR](./staging-environment-design.md))]**
> Phase 3.5 introduced a dedicated staging environment (`staging-api.vaultmtg.app` +
> staging RDS + staging Clerk instance). Vercel Preview builds now resolve
> `VITE_BFF_URL` to the staging BFF via the Preview-scoped environment variable,
> so PR review traffic no longer reads from or writes to production data.
> Resolution work shipped in Wave 3: CloudWatch alarms (#1433), staging env vars
> wired into the Vercel project (#1442), and the E2E CI fix that exercises the
> staging surface (#1458).

### 5. HTTPS / TLS termination

nginx on EC2 terminates TLS at port 443 and reverse-proxies to the BFF on port 8080.
The BFF itself speaks plain HTTP internally. Let's Encrypt (via Certbot) is the
certificate provider. This architecture is already planned in issue #977 and is not
changed by this ADR.

SSE connections benefit from HTTP/2 multiplexing when nginx is configured with
`http2 on` (nginx 1.25+). This is recommended but not required for correctness.

### 6. `VITE_` variable exposure

`VITE_BFF_URL` is inlined into the static JS bundle at build time by Vite. It must
never contain secrets. The value is always a public HTTPS URL. This is consistent with
how Vite handles all `VITE_*` variables.

---

## Consequences

### Easier

- Frontend deployments are fully decoupled from EC2 deploys; each can be promoted
  independently.
- Vercel preview deploys give every PR a live, testable frontend without any EC2 changes.
- CORS is operator-configurable without a code deploy — change `ALLOWED_ORIGINS` and
  restart the BFF.

### Harder

- Preview deploys share the production BFF and database until a staging environment is
  provisioned. Accidental data mutation during PR testing is possible.
- `VITE_BFF_URL` must be kept in sync in the Vercel dashboard and in any local `.env`
  files used by developers who run against a non-default BFF URL.

---

## Implementation Checklist

These changes flow from this ADR and are tracked as separate implementation tickets:

- [ ] **Backend** (#TBD): Add `AllowedOrigins []string` to `services/bff/internal/config/config.go`;
      parse `ALLOWED_ORIGINS` env var (comma-split, default localhost values); wire into
      `cors.Handler` in `services/bff/cmd/main.go`.
- [ ] **Frontend** (#TBD): Update `frontend/src/services/apiClient.ts` to read
      `import.meta.env.VITE_BFF_URL` with localhost fallback.
- [ ] **Infrastructure** (#TBD): Set `ALLOWED_ORIGINS` and any other required env vars on
      the EC2 BFF systemd unit; set `VITE_BFF_URL` in the Vercel project settings for
      Production and Preview environments.
- [ ] **Documentation** (#TBD): Add `.env.example` to `frontend/` documenting `VITE_BFF_URL`.

---

## Alternatives Considered

### API Gateway in front of EC2

AWS API Gateway could handle TLS, CORS, and throttling in front of the BFF. Rejected:
API Gateway adds per-request cost and operational complexity. The existing nginx setup on
EC2 handles TLS at no marginal cost and the BFF already implements its own CORS middleware.

### Custom domain via CloudFront

A CloudFront distribution in front of EC2 would eliminate the `vercel.app` CORS surface
by serving both the frontend and API from the same origin. Rejected: CloudFront adds
per-GB data-transfer cost and a complex routing config for no functional improvement at
current traffic levels. The current Vercel + EC2 split is simple, cost-effective, and
easily migrated to CloudFront if traffic justifies it.

### Hardcoding the Vercel production domain in BFF CORS config

Rejected: any code change to the domain list requires a BFF redeploy. Env var is
strictly more flexible and the only acceptable approach per project coding guidelines.
