- Always write Playwright e2e tests for new UI and UI changes
- Use the REST API adapter for new components (enables E2E testing)
- Run `npx tsc --noEmit` for TypeScript type checking (separate from vitest)
- Run `npm run test:run` to run vitest component tests

## Local Go Build Setup (Required)

Local Go builds require `GOPRIVATE=github.com/RdHamilton/vault-mtg` to be set. Without it, `go build`, `go mod tidy`, and `go test` fail with a cryptic 404 from `proxy.golang.org` because the public proxy holds stale pre-rename cached module versions (root cause: ADR-023 Addendum II — module proxy cache poisoning post-rename).

One-time setup:

```bash
go env -w GOPRIVATE=github.com/RdHamilton/vault-mtg
```

`scripts/dev.sh` also exports this defensively, so `./scripts/dev.sh build` works without the one-time setup. CI workflows (`integration.yml`, `daemon.yml`, `deploy-script-integration-test.yml`) already set `GOPRIVATE` at the job level — this requirement only affects local developer machines.

## Context Management - Use Subagents

To keep the main conversation focused and avoid context bloat:

- **Investigation/Exploration**: Use the `Explore` subagent to find code, trace implementations, or answer "where is X?" questions. Return only the summary to main context.
- **Planning**: Use the `Plan` subagent for designing implementations before writing code. Get user approval on the plan before implementing.
- **Parallel research**: When multiple things need investigation, spawn parallel agents to research simultaneously. Cap at 2 concurrent agents.
- **Self-contained tasks**: Use `general-purpose` subagent for tasks like "run tests and summarize failures" or "check all files importing X".

The main conversation should focus on:
- High-level goals and decisions
- Implementation based on agent summaries
- User communication and approval

Avoid in main context:
- Reading many files directly (use Explore agent)
- Long investigation chains
- Raw test output dumps

## Memory Management

To prevent laptop memory spikes when running agents:

- **Max 3 concurrent agents, maximize utilization** — never spawn more than 3 agents in a single message. If only one agent type has queued work, all 3 slots can go to that type. Across multiple types, fill up to 3 total. Serialize anything beyond 3.
- **Use `haiku` model for simple agents** — pass `model: "haiku"` to the Agent tool for any task that is: moving a ticket, writing a changelog entry, reading files, or short research queries. Reserve default (sonnet) for implementation tasks.
- **One instance per type by default** — prefer running one instance of a given agent type at a time. Run multiple only when the work is fully independent (e.g., 3 separate tickets with no shared files).
- **Foreground over background for long tasks** — `run_in_background: true` keeps a full process alive until it completes. Only use background when you have genuinely independent parallel work to do immediately.
- **Context compaction is automatic** — Claude Code compacts context automatically when the window fills. The `/compact` command triggers it early if needed.

## Authentication (Clerk)

Clerk is the authentication provider for VaultMTG (ADR-009). All user-facing auth flows through Clerk — both the React SPA (Clerk JS SDK) and the Go BFF (`clerk-sdk-go v2` for JWT verification). The legacy HMAC user-auth path is being removed in #1315.

### Forbidden patterns — never do these

- Do not roll custom JWT signing or verification logic. Use `clerk-sdk-go v2` only on the BFF; use the Clerk React SDK only on the frontend.
- Do not read `DAEMON_JWT_SECRET` for user-facing auth. That secret is M2M (daemon → BFF ingest) only and will be removed after #1315.
- Do not store Clerk session tokens in `localStorage`, `sessionStorage`, or hand-rolled cookies. The Clerk SDK manages session lifecycle — let it.
- Do not bypass `ClerkAuthMiddleware` on protected BFF routes. No `// TODO: add auth later` placeholders, no commented-out middleware lines.
- Do not expose `CLERK_SECRET_KEY` (or any `sk_*` value) in frontend bundles, browser-shipped env vars, logs, error messages, or PR descriptions. Only `VITE_CLERK_PUBLISHABLE_KEY` (`pk_*`) belongs in the frontend.
- Do not use Clerk dev/test keys (`pk_test_*`, `sk_test_*`) in production. Production uses `pk_live_*` / `sk_live_*` only.
- Do not duplicate Clerk session state in Redux, Context, Zustand, or component state. The Clerk hooks are the single source of truth for auth state.

### Required patterns — always do these

- Every protected BFF route MUST be mounted under the `ClerkAuthMiddleware`-wrapped router group. New routes serving user-specific data are protected by default.
- Extract the authenticated user id from request context using the established helper (e.g., `auth.UserIDFromContext(ctx)`) — never parse raw JWT claims by hand in handlers.
- Frontend auth state comes from Clerk hooks (`useAuth()`, `useUser()`, `useSession()`) only. If a component needs the user id, call `useUser()` — do not pass it through props from a custom store.
- Wrap every authenticated page/route in the React router with `ProtectedRoute`. Public routes (marketing, sign-in, sign-up) are explicit exceptions.
- Multi-tenancy: the Clerk user id resolves to an `account_id` server-side — every user-data query still scopes by `account_id`.

### Agent-specific guidance

- **backend-engineer**: When adding a new BFF route, ask first whether it serves user-specific data. If yes, mount it inside the Clerk-protected route group — never leave it open. If a route is intentionally public (health, public metadata), call that out explicitly in the PR description.
- **front-engineer**: Do not introduce local auth state (Redux slice, Context, Zustand store) that mirrors Clerk session state. Use `useAuth()` / `useUser()` directly at the call site. Do not read or write Clerk session tokens manually.

## Dependency Management

- When upgrading a dependency with a **major version bump**: read the changelog for breaking changes before merging. Create a companion fix PR (or a tracking ticket) for any deprecated API call sites. Never merge a major-version upgrade without either a companion migration or an explicit acceptance note in the PR.
- If you upgrade a dep and the changelog mentions deprecated APIs you call: fix the call sites in the same PR, not in a follow-up.

## Test Coverage Guidelines
- Always update UI/component tests when making UI changes
- Add integration tests for backend changes (repository, handlers, services)
- Add missing test coverage to files that are touched but lacking coverage
- Test types required for code changes:
  - Unit tests: For utility functions and business logic
  - Component tests: For React components (MatchHistory.test.tsx pattern)
  - Integration tests: For backend repository/handler changes (match_repo_test.go pattern)
  - E2E tests: For critical user flows when applicable
