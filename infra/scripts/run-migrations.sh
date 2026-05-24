#!/usr/bin/env bash
# run-migrations.sh
#
# Applies all golang-migrate migrations from
# services/bff/internal/storage/migrations/postgres/ to the production database.
#
# Idempotent: golang-migrate tracks applied versions in the schema_migrations
# table. Re-running this script when already at HEAD is a no-op.
#
# Credential model (#2461 env-file pattern):
#   DATABASE_URL is sourced from $BFF_ENV_FILE (/etc/mtga-companion/env).
#   That file is written by provision-db-url.sh at deploy time with inline
#   credentials spliced from Secrets Manager under the provisioner role.
#   The EC2 instance role (mtga-companion-ec2-role-production) is NOT granted
#   secretsmanager:GetSecretValue on the RDS-managed credential (S-03 least-
#   privilege); using the env file avoids any direct SM call from this script.
#   This mirrors the pattern established in PR #2539/#2540 for the BFF binary.
#
# SSM parameter names and env file path are sourced from
# infra/config/deploy-env.sh — do NOT hardcode them here.
#
# Usage (via SSM from the deploy workflow — not run locally):
#   SSM command with DEPLOY_BUCKET and AWS_REGION env vars injected.

set -euo pipefail

# Source canonical deploy facts.  deploy-env.sh is downloaded alongside
# this script from S3 into /tmp/ before execution.
. /tmp/deploy-env.sh

REGION="${AWS_REGION:-$DEPLOY_REGION}"
DEPLOY_BUCKET="${DEPLOY_BUCKET:-}"

# Download migrations from S3 (uploaded by release.yml).
if [[ -n "$DEPLOY_BUCKET" ]]; then
    MIGRATIONS_DIR="/tmp/prod-migrations-postgres"
    mkdir -p "$MIGRATIONS_DIR"
    echo "[run-migrations] Downloading migrations from s3://$DEPLOY_BUCKET/migrations/postgres/ ..."
    aws s3 sync "s3://$DEPLOY_BUCKET/migrations/postgres/" "$MIGRATIONS_DIR/" --region "$REGION"
    echo "[run-migrations] Migrations downloaded."
else
    echo "[run-migrations] ERROR: DEPLOY_BUCKET is required." >&2
    exit 1
fi

# Install golang-migrate CLI if not present.
if ! command -v migrate &>/dev/null; then
    echo "[run-migrations] migrate CLI not found -- installing ..."
    MIGRATE_VERSION="v4.18.3"
    curl -fsSL \
        "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
        -o /tmp/migrate.tar.gz
    tar -xzf /tmp/migrate.tar.gz -C /tmp
    install -m 0755 /tmp/migrate /usr/local/bin/migrate
    echo "[run-migrations] migrate ${MIGRATE_VERSION} installed."
fi

# Install psql if not present (EC2 UserData does not install postgresql).
if ! command -v psql &>/dev/null; then
    echo "[run-migrations] psql not found -- installing postgresql15 ..."
    dnf install -y postgresql15
    echo "[run-migrations] postgresql15 installed."
fi

# Source DATABASE_URL from the production env file.
# provision-db-url.sh (PR #2540) writes inline credentials into this file at
# deploy time under the vaultmtg-staging-deploy-provisioner role.  The EC2
# instance role lacks secretsmanager:GetSecretValue on the RDS secret (per
# S-03 / #2375) so we must not call SM here — the env file is the single
# source of truth for credentials.
if [[ ! -f "$BFF_ENV_FILE" ]]; then
    echo "[run-migrations] ERROR: env file not found at $BFF_ENV_FILE" >&2
    echo "  Ensure provision-db-url.sh ran successfully before this step." >&2
    exit 1
fi

# shellcheck source=/etc/mtga-companion/env
. "$BFF_ENV_FILE"

if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "[run-migrations] ERROR: DATABASE_URL is empty or unset in $BFF_ENV_FILE" >&2
    exit 1
fi

# golang-migrate requires a postgres:// scheme; the env file uses postgresql://.
MIGRATE_DB_URL="${DATABASE_URL/postgresql:\/\//postgres://}"

# Parse credentials and connection details from DATABASE_URL for psql.
# URL shape: postgresql://USER:PASS@HOST:PORT/DB?sslmode=...
# Use python3 (already on Amazon Linux 2023) for URL parsing — shell
# parameter expansion is not robust against special characters in passwords.
MASTER_USER=$(python3 -c "
import sys, urllib.parse
u = urllib.parse.urlparse('${DATABASE_URL}')
print(u.username)
")
MASTER_PASSWORD=$(python3 -c "
import sys, urllib.parse
u = urllib.parse.urlparse('${DATABASE_URL}')
print(urllib.parse.unquote(u.password))
")
DB_ENDPOINT=$(python3 -c "
import sys, urllib.parse
u = urllib.parse.urlparse('${DATABASE_URL}')
print(u.hostname)
")
DB_NAME=$(python3 -c "
import sys, urllib.parse
u = urllib.parse.urlparse('${DATABASE_URL}')
print(u.path.lstrip('/'))
")

if [[ -z "$MASTER_USER" || -z "$MASTER_PASSWORD" || -z "$DB_ENDPOINT" || -z "$DB_NAME" ]]; then
    echo "[run-migrations] ERROR: could not parse credentials from DATABASE_URL" >&2
    exit 1
fi

echo "[run-migrations] Applying migrations ..."
echo "[run-migrations] Target DB: postgres://${MASTER_USER}@${DB_ENDPOINT}:${DB_PORT}/${DB_NAME}?${DB_SSL_MODE}"

# If a previous migration run failed mid-flight, golang-migrate marks the
# schema_migrations table as dirty and refuses to proceed. Detect that state
# and force the version back to the last clean version so the fixed migration
# can re-run automatically without manual intervention.
VERSION_OUTPUT=$(migrate \
    -path     "$MIGRATIONS_DIR" \
    -database "$MIGRATE_DB_URL" \
    version 2>&1 || true)
if echo "$VERSION_OUTPUT" | grep -qi "dirty"; then
    DIRTY_VER=$(echo "$VERSION_OUTPUT" | grep -oE '[0-9]+' | head -1)
    CLEAN_VER=$((DIRTY_VER - 1))
    # CLEAN_VER -1 tells golang-migrate to revert to "no migrations applied" which
    # is safe and will let the full migration set re-run from scratch.
    echo "[run-migrations] Dirty state detected at version $DIRTY_VER — forcing back to $CLEAN_VER ..."
    migrate \
        -path     "$MIGRATIONS_DIR" \
        -database "$MIGRATE_DB_URL" \
        force "$CLEAN_VER"
    echo "[run-migrations] Forced to version $CLEAN_VER."
fi

migrate \
    -path     "$MIGRATIONS_DIR" \
    -database "$MIGRATE_DB_URL" \
    up

echo "[run-migrations] Migrations complete."

# Apply table-level grants to the production app user.
echo "[run-migrations] Applying production table grants ..."

GRANT_SQL="/tmp/grant-production-tables.sql"
aws s3 cp "s3://$DEPLOY_BUCKET/infra-db/grant-production-tables.sql" "$GRANT_SQL" --region "$REGION"

PGPASSWORD="$MASTER_PASSWORD" psql \
    -h "$DB_ENDPOINT" \
    -U "$MASTER_USER" \
    -d "$DB_NAME" \
    -v ON_ERROR_STOP=1 \
    -f "$GRANT_SQL"

echo "[run-migrations] Production table grants applied."
echo "[run-migrations] Production database fully migrated and ready."
