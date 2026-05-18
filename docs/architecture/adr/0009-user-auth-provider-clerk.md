# ADR-009: User Auth Provider = Clerk

**Date**: 2026-05-06
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-006 (BFF connectivity / CORS), ADR-008 (frontend serving model)

---

## Context

Phase 3 of VaultMTG (milestone #61) introduces user accounts,
monetization, and per-user API keys for the desktop daemon. Until now the
product has run as a single-tenant local app with the BFF accepting
HMAC-signed JWTs minted by the daemon (`DAEMON_JWT_SECRET`). That model does
not extend to a multi-tenant SaaS:

- There is no end-user identity — no sign-in, no email-verified accounts, no
  per-user data isolation other than `account_id` columns the daemon
  asserts.
- There is no user-managed API key UX. Daemon credentials are a single
  shared HMAC secret, not a per-user, revocable token.
- There is no organization / team primitive for future shared-account use
  cases (drafting groups, content creators with multiple bots, etc.).
- There is no React sign-in/sign-up surface in the SPA; the SPA assumes the
  user is already on their own machine.

Phase 3 requires all of the above. The architect ran a structured
comparison of **Clerk** vs **Supabase Auth** (see
`.claude/plans/auth-provider-comparison.md`) with the explicit override
threshold "the user prefers Clerk; Supabase must clearly win on >=2 of
{cost-at-scale, Go reliability, API key management}". Supabase won zero of
three.

---

## Decision

**Clerk is the canonical user-auth provider for VaultMTG.**

### Specifics

1. **Identity & sessions**: Clerk hosts user identities (email/password,
   Google OAuth, optional Discord OAuth). Clerk issues short-lived JWTs.
   Clerk is the system of record for `user_id`, email, OAuth links, and
   MFA secrets.
2. **Frontend integration (React + Vite SPA on `app.vaultmtg.app`)**:
   - Package: `@clerk/react@latest`.
   - Env var: `VITE_CLERK_PUBLISHABLE_KEY` set in `.env.local` for local
     dev and in the production build environment for tagged releases. The
     `VITE_` prefix is required for Vite to expose the value to the
     browser bundle.
   - `<ClerkProvider afterSignOutUrl="/">` wraps the app **only in
     `main.tsx`**. The provider reads the publishable key from
     `import.meta.env.VITE_CLERK_PUBLISHABLE_KEY` automatically — do
     **not** pass `publishableKey` as a manual prop.
   - Auth state in components uses `<Show when="signed-in">` and
     `<Show when="signed-out">` (the modern React API). The legacy
     `<SignedIn>` / `<SignedOut>` components are **not** used.
   - Sign-in / sign-up / user-menu UI uses `<SignInButton>`,
     `<SignUpButton>`, and `<UserButton>` from `@clerk/react`. Clerk's
     hosted modals are used for the auth flow itself; we do not build a
     custom sign-in form in Phase 3.
   - Hooks: `useAuth()` for the session token (`getToken()`), `useUser()`
     for the user object. The SPA's existing `apiClient` reads the
     session token via `getToken()` and attaches it as
     `Authorization: Bearer <jwt>` to every BFF request.
   - **Forbidden patterns** (do not use; will be rejected in review):
     - `frontendApi` prop on `<ClerkProvider>` (legacy)
     - Env vars `REACT_APP_CLERK_FRONTEND_API` or
       `VITE_REACT_APP_CLERK_PUBLISHABLE_KEY` (legacy / wrong prefix)
     - Manual `publishableKey={...}` prop on `<ClerkProvider>` (the SDK
       reads the env var directly)
     - `<SignedIn>` / `<SignedOut>` components (outdated; use
       `<Show when="signed-in">` / `<Show when="signed-out">`)
     - `<ClerkProvider>` mounted anywhere except `main.tsx`
   - Reference: <https://clerk.com/docs/react/getting-started/quickstart>.
3. **Backend integration (Go BFF on `api.vaultmtg.app`)**:
   - Package: `github.com/clerk/clerk-sdk-go/v2` (official Clerk Go SDK).
   - JWT verification via Clerk's JWKS endpoint with in-memory key cache
     (refresh interval ≤1h). All requests to `/api/v1/*` flow through a
     `ClerkAuthMiddleware` that calls `clerk.VerifyToken` and injects
     `user_id`, `org_id` (if any), and `account_id` (from a server-side
     lookup keyed by `user_id`) into the request `context.Context`.
   - Repositories continue to enforce `account_id`-scoped queries. The
     middleware is the only place that resolves `user_id → account_id`;
     handlers must not call Clerk APIs directly.
   - Handlers reject requests where the resolved `account_id` does not
     match the path/body `account_id` parameter (defense in depth against
     IDOR).
4. **Daemon authentication (post-cutover)**:
   - The desktop daemon authenticates to the BFF using a **Clerk API
     Key** (Clerk's native API Keys product, Pro plan). Each user
     generates a key in the SPA via Clerk's prebuilt API-Keys component;
     the daemon stores it in the OS keychain.
   - The BFF middleware accepts both Clerk session JWTs (browser
     traffic) and Clerk API keys (daemon traffic). Both resolve to the
     same `(user_id, account_id)` context.
   - `DAEMON_JWT_SECRET` and the HMAC daemon-token path are removed once
     Clerk API keys are in place. This is a separate migration ticket;
     during the transition the BFF accepts both.
5. **Tier / role gating**:
   - Use Clerk **Organizations + Roles + Permissions** for tier gating
     (`free`, `pro`, future team plans). Tier is stored as a custom
     session-token claim populated from Clerk metadata.
   - The BFF middleware reads the tier claim from the verified JWT and
     attaches it to the request context. Handlers gate Pro-only features
     on `ctx.Tier()`.
6. **Plan & pricing**:
   - Start on Clerk **Hobby (free)** for dev and pre-launch.
   - Move to **Clerk Pro ($25/mo flat at our scale)** at the moment we
     ship the API Keys feature for the daemon. Budget signoff captured
     in the cost line below.
7. **Outage / degraded-mode policy**:
   - JWT TTL ≥ 60 minutes; the SPA caches the token until expiry.
   - The BFF caches Clerk JWKS for 1 hour and serves verifies from
     cache; a Clerk JWKS outage does not break in-flight sessions.
   - On Clerk auth-API outage: existing sessions continue to work; new
     sign-ins are degraded. The BFF surfaces a "Clerk-degraded" header
     for observability.

### What this changes

- **Adds** the Clerk SDK to the SPA and the BFF.
- **Adds** a new `ClerkAuthMiddleware` to the BFF, replacing (eventually)
  the HMAC daemon-JWT middleware.
- **Adds** a new `users` table in the BFF Postgres schema mapping
  `clerk_user_id → account_id` (one-time provisioned on first sign-in).
- **Removes** (post-cutover) `DAEMON_JWT_SECRET` and its associated
  middleware path.
- **Does not** change `account_id`-scoped query patterns in repositories.
  All existing isolation invariants hold; the only new piece is how
  `account_id` enters the request context.

---

## Consequences

### Positive

- **Phase 3 unblocked.** User accounts, sign-in, OAuth, and per-user API
  keys are productized features rather than custom code.
- **First-class Go SDK.** `clerk-sdk-go/v2` ships middleware primitives
  and JWKS caching; the BFF auth surface stays small.
- **Polished React UX.** `<SignInButton>`, `<UserButton>`, and the hosted
  Clerk modals give us a production sign-in flow with zero custom UI
  work in Phase 3. The modern `<Show when="signed-in">` API keeps the
  SPA's auth-conditional rendering simple and idiomatic.
- **Native API keys.** End-user API key issuance, rotation, and
  revocation is a Clerk product, not code we maintain.
- **Operational simplicity.** Hosted-only — no GoTrue binary, no auth
  schema migrations, no MFA-secret storage on our infra.
- **Cost is predictable and small.** $0/mo on Hobby through pre-launch;
  $25/mo flat once we move to Pro for API Keys. Well within AWS Activate
  credit headroom (separately tracked, but the Clerk line is unrelated
  to AWS spend).

### Negative

- **Hosted-only / vendor lock-in.** No self-host escape hatch. Mitigated
  by JWT TTLs, JWKS caching, and a documented degraded-mode policy.
  Migration off Clerk is a multi-week project (password hashes, OAuth
  links, MFA secrets) and is acknowledged as known risk.
- **MRU billing model.** Returning users count each month they sign in.
  At our projected scale the bill is dominated by the $25 Pro flat fee;
  MRU overages are negligible until we cross 10K monthly retained
  users.
- **API Keys feature is Pro-tier.** We pay $25/mo the moment we ship the
  daemon API-key UX. Acceptable and budgeted.
- **Two auth paths during cutover.** Until daemon migration completes,
  the BFF accepts both Clerk sessions and HMAC daemon JWTs. Cutover is
  a tracked ticket.

### Neutral

- **Frontend bundle size.** `@clerk/react` adds ~80–100KB gzipped. Not
  material against the SPA's existing bundle.

---

## Implementation Tickets

These tickets land in milestone #61 ("Phase 3: VaultMTG Brand &
Production Infrastructure") on project board #27. Project Manager owns
final ticket creation and sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Spike: wire `clerk-sdk-go/v2` into BFF middleware, verify a Clerk JWT, benchmark JWKS cache | backend-engineer |
| **TBD-B** | Spike: drop `<ClerkProvider>` + `<SignInButton>` + `<UserButton>` + `<Show when="signed-in">` into the SPA shell behind a feature flag; verify Vite production build is clean | front-engineer |
| **TBD-C** | DBA: design `users` table (`clerk_user_id` PK, `account_id` FK, provisioned-at, tier-cache); migration in `services/bff/internal/storage/migrations/postgres/` | dba |
| **TBD-D** | Backend: implement `ClerkAuthMiddleware` (verify JWT, resolve `user_id → account_id`, inject context); replace HMAC middleware on `/api/v1/*` for browser traffic | backend-engineer |
| **TBD-E** | Frontend: replace existing local-only auth shim with Clerk sign-in; route guards on protected pages via `<Show when="signed-in">` and `<Protect>` | front-engineer |
| **TBD-F** | Backend + Frontend: end-user API key UX. Clerk Pro plan enabled. Daemon reads key from OS keychain. BFF middleware accepts both Clerk JWT and Clerk API key | backend-engineer + front-engineer |
| **TBD-G** | Backend: remove `DAEMON_JWT_SECRET` and HMAC daemon-token path once all daemons are on Clerk API keys | backend-engineer |
| **TBD-H** | Infra: add `CLERK_SECRET_KEY` to BFF SSM parameters; add `VITE_CLERK_PUBLISHABLE_KEY` to the SPA build environment | infrastructure |
| **TBD-I** | Docs: update `RELEASE_CHECKLIST.md` and `docs/CLAUDE_CODE_GUIDE.md` to reference Clerk patterns; flag the "forbidden patterns" list as review criteria | architect |

Each ticket gets its own acceptance criteria when the Project Manager
files it.

---

## Alternatives Considered

### A. Supabase Auth (GoTrue) — hosted

**Rejected.** Comparison detailed in
`.claude/plans/auth-provider-comparison.md`. Supabase failed all three
override-threshold dimensions:

- No native API key product (we would build it from scratch).
- No official Go SDK (community-maintained client only).
- Cost-at-scale tie ($0/mo at projected scale on both providers).

Auxiliary downsides: bundling auth into the wider Supabase platform
creates a two-DB topology with our existing RDS instance.

### B. Supabase Auth (GoTrue) — self-hosted against RDS

**Rejected.** The self-host option recovers some lock-in flexibility but
adds a stateful service we must run, patch, and migrate alongside the
auth schema. Operational toil outweighs the lock-in benefit at our team
size.

### C. Auth0

**Not formally compared** (user lean was Clerk-vs-Supabase). Auth0 is a
mature option but is more expensive at the API-keys tier and the
React/Go SDK ergonomics are not materially better than Clerk for our
use case. Reconsider only if Clerk pricing shifts unfavorably.

### D. Roll our own (Postgres + `golang-jwt` + custom UI)

**Rejected.** Auth is not a differentiator for VaultMTG. Building
sign-in, OAuth, MFA, password reset, email verification, and an API-key
UX is months of work that does not move the product forward. Buy.

### E. Keep HMAC `DAEMON_JWT_SECRET` and skip user accounts

**Rejected.** Phase 3 monetization explicitly requires per-user
identities and per-user API keys. The HMAC path cannot extend to
multi-tenant.

---

## References

- ADR-006 — Vercel→BFF connectivity (CORS, cross-origin BFF). Still
  applies; CORS allow-list updated for `app.vaultmtg.app`.
- ADR-008 — frontend serving model (S3+CloudFront for SPA). The Clerk
  SDK lives in the SPA bundle served from S3+CloudFront.
- `.claude/plans/auth-provider-comparison.md` — Clerk vs Supabase
  comparison, override-threshold reasoning.
- Clerk documentation (canonical reference for all Phase 3/4 Clerk
  implementation work — backend SDK, frontend SDK, M2M tokens,
  webhooks) — <https://clerk.com/docs>.
- Clerk React quickstart — <https://clerk.com/docs/react/getting-started/quickstart>.
- Clerk Go SDK — <https://github.com/clerk/clerk-sdk-go>.
- Phase 3 site plan — `.claude/projects/.../project_phase3_site_plan.md`
  (memory note).
