# Deployment

This document describes the production deployment model for VaultMTG and
the two related properties owned by the same operator (`vaultmtg.app` and
`rhamiltoneng.com`). It is the source of truth for **how production traffic is
served and how releases ship**.

The architectural decision behind this model is recorded in
[ADR-008: Frontend Serving Model — S3+CloudFront Canonical, Vercel
Preview-Only](adr/ADR-008-frontend-serving-model.md). ADR-008 supersedes
ADR-001 (EC2 nginx canonical) and ADR-007 (Vercel canonical). If anything in
this document conflicts with ADR-008, ADR-008 wins and this document is the
bug.

---

## Architecture overview

Production has three frontend properties and one backend service. They are
served from two distinct AWS layers:

```
                              ACM (us-east-1, public certs)
                                          |
                                          v
        +----------------------------- CloudFront -----------------------------+
        |                                  |                                   |
        v                                  v                                   v
  app.vaultmtg.app                  vaultmtg.app                       rhamiltoneng.com
  (React SPA)                       (marketing site)                   (engineering site)
        |                                  |                                   |
        v                                  v                                   v
  s3://vaultmtg-app-spa         s3://vaultmtg-app-marketing            s3://rhamiltoneng-site

  -----------------------------------------------------------------------------

                              api.vaultmtg.app
                                    |
                                    v
                              EC2 (t3.small)
                              nginx :443  -->  Go BFF :8080
                              (API only — no static assets)
                                    |
                                    v
                              RDS PostgreSQL
                              (multi-tenant, account_id-scoped)
```

Key properties of this model:

- **Three frontends, one serving model.** Every production frontend is
  S3-origin + CloudFront distribution + ACM TLS. There is no per-property
  custom infrastructure.
- **EC2 is API-only.** `api.vaultmtg.app` is the only hostname served by the
  EC2 nginx instance. The nginx config does not contain a `location /`
  static-serve block. See [`infrastructure/nginx/api.vaultmtg.app.conf`](../infrastructure/nginx/api.vaultmtg.app.conf).
- **Vercel is preview-only.** Production tags (`v*`) skip Vercel deploys via
  `vercel.json` `ignoreCommand`. Vercel still builds preview deploys for every
  PR (review use only). Vercel does **not** serve any production hostname.
- **No infrastructure IDs in source.** S3 bucket names and CloudFront
  distribution IDs are read from SSM Parameter Store at deploy time. CI
  workflows know the SSM parameter names; they do not know the bucket or
  distribution IDs.

---

## Frontend properties

| Property | Hostname | Repo | S3 bucket | CloudFront SSM | Build dir |
|---|---|---|---|---|---|
| VaultMTG SPA | `app.vaultmtg.app` | `RdHamilton/vault-mtg` | `vaultmtg-app-spa` | `/vaultmtg/production/spa-distribution-id` | `frontend/dist/` |
| VaultMTG Marketing | `vaultmtg.app` (apex + `www`) | `RdHamilton/vault-mtg-web` | `vaultmtg-app-marketing` | `/vaultmtg/production/marketing-distribution-id` | per-repo build output |
| Ray Hamilton Engineering | `rhamiltoneng.com` (apex + `www`) | `RdHamilton/rhamiltoneng-web` | `rhamiltoneng-site` | `/rhamiltoneng/production/distribution-id` | per-repo build output |

Each CloudFront distribution has:

- An ACM certificate in `us-east-1` covering its apex hostname plus `www` if
  applicable.
- An Origin Access Control (OAC) for its S3 origin so the bucket is private.
- A default cache policy and (for `app.vaultmtg.app`) an SPA-fallback rule
  that maps any unmatched path to `/index.html` with a 200 status.

The CloudFront and S3 resources for `vaultmtg.app` + `app.vaultmtg.app` are
defined in
[`infrastructure/cloudformation/vaultmtg-app-cdn.yaml`](../infrastructure/cloudformation/vaultmtg-app-cdn.yaml).
The resources for `rhamiltoneng.com` are defined in
[`infrastructure/cloudformation/rhamiltoneng-cdn.yaml`](../infrastructure/cloudformation/rhamiltoneng-cdn.yaml).

---

## Backend (BFF) — `api.vaultmtg.app`

The Go BFF runs as a systemd service on a single EC2 instance behind nginx.

- **Hostname:** `api.vaultmtg.app`
- **Port (BFF):** `127.0.0.1:8080`
- **Port (nginx):** `:443` (TLS), `:80` (Certbot ACME challenge + HTTP→HTTPS
  redirect)
- **TLS:** Let's Encrypt via Certbot. nginx config preserves Certbot-managed
  blocks across renewals.
- **Rate limit:** `30r/m` per source IP on `/api/v1/`, burst 10 (configured in
  the nginx server block).
- **Static files:** none. There is no `location /` static-serve block. If a
  request lands on the EC2 host that is not `/api/v1/*` or `/health`, nginx
  rejects it.

The nginx config lives at
[`infrastructure/nginx/api.vaultmtg.app.conf`](../infrastructure/nginx/api.vaultmtg.app.conf).

The daemon binary (the local agent that reads `Player.log` and POSTs to the
BFF) ships via GitHub Releases — see [Daemon Installation](DAEMON_INSTALLATION.md).

---

## Deploy process

All three frontend properties use the same shape of deploy workflow:

1. Trigger fires.
2. Workflow checks out the source.
3. Workflow installs deps and builds the static output (e.g.
   `npm ci && npm run build`).
4. Workflow assumes the OIDC deploy role.
5. Workflow reads bucket name + distribution ID from SSM at runtime.
6. Workflow runs `aws s3 sync <build-dir>/ s3://<bucket> --delete`.
7. Workflow issues a CloudFront invalidation for `/*`.

### Triggers

| Trigger | Behavior |
|---|---|
| `push: tags: ['v*']` | Production deploy. Pushes the tag's build to the canonical S3 bucket and invalidates CloudFront. |
| `workflow_dispatch` | Manual deploy from the GitHub Actions UI. Useful for re-running an immediate redeploy without cutting a new tag. |

There is no `push: branches: [main]` trigger on any production frontend
deploy workflow. Merging to `main` does **not** ship to production. Cutting
a tag does.

### Auth — OIDC, no long-lived AWS keys

Every deploy workflow assumes a single IAM role via GitHub OIDC:

- **Role:** `github-actions-oidc-deploy`
- **Repo secret:** `AWS_DEPLOY_ROLE_ARN` (set per repo)
- **Workflow permissions:** `id-token: write`, `contents: read`
- **Action:** `aws-actions/configure-aws-credentials@v4`

The role's trust policy restricts the assumable repos to:

- `RdHamilton/vault-mtg`
- `RdHamilton/vault-mtg-web`
- `RdHamilton/rhamiltoneng-web`

No static AWS access keys are stored in GitHub secrets.

### Workflow location

| Property | Workflow |
|---|---|
| VaultMTG SPA | [`.github/workflows/deploy-spa.yml`](../.github/workflows/deploy-spa.yml) |
| VaultMTG Marketing | `.github/workflows/deploy-*.yml` in `RdHamilton/vault-mtg-web` |
| rhamiltoneng.com | `.github/workflows/deploy-*.yml` in `RdHamilton/rhamiltoneng-web` |

---

## How to trigger a production deploy

### Tag-based (the normal path)

```bash
# from the property's repo, on a clean main:
git fetch origin && git checkout main && git pull origin main
git tag -a v1.5.0 -m "Release v1.5.0"
git push origin v1.5.0
```

The deploy workflow runs automatically. Watch it under the **Actions** tab of
the property's repo.

### Manual dispatch (re-deploy current main)

GitHub UI: **Actions → "Deploy SPA to S3 + CloudFront" (or equivalent) → Run
workflow → Branch: `main`**.

Use this when:

- You need to retry a failed deploy without bumping a version.
- An SSM parameter has changed (e.g., a new build-time env var) and the
  current production build needs to pick it up.

### Verification after deploy

1. Open the production hostname in a browser and confirm the new build is
   served (check a known asset hash or a footer build-stamp).
2. From a terminal, confirm the cache invalidation finished:
   ```bash
   aws cloudfront list-invalidations \
     --distribution-id <id-from-ssm> \
     --region us-east-1 \
     --profile personal
   ```
3. For the SPA, hit a deep route directly (e.g., `https://app.vaultmtg.app/decks`)
   and confirm SPA fallback returns the new `index.html`.

---

## Rollback

S3 versioning is enabled on all three production buckets. Rollback is a
re-sync from a prior version plus a CloudFront invalidation:

```bash
# 1. Identify the previous good version:
aws s3api list-object-versions \
  --bucket vaultmtg-app-spa \
  --prefix index.html \
  --region us-east-1 \
  --profile personal

# 2. Re-sync from that version (typically: re-run the deploy workflow on the
#    previous tag via workflow_dispatch).

# 3. Invalidate:
aws cloudfront create-invalidation \
  --distribution-id $(aws ssm get-parameter \
    --name /vaultmtg/production/spa-distribution-id \
    --query Parameter.Value --output text \
    --region us-east-1 --profile personal) \
  --paths "/*" \
  --region us-east-1 \
  --profile personal
```

Edge propagation typically completes in 30–60s.

---

## SSM parameter inventory

All deploy-time configuration is read from SSM Parameter Store in `us-east-1`.
The full inventory is documented in
[`infrastructure/ssm/parameters.md`](../infrastructure/ssm/parameters.md). The
parameters relevant to deploys are:

### `/vaultmtg/production/`

| Parameter | Type | Purpose |
|---|---|---|
| `spa-bucket-name` | String | S3 bucket for `app.vaultmtg.app` (`vaultmtg-app-spa`) |
| `spa-distribution-id` | String | CloudFront distribution ID for the SPA |
| `marketing-bucket-name` | String | S3 bucket for `vaultmtg.app` (`vaultmtg-app-marketing`) |
| `marketing-distribution-id` | String | CloudFront distribution ID for the marketing site |
| `ALLOWED_ORIGINS` | String | Comma-separated CORS origins for the BFF |
| `DATABASE_URL` | SecureString | PostgreSQL connection string for the BFF |
| `DAEMON_JWT_SECRET` | SecureString | Daemon-to-BFF JWT signing secret |
| `JWT_SECRET` | SecureString | User-session JWT secret |

### `/rhamiltoneng/production/`

| Parameter | Type | Purpose |
|---|---|---|
| `site-bucket-name` | String | S3 bucket for `rhamiltoneng.com` (`rhamiltoneng-site`) |
| `distribution-id` | String | CloudFront distribution ID for `rhamiltoneng.com` |

### Refreshing SSM after a stack update

When `vaultmtg-app-cdn` or `rhamiltoneng-cdn` CloudFormation stacks are
updated and emit new outputs, run:

```bash
./infrastructure/scripts/sync-ssm-params.sh --profile personal --region us-east-1
```

This pulls stack outputs and overwrites the SSM parameters in place. Deploy
workflows pick up the new values on their next run.

---

## Vercel — preview-only

Vercel is wired up for the VaultMTG frontend but is **demoted to PR
preview-only**. Two specific behaviors enforce this:

1. **`vercel.json` `ignoreCommand`** at the repo root:
   ```json
   {
     "ignoreCommand": "! git describe --exact-match --tags HEAD 2>/dev/null | grep -q '^v'"
   }
   ```
   When a commit corresponds to a `v*` tag (a production tag), `git describe`
   succeeds, the grep matches, the negation flips to false, and Vercel skips
   the build. PR commits are not on a tag, so Vercel builds the preview as
   normal.

2. **No production aliases in `vercel.json`.** The `alias` and `domains`
   fields are not set. `vaultmtg.app` and `app.vaultmtg.app` are owned by
   CloudFront. Vercel preview URLs (`*.vercel.app`) remain in
   `ALLOWED_ORIGINS` so previews can talk to the BFF.

When opening a PR on `frontend/**`:

- Vercel builds the preview automatically.
- The Vercel bot comments the preview URL on the PR.
- The preview hits the production BFF over CORS (the preview origin matches
  `https://*.vercel.app` in `ALLOWED_ORIGINS`).
- The preview is **for review only**. Production traffic continues to hit the
  CloudFront-backed `app.vaultmtg.app` build that was deployed by the most
  recent `v*` tag.

When the PR merges and a tag is later cut, the production deploy workflow
runs and updates CloudFront. Vercel does not deploy production tags.

---

## EC2 / nginx — API only

Per ADR-008, the EC2 instance serves `api.vaultmtg.app` and nothing else.
Specifically:

- nginx has **one** server block: `api.vaultmtg.app`. There is no
  `location /` static-serve in production. There is no `try_files` falling
  back to `index.html`. There is no `/var/www/<frontend>/` root.
- Requests to the EC2 host for any path other than `/api/v1/*` or `/health`
  are rejected by nginx.
- The instance's TLS cert is for `api.vaultmtg.app` only. ACM certs for the
  three frontend hostnames live in `us-east-1` and are attached to CloudFront
  distributions.

If the EC2 host needs to be replaced, the work scope is:

1. Provision a new instance (CloudFormation stack `mtga-companion-ec2`).
2. Install the BFF binary, the systemd unit, and the nginx config from
   `infrastructure/nginx/api.vaultmtg.app.conf`.
3. Run Certbot to issue a Let's Encrypt cert for `api.vaultmtg.app`.
4. Update the Route53 A/AAAA record for `api.vaultmtg.app` to point at the
   new instance.
5. Restart the BFF and verify `/health` returns 200 over TLS.

There is no static-asset migration step because the host carries no static
assets.

---

## CI gates before a tag is cut

Before pushing a `v*` tag on any property, the producing repo must pass:

- Unit + component tests (per repo).
- Lint (`gofumpt -l .` and `go vet ./...` for Go modules; `eslint` and
  `tsc --noEmit` for the SPA).
- E2E smoke (Playwright for the SPA; equivalent per other frontends).
- For Go modules: `go test -race ./...` in every touched module.

These gates are enforced by the `ci.yml`, `e2e-smoke.yml`, `frontend.yml`,
and `release.yml` workflows in this repo and the equivalents in the
sibling repos. The deploy workflow itself does **not** re-run tests — it
trusts that the tag was cut on a green commit.

---

## Related docs and ADRs

- [ADR-008: Frontend Serving Model — S3+CloudFront Canonical, Vercel
  Preview-Only](adr/ADR-008-frontend-serving-model.md) — the architectural
  source of truth this document implements.
- [ADR-007: Frontend Serving Model](adr/007-frontend-serving-model.md) —
  superseded; kept for historical context.
- [ADR-006: Vercel BFF Connectivity](adr/006-vercel-bff-connectivity.md) —
  CORS and `VITE_BFF_URL` semantics. Still applies; preview origins now
  matter more than production origins under ADR-008.
- [ADR-001: Original EC2 nginx serving model](adr/) — superseded by ADR-008.
- [`infrastructure/ssm/parameters.md`](../infrastructure/ssm/parameters.md) —
  full SSM parameter inventory.
- [`infrastructure/cloudformation/vaultmtg-app-cdn.yaml`](../infrastructure/cloudformation/vaultmtg-app-cdn.yaml) —
  S3 + CloudFront for `vaultmtg.app` + `app.vaultmtg.app`.
- [`infrastructure/cloudformation/rhamiltoneng-cdn.yaml`](../infrastructure/cloudformation/rhamiltoneng-cdn.yaml) —
  S3 + CloudFront for `rhamiltoneng.com`.
- [Daemon Installation](DAEMON_INSTALLATION.md) — how the desktop daemon ships
  to end users.
- [Release Checklist](RELEASE_CHECKLIST.md) — pre-tag gates and post-deploy
  verification.
