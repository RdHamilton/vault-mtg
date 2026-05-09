# ADR: Staging Environment Design

**Date**: 2026-05-06
**Status**: Proposed
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-006 (Vercel→BFF connectivity), ADR-008 (frontend serving model), ADR-009 (Clerk auth)

---

## Context

VaultMTG is approaching beta. Today every code path lands directly in
production:

- The Go BFF runs on a single EC2 instance behind nginx (`api.vaultmtg.app`).
- A single RDS PostgreSQL instance holds all data.
- The React SPA is served from S3 + CloudFront at `app.vaultmtg.app`
  (canonical, per ADR-008). Vercel still produces a per-PR preview build
  for review (per ADR-006), and those previews currently point at the
  production BFF (acknowledged risk in ADR-006 §4).
- Auth is Clerk (per ADR-009); the production Clerk instance issues
  `pk_live_*` / `sk_live_*` keys.
- Daemon is a desktop binary; it has no server-side staging concern.
- Sentry uses production DSNs sourced from SSM at `/vaultmtg/prod/`.

Three problems make this untenable for beta:

1. **No safe place to validate schema migrations or destructive changes**
   before they hit prod data.
2. **Vercel preview deploys mutate prod data.** Any tester who clicks
   through a PR preview is writing to the real database with real Clerk
   identities.
3. **No automated pre-prod smoke target.** CI E2E runs against ephemeral
   localhost; nothing exercises the full deployed stack before a tag.

We need a staging environment that:

- Is structurally identical to prod (same code paths, same auth provider,
  same migrations, same SSE/JWT shape).
- Costs no more than $15–20/month incremental against the current ~$38/mo
  AWS burn (covered by the $1,000 AWS Activate credit).
- Stays maintainable by a one-engineer shop (no second production-grade
  ops surface).
- Lets Vercel preview deploys point at a real but isolated backend.

---

## Decision

**Adopt Option A: a second `bff-staging` systemd service co-located on the
existing EC2 instance, served behind nginx at `staging-api.vaultmtg.app`,
backed by a separate logical database on the existing RDS instance.**

A staging copy is provisioned for every component that holds state or
issues credentials. Compute, the database server, and the SSL cert are
shared with prod where doing so cannot cause a prod incident.

### What gets a staging copy vs. what is shared

| Component | Staging strategy | Rationale |
|---|---|---|
| **Go BFF** | Separate `bff-staging.service` systemd unit on the **same EC2 instance**, listening on `127.0.0.1:8081` (prod stays on `127.0.0.1:8080`). | Separate process gives isolated env vars, log file, SSM hierarchy, and Sentry env. Co-located avoids a second EC2 bill. |
| **PostgreSQL** | **Separate logical database** (`vaultmtg_staging`) on the **same RDS instance**, owned by a distinct role (`vaultmtg_staging_app`). Migrations run via the same migration tooling against `vaultmtg_staging`. | Hard isolation between schemas, no shared tables, separate credentials. Avoids the $13–15/mo cost of a second `db.t4g.micro`. RDS-instance-level outages still hit both — acceptable for staging. |
| **RDS instance** | Shared with prod. | Cost. Capacity is not a concern at our scale. |
| **Clerk** | **Use Clerk's existing "Development" instance** (free, included on the Hobby plan) for staging. SPA preview builds use `pk_test_*`; staging BFF uses `sk_test_*`. | ADR-009 already provisions a Development instance. Free; no second paid Clerk org. Test users do not pollute prod MRU billing. |
| **SSM** | New `/vaultmtg/staging/` parameter hierarchy mirroring `/vaultmtg/prod/`. Same parameter names, different values. | Per-env secrets isolation; same SSM access pattern in code; supports `MTGA_ENV=staging` selecting the right path. |
| **Sentry** | **Same Sentry projects (one per app)** with `environment: staging` tag. Separate DSN is **not** required because the project is the same. | Sentry's free tier supports unlimited environments. Filtering by `environment` in the Sentry UI is the standard pattern. Avoids managing a second DSN per service. |
| **nginx** | Single nginx process serves both `api.vaultmtg.app` (→ `:8080`) and `staging-api.vaultmtg.app` (→ `:8081`) via two `server {}` blocks. | One config, one cert renewal cycle, one reload surface. |
| **TLS cert** | A single ACM cert (or a single Let's Encrypt cert) covering both `api.vaultmtg.app` and `staging-api.vaultmtg.app` via SAN. Issued to nginx on EC2. | One cert lifecycle. nginx already terminates TLS for prod; adding a SAN is free. |
| **Daemon** | No staging copy. Developers run `services/daemon` locally and point it at staging via env var (`VAULTMTG_BFF_URL=https://staging-api.vaultmtg.app/api/v1`). | Daemon is a desktop binary; per-env build is unnecessary. |
| **Frontend SPA — staging build** | No dedicated S3+CloudFront staging distribution. Vercel preview deploys serve as the staging frontend (see next section). | Avoids a third CloudFront + bucket. Vercel previews are already free per-PR. |

### Vercel preview → staging BFF wiring

Vercel supports per-environment env-var overrides in the project dashboard:

- **Production environment** (target: tag-based deploy, currently disabled per ADR-008): `VITE_BFF_URL=https://api.vaultmtg.app/api/v1`, `VITE_CLERK_PUBLISHABLE_KEY=pk_live_*`.
- **Preview environment** (target: every PR): `VITE_BFF_URL=https://staging-api.vaultmtg.app/api/v1`, `VITE_CLERK_PUBLISHABLE_KEY=pk_test_*` (Clerk Development instance).
- **Development environment** (target: `vercel dev`): unset; falls back to `http://localhost:8080/api/v1`.

This means the moment this ADR's tickets land, every PR preview talks to
staging — solving the ADR-006 §4 risk without any frontend code change.

The staging BFF's `ALLOWED_ORIGINS` must include the Vercel preview-URL
glob (`https://*.vercel.app`) and any custom preview alias domains. This
mirrors the prod CORS model.

### CI/CD trigger model

Staging deploys are triggered on **every merge to `main`**, with a
manual-dispatch fallback for re-deploying the same commit.

- **BFF staging deploy**: a GitHub Actions workflow (`.github/workflows/staging-deploy.yml`) triggers on push to `main` for paths under `services/bff/**` and `services/bff/internal/storage/migrations/postgres/**`. It builds the binary, SCPs it to EC2, runs migrations against `vaultmtg_staging`, and restarts `bff-staging.service`.
- **Frontend preview**: Vercel handles this automatically via its existing GitHub integration. Every PR open triggers a preview build that reads the Preview env vars (above).
- **Manual re-deploy**: `workflow_dispatch` input on the staging-deploy workflow accepts a Git ref, useful for re-running staging against an older commit during incident triage.
- **Production deploys** remain tag-driven (per ADR-008) and are unchanged. Promotion from staging to prod is "tag the same commit you validated on staging, then run the existing tag workflow."

### DNS, TLS, and nginx

- **DNS**: a Route53 `A` (or `CNAME`) record for `staging-api.vaultmtg.app` pointing at the same EC2 Elastic IP as `api.vaultmtg.app`.
- **TLS**: extend the existing nginx cert (whether ACM-imported or Let's Encrypt) with `staging-api.vaultmtg.app` as a SAN. If using Let's Encrypt + Certbot, this is a single `--expand` reissue.
- **nginx config**: one new `server {}` block listening on `:443 ssl` for `server_name staging-api.vaultmtg.app;`, with `proxy_pass http://127.0.0.1:8081;` and the same SSE-friendly settings as the prod block (`proxy_buffering off`, `proxy_read_timeout 24h`, `Cache-Control: no-cache` on event streams).
- **Health check**: nginx exposes `/healthz` on the staging vhost just like prod.

### `bff-staging` systemd unit shape

The unit file (provisioned by the infra agent — not authored here) sets:

```
Environment=MTGA_ENV=staging
Environment=BFF_PORT=8081
EnvironmentFile=/etc/vaultmtg/bff-staging.env   # SSM-fetched secrets
ExecStart=/opt/vaultmtg/bin/bff
Restart=on-failure
```

The `bff-staging.env` file is rendered at deploy time from SSM
`/vaultmtg/staging/*` parameters by the same boot script that renders the
prod file from `/vaultmtg/prod/*`.

### Cost estimate

| Line item | Monthly cost |
|---|---|
| EC2 (no change — same `t3.small`) | $0 |
| RDS (no change — same `db.t4g.micro`) | $0 |
| Route53 record for `staging-api.vaultmtg.app` | ~$0.01 |
| Additional ACM cert SAN (or LE renewal) | $0 |
| Sentry environment tag (existing project) | $0 |
| Clerk Development instance | $0 (Hobby plan) |
| Vercel preview deploys (existing free tier) | $0 |
| Additional CloudWatch logs from `bff-staging` | ~$1–3 |
| **Total incremental** | **~$1–3/mo** |

This is well inside the $15–20/mo budget. The slack is intentional: it
leaves room to upgrade to Option C (separate RDS instance) later without
a budget conversation.

---

## Consequences

### Positive

- **Beta unblocked.** Vercel previews stop mutating prod data the moment the staging BFF is online and the Vercel env vars are flipped.
- **Migration safety.** Schema changes ship to `vaultmtg_staging` first via the standard CI flow; failures surface before prod.
- **Auth isolation.** Test users created during PR review go into Clerk Development; production MRU billing stays clean.
- **Same code, two envs.** No `if env == "staging"` branches in BFF code — only env vars differ. `MTGA_ENV` selects the SSM path; everything else is parameterized.
- **Single nginx, single cert.** Operational toil stays flat.
- **Incremental cost ~$1–3/mo.** Fully inside the AWS Activate credit envelope.

### Negative

- **Shared blast radius for the EC2 instance.** A bad deploy that wedges the kernel, fills the disk, or saturates the NIC takes down both prod and staging. Mitigation: each systemd unit gets a `MemoryMax=` and `CPUQuota=` cap; staging gets the smaller cap. Disk fill is monitored by an existing CloudWatch alarm on the prod side and will be extended to alert at 70% (lower than prod's 85%).
- **Shared blast radius for the RDS instance.** A `db.t4g.micro` has limited IOPS; staging load tests could starve prod. Mitigation: staging reads/writes are explicitly low-rate (PR-driven, not load-test-driven). Anything resembling load testing must run against a one-off RDS instance spun up for the test and torn down after.
- **Shared TLS cert.** A misconfigured cert renewal that breaks `staging-api.vaultmtg.app` could also break `api.vaultmtg.app`. Mitigation: cert renewal runs through the existing Let's Encrypt automation; a renewal failure alerts before expiry.
- **Vercel preview deploys depend on a healthy staging BFF.** If staging is down, PR review is partially blocked (frontend builds still succeed; live API calls fail). Acceptable; staging downtime is rare and the prod path is unaffected.
- **No separate DR story for staging.** If staging data is lost, we re-seed it from a script. No backups required.

### Neutral

- **`MTGA_ENV=staging` is now a real env value.** Existing code paths that compare `env == "production"` continue to work; staging is treated as non-prod. Any future code that needs to differentiate "staging vs. local dev" must use an explicit comparison, not a "non-prod" boolean.

---

## Alternatives Considered

### Option B — Separate t3.micro EC2 for staging

A dedicated `t3.micro` (~$7.50/mo on-demand, or $0 with a free-tier
allowance for the first 12 months only) running its own nginx, BFF, and
systemd. RDS is still a separate database on the prod instance.

**Rejected.** Doubles the operational surface (two AMIs to patch, two
nginx configs, two systemd unit sets) for a benefit (compute isolation)
we don't currently need at our load. Reconsider when prod EC2 sustains
>50% CPU.

### Option C — Separate RDS instance for staging

A dedicated `db.t4g.micro` (~$13–15/mo) gives full database isolation,
including IOPS, parameter group, and version. This is the textbook
correct answer.

**Rejected for now.** Costs ~5x what Option A costs and the only thing
it buys is "load tests on staging cannot starve prod IOPS." We can buy
that protection cheaper by writing "no load tests on staging" into the
runbook. Reconsider if (a) we want to test PostgreSQL major-version
upgrades against staging before prod, or (b) we add real load testing.

### Option D — Use Vercel previews with a mocked BFF

Configure the SPA in preview builds to use a mock service worker (MSW)
instead of a real BFF.

**Rejected.** Mocks miss the entire class of bugs that staging exists
to catch (auth, CORS, migration, JWT verification, JWKS caching, SSE
framing). Mock-vs-real-API parity is exactly the gap that bit us in PR
#1175 (daemon JWT mid-session expiry).

### Option E — Multi-tenant staging on prod with a `account_id` allowlist

Run all staging traffic against the prod BFF, prod RDS, and prod Clerk,
and gate "staging" behavior by an allowlisted `account_id` set.

**Rejected.** Violates the principle that staging exists precisely to
catch bugs that hit prod data. Also makes Clerk MRU billing harder to
reason about.

---

## Implementation Tickets

The following tickets implement this ADR. The Project Manager owns final
ticket creation, milestone assignment (recommend a new milestone
**"Phase 3.5: Staging Environment"** under project board #27), and
sequencing. This ADR is the source of truth.

### Infrastructure agent (`infrastructure`)

| # | Scope | Acceptance |
|---|---|---|
| **INFRA-1** | CloudFormation: add `staging-api.vaultmtg.app` Route53 `A` record pointing at the existing EC2 Elastic IP. | DNS resolves; record visible in Route53 console. |
| **INFRA-2** | TLS: extend the EC2 cert (Let's Encrypt or ACM) to cover `staging-api.vaultmtg.app` as a SAN. Document the renewal command in the runbook. | `curl -I https://staging-api.vaultmtg.app/healthz` returns 200 with valid cert. |
| **INFRA-3** | nginx: add a `server {}` block for `staging-api.vaultmtg.app` proxying to `127.0.0.1:8081`, with the same SSE-friendly settings as prod. Reload nginx via systemctl. | `nginx -t` clean; staging vhost serves the staging BFF's `/healthz`. |
| **INFRA-4** | systemd: create `bff-staging.service` unit with `MTGA_ENV=staging`, `EnvironmentFile=/etc/vaultmtg/bff-staging.env`, `MemoryMax=`, `CPUQuota=`. Include a boot script that renders the env file from `/vaultmtg/staging/*` SSM parameters. | `systemctl status bff-staging` shows active; service restarts cleanly. |
| **INFRA-5** | SSM: provision the `/vaultmtg/staging/` parameter hierarchy (`DATABASE_URL`, `CLERK_SECRET_KEY` (sk_test_*), `SENTRY_DSN`, `ALLOWED_ORIGINS`, `DAEMON_JWT_SECRET` for transition). | All staging params present and readable by the EC2 instance role. |
| **INFRA-6** | CloudWatch: extend the existing disk-fill alarm to fire at 70% on the staging side; add a separate alarm for `bff-staging.service` restart loops. | Alarms visible in CloudWatch; test by stopping the service. |
| **INFRA-7** | GitHub Actions: `.github/workflows/staging-deploy.yml` — triggers on push to `main` for `services/bff/**` and the migrations path; supports `workflow_dispatch` with a `ref` input. Builds the binary, SCPs to EC2, runs migrations against `vaultmtg_staging`, restarts `bff-staging.service`. **Must include `GONOSUMDB` and `GOPRIVATE` env vars on every Go step (per architect guideline).** | Push to `main` deploys to staging within 5 minutes; manual dispatch works for arbitrary refs. |
| **INFRA-8** | Vercel: configure Preview environment vars in the Vercel project dashboard — `VITE_BFF_URL=https://staging-api.vaultmtg.app/api/v1`, `VITE_CLERK_PUBLISHABLE_KEY=pk_test_*`. Document the override in `frontend/.env.example`. | A new PR preview build hits staging-api and uses the Clerk Development instance. |

### DBA agent (`dba`)

| # | Scope | Acceptance |
|---|---|---|
| **DBA-1** | Provision `vaultmtg_staging` database on the existing RDS instance with a distinct role (`vaultmtg_staging_app`) holding only the privileges it needs for that database. Document role provisioning in `docs/database-provisioning.md`. | Role connects; cannot read or write `vaultmtg_prod` tables. |
| **DBA-2** | Verify the migration tooling (`services/bff/internal/storage/migrations/postgres/`) runs cleanly against `vaultmtg_staging` from a fresh state. Document the staging-bootstrap command. | Fresh `vaultmtg_staging` migrates to head; `up` and `down` are idempotent. |
| **DBA-3** | Add a periodic (weekly) job or runbook step that truncates non-essential staging tables to keep the staging DB small and avoid drift. (Not destructive to schema; only data.) | Runbook exists; truncation script lives in `services/bff/scripts/`. |

### Backend engineer (`backend-engineer`)

| # | Scope | Acceptance |
|---|---|---|
| **BE-1** | Update `services/bff/internal/config/config.go` to recognize `MTGA_ENV=staging` as a valid env (alongside `production` and the dev default). Staging requires `DATABASE_URL` and `CLERK_SECRET_KEY` to be set, same as production (fail-fast). | Unit test added covering `MTGA_ENV=staging` happy path and missing-secret failure path. |
| **BE-2** | Sentry init in `services/bff/cmd/main.go` reads `MTGA_ENV` and sets it as the Sentry `Environment` tag. No DSN change. | Staging events appear in Sentry under `environment: staging`. |
| **BE-3** | Add a `/healthz` endpoint that returns the current `MTGA_ENV` and the migration head version, for use by the staging deploy workflow's post-deploy verification step. | Endpoint returns 200 with JSON body containing `env` and `migration_version`. |

### Front-engineer (`front-engineer`)

| # | Scope | Acceptance |
|---|---|---|
| **FE-1** | Update `frontend/.env.example` to document the three Vercel environments and the env vars each requires (`VITE_BFF_URL`, `VITE_CLERK_PUBLISHABLE_KEY`). No code change required if `apiClient.ts` already honors `VITE_BFF_URL` (it does, per ADR-006). | `.env.example` reviewed; matches the Vercel dashboard configuration. |
| **FE-2** | Add a small "Environment" badge in the SPA footer that reads `import.meta.env.MODE` (or a new `VITE_ENV_LABEL`) so testers can see at a glance whether they're on a preview build. Hidden in production. | Badge visible on Vercel previews; not visible on production builds (`MODE === 'production'`). |
| **FE-3** | Add a Playwright E2E smoke test that runs against `staging-api.vaultmtg.app` post-deploy. Triggered by the staging deploy workflow on success. Suite is small (sign-in stub, one authenticated GET, one SSE connect). | Smoke runs in <60s; failures fail the staging deploy workflow's post-step. |

### Architect (this ADR)

| # | Scope | Acceptance |
|---|---|---|
| **ARCH-1** | Update `docs/CLAUDE_CODE_GUIDE.md` to document the staging environment, the `MTGA_ENV=staging` value, the `/vaultmtg/staging/` SSM path, and the rule that PR review uses the Vercel preview pointing at staging. | Doc updated; backend and frontend agents reference it in subsequent PRs. |
| **ARCH-2** | Update ADR-006 §4 to mark the "preview deploys share prod" risk as resolved, with a forward link to this ADR. | ADR-006 amended with a note. |

---

## References

- ADR-006 — Vercel→BFF connectivity (CORS, cross-origin, preview-shares-prod risk).
- ADR-008 — Frontend serving model: S3+CloudFront canonical, Vercel preview-only.
- ADR-009 — User auth provider: Clerk (Development instance is the staging auth surface).
- AWS Activate credits — $1,000 approved 2026-05-05; covers staging cost lines below.
- `docs/COMPANY_VISION.md` — North star: 50,000 MAU; staging is a beta-readiness prerequisite.
