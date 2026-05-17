#!/usr/bin/env bash
# run-migrations.sh
#
# Applies all golang-migrate migrations from
# services/bff/internal/storage/migrations/postgres/ to the production database.
#
# Idempotent: golang-migrate tracks applied versions in the schema_migrations
# table. Re-running this script when already at HEAD is a no-op.
#
# The DATABASE_URL is read from /etc/mtga-companion/env (written by
# provision-db-url.sh earlier in the deploy pipeline).
#
# Usage (via SSM from the deploy workflow — not run locally):
#   SSM command with DEPLOY_BUCKET and AWS_REGION env vars injected.

set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
DEPLOY_BUCKET="${DEPLOY_BUCKET:-}"
ENV_FILE="/etc/mtga-companion/env"

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

# Read DATABASE_URL from the env file provisioned earlier in the deploy.
if [[ ! -f "$ENV_FILE" ]]; then
    echo "[run-migrations] ERROR: $ENV_FILE not found. Run provision-db-url.sh first." >&2
    exit 1
fi

DATABASE_URL=$(grep '^DATABASE_URL=' "$ENV_FILE" | cut -d= -f2- | tr -d '"')
if [[ -z "$DATABASE_URL" ]]; then
    echo "[run-migrations] ERROR: DATABASE_URL not found in $ENV_FILE." >&2
    exit 1
fi

# Normalize to postgres:// scheme.
DATABASE_URL="${DATABASE_URL/pgx5:\/\//postgres://}"
DATABASE_URL="${DATABASE_URL/postgresql:\/\//postgres://}"

echo "[run-migrations] Applying migrations ..."
echo "[run-migrations] Target DB: ${DATABASE_URL%%@*}@<host redacted>"

migrate \
    -path     "$MIGRATIONS_DIR" \
    -database "$DATABASE_URL" \
    up

echo "[run-migrations] Migrations complete."

# Apply table-level grants to the production app user.
echo "[run-migrations] Applying production table grants ..."

# Install psql if not present (EC2 UserData does not install postgresql).
if ! command -v psql &>/dev/null; then
    echo "[run-migrations] psql not found -- installing postgresql15 ..."
    dnf install -y postgresql15
    echo "[run-migrations] postgresql15 installed."
fi

GRANT_SQL="/tmp/grant-production-tables.sql"
aws s3 cp "s3://$DEPLOY_BUCKET/infra-db/grant-production-tables.sql" "$GRANT_SQL" --region "$REGION"

SECRET_ARN=$(aws ssm get-parameter \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-secret-arn" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_JSON=$(aws secretsmanager get-secret-value \
    --region    "$REGION" \
    --secret-id "$SECRET_ARN" \
    --query     "SecretString" \
    --output    text)

MASTER_PASSWORD=$(echo "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
MASTER_USER=$(echo     "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['username'])")

DB_ENDPOINT=$(aws ssm get-parameter \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-endpoint" \
    --query   "Parameter.Value" \
    --output  text)

DB_NAME=$(aws ssm get-parameter \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-name" \
    --query   "Parameter.Value" \
    --output  text)

PGPASSWORD="$MASTER_PASSWORD" psql \
    -h "$DB_ENDPOINT" \
    -U "$MASTER_USER" \
    -d "$DB_NAME" \
    -v ON_ERROR_STOP=1 \
    -f "$GRANT_SQL"

echo "[run-migrations] Production table grants applied."
echo "[run-migrations] Production database fully migrated and ready."
