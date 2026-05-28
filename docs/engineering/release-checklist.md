# Pre-Release Checklist

Ordered runbook. Execute top to bottom. Do not skip a section.

---

## 0. Pre-Release Staging Gate

**Must be completed before any release tag is cut. Infrastructure executes; PM confirms.**

- [ ] Run the staging deploy pipeline from scratch (verify all deploy scripts exist and complete without error)
- [ ] BFF starts on staging: `curl -f https://api-staging.vaultmtg.app/healthz` returns HTTP 200
- [ ] nginx `/healthz` path present in **both** staging and production vhost configs (check `api.vaultmtg.app.conf`)
- [ ] systemd unit port matches BFF port binding (staging: confirm `BFF_PORT` env var matches `-port` flag or env-var fallback)
- [ ] All required env vars present in SSM / staging env file — no missing secrets
- [ ] Playwright staging smoke suite passes: `cd frontend && npx playwright test --grep @smoke --project=pipeline`
- [ ] All smoke tests green — no failures, no skips

If any item fails: **stop**. Fix the root cause. Do not proceed to Section 1 until all items are green.

---

## 1. Pre-Deploy Gates

All of these must be green before you tag.

### 1.1 CI is Green

- [ ] GitHub Actions shows all checks passing on the release branch
- [ ] No open PRs in state "BLOCKED" that touch this release
- [ ] Infra repo (`mtga-companion-infra`) required status checks are all enabled — verify with:
  ```bash
  gh api repos/RdHamilton/mtga-companion-infra/branches/main/protection/required_status_checks \
    --jq '.contexts[]'
  # Expected: all four gates present
  # - Changeset gate — replacement check
  # - Security Review Gate
  # - Local Verification heuristic check
  # - Validate IAM policies (S-10)
  ```
- [ ] `go test -race ./...` passes in every touched Go module (run locally if CI was skipped):
  ```bash
  cd services/bff && go test -race ./...
  cd services/daemon && go test -race ./...
  ```
- [ ] Frontend tests pass:
  ```bash
  cd frontend && npm run test:run
  cd frontend && npx tsc --noEmit
  ```

### 1.2 No Blocking Open Issues

- [ ] Check GitHub project board — no tickets in "Blocked" or "In Progress" that must land in this release
- [ ] Any known bugs that could affect the P0 regression flows are either fixed or explicitly deferred

### 1.3 Database Migrations Reviewed

- [ ] List any new migration files: `git diff main...HEAD --name-only | grep migration`
- [ ] For each new migration: confirm a `down.sql` rollback script exists
- [ ] Confirm migrations have been tested against a copy of the production schema (not just dev)
- [ ] Note migration files here: _(fill in before shipping)_

### 1.4 Daemon Installation Doc Up To Date

- [ ] `docs/support/daemon-installation.md` reflects any changed install steps, port changes, or new env vars for this release
- [ ] Verified `docs/support/daemon-installation.md` platform verification status is accurate (macOS tested, Windows/Linux status noted)

### 1.5 Environment Variables Confirmed

- [ ] All required BFF env vars are set in production:
  - `DAEMON_JWT_SECRET` (required — BFF fails fast if missing)
  - `DATABASE_URL` or equivalent DB connection
  - `ALLOWED_ORIGINS` — includes the production frontend domain
- [ ] Vercel deployment has `VITE_API_BASE_URL` pointing to the production BFF URL
- [ ] No dev-only env vars (`localhost`, debug flags) present in production config

---

## 2. Tag and Deploy

### 2.1 Tag the Release

```bash
git fetch origin && git checkout main && git pull origin main
git tag v<MAJOR.MINOR.PATCH>
git push origin v<MAJOR.MINOR.PATCH>
```

For daemon releases, also tag:
```bash
git tag daemon/v<MAJOR.MINOR.PATCH>
git push origin daemon/v<MAJOR.MINOR.PATCH>
```

The Vercel deploy for the frontend fires automatically on the `daemon/v*` tag pattern (per current Vercel ignore command config). Confirm this in the Vercel dashboard.

### 2.2 Monitor CI Deploy

- [ ] GitHub Actions deploy job triggered by tag push
- [ ] BFF deploy job completes (EC2 or target host) — check Actions tab
- [ ] Frontend Vercel deploy triggered — check Vercel dashboard

### 2.3 Verify Deploy Completed

- [ ] Vercel shows the new deployment as "Ready" for the production domain
- [ ] BFF on EC2: SSH in and confirm new binary is running:
  ```bash
  ssh <ec2-instance>
  systemctl status bff   # or whatever the service name is
  journalctl -u bff -n 30
  ```
- [ ] BFF process logs show the expected startup version string

---

## 3. Post-Deploy Smoke Checks

Run immediately after deploy completes. These mirror the automated smoke tests but target production.

### 3.1 BFF Health (Automated — manual verify here)

```bash
curl -s https://<production-bff-url>/api/v1/health
```

- [ ] HTTP 200
- [ ] Response body indicates healthy status

If this fails, stop. Do not proceed. See Section 4 (Rollback).

### 3.2 SPA Loads at Production URL

1. Open `https://<production-frontend-url>` in a fresh incognito browser tab
2. Wait up to 15 seconds

- [ ] App container renders (not blank)
- [ ] Page title is non-empty
- [ ] Browser console has no uncaught errors

### 3.3 BFF Decks Endpoint Reachable

```bash
curl -s -o /dev/null -w "%{http_code}" https://<production-bff-url>/api/v1/decks
```

- [ ] Returns 200 or 401 (any HTTP response confirms BFF is responding and CORS is not blocking)
- [ ] No CORS error when called from the production frontend domain (check browser console)

### 3.4 One Full Draft Pick End-to-End

Perform this manually with MTGA open:

1. Start the daemon: `./mtga-companion service start`
2. Open a draft event in MTGA
3. Open the production app in a browser
4. Navigate to the Draft tab
5. Verify the current pack renders with cards
6. Make one pick in MTGA
7. Verify the pick registers in the app

- [ ] Pack cards visible before pick
- [ ] Pick registered without page error after pick

### 3.5 Daemon Connects to Production App

1. Install fresh release daemon binary (do not reuse dev binary)
2. `./mtga-companion service install && ./mtga-companion service start`
3. Open production app → Settings → Daemon Connection

- [ ] Status shows Connected
- [ ] `curl http://localhost:9999/status` returns `"status":"running"`

### 3.6 Daemon Version Endpoint Returns Current Version

```bash
curl -s https://<production-bff-url>/api/v1/daemon/version
```

- [ ] Response contains the version matching the tag just released

---

## 4. Rollback Procedure

Use this section if a P0 smoke check fails after deploy.

### 4.1 Revert the Frontend (Vercel)

1. Open Vercel dashboard → project → Deployments
2. Find the previous successful deployment
3. Click "..." → "Promote to Production"
4. Verify the old deployment is live at the production URL
5. Re-run Section 3.1 and 3.2

### 4.2 Revert the BFF (EC2)

If the BFF binary was updated on EC2:

```bash
ssh <ec2-instance>

# Stop the running service
sudo systemctl stop bff

# Restore the previous binary (assumes you kept a backup)
sudo cp /opt/bff/bff.backup /opt/bff/bff

# Or pull the previous release tag and rebuild:
# git fetch --tags && git checkout v<PREVIOUS_VERSION>
# cd services/bff && go build -o /opt/bff/bff ./cmd/bff

# Restart
sudo systemctl start bff
sudo systemctl status bff
```

- [ ] BFF health returns 200 after restart
- [ ] Re-run smoke checks 3.1–3.3

### 4.3 Revert a Database Migration

If the release included a DB migration that must be rolled back:

1. Locate the `down.sql` file for the migration that was applied
2. Connect to the production DB:
   ```bash
   psql $DATABASE_URL   # or sqlite3 <path> for SQLite
   ```
3. Run the down migration manually:
   ```sql
   \i path/to/migration/down.sql
   ```
4. Confirm the schema is back to the previous state
5. Redeploy the previous BFF binary (see 4.2) which expects the old schema

- [ ] Down migration applied without errors
- [ ] Previous BFF binary running against rolled-back schema
- [ ] Health check passes

### 4.4 Communication

- [ ] If rollback was needed: note the reason in the GitHub release notes
- [ ] Open a post-mortem issue in GitHub with label `release-incident`
- [ ] Do not re-attempt the release until the root cause is identified

---

## 5. Sign-Off

Fill in before closing this checklist. **Both PM and LE must sign. No release without both.**

| Item | Status | Notes |
|---|---|---|
| Section 0 staging gate | | |
| Pre-deploy gates (Section 1) | | |
| Deploy completed | | |
| BFF health | | |
| SPA loads | | |
| Draft pick end-to-end | | |
| Daemon connects | | |
| Daemon version endpoint | | |
| Regression plan P0 flows | | |

**Release version**: `v________`
**Deploy date/time**: `__________`
**PM sign-off**: `__________`
**LE sign-off**: `__________`

Once all rows are green: the release is live. Move all "Done" tickets to "Released" on the project board.

---

## Reference

- Automated smoke tests: `frontend/tests/e2e/smoke.spec.ts`
- Manual regression flows: `docs/engineering/regression.md`
- Daemon installation: `docs/support/daemon-installation.md`
- Architecture overview: `docs/architecture/overview.md`
