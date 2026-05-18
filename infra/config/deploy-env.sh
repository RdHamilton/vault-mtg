#!/bin/sh
# infra/config/deploy-env.sh
#
# SINGLE SOURCE OF TRUTH for VaultMTG deploy environment facts.
# shellcheck disable=SC2034  # Variables are used by sourcing scripts, not in this file.
#
# Every deploy script and infra script MUST source this file rather than
# defining these facts inline.  One change here propagates to all consumers.
#
# Usage (POSIX sh):
#   . "$(dirname "$0")/../../infra/config/deploy-env.sh"    # relative
#   . /tmp/deploy-env.sh                                      # after S3 download
#
# On EC2 (SSM RunShellScript): scripts/deploy/ are downloaded from S3 into
# /tmp/ alongside this file before any script runs.  Each script sources it
# with: . /tmp/deploy-env.sh
#
# Do NOT export secrets here.  This file holds structural facts only:
# names, paths, and SSM parameter keys.  Actual secret values are always
# fetched at runtime from Secrets Manager / SSM by the consuming script.
#
# ───────────────────────────────────────────────────────────────────────────
# AWS REGION
# ───────────────────────────────────────────────────────────────────────────
DEPLOY_REGION="us-east-1"

# ───────────────────────────────────────────────────────────────────────────
# SYSTEMD SERVICE NAMES
#   Used by: restart-bff.sh, restart-bff-staging.sh
#   Mismatch impact: systemctl restart fails; deploy silently does nothing
# ───────────────────────────────────────────────────────────────────────────
BFF_SERVICE="mtga-companion"
BFF_STAGING_SERVICE="mtga-bff-staging"

# ───────────────────────────────────────────────────────────────────────────
# BINARY NAMES (installed under /usr/local/bin/)
#   Used by: stage-binary.sh, stage-binary-staging.sh
# ───────────────────────────────────────────────────────────────────────────
BFF_BINARY="mtga-bff"
BFF_STAGING_BINARY="mtga-bff-staging"

# ───────────────────────────────────────────────────────────────────────────
# ENV FILES (read by systemd EnvironmentFile= and provisioning scripts)
#   Used by: provision-db-url.sh, provision-env.sh, provision-staging-env.sh,
#            run-migrations.sh
#   Mismatch impact: BFF starts without the provisioned env; crashes or
#                    falls back to insecure defaults
# ───────────────────────────────────────────────────────────────────────────
BFF_ENV_DIR="/etc/mtga-companion"
BFF_ENV_FILE="/etc/mtga-companion/env"
BFF_STAGING_ENV_DIR="/etc/mtga-companion-staging"
BFF_STAGING_ENV_FILE="/etc/mtga-companion-staging/env"

# ───────────────────────────────────────────────────────────────────────────
# NGINX WEB-ROOT DIRECTORIES
#   Used by: deploy-frontend.sh
# ───────────────────────────────────────────────────────────────────────────
FRONTEND_DEPLOY_DIR="/var/www/mtga-companion"
FRONTEND_STAGING_DIR="/var/www/mtga-companion-staging"

# ───────────────────────────────────────────────────────────────────────────
# DB CREDENTIAL MODEL
#   The BFF never reads a plaintext password from the env file.  Instead it
#   reads DB_SECRET_ARN and calls Secrets Manager at startup to obtain the
#   current username/password.  This allows RDS credential rotation without
#   a redeploy.
#
#   DATABASE_URL contains ONLY host/port/dbname — no credentials.
#   DB_SECRET_ARN is the Secrets Manager ARN for the JSON secret
#   {"username":"...","password":"..."}.
#
#   Mismatch impact: BFF fails to connect after an RDS rotation if a
#   plaintext password is hardcoded or if the wrong secret ARN is used.
# ───────────────────────────────────────────────────────────────────────────
DB_PORT="5432"
DB_SSL_MODE="sslmode=require"

# ───────────────────────────────────────────────────────────────────────────
# POSTGRESQL DB / ROLE NAMES
#   Production app role: vaultmtg_app
#   Staging app role:    vaultmtg_staging_app
#   Staging database:    vaultmtg_staging
#
#   These names must match what is provisioned in CloudFormation / RDS
#   and in infra/db/grant-production-tables.sql,
#   infra/db/grant-staging-tables.sql, and create-staging-db.sh.
#
#   Mismatch impact: GRANT statements target the wrong role; app role has
#   no table permissions; all BFF queries fail with permission-denied.
# ───────────────────────────────────────────────────────────────────────────
DB_APP_ROLE="vaultmtg_app"
DB_STAGING_APP_ROLE="vaultmtg_staging_app"
DB_STAGING_NAME="vaultmtg_staging"

# ───────────────────────────────────────────────────────────────────────────
# SSM PARAMETER PATHS — PRODUCTION
#   Each constant is the canonical SSM parameter name consumed by deploy
#   scripts and the secrets-inventory pre-flight check in release.yml.
#
#   Mismatch impact: get-parameter returns NoSuchParameter; provisioning
#   step fails and the deploy aborts (or silently skips the env write).
# ───────────────────────────────────────────────────────────────────────────
SSM_PROD_DB_SECRET_ARN="/vaultmtg/production/db-secret-arn"
SSM_PROD_DB_ENDPOINT="/vaultmtg/production/db-endpoint"
SSM_PROD_DB_NAME="/vaultmtg/production/db-name"
SSM_PROD_ALLOWED_ORIGINS="/vaultmtg/production/ALLOWED_ORIGINS"
SSM_PROD_CLERK_SECRET_KEY="/vaultmtg/production/CLERK_SECRET_KEY"
SSM_PROD_CLERK_PUBLISHABLE_KEY="/vaultmtg/production/CLERK_PUBLISHABLE_KEY"
SSM_PROD_CLERK_FRONTEND_API="/vaultmtg/production/CLERK_FRONTEND_API"
SSM_PROD_PORT="/vaultmtg/production/PORT"

# ───────────────────────────────────────────────────────────────────────────
# SSM PARAMETER PATHS — STAGING
# ───────────────────────────────────────────────────────────────────────────
SSM_STAGING_DB_SECRET_ARN="/vaultmtg/staging/db-secret-arn"
SSM_STAGING_DB_ENDPOINT="/vaultmtg/staging/db-endpoint"
SSM_STAGING_DB_NAME="/vaultmtg/staging/db-name"
SSM_STAGING_DB_PASSWORD="/vaultmtg/staging/db-password"
SSM_STAGING_DATABASE_URL="/vaultmtg/staging/database-url"
SSM_STAGING_ALLOWED_ORIGINS="/vaultmtg/staging/ALLOWED_ORIGINS"
SSM_STAGING_CLERK_SECRET_KEY="/vaultmtg/staging/CLERK_SECRET_KEY"
SSM_STAGING_CLERK_PUBLISHABLE_KEY="/vaultmtg/staging/CLERK_PUBLISHABLE_KEY"
SSM_STAGING_CLERK_FRONTEND_API="/vaultmtg/staging/CLERK_FRONTEND_API"
SSM_STAGING_PORT="/vaultmtg/staging/PORT"

# ───────────────────────────────────────────────────────────────────────────
# SSM PARAMETER PATHS — VAULTMTG SHARED SERVICES
# ───────────────────────────────────────────────────────────────────────────
SSM_VAULTMTG_STAGING_RESEND_API_KEY="/vaultmtg/staging/resend-api-key"
SSM_VAULTMTG_STAGING_SENTRY_DSN="/vaultmtg/staging/sentry-bff-dsn"
SSM_VAULTMTG_STAGING_DISCORD_BOT_TOKEN="/vaultmtg/staging/discord-bot-token"
SSM_VAULTMTG_STAGING_DISCORD_GUILD_ID="/vaultmtg/staging/discord-guild-id"
SSM_VAULTMTG_STAGING_MAILCHIMP_API_KEY="/vaultmtg/staging/mailchimp-api-key"
SSM_VAULTMTG_STAGING_MAILCHIMP_LIST_ID="/vaultmtg/staging/mailchimp-list-id"
SSM_VAULTMTG_STAGING_CRISP_WEBSITE_ID="/vaultmtg/staging/crisp-website-id"

# ───────────────────────────────────────────────────────────────────────────
# EC2 / INFRASTRUCTURE FACTS
# ───────────────────────────────────────────────────────────────────────────
EC2_INSTANCE_TAG="mtga-companion-bff-production"
EC2_INSTANCE_STATE="running"
