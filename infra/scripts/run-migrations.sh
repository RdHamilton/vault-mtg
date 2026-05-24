#!/usr/bin/env bash
# run-migrations.sh
#
# Applies all golang-migrate migrations from
# services/bff/internal/storage/migrations/postgres/ to the production database.
#
# Idempotent: golang-migrate tracks applied versions in the schema_migrations
# table. Re-running this script when already at HEAD is a no-op.
#
# Credential model (post #2223 / ADR-024):
#   As of #2223, provision-db-url.sh writes vaultmtg_app (least-privilege DML)
#   credentials into $BFF_ENV_FILE. vaultmtg_app cannot run DDL migrations.
#   This script independently resolves the master (RDS-managed) credential via
#   SSM_PROD_DB_SECRET_ARN and the provisioner role, bypassing $BFF_ENV_FILE
#   for the migration credential. This is required so migrations run under the
#   master user that has DDL rights.
#
#   The two-secret separation is enforced by deploy-chain contract test C4/C5:
#     provision-db-url.sh -> SSM_PROD_APP_DB_SECRET_ARN (vaultmtg_app DML role)
#     run-migrations.sh   -> SSM_PROD_DB_SECRET_ARN     (master DDL role)
#
#   The EC2 instance role (mtga-companion-ec2-role-production) is NOT granted
#   secretsmanager:GetSecretValue on the RDS-managed credential (S-03 least-
#   privilege); this script assumes into vaultmtg-staging-deploy-provisioner
#   (the same role used by provision-db-url.sh) which does hold that grant.
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

# Resolve master (DDL) credentials independently via the provisioner role.
#
# As of #2223: provision-db-url.sh now writes vaultmtg_app (least-privilege
# DML) credentials into $BFF_ENV_FILE. vaultmtg_app lacks DDL rights, so
# migrations must run under the master user. This block independently reads
# SSM_PROD_DB_SECRET_ARN (master/RDS-managed secret) via the provisioner role
# — the same assume-role pattern used by provision-db-url.sh. The provisioner
# role (vaultmtg-staging-deploy-provisioner) holds GetSecretValue on the
# RDS-managed credential; the EC2 instance role does not (S-03 / #2375).

echo "[run-migrations] Reading production SSM parameters under instance role..."
DB_SECRET_ARN_VALUE=$(aws ssm get-parameter \
    --name    "$SSM_PROD_DB_SECRET_ARN" \
    --region  "$REGION" \
    --query   Parameter.Value \
    --output  text)

DB_ENDPOINT=$(aws ssm get-parameter \
    --name    "$SSM_PROD_DB_ENDPOINT" \
    --region  "$REGION" \
    --query   Parameter.Value \
    --output  text)

DB_NAME=$(aws ssm get-parameter \
    --name    "$SSM_PROD_DB_NAME" \
    --region  "$REGION" \
    --query   Parameter.Value \
    --output  text)

if [[ -z "$DB_SECRET_ARN_VALUE" || -z "$DB_ENDPOINT" || -z "$DB_NAME" ]]; then
    echo "[run-migrations] ERROR: one or more production DB SSM parameters returned empty." >&2
    exit 1
fi

echo "[run-migrations] Assuming provisioner role for master credential..."
PROVISIONER_ROLE_ARN="arn:aws:iam::901347789205:role/vaultmtg-staging-deploy-provisioner"
SESSION_NAME="run-migrations-$(date +%s)"

# Defense in depth: clear temporary credentials AND any secret-bearing
# variables on any exit (success or failure).
cleanup_creds() {
    unset AWS_ACCESS_KEY_ID
    unset AWS_SECRET_ACCESS_KEY
    unset AWS_SESSION_TOKEN
    unset DB_SECRET_JSON MASTER_USER MASTER_PASSWORD
}
trap cleanup_creds EXIT

ASSUME_OUTPUT=$(aws sts assume-role \
    --role-arn         "$PROVISIONER_ROLE_ARN" \
    --role-session-name "$SESSION_NAME" \
    --duration-seconds 900 \
    --region           "$REGION" \
    --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' \
    --output text)

if [[ -z "$ASSUME_OUTPUT" ]]; then
    echo "[run-migrations] ERROR: aws sts assume-role returned empty credentials." >&2
    exit 1
fi

AWS_ACCESS_KEY_ID=$(echo "$ASSUME_OUTPUT"    | awk '{print $1}')
AWS_SECRET_ACCESS_KEY=$(echo "$ASSUME_OUTPUT" | awk '{print $2}')
AWS_SESSION_TOKEN=$(echo "$ASSUME_OUTPUT"     | awk '{print $3}')

if [[ -z "$AWS_ACCESS_KEY_ID" || -z "$AWS_SECRET_ACCESS_KEY" || -z "$AWS_SESSION_TOKEN" ]]; then
    echo "[run-migrations] ERROR: aws sts assume-role returned incomplete credentials." >&2
    exit 1
fi

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY
export AWS_SESSION_TOKEN

CALLER_ARN=$(aws sts get-caller-identity --query Arn --output text)
case "$CALLER_ARN" in
    *":assumed-role/vaultmtg-staging-deploy-provisioner/${SESSION_NAME}")
        echo "[run-migrations] Assumed provisioner role: ${CALLER_ARN}"
        ;;
    *)
        echo "[run-migrations] ERROR: caller identity ${CALLER_ARN} is not the provisioner role." >&2
        exit 1
        ;;
esac

DB_SECRET_JSON=$(aws secretsmanager get-secret-value \
    --secret-id "$DB_SECRET_ARN_VALUE" \
    --region    "$REGION" \
    --query     SecretString \
    --output    text)
MASTER_USER=$(printf '%s' "$DB_SECRET_JSON" | jq -r '.username // empty')
MASTER_PASSWORD=$(printf '%s' "$DB_SECRET_JSON" | jq -r '.password // empty')
unset DB_SECRET_JSON

if [[ -z "$MASTER_USER" || -z "$MASTER_PASSWORD" ]]; then
    echo "[run-migrations] ERROR: master secret JSON missing username or password." >&2
    exit 1
fi

# Drop temporary provisioner credentials before running psql/migrate so only
# the parsed username/password are in scope for the migration commands.
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN

# Construct DATABASE_URL from the parsed master credentials.
# URL-encode username and password so special characters do not break parsing.
MASTER_USER_ENC=$(jq -rn --arg v "$MASTER_USER" '$v|@uri')
MASTER_PASSWORD_ENC=$(jq -rn --arg v "$MASTER_PASSWORD" '$v|@uri')
DATABASE_URL=$(printf 'postgresql://%s:%s@%s:%s/%s?%s' \
    "$MASTER_USER_ENC" "$MASTER_PASSWORD_ENC" "$DB_ENDPOINT" "$DB_PORT" "$DB_NAME" "$DB_SSL_MODE")
unset MASTER_USER_ENC MASTER_PASSWORD_ENC

if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "[run-migrations] ERROR: DATABASE_URL construction failed." >&2
    exit 1
fi

# golang-migrate requires a postgres:// scheme; the env file uses postgresql://.
MIGRATE_DB_URL="${DATABASE_URL/postgresql:\/\//postgres://}"

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
