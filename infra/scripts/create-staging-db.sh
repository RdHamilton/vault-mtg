#!/usr/bin/env bash
# create-staging-db.sh
#
# Provisions the vaultmtg_staging database and vaultmtg_staging_app role on
# the shared production RDS instance via SSM Run Command on the EC2 instance.
#
# Prerequisites:
#   - AWS CLI configured with the `personal` profile
#   - EC2 instance has SSM agent installed and the instance role allows
#     ssm:SendCommand + secretsmanager:GetSecretValue
#   - psql is installed on the EC2 instance
#   - The RDS instance is NOT publicly accessible; all queries go through the
#     EC2 host over the private VPC subnet
#
# Usage (from your local machine):
#   AWS_PROFILE=personal bash infra/scripts/create-staging-db.sh
#
# SAFETY: This script does NOT execute SQL directly. It sends commands to the
# EC2 instance via SSM Run Command. Review the generated SQL before running.

set -euo pipefail

PROFILE="${AWS_PROFILE:-personal}"
REGION="${AWS_REGION:-us-east-1}"

echo "[create-staging-db] Fetching RDS connection details from SSM..."

DB_ENDPOINT=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-endpoint" \
    --query   "Parameter.Value" \
    --output  text)

DB_NAME=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-name" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_ARN=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "/mtga-companion/production/db-secret-arn" \
    --query   "Parameter.Value" \
    --output  text)

echo "[create-staging-db] Fetching master password from Secrets Manager..."

SECRET_JSON=$(aws secretsmanager get-secret-value \
    --profile  "$PROFILE" \
    --region   "$REGION" \
    --secret-id "$SECRET_ARN" \
    --query    "SecretString" \
    --output   text)

# The secret is a JSON object: {"username":"postgres","password":"..."}
MASTER_PASSWORD=$(echo "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
MASTER_USER=$(echo     "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['username'])")

# ---------------------------------------------------------------------------
# Generate a staging password and store it in SSM before executing SQL.
# Uses openssl for a 32-char alphanumeric password (safe in psql connection
# strings — no special chars that require quoting).
# ---------------------------------------------------------------------------
echo "[create-staging-db] Generating staging DB password..."
STAGING_PASSWORD=$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | head -c 32)

echo "[create-staging-db] Storing staging password in SSM at /mtga-companion/staging/db-password ..."
aws ssm put-parameter \
    --profile  "$PROFILE" \
    --region   "$REGION" \
    --name     "/mtga-companion/staging/db-password" \
    --value    "$STAGING_PASSWORD" \
    --type     "SecureString" \
    --overwrite \
    --description "vaultmtg_staging_app PostgreSQL role password" \
    > /dev/null

# Also store a full DATABASE_URL for use by the BFF and migration tooling.
STAGING_DB_URL="postgres://vaultmtg_staging_app:${STAGING_PASSWORD}@${DB_ENDPOINT}:5432/vaultmtg_staging?sslmode=require"
aws ssm put-parameter \
    --profile  "$PROFILE" \
    --region   "$REGION" \
    --name     "/mtga-companion/staging/database-url" \
    --value    "$STAGING_DB_URL" \
    --type     "SecureString" \
    --overwrite \
    --description "Full DATABASE_URL for vaultmtg_staging (used by bff-staging)" \
    > /dev/null

echo "[create-staging-db] SSM parameters written."

# ---------------------------------------------------------------------------
# Locate the EC2 instance ID (the instance running the BFF).
# ---------------------------------------------------------------------------
EC2_INSTANCE_ID=$(aws ec2 describe-instances \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --filters "Name=tag:Name,Values=mtga-companion" "Name=instance-state-name,Values=running" \
    --query   "Reservations[0].Instances[0].InstanceId" \
    --output  text)

if [[ -z "$EC2_INSTANCE_ID" || "$EC2_INSTANCE_ID" == "None" ]]; then
    echo "[create-staging-db] ERROR: Could not find EC2 instance tagged 'mtga-companion' in running state."
    exit 1
fi

echo "[create-staging-db] Using EC2 instance: $EC2_INSTANCE_ID"

# ---------------------------------------------------------------------------
# Build the psql commands to run on the EC2 host.
# We substitute the real password inline — SSM Run Command output is not
# stored in CloudTrail in plaintext, but exercise caution.
# ---------------------------------------------------------------------------
SQL_COMMANDS=$(cat <<ENDSQL
-- Step 1: Create database and role (connected to postgres DB as master user)
CREATE DATABASE vaultmtg_staging
    WITH OWNER     = postgres
         ENCODING  = 'UTF8'
         LC_COLLATE = 'en_US.UTF-8'
         LC_CTYPE   = 'en_US.UTF-8'
         TEMPLATE   = template0;

DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'vaultmtg_staging_app') THEN
        CREATE ROLE vaultmtg_staging_app
            WITH LOGIN
                 PASSWORD '${STAGING_PASSWORD}'
                 CONNECTION LIMIT 10;
    ELSE
        ALTER ROLE vaultmtg_staging_app WITH PASSWORD '${STAGING_PASSWORD}';
    END IF;
END
\$\$;

GRANT ALL PRIVILEGES ON DATABASE vaultmtg_staging TO vaultmtg_staging_app;
ENDSQL
)

SCHEMA_SQL=$(cat <<ENDSQL
-- Step 2: Schema-level grants (connected to vaultmtg_staging)
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT  CREATE ON SCHEMA public TO vaultmtg_staging_app;
GRANT  USAGE  ON SCHEMA public TO vaultmtg_staging_app;
ENDSQL
)

REMOTE_SCRIPT=$(cat <<'ENDBASH'
set -euo pipefail
DB_ENDPOINT="__DB_ENDPOINT__"
MASTER_USER="__MASTER_USER__"
export PGPASSWORD="__MASTER_PASSWORD__"

echo "[remote] Running database creation SQL..."
psql -h "$DB_ENDPOINT" -U "$MASTER_USER" -d postgres -v ON_ERROR_STOP=1 <<SQL
__SQL_COMMANDS__
SQL

echo "[remote] Running schema-level grant SQL on vaultmtg_staging..."
psql -h "$DB_ENDPOINT" -U "$MASTER_USER" -d vaultmtg_staging -v ON_ERROR_STOP=1 <<SQL
__SCHEMA_SQL__
SQL

echo "[remote] Done. vaultmtg_staging and vaultmtg_staging_app are ready."
ENDBASH
)

# Substitute placeholders (passwords are in the SSM Run Command payload,
# which is encrypted in transit but visible in AWS console logs — consider
# using AWS Secrets Manager data key encryption for higher-security envs).
REMOTE_SCRIPT="${REMOTE_SCRIPT//__DB_ENDPOINT__/$DB_ENDPOINT}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__MASTER_USER__/$MASTER_USER}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__MASTER_PASSWORD__/$MASTER_PASSWORD}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__SQL_COMMANDS__/$SQL_COMMANDS}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__SCHEMA_SQL__/$SCHEMA_SQL}"

echo "[create-staging-db] Sending SSM Run Command to $EC2_INSTANCE_ID ..."

COMMAND_ID=$(aws ssm send-command \
    --profile      "$PROFILE" \
    --region       "$REGION" \
    --instance-ids "$EC2_INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters   "commands=[\"$(echo "$REMOTE_SCRIPT" | base64 | tr -d '\n' | xargs -I{} echo 'echo {} | base64 -d | bash')\"]" \
    --comment      "create vaultmtg_staging database and role" \
    --query        "Command.CommandId" \
    --output       text)

echo "[create-staging-db] SSM Command ID: $COMMAND_ID"
echo "[create-staging-db] Waiting for command to complete..."

aws ssm wait command-executed \
    --profile    "$PROFILE" \
    --region     "$REGION" \
    --command-id "$COMMAND_ID" \
    --instance-id "$EC2_INSTANCE_ID" 2>/dev/null || true

STATUS=$(aws ssm get-command-invocation \
    --profile     "$PROFILE" \
    --region      "$REGION" \
    --command-id  "$COMMAND_ID" \
    --instance-id "$EC2_INSTANCE_ID" \
    --query       "Status" \
    --output      text)

if [[ "$STATUS" == "Success" ]]; then
    echo "[create-staging-db] SUCCESS. vaultmtg_staging is ready."
    echo ""
    echo "Next step: run infra/scripts/run-staging-migrations.sh to apply all schema migrations."
else
    echo "[create-staging-db] FAILED (status: $STATUS). Fetching output..."
    aws ssm get-command-invocation \
        --profile     "$PROFILE" \
        --region      "$REGION" \
        --command-id  "$COMMAND_ID" \
        --instance-id "$EC2_INSTANCE_ID" \
        --query       "StandardErrorContent" \
        --output      text
    exit 1
fi
