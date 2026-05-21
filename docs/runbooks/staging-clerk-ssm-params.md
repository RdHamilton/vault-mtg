# Runbook: Staging Clerk SSM Parameters — Management and Rotation

**Service**: Staging BFF (`mtga-bff-staging`)
**Env file**: `/etc/mtga-companion-staging/env`
**SSM paths**: `/vaultmtg/staging/CLERK_SECRET_KEY`, `/vaultmtg/staging/CLERK_PUBLISHABLE_KEY`, `/vaultmtg/staging/CLERK_FRONTEND_API`
**Issue ref**: #2408

---

## Context

The staging BFF authenticates requests using the Clerk SDK. Three SSM parameters supply the
Clerk configuration:

| SSM path | Type | Env var written to env file | Notes |
|---|---|---|---|
| `/vaultmtg/staging/CLERK_SECRET_KEY` | SecureString | `CLERK_SECRET_KEY` | `sk_live_*` — never expose in logs or PRs |
| `/vaultmtg/staging/CLERK_PUBLISHABLE_KEY` | String | `CLERK_PUBLISHABLE_KEY` | `pk_live_*` |
| `/vaultmtg/staging/CLERK_FRONTEND_API` | String | `CLERK_FRONTEND_API` | FAPI URL, e.g. `clerk.<domain>.lcl.dev` or `<domain>.clerk.accounts.dev` |

These parameters are read at deploy time by `provision-staging-env.sh`, which writes them into
`/etc/mtga-companion-staging/env`. The BFF reads them on startup.

> **Important**: Staging uses `pk_live_` / `sk_live_` keys from the Clerk staging instance (same
> key type as production but scoped to the staging Clerk application). Never use `pk_test_` /
> `sk_test_` keys — they indicate Clerk's "development mode" and are rejected by the JWKS endpoint
> that the BFF uses for JWT verification.

### Parameter origin

All three parameters were initially migrated on 2026-05-21 from the legacy
`/mtga-companion/staging/` path hierarchy (pre-VaultMTG rename) to their current canonical
paths. The source values came from the old paths:

- `/mtga-companion/staging/CLERK_SECRET_KEY` → `/vaultmtg/staging/CLERK_SECRET_KEY`
- `/mtga-companion/staging/CLERK_PUBLISHABLE_KEY` → `/vaultmtg/staging/CLERK_PUBLISHABLE_KEY`
- `/mtga-companion/staging/CLERK_FRONTEND_API` → `/vaultmtg/staging/CLERK_FRONTEND_API`

---

## Clerk key rotation procedure

When Clerk keys need to be rotated (security incident, annual rotation, or Clerk application
recreation):

### 1. Obtain new keys from the Clerk dashboard

Log in to [clerk.com](https://clerk.com), navigate to the **VaultMTG staging** application, and
retrieve the new values from **API Keys**:

- `Secret key` → `CLERK_SECRET_KEY` (`sk_live_*`)
- `Publishable key` → `CLERK_PUBLISHABLE_KEY` (`pk_live_*`)
- `Frontend API URL` → `CLERK_FRONTEND_API`

### 2. Update SSM parameters

```bash
# CLERK_SECRET_KEY — always SecureString
aws ssm put-parameter \
  --name "/vaultmtg/staging/CLERK_SECRET_KEY" \
  --value "<new-sk_live_value>" \
  --type "SecureString" \
  --overwrite \
  --profile personal

# CLERK_PUBLISHABLE_KEY
aws ssm put-parameter \
  --name "/vaultmtg/staging/CLERK_PUBLISHABLE_KEY" \
  --value "<new-pk_live_value>" \
  --type "String" \
  --overwrite \
  --profile personal

# CLERK_FRONTEND_API
aws ssm put-parameter \
  --name "/vaultmtg/staging/CLERK_FRONTEND_API" \
  --value "<new-fapi-url>" \
  --type "String" \
  --overwrite \
  --profile personal
```

### 3. Verify parameters are written (do NOT use --with-decryption in transcripts)

```bash
aws ssm get-parameters \
  --names \
    "/vaultmtg/staging/CLERK_SECRET_KEY" \
    "/vaultmtg/staging/CLERK_PUBLISHABLE_KEY" \
    "/vaultmtg/staging/CLERK_FRONTEND_API" \
  --profile personal \
  --query "Parameters[*].{Name:Name,Type:Type,Version:Version}" \
  --output table
```

Expected output — all three present, CLERK_SECRET_KEY as SecureString:

```
+------------------------------------------+---------------+---------+
| Name                                     | Type          | Version |
+------------------------------------------+---------------+---------+
| /vaultmtg/staging/CLERK_FRONTEND_API     | String        | N       |
| /vaultmtg/staging/CLERK_PUBLISHABLE_KEY  | String        | N       |
| /vaultmtg/staging/CLERK_SECRET_KEY       | SecureString  | N       |
+------------------------------------------+---------------+---------+
```

### 4. Re-provision the env file on the EC2 instance

Trigger the full `provision-staging-env.sh` via SSM RunCommand (see
`staging-allowed-origins.md` — Step 3 for the full SSM send-command invocation).

Alternatively, trigger a full Staging Deploy from GitHub Actions:

```bash
gh workflow run staging-deploy.yml --repo RdHamilton/vault-mtg --ref main
```

### 5. Verify the staging BFF is accepting authenticated requests

```bash
# Health check (unauthenticated — should be 200)
curl -s -o /dev/null -w "%{http_code}" https://staging-api.vaultmtg.app/health

# Auth smoke test (authenticated — requires a valid staging session token)
bash scripts/test/staging-auth-smoke.sh
```

---

## Initial population (first-time setup or disaster recovery)

If all three CLERK parameters are missing (e.g. after a path rename or fresh environment setup):

1. Retrieve values from Clerk dashboard (see Step 1 in rotation procedure above).
2. If values exist at legacy path `/mtga-companion/staging/CLERK_SECRET_KEY`, copy them:

```bash
for PARAM in CLERK_SECRET_KEY CLERK_PUBLISHABLE_KEY CLERK_FRONTEND_API; do
  VALUE=$(aws ssm get-parameter \
    --name "/mtga-companion/staging/${PARAM}" \
    --with-decryption \
    --profile personal \
    --query "Parameter.Value" \
    --output text)
  TYPE="String"
  [ "$PARAM" = "CLERK_SECRET_KEY" ] && TYPE="SecureString"
  aws ssm put-parameter \
    --name "/vaultmtg/staging/${PARAM}" \
    --value "$VALUE" \
    --type "$TYPE" \
    --profile personal \
    --description "Migrated from /mtga-companion/staging/${PARAM}"
done
```

3. If legacy path values are also absent, obtain fresh values from the Clerk dashboard and use
   the rotation procedure above (Steps 2–5).

---

## DB-related staging SSM parameters

The migration script (`run-staging-migrations.sh`) and provisioning script also require:

| SSM path | Type | Notes |
|---|---|---|
| `/vaultmtg/staging/db-secret-arn` | String | Secrets Manager ARN for RDS master credentials |
| `/vaultmtg/staging/db-endpoint` | String | RDS endpoint hostname |
| `/vaultmtg/staging/db-name` | String | Database name (`vaultmtg_staging`) |
| `/vaultmtg/staging/database-url` | SecureString | Full DSN with credentials for golang-migrate |

These are read from the legacy `/mtga-companion/staging/` path hierarchy on initial setup and
should never need manual rotation (they are managed by CloudFormation RDS stack outputs and
Secrets Manager rotation). If a Secrets Manager rotation occurs, update `database-url` to match.

---

## Production

Production Clerk parameters are at `/vaultmtg/production/CLERK_SECRET_KEY` etc. and are managed
separately. Never update them through this runbook. Production key rotation requires a full
production deploy via `release.yml`.
