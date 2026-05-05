# Auth Provider Comparison: Clerk vs Supabase Auth

**Date**: 2026-05-05
**Author**: Architect Agent
**Context**: Phase 3 monetization (ticket #980). Choosing user-auth provider for MTGA Companion (Go BFF + React SPA + RDS Postgres on AWS).
**User lean**: Clerk (met team at React conference). Override threshold: Supabase must be clearly superior on >=2 of {cost at scale, Go reliability, API key management}.

---

## Summary Recommendation

**Go with Clerk.** It is the better product fit for a Go+React desktop app with first-class API key management, an officially-supported Go SDK, and prebuilt React components — Supabase Auth does not clear the override threshold on any of the three required dimensions.

---

## Side-by-Side Comparison

| Dimension | Clerk | Supabase Auth |
|---|---|---|
| **1. Cost (free tier)** | 10K MRU/app on Hobby (free, $0) | 50K MAU on Free plan |
| **1. Cost (paid entry)** | Pro $25/mo, 10K MRU included, then $0.02/MAU tier 1 | Pro $25/mo, 100K MAU included, then $0.00325/MAU. **Pro is bundled with the whole DB platform — not auth-only** |
| **2. Reliability — public uptime** | 99.95% (status.clerk.com, multi-component shown) | 100.000% Auth component shown on status.supabase.com (rolling window) |
| **2. SLA** | 99.9% on Pro, 99.99% on Enterprise | 99.9% on Team plan, custom on Enterprise |
| **2. Self-host** | No (managed only) | Yes — GoTrue is OSS Go binary; full self-host viable |
| **3. Go BFF fit** | **Official `clerk-sdk-go` v2** with middleware, JWT verification helpers, JWKS caching, and `authenticateRequest()` primitive | **No official Go SDK.** `supabase-community/supabase-go` is community-maintained. JWT verification via standard `golang-jwt` against Supabase JWKS works fine but you write the middleware |
| **4. React fit** | First-class. `<SignIn/>`, `<UserButton/>`, `<Protect/>`, `useAuth()`, `useUser()`. Vite-compatible (`@clerk/clerk-react`). Drop-in | First-class. `@supabase/auth-ui-react`, `@supabase/auth-helpers-react`. Vite-compatible. Less polished UI components, more DIY |
| **5. API keys / M2M** | **Native API Keys + M2M Tokens primitives** (Pro plan: 1K free, $0.001 each after; M2M tokens: 100K verifications free). Built-in dashboard UX for end-users to manage their own keys | **No native API key feature.** Must roll your own (table, hashing, revocation, rate limiting) on top of Postgres. RLS gives you the row-scoping piece, not the issuance/management UX |
| **5. JWT customization** | Session token templates, custom claims, JWT templates per audience | Custom claims via Postgres function (`custom_access_token_hook`). Powerful but Postgres-flavored |
| **5. Tier/role gating** | Organizations + Roles + Permissions built-in; works for tier gating | RLS policies + `auth.jwt()` claims; tier gating done at DB layer |
| **6. Operational complexity** | **Lowest.** Hosted only. Add SDK, configure dashboard, done. No DB migrations for user tables | Medium. Auth is part of your Supabase Postgres. If you self-host: you own GoTrue + Postgres schema. If hosted: tied into your supabase.com project |
| **7. Lock-in risk** | **Medium.** User export available ("Full data exports" listed in Pro). Migration off requires JWT-format swap and rewriting middleware. No way to take the running auth service with you | **Low.** GoTrue is OSS — worst case you self-host the same binary against your own Postgres. JWT format is standard. Schema lives in your DB |
| **Pricing predictability** | High — auth-only, MRU billing is the dominant variable | Lower — auth pricing is bundled into the whole platform plan. Compute, egress, and DB size all stack on top |

MRU (Clerk) = "Monthly Retained User" — a user who signs in or has an active session in the billing month, with a 1-month free grace period after signup. MAU (Supabase) = standard "Monthly Active User."

---

## Cost Breakdown

### Clerk

| Users (MRU) | Plan | Monthly Cost |
|---|---|---|
| 100 | Hobby (free) | **$0** |
| 500 | Hobby (free) | **$0** |
| 1,000 | Hobby (free) | **$0** |
| 5,000 | Hobby (free) | **$0** |
| 10,000 | Hobby or Pro | **$0** (Hobby) or $25 (Pro) |
| 25,000 | Pro | $25 + 15K * $0.02 = **$325** |
| 50,000 | Pro | $25 + 40K * $0.02 = **$825** |

Notes:
- Hobby free tier covers MTGA Companion's projected scale (hundreds to low thousands) at $0/mo.
- Pro is needed once we want: API Keys feature, M2M tokens, custom session lifetime, satellite domains, "Remove Clerk branding," SLA.
- API Keys add-on: 1,000 verifications/mo included on Pro; $0.001 each beyond. Totally fine for our scale.
- A typical real bill at 1K paid users: **$25/mo flat** (still inside the included MRU bucket).

### Supabase

| Users (MAU) | Plan | Monthly Cost (auth portion) | Realistic total bill |
|---|---|---|---|
| 100 | Free | **$0** | $0 (also free DB on shared compute) |
| 500 | Free | **$0** | $0 |
| 1,000 | Free | **$0** | $0 |
| 5,000 | Free | **$0** | $0 |
| 10,000 | Free | **$0** | $0 |
| 50,000 | Free (cap) | **$0** | $0 (auth-only counts; DB compute extra if you use Supabase DB) |
| 100,000 | Pro | **$25** (bundled) | $25 + DB compute |
| 200,000 | Pro | $25 + 100K * $0.00325 = **$350** | $350 + DB compute |

Notes:
- **Important:** Supabase pricing assumes you also use Supabase's Postgres. We already run RDS on AWS. To use Supabase Auth standalone you would either (a) self-host GoTrue against RDS (free but operational burden), or (b) use Supabase's hosted Postgres for the auth schema in addition to RDS — duplicate infra.
- Free tier MAU ceiling (50K) is genuinely better than Clerk's 10K, but at our projected scale the difference is academic ($0 vs $0).
- Auth-only standalone is not Supabase's intended usage model — it is a side-effect of GoTrue being OSS.

### Cost verdict

**At every projected scale point (100, 500, 1K, 5K MAU), both are $0/mo.** Supabase has a higher free ceiling but we never reach it. Clerk's paid plan kicks in at 10K MRU with API Keys included; Supabase's bundled plan price assumes you are also using their database, which we are not. **Cost is not a meaningful differentiator and does not clear the override threshold.**

---

## Red Flags / Caveats

### Clerk
- **MRU billing model.** A returning user is billable each month they sign in. For an MTG Arena tracker where users open the app every weekly draft, expect MRU ~= total active accounts. Not unfair, but plan for it.
- **Hosted-only.** No self-host escape hatch. If Clerk has an outage your users cannot sign in. Mitigate with: aggressive JWT TTLs cached on the daemon, graceful "stale token" mode in BFF.
- **API Keys feature is Pro-tier.** Free Hobby tier does NOT include API Keys. We will need to pay $25/mo the moment we ship the API key UX (likely required for daemon auth refactor).
- **Pricing changes.** Clerk has revised pricing in the past; long-term cost trajectory is not guaranteed.

### Supabase Auth
- **No official Go SDK.** `supabase-community/supabase-go` is community-maintained, lower commit velocity, smaller surface area. JWT verification is fine without it (standard JWKS), but anything beyond "verify token" is DIY.
- **No native API key product.** This is the biggest gap for our use case. Phase 3 needs end-user API key management (daemon credentials, future public API). On Supabase you build it from scratch on `auth.users` + a `api_keys` table + RLS. That is a real chunk of code we do not have to write on Clerk.
- **Tightly coupled to Supabase Postgres.** Self-host of GoTrue against RDS is possible but requires running a separate service, owning migrations for the `auth.*` schema, and matching versions. Nontrivial ops.
- **Two-DB topology if hosted.** Auth schema in supabase.com + product data in RDS = cross-DB joins are no longer free. Awkward.

### Both
- Neither is a drop-in replacement for the other later — migrating user identities (password hashes, OAuth links, MFA secrets) between providers is a multi-week project. Choose carefully now.

---

## Final Verdict

**Choose Clerk.**

Override threshold check (Supabase must win >=2 of {cost-at-scale, Go reliability, API key fit}):

1. **Cost at scale**: Tie. Both $0 at projected scale. Supabase has a higher ceiling but irrelevant. **Not a Supabase win.**
2. **Go reliability**: Clerk wins. Official Go SDK with v2 maturity vs. community-maintained Supabase Go client. **Not a Supabase win.**
3. **API key management fit**: Clerk wins decisively. Native API Keys + M2M tokens product vs. roll-your-own on Supabase. **Not a Supabase win.**

Supabase wins zero of three. The user lean toward Clerk holds.

### Rationale
- **Phase 3 needs API key management.** Clerk gives this as a product. Supabase makes us build it.
- **Go BFF with first-class SDK.** Less middleware code to write and maintain; one less thing that can drift.
- **React SPA with prebuilt UI.** Faster path to a polished sign-in/profile/org-switcher UX. Saves frontend engineering hours we do not have.
- **Operational simplicity.** Hosted-only is a feature for a small team — no GoTrue binary to keep patched, no auth schema migrations.
- **Lock-in risk is acceptable.** Clerk supports user export. If we ever migrate, the cost is real but bounded — and we don't anticipate the move.

### Caveats to acknowledge
- We accept hosted-only and Clerk's outage risk; mitigate with JWT TTLs and a "degraded mode" in the BFF.
- We accept MRU billing and budget for ~$25/mo flat at our current scale once we move to Pro for the API Keys feature.
- We document the migration cost as a known risk in the ADR.

---

## Recommended Next Steps

1. **Write ADR-NNN: User Auth Provider = Clerk** — capture this decision and the override-threshold reasoning.
2. **Spike ticket (backend-engineer)**: Wire `clerk-sdk-go` v2 into the BFF; verify a Clerk-issued JWT in middleware; benchmark JWKS caching latency. ~2 hours.
3. **Spike ticket (front-engineer)**: Drop `<SignIn/>` + `<UserButton/>` into the SPA shell behind a feature flag; verify Vite build clean. ~1 hour.
4. **Design ticket (architect)**: How do daemon credentials fit Clerk API Keys? Does the daemon become a "machine" with an M2M token, replacing `DAEMON_JWT_SECRET`? Decide before implementation.
5. **Pricing ticket (project-manager)**: Confirm budget signoff for $25/mo Pro plan timed to API Keys rollout.
