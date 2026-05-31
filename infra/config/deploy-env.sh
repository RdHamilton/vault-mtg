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
BFF_SERVICE="vaultmtg-bff"
BFF_STAGING_SERVICE="vault-mtg-bff-staging"

# Legacy rename-prep constant retained for callers that may still reference it
# (ADR-022 Phase 5c, original ticket #1755).  Now equal to BFF_SERVICE since
# the production unit cutover (Window B PRs #2532, #2536, #2540) is complete.
BFF_SERVICE_VAULTMTG="vaultmtg-bff"

# ───────────────────────────────────────────────────────────────────────────
# BINARY NAMES (installed under /usr/local/bin/)
#   Used by: stage-binary.sh, stage-binary-staging.sh
#
#   NOTE: The release pipeline (.github/workflows/release.yml) produces the
#   artifact `mtga-bff` and uploads it under that S3 key.  Renaming the
#   installed-binary filename here would cascade through release.yml,
#   ci.yml, e2e-smoke.yml, and staging-deploy.yml.  That cascade is out of
#   scope for ticket #1755 (filesystem/systemd rename) and is deferred.
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
BFF_ENV_DIR="/etc/vaultmtg"
BFF_ENV_FILE="/etc/vaultmtg/env"
BFF_STAGING_ENV_DIR="/etc/mtga-companion-staging"
BFF_STAGING_ENV_FILE="/etc/mtga-companion-staging/env"

# Legacy rename-prep constants retained for callers that may still reference
# them (ADR-022 Phase 5c, original ticket #1755).  Now equal to the canonical
# BFF_ENV_DIR / BFF_ENV_FILE since the production filesystem cutover is
# complete.  The symlink /etc/mtga-companion -> /etc/vaultmtg remains in
# place on the EC2 instance for any out-of-band tooling that still references
# the legacy path.
BFF_ENV_DIR_VAULTMTG="/etc/vaultmtg"
BFF_ENV_FILE_VAULTMTG="/etc/vaultmtg/env"

# ───────────────────────────────────────────────────────────────────────────
# NGINX WEB-ROOT DIRECTORIES
#   Used by: deploy-frontend.sh
# ───────────────────────────────────────────────────────────────────────────
FRONTEND_DEPLOY_DIR="/var/www/mtga-companion"
FRONTEND_STAGING_DIR="/var/www/mtga-companion-staging"

# Rename-prep target web-root (ADR-022 Phase 5c, ticket #1755).  PR A
# ships symlink shim /var/www/mtga-companion -> /var/www/vaultmtg in
# ec2-bootstrap.sh.  Inert until PR A lands.
FRONTEND_DEPLOY_DIR_VAULTMTG="/var/www/vaultmtg"

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
SSM_PROD_DB_SECRET_ARN="/vaultmtg/app/production/db-secret-arn"
SSM_PROD_APP_DB_SECRET_ARN="/vaultmtg/app/production/app-db-secret-arn"
SSM_PROD_DB_ENDPOINT="/vaultmtg/app/production/db-endpoint"
SSM_PROD_DB_NAME="/vaultmtg/app/production/db-name"
SSM_PROD_ALLOWED_ORIGINS="/vaultmtg/app/production/ALLOWED_ORIGINS"
SSM_PROD_CLERK_SECRET_KEY="/vaultmtg/app/production/CLERK_SECRET_KEY"
SSM_PROD_CLERK_PUBLISHABLE_KEY="/vaultmtg/app/production/CLERK_PUBLISHABLE_KEY"
SSM_PROD_CLERK_FRONTEND_API="/vaultmtg/app/production/CLERK_FRONTEND_API"
SSM_PROD_PORT="/vaultmtg/app/production/PORT"

# v0.3.3 observability — Sentry + PostHog (ticket #16 / N1)
#   SENTRY_DSN_BFF       — Sentry Go project DSN, consumed by BFF at startup
#                          via /etc/vaultmtg/env (provisioned by provision-env.sh).
#   SENTRY_DSN_SPA       — Sentry React project DSN, consumed by the SPA Vite
#                          build at build time. NOT written to /etc/vaultmtg/env;
#                          the SPA build job in deploy-bff.yml will read it
#                          directly and inject as VITE_SENTRY_DSN.
#   POSTHOG_API_KEY      — PostHog project API key, consumed by the SPA Vite
#                          build at build time (VITE_POSTHOG_KEY). NOT written
#                          to /etc/vaultmtg/env.
#   POSTHOG_HOST         — PostHog ingestion host (us.i.posthog.com).
#                          Plain String — not a secret.
#   SecureString for all DSN/API-key paths; String for POSTHOG_HOST.
#   Staging mirror is deliberately deferred: the existing staging
#   `sentry-bff-dsn` parameter (provision-staging-env.sh line 231)
#   covers staging BFF observability. PostHog and Sentry SDKs tag events
#   with `environment: production|staging` at init time, so a single
#   PostHog API key + single SPA Sentry DSN per surface is workable.
SSM_PROD_SENTRY_DSN_BFF="/vaultmtg/app/production/sentry-dsn-bff"
SSM_PROD_SENTRY_DSN_SPA="/vaultmtg/app/production/sentry-dsn-spa"
SSM_PROD_POSTHOG_API_KEY="/vaultmtg/app/production/posthog-api-key"
SSM_PROD_POSTHOG_HOST="/vaultmtg/app/production/posthog-host"

# Daemon version metadata — read by BFF at startup to serve GET /api/v1/daemon/version.
#   BFF_DAEMON_LATEST_VERSION — semver of the latest published daemon release (e.g. 0.3.5)
#   BFF_DAEMON_RELEASED_AT    — RFC3339 publish timestamp (e.g. 2026-05-30T17:51:09Z)
#   Both are plain String params (no encryption needed; not secrets).
SSM_PROD_BFF_DAEMON_LATEST_VERSION="/vaultmtg/app/production/BFF_DAEMON_LATEST_VERSION"
SSM_PROD_BFF_DAEMON_RELEASED_AT="/vaultmtg/app/production/BFF_DAEMON_RELEASED_AT"

# ───────────────────────────────────────────────────────────────────────────
# SSM PARAMETER PATHS — STAGING
# ───────────────────────────────────────────────────────────────────────────
SSM_STAGING_DB_SECRET_ARN="/vaultmtg/app/staging/db-secret-arn"
SSM_STAGING_DB_ENDPOINT="/vaultmtg/app/staging/db-endpoint"
SSM_STAGING_DB_NAME="/vaultmtg/app/staging/db-name"
SSM_STAGING_DB_PASSWORD="/vaultmtg/app/staging/db-password"
SSM_STAGING_DATABASE_URL="/vaultmtg/app/staging/database-url"
SSM_STAGING_ALLOWED_ORIGINS="/vaultmtg/app/staging/ALLOWED_ORIGINS"
SSM_STAGING_CLERK_SECRET_KEY="/vaultmtg/app/staging/CLERK_SECRET_KEY"
SSM_STAGING_CLERK_PUBLISHABLE_KEY="/vaultmtg/app/staging/CLERK_PUBLISHABLE_KEY"
SSM_STAGING_CLERK_FRONTEND_API="/vaultmtg/app/staging/CLERK_FRONTEND_API"
SSM_STAGING_PORT="/vaultmtg/app/staging/PORT"

# Daemon version metadata — staging mirror (see production block above).
SSM_STAGING_BFF_DAEMON_LATEST_VERSION="/vaultmtg/app/staging/BFF_DAEMON_LATEST_VERSION"
SSM_STAGING_BFF_DAEMON_RELEASED_AT="/vaultmtg/app/staging/BFF_DAEMON_RELEASED_AT"

# ───────────────────────────────────────────────────────────────────────────
# SSM PARAMETER PATHS — VAULTMTG SHARED SERVICES
# ───────────────────────────────────────────────────────────────────────────
SSM_VAULTMTG_STAGING_RESEND_API_KEY="/vaultmtg/app/staging/resend-api-key"
SSM_VAULTMTG_STAGING_SENTRY_DSN="/vaultmtg/app/staging/sentry-bff-dsn"
SSM_VAULTMTG_STAGING_DISCORD_BOT_TOKEN="/vaultmtg/app/staging/discord-bot-token"
SSM_VAULTMTG_STAGING_DISCORD_GUILD_ID="/vaultmtg/app/staging/discord-guild-id"
SSM_VAULTMTG_STAGING_MAILCHIMP_API_KEY="/vaultmtg/app/staging/mailchimp-api-key"
SSM_VAULTMTG_STAGING_MAILCHIMP_LIST_ID="/vaultmtg/app/staging/mailchimp-list-id"
SSM_VAULTMTG_STAGING_CRISP_WEBSITE_ID="/vaultmtg/app/staging/crisp-website-id"

# ───────────────────────────────────────────────────────────────────────────
# EC2 / INFRASTRUCTURE FACTS
# ───────────────────────────────────────────────────────────────────────────
EC2_INSTANCE_TAG="vaultmtg-bff-production"
EC2_INSTANCE_STATE="running"
