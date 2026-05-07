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

PROFILE="${AWS_PROFILE:-personal}"
REGION="${AWS_REGION:-us-east-1}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIGRATIONS_DIR="$REPO_ROOT/services/bff/internal/storage/migrations/postgres"

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
    echo "[run-staging-migrations] ERROR: migrations directory not found at $MIGRATIONS_DIR"
    exit 1
fi

echo "[run-staging-migrations] Fetching staging DATABASE_URL from SSM..."

DATABASE_URL=$(aws ssm get-parameter \
    --profile  "$PROFILE" \
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

SECRET_ARN=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-secret-arn" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_JSON=$(aws secretsmanager get-secret-value \
    --profile   "$PROFILE" \
    --region    "$REGION" \
    --secret-id "$SECRET_ARN" \
    --query     "SecretString" \
    --output    text)

MASTER_PASSWORD=$(echo "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
MASTER_USER=$(echo     "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['username'])")

DB_ENDPOINT=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-endpoint" \
    --query   "Parameter.Value" \
    --output  text)

PGPASSWORD="$MASTER_PASSWORD" psql \
    -h "$DB_ENDPOINT" \
    -U "$MASTER_USER" \
    -d vaultmtg_staging \
    -v ON_ERROR_STOP=1 \
    -f "$REPO_ROOT/infra/db/grant-staging-tables.sql"

echo "[run-staging-migrations] Table grants applied."
echo ""
echo "[run-staging-migrations] vaultmtg_staging is fully initialized and ready."
echo ""
echo "Verify migration head:"
echo "  migrate -path $MIGRATIONS_DIR -database \"\$DATABASE_URL\" version"
