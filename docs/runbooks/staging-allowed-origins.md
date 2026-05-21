# Runbook: Staging BFF ALLOWED_ORIGINS — SSM Parameter Management

**Service**: Staging BFF (`mtga-bff-staging`)
**Env file**: `/etc/mtga-companion-staging/env`
**SSM path**: `/vaultmtg/staging/ALLOWED_ORIGINS`
**Issue ref**: #2382

---

## Context

The staging BFF reads allowed CORS origins from the `ALLOWED_ORIGINS` environment variable at
startup. The environment variable is written to `/etc/mtga-companion-staging/env` by
`provision-staging-env.sh`, which reads its value from the SSM parameter
`/vaultmtg/staging/ALLOWED_ORIGINS`.

The BFF's CORS middleware splits the value on commas:

```
https://stg-app.vaultmtg.app,https://another-origin.example.com
```

---

## Adding or updating an allowed origin in staging

### 1. Read the current value

```bash
aws ssm get-parameter \
  --name "/vaultmtg/staging/ALLOWED_ORIGINS" \
  --profile personal
```

### 2. Update the parameter

Replace `<current-value>` with the existing value (omit if creating from scratch).

```bash
aws ssm put-parameter \
  --name "/vaultmtg/staging/ALLOWED_ORIGINS" \
  --value "https://stg-app.vaultmtg.app" \
  --type "String" \
  --overwrite \
  --profile personal
```

Multiple origins are comma-separated:

```bash
aws ssm put-parameter \
  --name "/vaultmtg/staging/ALLOWED_ORIGINS" \
  --value "https://stg-app.vaultmtg.app,https://stg-admin.vaultmtg.app" \
  --type "String" \
  --overwrite \
  --profile personal
```

### 3. Re-provision the env file on the EC2 instance

The full provision script is `provision-staging-env.sh`. It reads all staging SSM parameters and
re-writes `/etc/mtga-companion-staging/env` from scratch. Run via SSM RunCommand (requires all
staging SSM parameters to be present — see Prerequisites below):

```bash
INSTANCE_ID="i-0226bf51fcf09b506"
BUCKET="vaultmtg-deploy-artifacts-staging"

COMMAND_ID=$(aws ssm send-command \
  --instance-ids "$INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters "commands=[\"aws s3 cp s3://${BUCKET}/scripts/deploy-env.sh /tmp/deploy-env.sh && aws s3 cp s3://${BUCKET}/scripts/provision-staging-env.sh /tmp/provision-staging-env.sh && chmod +x /tmp/provision-staging-env.sh && /tmp/provision-staging-env.sh\"]" \
  --region us-east-1 \
  --profile personal \
  --query "Command.CommandId" \
  --output text)
```

Alternatively, for a targeted single-key update without running the full provisioner (e.g. during
partial environment bootstrapping), write only the changed line:

```bash
INSTANCE_ID="i-0226bf51fcf09b506"
VALUE=$(aws ssm get-parameter \
  --name "/vaultmtg/staging/ALLOWED_ORIGINS" \
  --query "Parameter.Value" \
  --output text \
  --profile personal)

COMMAND_ID=$(aws ssm send-command \
  --instance-ids "$INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters "commands=[\"sed -i '/^ALLOWED_ORIGINS=/d' /etc/mtga-companion-staging/env && echo \\\"ALLOWED_ORIGINS=${VALUE}\\\" >> /etc/mtga-companion-staging/env\"]" \
  --region us-east-1 \
  --profile personal \
  --query "Command.CommandId" \
  --output text)
```

### 4. Restart the staging BFF service

```bash
INSTANCE_ID="i-0226bf51fcf09b506"
BUCKET="vaultmtg-deploy-artifacts-staging"

COMMAND_ID=$(aws ssm send-command \
  --instance-ids "$INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters "commands=[\"aws s3 cp s3://${BUCKET}/scripts/deploy-env.sh /tmp/deploy-env.sh && aws s3 cp s3://${BUCKET}/scripts/restart-bff-staging.sh /tmp/restart-bff-staging.sh && chmod +x /tmp/restart-bff-staging.sh && /tmp/restart-bff-staging.sh\"]" \
  --region us-east-1 \
  --profile personal \
  --query "Command.CommandId" \
  --output text)
```

### 5. Verify CORS headers

```bash
curl -s -i \
  -H "Origin: https://stg-app.vaultmtg.app" \
  -X OPTIONS \
  https://staging-api.vaultmtg.app/health \
  | grep -i "access-control"
```

Expected response header:

```
Access-Control-Allow-Origin: https://stg-app.vaultmtg.app
```

---

## Prerequisites for full staging environment

The following SSM parameters must all exist before `provision-staging-env.sh` can complete:

| SSM path | Purpose |
|---|---|
| `/vaultmtg/staging/ALLOWED_ORIGINS` | CORS allowed origins (created by #2382) |
| `/vaultmtg/staging/PORT` | BFF listen port |
| `/vaultmtg/staging/CLERK_SECRET_KEY` | Clerk backend secret (SecureString) |
| `/vaultmtg/staging/CLERK_PUBLISHABLE_KEY` | Clerk publishable key |
| `/vaultmtg/staging/CLERK_FRONTEND_API` | Clerk Frontend API URL |
| `/vaultmtg/staging/db-secret-arn` | Secrets Manager ARN for DB credentials |
| `/vaultmtg/staging/db-endpoint` | RDS endpoint host |
| `/vaultmtg/staging/db-name` | Database name |
| `/vaultmtg/staging/resend-api-key` | Resend API key |
| `/vaultmtg/staging/sentry-bff-dsn` | Sentry DSN for BFF |
| `/vaultmtg/staging/discord-bot-token` | Discord bot token |
| `/vaultmtg/staging/discord-guild-id` | Discord guild ID |
| `/vaultmtg/staging/mailchimp-api-key` | Mailchimp API key |
| `/vaultmtg/staging/mailchimp-list-id` | Mailchimp list ID |
| `/vaultmtg/staging/crisp-website-id` | Crisp website ID |

The staging BFF systemd unit (`/etc/systemd/system/mtga-bff-staging.service`) must also be
installed and enabled on the EC2 instance. See `infra/scripts/install-staging-service.sh`.

---

## Production

Production `ALLOWED_ORIGINS` is at `/vaultmtg/production/ALLOWED_ORIGINS` and is managed by the
production deploy pipeline (`provision-env.sh`). Never update it through this runbook — it is
only touched by `release.yml`.
