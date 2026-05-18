# ADR-008: Frontend Serving Model: S3+CloudFront Canonical, Vercel Preview-Only

**Date**: 2026-05-05
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Supersedes**: ADR-001 (original frontend deploy on EC2 nginx), ADR-007 (Vercel canonical — that decision is now reversed)

---

## Context

Phase 3 of the project introduces a brand split and two newly registered
production domains:

- **`vaultmtg.app`** — public marketing site for the rebranded product.
- **`app.vaultmtg.app`** — the React SPA (formerly served at
  `vaultmtg.app`).
- **`rhamiltoneng.com`** — the Ray Hamilton Engineering portfolio / engineering
  brand site (separate Next.js project, repo `vault-mtg-web`).

ADR-007 declared **Vercel** the canonical production host for the React SPA.
That decision was correct given the constraints at the time (no AWS budget,
single domain, no marketing site). Three things have changed since:

1. **AWS Activate credits ($1,000) were approved on 2026-05-05.** The cost
   blocker that originally favored Vercel's free tier no longer applies.
   S3+CloudFront for three sites is approximately **$4/month** of credit
   consumption.
2. **Three frontends now ship in parallel.** Vercel's free tier limits on
   custom domains, team members, and concurrent builds become friction at this
   surface area. Two of the three sites (vaultmtg.app, rhamiltoneng.com) are
   marketing/portfolio sites where Vercel cold starts on edge functions
   produce visible latency on first paint.
3. **We need full routing control.** The React SPA on `app.vaultmtg.app`
   requires SPA fallback (`/*` → `index.html`); the marketing site on the apex
   `vaultmtg.app` requires distinct caching, security headers, and redirect
   rules; `rhamiltoneng.com` is a third independent property. Mixing these on
   Vercel is possible but each rule is configured per-project in a vendor
   dashboard rather than as code in our infra repo.

S3+CloudFront with ACM-managed TLS gives us:

- A single canonical serving layer that we own end-to-end.
- Full routing control (CloudFront distributions + Origin Request policies +
  Function code) defined in CloudFormation in `vault-mtg-infra`.
- Custom-domain TLS via ACM at no additional cost.
- A predictable, credit-funded cost (~$4/month for three distributions at
  current traffic).
- No cold starts on first paint of any of the three properties.

Vercel's value proposition (preview deploys per PR, zero-config TLS, instant
rollbacks) remains real for **PR review** and is already wired up via the
tag-based `ignoreCommand` from PR #1238. Demoting Vercel to preview-only keeps
that benefit while removing it from the production path.

---

## Decision

**S3+CloudFront is the canonical production serving layer for all three
frontends.**

### Specifics

1. **Canonical production hosts (S3+CloudFront):**
   - `app.vaultmtg.app` — React SPA bucket + CloudFront distribution with SPA
     fallback (`/*` → `/index.html`, 200 status).
   - `vaultmtg.app` (and `www.vaultmtg.app`) — marketing site bucket +
     CloudFront distribution. `www` redirects to apex via CloudFront Function
     or S3 redirect bucket.
   - `rhamiltoneng.com` (and `www.rhamiltoneng.com`) — Ray Hamilton
     Engineering site bucket + CloudFront distribution.
2. **TLS:** ACM certificates (us-east-1, required for CloudFront) cover all
   three apex + `www` hostnames. Certificates are managed in CloudFormation
   in `vault-mtg-infra`.
3. **Vercel:** demoted to **PR preview-only**. Its `ignoreCommand` is already
   tag-based per PR #1238 — production tags do not deploy to Vercel.
   Preview deploys on every PR continue to work and are valuable for review.
   Vercel does **not** serve any production traffic.
4. **EC2 nginx:** serves **`api.vaultmtg.app` (BFF/API) only**. nginx no
   longer serves frontend static assets. The static-serve `location /` block
   in the nginx config is removed (or commented out as DR-only — see
   "Consequences" below).
5. **CI/CD:** GitHub Actions on production tag pushes runs `aws s3 sync` to
   the appropriate bucket and issues a CloudFront invalidation
   (`/index.html` plus any non-fingerprinted asset paths). One workflow per
   property, scoped by path filter (`frontend/**` for the SPA;
   `vault-mtg-web/**` for the marketing site; the `vault-mtg-web` repo
   for `rhamiltoneng.com`).
6. **Rollback:** the previous build remains in S3 under a versioned prefix
   (or via S3 versioning); rollback is `aws s3 sync` from the prior version
   plus a CloudFront invalidation. Documented in `RELEASE_CHECKLIST.md` as a
   follow-on.

### What this reverses from ADR-007

- ADR-007 §Decision "Vercel is the canonical production frontend host" — now
  reversed. Vercel is preview-only.
- ADR-007 §Specifics #1 (DNS for `vaultmtg.app` points at Vercel) — moot;
  the new DNS targets are `app.vaultmtg.app`, `vaultmtg.app`, and
  `rhamiltoneng.com`, all pointing at CloudFront.
- ADR-007 §Specifics #6 (nginx static-serve block kept for DR) — superseded.
  EC2 nginx is API-only; no static assets.
- ADR-006 §CORS configuration — still applies, with `ALLOWED_ORIGINS` now
  set to `https://app.vaultmtg.app` (production) plus the preview-deploy
  glob for Vercel. This is covered by implementation ticket #1251.

---

## Consequences

### Positive

- **Single authoritative production serving layer.** All three properties
  share one model (S3+CloudFront+ACM), defined as code in
  `vault-mtg-infra`.
- **Full routing control.** SPA fallback, redirects, security headers,
  caching policies, and Origin Request behavior are all CloudFormation —
  versioned, reviewable, and reproducible.
- **Cost-efficient under AWS Activate credits.** Three distributions at
  current traffic is ~$4/month, fully covered by the $1,000 credit balance
  through the foreseeable future.
- **No cold starts on production SPA or marketing pages.**
- **Vercel previews still work for PR review.** No regression in review
  velocity; the tag-based `ignoreCommand` from PR #1238 already prevents
  Vercel from deploying production tags.
- **EC2 blast radius narrows.** The instance's job becomes "serve the BFF
  on `api.vaultmtg.app`" — no static-asset responsibility.

### Negative

- **More CloudFormation to maintain.** Three CloudFront distributions,
  three S3 buckets, three ACM certs, and Route53 records for two new domains.
  This work lands in `vault-mtg-infra` via tickets #1246 and #1253.
- **Deploy pipeline complexity.** `aws s3 sync` + CloudFront invalidation is
  more steps than `vercel --prod`. Mitigated by codifying the deploy steps
  in a reusable GitHub Actions workflow (ticket #1249).
- **CloudFront invalidation latency.** Edge propagation can take 30–60s
  versus Vercel's near-instant cutover. Acceptable for our deploy cadence
  and traffic level.
- **DR / multi-region story is unchanged.** A single CloudFront distribution
  is global but a single S3 origin is regional. If we later need
  multi-region origin failover, that is a separate ADR.

---

## Implementation Tickets

The following implementation tickets cover the work to land this ADR. All are
tracked in milestone #61 ("Phase 3: VaultMTG Brand & Production
Infrastructure") on project board #27.

| Ticket | Scope | Owner |
|---|---|---|
| **#1246** | CloudFormation for `vaultmtg.app` + `app.vaultmtg.app` (S3 buckets, CloudFront distributions, ACM certs, Route53 records, OAC) | infrastructure |
| **#1253** | CloudFormation for `rhamiltoneng.com` (S3 bucket, CloudFront distribution, ACM cert, Route53 records, OAC) | infrastructure |
| **#1247** | nginx config update for `api.vaultmtg.app` only (remove static-serve block, update server_name, update TLS cert path) | infrastructure |
| **#1249** | CI/CD pipeline — GitHub Actions workflows for `aws s3 sync` + CloudFront invalidation on release tag, one per property | infrastructure |
| **#1251** | Update `ALLOWED_ORIGINS` SSM parameter on EC2 BFF to reflect new origins (`https://app.vaultmtg.app` plus Vercel preview glob) | backend |
| **#1252** | Update `VITE_API_URL` in the React SPA build config (and any environment files) to `https://api.vaultmtg.app/api/v1` | frontend |

Each ticket has its own acceptance criteria; this ADR is the architectural
source of truth they reference.

---

## Alternatives Considered

### A. Keep ADR-007 in force — Vercel canonical for all three properties

Add `vaultmtg.app` and `rhamiltoneng.com` as additional Vercel projects.
**Rejected** because:

- Vercel free-tier custom-domain and team-member limits become a friction
  point at three properties.
- Cold starts on edge functions produce visible latency on marketing-site
  first paint.
- Routing rules (especially the SPA fallback for `app.vaultmtg.app` plus
  apex/`www` redirects for the two marketing sites) are configured
  per-project in a vendor dashboard rather than as code we own.
- AWS Activate credits eliminate the original cost rationale that drove
  ADR-007.

### B. Mix — Vercel for the SPA, S3+CloudFront for the marketing sites

Keep `app.vaultmtg.app` on Vercel (as ADR-007 directs) and put
`vaultmtg.app` + `rhamiltoneng.com` on S3+CloudFront. **Rejected** because:

- Two production serving models doubles the operational surface
  (CI/CD, DNS, TLS cert lifecycle, deploy semantics).
- The SPA is the highest-traffic property and benefits most from
  CloudFront-edge cache control. Keeping it on Vercel forfeits that.
- "One canonical serving layer" was a stated goal of ADR-007; this option
  abandons it.

### C. EC2 nginx canonical for all three properties

Run nginx on the existing EC2 instance and serve all three sites from
`/var/www`. **Rejected** because:

- EC2 is a single point of failure with no edge cache.
- TLS cert lifecycle (Let's Encrypt renewal, nginx reload) is operational
  toil that ACM eliminates.
- Marketing-site latency from a single us-east-1 instance is worse than
  CloudFront-edge for global visitors.
- The instance is already at modest capacity; static-serve duty competes
  with the BFF.

### D. CloudFront in front of EC2 (cache, but origin is EC2 nginx)

**Rejected** because:

- nginx still owns static assets, which means we still maintain that path
  on the instance.
- S3 is a cheaper, more durable origin than EC2 for static assets.
- This option doesn't actually simplify anything versus S3 origin.

---

## References

- ADR-001 — original nginx-served frontend on EC2 (superseded).
- ADR-006 — Vercel→BFF connectivity model (CORS + cross-origin BFF). Still
  applies; `ALLOWED_ORIGINS` updated per ticket #1251.
- ADR-007 — Vercel canonical (superseded by this ADR).
- PR #1238 — tag-based Vercel `ignoreCommand` (production tags do not deploy
  to Vercel).
- AWS Activate credits — $1,000 approved 2026-05-05.
