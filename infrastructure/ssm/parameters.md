# SSM Parameter Store Reference

All parameters live in the `us-east-1` region under the `/mtga-companion/production/` prefix.

| Parameter Path | Type | Description |
|---|---|---|
| `/mtga-companion/production/ALLOWED_ORIGINS` | String | Comma-separated list of origins the BFF CORS middleware allows. Read at startup. |
| `/mtga-companion/production/DATABASE_URL` | SecureString | PostgreSQL connection string for the RDS instance. |
| `/mtga-companion/production/DAEMON_JWT_SECRET` | SecureString | Shared secret used to sign and verify daemon-to-BFF JWTs. |
| `/mtga-companion/production/JWT_SECRET` | SecureString | Shared secret for user session JWTs issued by the BFF. |
| `/mtga-companion/production/CLERK_SECRET_KEY` | SecureString | **TODO (manual)**: Clerk backend secret key for verifying Clerk session JWTs on BFF protected routes. Obtain from Clerk Dashboard → API Keys → Secret keys. Required in production (see ADR-009). |

## CLERK_SECRET_KEY

**TODO (manual)**: Ray must add the Clerk backend secret key to SSM before deploying auth-protected routes to production.

1. Go to [Clerk Dashboard](https://dashboard.clerk.com) → your application → **API Keys**.
2. Copy the **Secret key** (starts with `sk_live_...` for production or `sk_test_...` for development).
3. Store it in SSM:

```bash
aws ssm put-parameter \
  --name "/mtga-companion/production/CLERK_SECRET_KEY" \
  --value "sk_live_<your-key>" \
  --type SecureString \
  --overwrite \
  --region us-east-1 \
  --profile personal
```

4. Restart the BFF after adding:

```bash
aws ssm send-command \
  --instance-ids i-065351fbb99da2d22 \
  --document-name "AWS-RunShellScript" \
  --parameters 'commands=["sudo systemctl restart mtga-companion-bff || sudo systemctl restart mtga-companion"]' \
  --region us-east-1 \
  --profile personal
```

The BFF reads `CLERK_SECRET_KEY` on startup from its environment file (`/etc/mtga-companion/env`).  The EC2 user-data script pulls SSM parameters into that file.  Without this key, the BFF will refuse to start in `MTGA_ENV=production` mode.

## ALLOWED_ORIGINS

Current value (as of 2026-05-05):

```
https://app.vaultmtg.app,https://vaultmtg.app,https://www.vaultmtg.app,https://mtga-companion.vercel.app,https://*.vercel.app
```

### Origin inventory

| Origin | Purpose |
|---|---|
| `https://app.vaultmtg.app` | VaultMTG React SPA — production (S3 + CloudFront) |
| `https://vaultmtg.app` | VaultMTG apex/marketing domain |
| `https://www.vaultmtg.app` | VaultMTG www redirect |
| `https://mtga-companion.vercel.app` | Legacy Vercel production URL (kept for backward compat) |
| `https://*.vercel.app` | Vercel preview deployments |

### Updating

To add a new origin, append it to the comma-separated list and overwrite the parameter:

```bash
aws ssm put-parameter \
  --name "/mtga-companion/production/ALLOWED_ORIGINS" \
  --value "<existing>,<new-origin>" \
  --type String \
  --overwrite \
  --region us-east-1 \
  --profile personal
```

After updating, restart the BFF to pick up the new value:

```bash
aws ssm send-command \
  --instance-ids i-065351fbb99da2d22 \
  --document-name "AWS-RunShellScript" \
  --parameters 'commands=["sudo systemctl restart mtga-companion-bff || sudo systemctl restart mtga-companion"]' \
  --region us-east-1 \
  --profile personal
```
