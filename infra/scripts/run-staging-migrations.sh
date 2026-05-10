#!/usr/bin/env bash
# run-staging-migrations.sh
#
# Applies all golang-migrate migrations from
# services/bff/internal/storage/migrations/postgres/ to the vaultmtg_staging
# database on the shared RDS instance.
#
# The BFF binary embeds migrations at compile time (migrate.go). For the
# staging bootstrap and CI deploy we run migrations via the standalone
# golang-migrate CLI so we don't need to build the full binary first.
#
# Idempotent: golang-migrate tracks applied versions in the schema_migrations
# table. Re-running this script when already at HEAD is a no-op.
#
# Prerequisites:
#   - golang-migrate CLI installed (see https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)
#     Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
#   - Access to AWS SSM (personal profile) to read /mtga-companion/staging/database-url
#   - Network access to RDS (run from the EC2 instance via SSM, or from a
#     machine with VPC access)
#
# Usage:
#   # Run locally (requires VPN or EC2 tunnel):
#   AWS_PROFILE=personal bash infra/scripts/run-staging-migrations.sh
#
#   # Run on EC2 via SSM:
#   aws ssm send-command --profile personal \
#     --instance-ids <EC2_INSTANCE_ID> \
#     --document-name AWS-RunShellScript \
#     --parameters 'commands=["cd /opt/vaultmtg && bash infra/scripts/run-staging-migrations.sh"]'

set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"

# DEPLOY_BUCKET is set by the staging deploy workflow (injected via SSM command
# environment). When set, migrations are downloaded from S3 instead of read from
# a local repo checkout -- this is the normal path on the EC2 staging instance.
DEPLOY_BUCKET="${DEPLOY_BUCKET:-}"

# When run on EC2 via SSM (from /tmp), BASH_SOURCE[0] resolves to /tmp and
# the relative ../../ traversal produces a broken path. Use the canonical EC2
# repo location when the relative path doesn't contain a services/ tree, then
# fall back to the relative path for local development use.
_SCRIPT_RELATIVE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if [[ -d "$_SCRIPT_RELATIVE_ROOT/services/bff" ]]; then
    REPO_ROOT="$_SCRIPT_RELATIVE_ROOT"
else
    REPO_ROOT="/opt/mtga-companion"
fi
MIGRATIONS_DIR="$REPO_ROOT/services/bff/internal/storage/migrations/postgres"

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
    if [[ -n "$DEPLOY_BUCKET" ]]; then
        # CI deploy path: download migrations from S3 (uploaded by staging-deploy.yml)
        echo "[run-staging-migrations] Repo not at $REPO_ROOT -- downloading migrations from s3://$DEPLOY_BUCKET/migrations/postgres/ ..."
        MIGRATIONS_DIR="/tmp/staging-migrations-postgres"
        mkdir -p "$MIGRATIONS_DIR"
        aws s3 sync "s3://$DEPLOY_BUCKET/migrations/postgres/" "$MIGRATIONS_DIR/" --region "$REGION"
        echo "[run-staging-migrations] Migrations downloaded to $MIGRATIONS_DIR"
    else
        echo "[run-staging-migrations] ERROR: migrations directory not found at $MIGRATIONS_DIR"
        echo "  Expected: $MIGRATIONS_DIR"
        echo "  On EC2 set DEPLOY_BUCKET to download migrations from S3."
        echo "  Locally: run from the repo root or set REPO_ROOT."
        exit 1
    fi
fi

# Install golang-migrate CLI if not present.
# On the EC2 staging instance this is not pre-installed; we fetch the latest
# linux/amd64 release tarball from GitHub and install to /usr/local/bin.
if ! command -v migrate &>/dev/null; then
    echo "[run-staging-migrations] migrate CLI not found -- installing ..."
    MIGRATE_VERSION="v4.18.3"
    MIGRATE_TARBALL="migrate.linux-amd64.tar.gz"
    curl -fsSL \
        "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/${MIGRATE_TARBALL}" \
        -o "/tmp/${MIGRATE_TARBALL}"
    tar -xzf "/tmp/${MIGRATE_TARBALL}" -C /tmp
    install -m 0755 /tmp/migrate /usr/local/bin/migrate
    echo "[run-staging-migrations] migrate ${MIGRATE_VERSION} installed."
fi

echo "[run-staging-migrations] Fetching staging DATABASE_URL from SSM..."

# On EC2 the instance IAM role provides credentials -- no named profile exists.
# Locally, AWS_PROFILE can be set to override (defaults to 'personal').
_PROFILE_ARG=()
if [[ -n "${AWS_PROFILE:-}" ]]; then
    _PROFILE_ARG=(--profile "$AWS_PROFILE")
fi

DATABASE_URL=$(aws ssm get-parameter \
    "${_PROFILE_ARG[@]}" \
    --region   "$REGION" \
    --name     "/mtga-companion/staging/database-url" \
    --with-decryption \
    --query    "Parameter.Value" \
    --output   text)

if [[ -z "$DATABASE_URL" ]]; then
    echo "[run-staging-migrations] ERROR: /mtga-companion/staging/database-url is empty."
    echo "  Run infra/scripts/create-staging-db.sh first."
    exit 1
fi

# golang-migrate expects a postgres:// DSN (not pgx5://).
# Normalize to postgres:// if the SSM value uses a different scheme.
DATABASE_URL="${DATABASE_URL/pgx5:\/\//postgres://}"
DATABASE_URL="${DATABASE_URL/postgresql:\/\//postgres://}"

echo "[run-staging-migrations] Applying migrations from $MIGRATIONS_DIR ..."
echo "[run-staging-migrations] Target DB: ${DATABASE_URL%%@*}@<host redacted>"

migrate \
    -path    "$MIGRATIONS_DIR" \
    -database "$DATABASE_URL" \
    up

echo "[run-staging-migrations] Migrations complete."

# ---------------------------------------------------------------------------
# Post-migration: grant table and sequence privileges to vaultmtg_staging_app.
# Executed as the master user (stored in the DATABASE_URL at this point we
# re-fetch master creds).
# ---------------------------------------------------------------------------
echo "[run-staging-migrations] Applying table-level grants..."

# Staging master credentials are stored under the staging SSM path tree,
# not production. Using production paths here was a bug that caused a
# permissions error on every staging deploy.
SECRET_ARN=$(aws ssm get-parameter \
    "${_PROFILE_ARG[@]}" \
    --region  "$REGION" \
    --name    "/mtga-companion/staging/db-secret-arn" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_JSON=$(aws secretsmanager get-secret-value \
    "${_PROFILE_ARG[@]}" \
    --region    "$REGION" \
    --secret-id "$SECRET_ARN" \
    --query     "SecretString" \
    --output    text)

MASTER_PASSWORD=$(echo "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
MASTER_USER=$(echo     "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['username'])")

DB_ENDPOINT=$(aws ssm get-parameter \
    "${_PROFILE_ARG[@]}" \
    --region  "$REGION" \
    --name    "/mtga-companion/staging/db-endpoint" \
    --query   "Parameter.Value" \
    --output  text)

GRANT_SQL="$REPO_ROOT/infra/db/grant-staging-tables.sql"
if [[ ! -f "$GRANT_SQL" ]] && [[ -n "$DEPLOY_BUCKET" ]]; then
    echo "[run-staging-migrations] Downloading grant-staging-tables.sql from S3 ..."
    aws s3 cp "s3://$DEPLOY_BUCKET/infra-db/grant-staging-tables.sql" /tmp/grant-staging-tables.sql --region "$REGION"
    GRANT_SQL="/tmp/grant-staging-tables.sql"
fi

PGPASSWORD="$MASTER_PASSWORD" psql \
    -h "$DB_ENDPOINT" \
    -U "$MASTER_USER" \
    -d vaultmtg_staging \
    -v ON_ERROR_STOP=1 \
    -f "$GRANT_SQL"

echo "[run-staging-migrations] Table grants applied."
echo ""
echo "[run-staging-migrations] vaultmtg_staging is fully initialized and ready."
echo ""
echo "Verify migration head:"
echo "  migrate -path $MIGRATIONS_DIR -database \"\$DATABASE_URL\" version"
