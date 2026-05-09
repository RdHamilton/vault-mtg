# ADR-002: CI/CD Path-Filtered, Tiered Test Strategy

**Date**: 2026-05-03
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent

---

## Context

The project exhausted its 2,000 GitHub Actions minutes/month private-repo limit. Root cause: every CI job runs on every PR regardless of which module changed. The monorepo contains three independently deployable concerns — daemon (`services/daemon/`), BFF (`services/bff/`), and frontend (`frontend/`) — plus a shared contract module (`services/contract/`). Running all jobs for a one-line frontend typo fix wastes ~30–60 minutes per PR.

Additionally, expensive test tiers (integration, Playwright E2E) run on every PR, when they only need to gate merges to `main` or releases.

### Observed waste sources

| Workflow | Trigger | Problem |
|---|---|---|
| `ci.yml` — Go Unit Tests | All PRs | Runs monorepo root Go tests even when only frontend changed |
| `ci.yml` — Frontend Component Tests | All PRs | Runs `npm test` even when only daemon changed |
| `daemon.yml` — Build matrix (3 binaries) | PRs touching daemon | Build matrix is appropriate; runs are legitimate |
| E2E workflows | `workflow_dispatch` only | Already gated — not a waste source |

---

## Decision

Adopt a two-axis strategy:

### Axis 1 — Path filtering (which module changed?)

Each CI job triggers only when its owning module's files change:

| Job | Path filter |
|---|---|
| BFF unit tests | `services/bff/**`, `services/contract/**`, `.github/workflows/ci.yml` |
| Frontend component tests | `frontend/**`, `.github/workflows/ci.yml` |
| Daemon tests + build | `services/daemon/**`, `services/contract/**`, `.github/workflows/daemon.yml` |

The root monorepo Go module (`ci.yml` `test` job) is **deprecated**. BFF now owns its own test job scoped to `services/bff/`. The root `go.mod` / `go.sum` are legacy artifacts from before the service split; they are not tested in CI going forward.

### Axis 2 — Tier gating (how expensive is the job?)

| Tier | Runs on | Jobs |
|---|---|---|
| **Tier 1 — Fast** | Every PR (path-filtered) | Go unit tests (daemon, BFF), frontend component tests |
| **Tier 2 — Integration** | Push to `main` only | BFF integration tests (requires DB), heavier Go tests |
| **Tier 3 — E2E / Release** | Release workflow (`workflow_dispatch`) | Playwright smoke, pipeline, full, cross-browser |

### Concrete workflow changes

1. **`ci.yml`** — Split into two path-filtered jobs:
   - `bff-unit-tests`: triggered by `services/bff/**` or `services/contract/**`
   - `frontend-component-tests`: triggered by `frontend/**`
   - Add a `bff-integration-tests` job gated to `push` on `main` (not PRs)
   - Remove the legacy root-module `test` job

2. **`daemon.yml`** — Already path-filtered correctly. No structural change needed. Build matrix (3 binaries) remains on PR; release job remains tag-gated.

3. **E2E workflows** (`e2e-smoke.yml`, `e2e-pipeline.yml`, `e2e-full.yml`) — Remain `workflow_dispatch` only. Called by `release.yml` on release. No change needed.

4. **`release.yml`** — Add a `workflow_call` trigger so the release workflow can be composed into future automated release pipelines.

5. **`release-gui.yml`**, **`screenshots.yml`**, **`dependency-submission.yml`** — Already `workflow_dispatch` only. No change.

---

## Consequences

**Easier**
- PRs touching only frontend do not start Go build runners.
- PRs touching only daemon do not start Node.js runners.
- Expensive integration and E2E tests no longer consume PR minutes.
- CI minute usage should drop ~60–70% based on typical PR distribution.

**Harder**
- Required status checks in GitHub branch protection must be updated. Jobs that are skipped due to path filters will not appear as "passed" — use the `paths-ignore` approach or configure required checks as optional where path-filtered skips are expected.
- Integration tests no longer gate PR merges; a bad integration regression could reach `main` before being caught. Mitigated by requiring PRs to pass unit tests and by the integration tier running immediately on merge.

**Neutral**
- `daemon.yml` already had correct path filters — no minute waste from that workflow.
- E2E workflows were already `workflow_dispatch` only — no change needed.

---

## Alternatives Considered

### A — Self-hosted runner
Run a Mac Mini or EC2 runner to avoid minute billing. Rejected: operational overhead, still need cloud runners for cross-platform daemon builds.

### B — Reduce parallelism in build matrix
Build only one daemon platform on PRs. Partially adopted: build matrix is fine since it only runs when daemon files change (already path-filtered). Not worth reducing further.

### C — Keep monorepo root tests, add path filter
Add a `paths` filter to the existing root `test` job. Rejected: the root `go.mod` is a legacy artifact from before the service split. It doesn't represent the BFF module. Better to point the test job directly at `services/bff/`.

### D — Merge all workflows into one file with job-level conditions
Use `if:` conditions on each job instead of path triggers on the workflow. Rejected: job-level `if:` does not prevent runners from spinning up and checking out the repo — it only skips the job steps. Path filters at the workflow `on:` level prevent the runner from being provisioned entirely.

---

## Implementation Tickets

- **#1037** — Implement path-filtered CI (Tier 1 PR jobs, infrastructure agent)
- **#1038** — Implement tiered test strategy (integration + E2E tier gating, infrastructure agent)
