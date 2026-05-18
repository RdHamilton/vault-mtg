# ADR-007: Frontend Serving Model

**Date**: 2026-05-05
**Status**: Proposed
**Deciders**: Ray Hamilton, Architect Agent
**Supersedes (in part)**: ADR-001 (nginx-served frontend on EC2)
**Refines**: ADR-006 (Vercel→BFF connectivity)

---

## Context

The project currently has **two production frontend serving paths** that were
introduced independently and now conflict:

1. **Vercel** (canonical, established by ADR-006). The React SPA is built and
   deployed to Vercel on every push to `main` from the `frontend/**` path. Vercel
   provides global CDN, atomic preview deploys per-PR, and zero-config TLS.
   Production traffic on `https://vaultmtg.app` is currently served by
   Vercel.

2. **EC2 nginx + S3 + SSM** (introduced by PR #1184, closes #1068). A second
   workflow `.github/workflows/frontend.yml` builds the same SPA with
   `VITE_BFF_URL=/api/v1` and deploys it to `/var/www/vaultmtg/` on the
   EC2 host via S3 upload + SSM `AWS-RunShellScript`. The nginx config in the
   infra repo is already set up to serve `/var/www/vaultmtg/` and to
   proxy `/api/v1/` to the BFF on port 8080.

ADR-006 implicitly assumed Vercel would be the only frontend host. PR #1184 was
merged without updating the ADR, creating an undocumented second serving path.
This is the root cause of the blocked state on tickets #1211 and #1066:

- **#1211** (`fix(infra): remove duplicate Deploy React SPA to EC2 workflow from
  infra repo`) — proposes deleting an old `deploy-frontend.yml` from the infra
  repo and treating the app-repo workflow as canonical, but does not address
  *which* of the two serving paths (Vercel vs. EC2 nginx) is the architectural
  source of truth.
- **#1066** (`docs(sync): rewrite services/sync README for Lambda deployment
  model`) — blocked because deployment docs cannot be updated coherently while
  the frontend serving model is ambiguous.

### Concrete points of conflict

| Concern | Vercel | EC2 nginx |
|---|---|---|
| `VITE_BFF_URL` | `https://api.vaultmtg.app/api/v1` (cross-origin) | `/api/v1` (same-origin) |
| CORS | Required (`ALLOWED_ORIGINS=https://*.vercel.app,...`) | Not required |
| TLS | Vercel-managed | nginx + Let's Encrypt on EC2 |
| Preview deploys | One per PR (Vercel automatic) | None |
| Cost | Free tier (current traffic) | Already paid (EC2 + S3 + SSM) |
| Deploy latency | ~30s | ~2 min (S3 + SSM round-trip) |
| Source of truth | ADR-006 | None — undocumented |

### What "Vercel canonical" implies operationally

- Public DNS (`vaultmtg.app`, `www.vaultmtg.app`) points at Vercel.
- The BFF on EC2 is reachable at a separate API host
  (`api.vaultmtg.app`) and applies CORS for the Vercel origin.
- nginx on EC2 is a *BFF reverse proxy only* — it does not serve static frontend
  assets in production.

### What "EC2 nginx canonical" implies operationally

- DNS points at the EC2 host. Vercel becomes redundant.
- The BFF and frontend are same-origin; no CORS needed.
- The Vercel project, `ALLOWED_ORIGINS` config, and `VITE_BFF_URL` cross-origin
  setup from ADR-006 all become dead code.

These models are mutually exclusive in production. Running both indefinitely
creates confusing deploy semantics (which build is live?), doubles the
surface for breakage, and contradicts ADR-006.

---

## Decision

**Vercel is the canonical production frontend host.**

The EC2 nginx static-serve path is **demoted to optional preview/staging-only**
and is not exercised by any automatic CI workflow on `main`. Its existence is
preserved (nginx config + workflow code on a non-`main` branch or manual
dispatch only) for two specific use cases:

1. **Disaster recovery** — if Vercel is unavailable or the project is removed,
   the EC2 instance can serve the SPA directly with a DNS cutover.
2. **Future internal staging** — when ADR-006's deferred staging environment
   is built out, the same nginx + S3 + SSM machinery may be reused on a
   staging EC2 host. It is *not* used for production traffic on `main`.

### Specifics

1. **Production DNS** (`vaultmtg.app`, `www.vaultmtg.app`,
   `app.vaultmtg.app` if used) **MUST** resolve to Vercel.
2. **`api.vaultmtg.app`** (or whatever subdomain the BFF uses) **MUST**
   resolve to the EC2 instance and serve the BFF only. nginx on EC2 does not
   serve `/` or any static asset paths in production.
3. **`.github/workflows/frontend.yml`** (the EC2 deploy workflow added in PR
   #1184) **MUST** be modified to:
   - remove the `push: branches: [main]` trigger (production builds run on
     Vercel, not on EC2);
   - keep `workflow_dispatch` as the only trigger, used for emergency manual
     deploys;
   - add a top-of-file comment block stating "EC2 frontend deploy is preview/DR
     only — see ADR-007. Production frontend is served by Vercel."
4. **The Vercel deploy** is the only path that runs on push to `main` for
   `frontend/**` changes. (Vercel's git integration handles this — there is no
   GitHub Actions workflow for Vercel in the app repo.)
5. **The infra-repo `deploy-frontend.yml`** referenced by issue #1211 is
   redundant and **MUST** be deleted, as #1211 proposes — this ADR confirms
   that direction.
6. **nginx config in `vault-mtg-infra`** keeps the `/var/www/vaultmtg/`
   `try_files` block for the DR/preview use case but a comment is added marking
   the static-serve location block as "DR/preview only — production is on
   Vercel; see ADR-007".
7. **No code change to the SPA itself** is required. `VITE_BFF_URL` continues
   to be set per environment via the Vercel dashboard (ADR-006). The EC2 build
   path (`VITE_BFF_URL=/api/v1`) remains valid for the manual-dispatch case.

### Rationale for choosing Vercel

- ADR-006 is already accepted and live in production. Reverting it requires a
  DNS cutover, deletion of Vercel project config, and reverting frontend code
  changes that read `VITE_BFF_URL`. EC2 nginx canonical has no compensating
  benefit at current traffic.
- Vercel preview deploys per-PR are already in active use and are valuable for
  PR review — there is no equivalent on the EC2 path.
- TLS on EC2 (cert renewal, nginx reloads) is operational toil that Vercel
  eliminates.
- Vercel cost is $0 at current traffic; the EC2 nginx static-serve adds zero
  marginal value while doubling the deploy surface.
- The EC2 instance's primary job is serving the BFF. Removing static-serve
  responsibility narrows its blast radius.

---

## Consequences

### Easier

- One canonical production frontend serving path (Vercel). No ambiguity in
  "where is the frontend served from?".
- ADR-006 becomes self-consistent — its CORS and `VITE_BFF_URL` decisions are
  the only model that matters for production.
- Issue #1211 becomes resolvable (delete the infra-repo workflow; this ADR
  confirms that's correct).
- Issue #1066 becomes unblocked — deployment docs can describe a single
  production frontend serving path.
- Preview deploys on every PR continue to work for free.

### Harder

- The EC2 frontend deploy workflow becomes near-dead code. It must be
  maintained (or deliberately removed in a follow-on) to avoid bit rot in the
  DR scenario. This ADR explicitly chooses to *keep* the workflow on
  `workflow_dispatch` only — the alternative (delete entirely) is recorded
  below.
- DR cutover from Vercel to EC2 nginx is a manual procedure requiring a DNS
  change. This is acceptable given Vercel's published uptime.
- The nginx config retains an unused `location /` static-serve block in
  production, requiring documentation to make clear it is for DR/preview only.

### Impact on the blocked tickets

- **#1211**: This ADR confirms the proposal — the infra-repo `deploy-frontend.yml`
  must be deleted. Additionally, the app-repo `frontend.yml` workflow must have
  its `push` trigger removed (only `workflow_dispatch` remains). #1211's scope
  expands to cover both edits.
- **#1066**: Unblocked. The deployment README can describe the model
  unambiguously: Vercel for the SPA, EC2 + Lambda for backend services.

---

## Implementation Notes

The implementation work for this ADR is broken into separate tickets coordinated
by the Project Manager. The tracking spec is in
`.claude/plans/adr-007-tickets.md`. At a high level:

1. Edit `.github/workflows/frontend.yml`: remove `push:` trigger, keep
   `workflow_dispatch`, add ADR-007 comment header.
2. Delete `vault-mtg-infra/.github/workflows/deploy-frontend.yml` (this
   work is already represented by #1211; expand its scope to include step 1).
3. Add a `# DR/preview only — see ADR-007` comment to the nginx static-serve
   `location /` block in `vault-mtg-infra`'s nginx config files.
4. Update any developer-facing docs (`README.md`, `docs/DEPLOYMENT.md` if
   present) to state that production frontend is served from Vercel and that
   nginx static-serve is DR-only.
5. Verify production DNS for `vaultmtg.app` resolves to Vercel; if it
   currently resolves to EC2, schedule a DNS cutover ticket.

Each step is a separate Sonnet-ready ticket; see the tickets spec for full
acceptance criteria.

---

## Alternatives Considered

### A. EC2 nginx canonical, retire Vercel

Make EC2 the single production serving path. Delete the Vercel project, revert
the `VITE_BFF_URL` cross-origin work from ADR-006, and remove the BFF's
`ALLOWED_ORIGINS` config. **Rejected** because:

- ADR-006 is already in production and reverting it is non-trivial.
- Loss of Vercel preview deploys per-PR is a meaningful regression for review
  velocity.
- TLS / cert lifecycle on EC2 is real ongoing toil that Vercel eliminates.
- EC2 is a single point of failure; Vercel's CDN survives EC2 outages.

### B. Run both paths in parallel as redundant production hosts

Active-active with weighted DNS or a load balancer. **Rejected** because:

- Doubles the deploy surface and creates cache-coherence questions when one
  path deploys but the other lags.
- No traffic justification — current load is a few RPS.
- Adds DNS / load balancer config the project does not currently have.

### C. Delete the EC2 frontend workflow entirely

Drop `.github/workflows/frontend.yml` and the nginx static-serve block.
**Rejected** because:

- Loses the DR option without compensating benefit. The workflow is small
  (~110 lines) and adds no per-deploy cost when not invoked.
- A future staging environment is likely to reuse the same machinery; deleting
  it now means re-deriving it later.
- The cost of keeping it on `workflow_dispatch` is one comment block and zero
  CI minutes.

This option remains a viable follow-on if the workflow shows signs of bit rot
or if a staging environment lands on a different deploy mechanism.

### D. Use CloudFront in front of EC2 instead of Vercel

Considered and rejected in ADR-006. Not revisited here.
