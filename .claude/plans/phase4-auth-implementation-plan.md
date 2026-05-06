# Phase 4: Authentication (Clerk) — Implementation Plan

**Status:** In Progress — paused 2026-05-06, resuming next session  
**Project board:** v2.0.0 (#27)  
**North star:** 50,000 MAU

---

## Completed ✅

| # | Work | PR |
|---|---|---|
| ADR-009 | Clerk as auth provider decision | #1299 |
| #1257/#1302 | clerk-sdk-go v2 JWT middleware (spike) | #1302 |
| #1258/#1300 | ClerkProvider, AuthBar, ProtectedRoute (spike) | #1300 |
| #1308 | Playwright E2E auth spec (5 tests) | #1308 |
| #1301 | Social login docs (Google/Facebook/Apple) | #1301 |
| #1310 | users table migration (clerk_user_id, subscription_tier) | #1338 |
| #1316 | CLERK_SECRET_KEY in SSM + provisioned to EC2 | manual |
| — | VITE_CLERK_PUBLISHABLE_KEY in GitHub Actions | manual |
| — | Social connections enabled in Clerk Dashboard | manual |

---

## Remaining Open Tickets

| # | Title | Blocked By | Owner |
|---|---|---|---|
| #1312 | feat(backend): implement ClerkAuthMiddleware on all protected routes | — | backend-engineer |
| #1313 | feat(frontend): replace local auth shim with Clerk session | #1312 | front-engineer |
| #1314 | feat(frontend): end-user API key UX (Clerk API Keys) | #1259 (pricing sign-off) | front-engineer |
| #1315 | chore(backend): remove HMAC DAEMON_JWT_SECRET after Clerk M2M cutover | #1312 | backend-engineer |
| #1317 | docs: update forbidden Clerk patterns in CLAUDE.md + agent defs | — | architect |
| #1259 | chore(pm): pricing sign-off — Clerk Pro $25/mo for API Keys | — | Ray |

---

## Sequencing

### Ready to start now (parallel)
- **#1312** (backend) — wire ClerkAuthMiddleware to all protected BFF routes
- **#1317** (architect/docs) — codify forbidden Clerk patterns in CLAUDE.md

### After #1312
- **#1313** (frontend) — replace any local auth state with Clerk hooks throughout SPA
- **#1315** (backend) — remove legacy DAEMON_JWT_SECRET HMAC path

### Gated on pricing sign-off (#1259)
- **#1314** (frontend) — API key UX for power users (requires Clerk Pro $25/mo)

---

## Notes
- Clerk is live and JWT verification is active on the running BFF
- #1314 (API Keys) requires Clerk Pro — Ray needs to decide on the $25/mo upgrade before this can start
- DAEMON_JWT_SECRET removal (#1315) should not happen until Clerk M2M token path is verified working end-to-end
