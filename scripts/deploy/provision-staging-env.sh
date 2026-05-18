#!/usr/bin/env bash
# provision-staging-env.sh
# Renders the staging env file from SSM parameter hierarchy.
# Reads from /vaultmtg/staging/* paths.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Secrets are read on the EC2 instance using the instance IAM role --
# no plaintext credentials are passed through CI/CD.
#
# SSM parameter names and file paths are sourced from
# infra/config/deploy-env.sh — do NOT hardcode them here.

set -e

# Source canonical deploy facts.  deploy-env.sh is downloaded alongside
# this script from S3 into /tmp/ before execution.
. /tmp/deploy-env.sh

REGION="$DEPLOY_REGION"
ENV_FILE="$BFF_STAGING_ENV_FILE"
ENV_DIR="$BFF_STAGING_ENV_DIR"

mkdir -p "$ENV_DIR"
# Start with an empty env file -- fully re-render on each deploy.
: > "$ENV_FILE"
chmod 600 "$ENV_FILE"

# Helper: fetch an SSM parameter value and append KEY=VALUE to the env file.
# Usage: write_param ENV_KEY SSM_PATH [--with-decryption]
write_param() {
  local key="$1"
  local path="$2"
  local decrypt="${3:-}"

  if [ "$decrypt" = "--with-decryption" ]; then
    VALUE=$(aws ssm get-parameter \
      --name "$path" \
      --with-decryption \
      --region "$REGION" \
      --query Parameter.Value \
      --output text)
  else
    VALUE=$(aws ssm get-parameter \
      --name "$path" \
      --region "$REGION" \
      --query Parameter.Value \
      --output text)
  fi

  if [ -z "$VALUE" ]; then
    echo "ERROR: SSM parameter ${path} is empty." >&2
    exit 1
  fi

  printf '%s=%s\n' "$key" "$VALUE" >> "$ENV_FILE"
  echo "${key} provisioned."
}

# AWS region — required by the BFF's Secrets Manager client at startup.
printf 'AWS_DEFAULT_REGION=%s\n' "$REGION" >> "$ENV_FILE"
echo "AWS_DEFAULT_REGION provisioned."

# Core BFF settings
write_param PORT                    "$SSM_STAGING_PORT"
write_param ALLOWED_ORIGINS         "$SSM_STAGING_ALLOWED_ORIGINS"
write_param CLERK_PUBLISHABLE_KEY   "$SSM_STAGING_CLERK_PUBLISHABLE_KEY"
write_param CLERK_SECRET_KEY        "$SSM_STAGING_CLERK_SECRET_KEY" --with-decryption
write_param CLERK_FRONTEND_API      "$SSM_STAGING_CLERK_FRONTEND_API"

# DB_SECRET_ARN causes the BFF to fetch credentials from Secrets Manager at
# startup, so the env file never holds a password that can go stale after
# an RDS rotation.  DATABASE_URL provides only the host/port/dbname.
write_param DB_SECRET_ARN           "$SSM_STAGING_DB_SECRET_ARN"
DB_ENDPOINT=$(aws ssm get-parameter --name "$SSM_STAGING_DB_ENDPOINT" --region "$REGION" --query Parameter.Value --output text)
DB_NAME=$(aws ssm get-parameter --name "$SSM_STAGING_DB_NAME" --region "$REGION" --query Parameter.Value --output text)
printf 'DATABASE_URL=postgresql://%s:%s/%s?%s\n' "$DB_ENDPOINT" "$DB_PORT" "$DB_NAME" "$DB_SSL_MODE" >> "$ENV_FILE"
echo "DATABASE_URL provisioned (credentials omitted; resolved via DB_SECRET_ARN at startup)."

# VaultMTG service keys
write_param RESEND_API_KEY          "$SSM_VAULTMTG_STAGING_RESEND_API_KEY"         --with-decryption
write_param SENTRY_DSN              "$SSM_VAULTMTG_STAGING_SENTRY_DSN"
write_param DISCORD_BOT_TOKEN       "$SSM_VAULTMTG_STAGING_DISCORD_BOT_TOKEN"      --with-decryption
write_param DISCORD_GUILD_ID        "$SSM_VAULTMTG_STAGING_DISCORD_GUILD_ID"
write_param MAILCHIMP_API_KEY       "$SSM_VAULTMTG_STAGING_MAILCHIMP_API_KEY"      --with-decryption
write_param MAILCHIMP_LIST_ID       "$SSM_VAULTMTG_STAGING_MAILCHIMP_LIST_ID"
write_param CRISP_WEBSITE_ID        "$SSM_VAULTMTG_STAGING_CRISP_WEBSITE_ID"

chmod 600 "$ENV_FILE"
echo "Staging env provisioned at ${ENV_FILE}."
