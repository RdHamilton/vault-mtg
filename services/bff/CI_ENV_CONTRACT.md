# BFF CI Environment Contract

All CI jobs that start the BFF process MUST set these env vars explicitly.
Never rely on `config.Load()` defaults — the default is `production`, which is always wrong in CI.

---

## Unit Tests

BFF unit tests run with `-short` and do not start the BFF process. No BFF env vars are
required, but Go module flags are always set to avoid GOSUM/GOPRIVATE failures.

| Var | Value | Notes |
|-----|-------|-------|
| GONOSUMDB | `github.com/RdHamilton/MTGA-Companion` | Allows private module downloads without checksum verification |
| GOPRIVATE | `github.com/RdHamilton/MTGA-Companion` | Bypasses public GOPROXY for private modules |

Workflow: `ci.yml` — job `bff-unit-tests`

---

## Integration Tests

Integration tests start the BFF against a real Postgres service container. No Clerk key is
required — integration tests exercise repository and handler logic below the auth layer.

| Var | Value | Notes |
|-----|-------|-------|
| DATABASE_URL | `postgres://mtga:mtga@localhost:5432/mtga_test?sslmode=disable` | Points to the `postgres:16` service container |
| INTEGRATION_TEST | `"true"` | Guard flag — integration test files skip when absent |
| GONOSUMDB | `github.com/RdHamilton/MTGA-Companion` | Same as unit tests |
| GOPRIVATE | `github.com/RdHamilton/MTGA-Companion` | Same as unit tests |

Workflow: `integration.yml` — job `bff-integration-tests`

---

## E2E Shards (ci.yml)

E2E shard jobs download a pre-built BFF binary and start it in-process alongside the
Playwright runner. There is no database — tests rely on in-memory fixtures served over SSE.

| Var | Value | Notes |
|-----|-------|-------|
| CI | `true` | Standard Playwright flag; adjusts timeouts and disables interactive output |
| MTGA_ENV | `development` | Required — prevents `config.Load()` from defaulting to `production`, which would require `DATABASE_URL` and `CLERK_SECRET_KEY` and cause a 503 on SSE routes |
| BFF_E2E_UNGUARDED_SSE | `true` | Mounts `GET /api/v1/events` with sentinel middleware instead of Clerk auth — allows pipeline log-fixture tests to receive SSE without a database or Clerk key. Safe only because `MTGA_ENV=development`; NEVER set in staging or production |

Workflow: `ci.yml` — job `frontend-e2e` (matrix shard 1-4)

---

## E2E Smoke (e2e-smoke.yml)

Smoke tests run against a real Postgres service container and a real Clerk staging key.
This is the full production-like stack minus live infrastructure.

| Var | Value | Notes |
|-----|-------|-------|
| CI | `true` | Same as E2E shards |
| DATABASE_URL | `postgres://mtga:mtga@localhost:5432/mtga_e2e?sslmode=disable` | Points to the `postgres:16` service container (db name differs from integration tests) |
| CLERK_SECRET_KEY | `${{ secrets.CLERK_STAGING_SECRET_KEY }}` | Clerk staging key stored as a GitHub Actions secret — must NOT be a production key |

Must stay in sync with the E2E Shards block above: if `MTGA_ENV` or `BFF_E2E_UNGUARDED_SSE`
semantics change in the shard job, review whether smoke tests are also affected.

Workflow: `e2e-smoke.yml` — job `e2e-smoke`

---

## Adding a New BFF Env Var

When adding a new BFF env var required by any CI job:

1. Add it to the relevant section(s) in this file.
2. Update the corresponding job's `env:` block in the workflow YAML.
3. Include both changes in the same PR — never split them.
4. If the var is a secret, store it in GitHub Actions Secrets (for CI) and in AWS SSM
   Parameter Store (for production/staging). Document the SSM path in `release.yml`
   and the `secrets-inventory` job's parameter list.
