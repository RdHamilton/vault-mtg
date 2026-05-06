## 2026-05-06 — [compliance-checker] Issue #1317 / PR #1345: docs(arch): Clerk auth forbidden/required patterns + BFF router enforcement

**PR**: #1345
**Ticket**: #1317
**Merged**: 2026-05-06 (without lead-engineer review — retroactive compliance review posted)
**Files changed**:
- `CLAUDE.md` — Added Clerk Authentication section: forbidden patterns, required patterns, agent-specific guidance
- `.claude/agents/backend-engineer.md` — Added rule 14: BFF routes serving user-specific data must be inside `ClerkAuthMiddleware` group
- `.claude/agents/front-engineer.md` — Added rule 10: no local auth state mirroring Clerk session; use `useAuth()`/`useUser()` at call site
- `services/bff/cmd/main.go` — Extracted `BuildRouter` as standalone function; moved `/api/v1/draft-ratings` behind Clerk-protected `chi.Group`; restructured all protected routes under explicit auth group
- `services/bff/cmd/router_test.go` — New: 351-line integration test suite verifying public/protected route boundaries with mock Clerk JWKS server

**Summary**: Codified Clerk authentication rules into CLAUDE.md and agent definitions (ADR-009 compliance), and enforced them structurally in the BFF by extracting a testable `BuildRouter` function and placing all user-facing routes inside the `ClerkAuthMiddleware` group — including `/api/v1/draft-ratings` which was previously public.

**Compliance violations flagged (retroactive)**:
- CRITICAL: PR labeled `docs:` but contained behavioral Go code changes (security-posture change on `/api/v1/draft-ratings`) — required lead-engineer review
- HIGH: `go test -race ./...` not run; only `go test ./cmd/... -run TestRouter` was executed
- MEDIUM: PR merged with unchecked test plan item (BFF integration smoke test)
- LOW: `sk_test_dummy` literal in test file matches `sk_*` pattern — rename to avoid scanner false positives
