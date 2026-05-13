#!/bin/sh
# provision-staging-env.sh
# Renders /etc/mtga-companion-staging/env from SSM parameter hierarchy.
# Reads from /mtga-companion/staging/* and /vaultmtg/staging/* paths.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Secrets are read on the EC2 instance using the instance IAM role --
# no plaintext credentials are passed through CI/CD.

set -e

REGION=us-east-1
ENV_FILE=/etc/mtga-companion-staging/env
ENV_DIR=/etc/mtga-companion-staging

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
write_param PORT                    /mtga-companion/staging/PORT
write_param ALLOWED_ORIGINS         /mtga-companion/staging/ALLOWED_ORIGINS
write_param CLERK_PUBLISHABLE_KEY   /mtga-companion/staging/CLERK_PUBLISHABLE_KEY
write_param CLERK_SECRET_KEY        /mtga-companion/staging/CLERK_SECRET_KEY --with-decryption
write_param CLERK_FRONTEND_API      /mtga-companion/staging/CLERK_FRONTEND_API

# DB_SECRET_ARN causes the BFF to fetch credentials from Secrets Manager at
# startup, so the env file never holds a password that can go stale after
# an RDS rotation.  DATABASE_URL provides only the host/port/dbname.
write_param DB_SECRET_ARN           /mtga-companion/staging/db-secret-arn
DB_ENDPOINT=$(aws ssm get-parameter --name /mtga-companion/staging/db-endpoint --region "$REGION" --query Parameter.Value --output text)
DB_NAME=$(aws ssm get-parameter --name /mtga-companion/staging/db-name --region "$REGION" --query Parameter.Value --output text)
printf 'DATABASE_URL=postgresql://%s:5432/%s?sslmode=require\n' "$DB_ENDPOINT" "$DB_NAME" >> "$ENV_FILE"
echo "DATABASE_URL provisioned (credentials omitted; resolved via DB_SECRET_ARN at startup)."

# VaultMTG service keys
write_param RESEND_API_KEY          /vaultmtg/staging/resend-api-key         --with-decryption
write_param SENTRY_DSN              /vaultmtg/staging/sentry-bff-dsn
write_param DISCORD_BOT_TOKEN       /vaultmtg/staging/discord-bot-token      --with-decryption
write_param DISCORD_GUILD_ID        /vaultmtg/staging/discord-guild-id
write_param MAILCHIMP_API_KEY       /vaultmtg/staging/mailchimp-api-key      --with-decryption
write_param MAILCHIMP_LIST_ID       /vaultmtg/staging/mailchimp-list-id
write_param CRISP_WEBSITE_ID        /vaultmtg/staging/crisp-website-id

chmod 600 "$ENV_FILE"
echo "Staging env provisioned at ${ENV_FILE}."
