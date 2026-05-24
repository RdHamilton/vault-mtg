#!/usr/bin/env bash
# create-production-db.sh
#
# Provisions the vaultmtg_app production application role on the shared RDS
# instance via SSM Run Command on the EC2 instance.
#
# This script is idempotent: the DO-block in create-production-db.sql uses an
# IF NOT EXISTS guard that converges an existing role (including the manually-
# created NOLOGIN role from the v0.3.1 incident) without error on a second run.
#
# SSM parameter names and role/database names are sourced from
# infra/config/deploy-env.sh — do NOT hardcode them here.
#
# Prerequisites:
#   - AWS CLI configured with the `personal` profile
#   - EC2 instance has SSM agent installed and the instance role allows
#     ssm:SendCommand
#   - psql is installed on the EC2 instance
#   - The RDS instance is NOT publicly accessible; all queries go through the
#     EC2 host over the private VPC subnet
#
# Usage (from your local machine):
#   AWS_PROFILE=personal bash infra/scripts/create-production-db.sh
#
# SAFETY: This script does NOT execute SQL directly. It sends commands to the
# EC2 instance via SSM Run Command. Review the generated SQL before running.
#
# Rotation: the vaultmtg_app Secrets Manager secret uses MANUAL rotation
# until S-19 (automated rotation re-emission) is implemented. When a
# rotation occurs, re-run this script to converge the role password and
# update the Secrets Manager secret, then re-run scripts/deploy/provision-db-url.sh
# to pick up the new password in the BFF env file.

set -euo pipefail

PROFILE="${AWS_PROFILE:-personal}"
REGION="${AWS_REGION:-us-east-1}"

# Source canonical deploy facts from the repo root.
_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${_SCRIPT_DIR}/../../infra/config/deploy-env.sh"

echo "[create-production-db] Fetching RDS connection details from SSM..."

DB_ENDPOINT=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "$SSM_PROD_DB_ENDPOINT" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_ARN=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "$SSM_PROD_DB_SECRET_ARN" \
    --query   "Parameter.Value" \
    --output  text)

DB_NAME=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "$SSM_PROD_DB_NAME" \
    --query   "Parameter.Value" \
    --output  text)

echo "[create-production-db] Fetching master password from Secrets Manager..."

SECRET_JSON=$(aws secretsmanager get-secret-value \
    --profile  "$PROFILE" \
    --region   "$REGION" \
    --secret-id "$SECRET_ARN" \
    --query    "SecretString" \
    --output   text)

# The secret is a JSON object: {"username":"...","password":"..."}
MASTER_PASSWORD=$(echo "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
MASTER_USER=$(echo     "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['username'])")

unset SECRET_JSON

# ---------------------------------------------------------------------------
# Generate a production app password and store it in Secrets Manager.
# Uses openssl for a 32-char alphanumeric password (safe in psql connection
# strings — no special chars that require quoting).
# ---------------------------------------------------------------------------
echo "[create-production-db] Generating production app DB password..."
APP_PASSWORD=$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | head -c 32)

# Check whether the app secret already exists so we can create vs. update.
EXISTING_SECRET_ARN=$(aws secretsmanager describe-secret \
    --profile  "$PROFILE" \
    --region   "$REGION" \
    --secret-id "vaultmtg-production-app-db" \
    --query    "ARN" \
    --output   text 2>/dev/null || echo "")

APP_SECRET_DESCRIPTION="Rotation: manual until S-19; re-run provision-db-url.sh to pick up new password."
APP_SECRET_VALUE="{\"username\":\"${DB_APP_ROLE}\",\"password\":\"${APP_PASSWORD}\"}"

if [[ -z "$EXISTING_SECRET_ARN" || "$EXISTING_SECRET_ARN" == "None" ]]; then
    echo "[create-production-db] Creating new Secrets Manager secret vaultmtg-production-app-db ..."
    APP_SECRET_ARN=$(aws secretsmanager create-secret \
        --profile    "$PROFILE" \
        --region     "$REGION" \
        --name       "vaultmtg-production-app-db" \
        --description "$APP_SECRET_DESCRIPTION" \
        --secret-string "$APP_SECRET_VALUE" \
        --query      "ARN" \
        --output     text)
else
    echo "[create-production-db] Updating existing Secrets Manager secret vaultmtg-production-app-db ..."
    APP_SECRET_ARN="$EXISTING_SECRET_ARN"
    aws secretsmanager put-secret-value \
        --profile    "$PROFILE" \
        --region     "$REGION" \
        --secret-id  "vaultmtg-production-app-db" \
        --secret-string "$APP_SECRET_VALUE" \
        > /dev/null
fi

unset APP_SECRET_VALUE

echo "[create-production-db] App secret ARN: $APP_SECRET_ARN"

echo "[create-production-db] Storing app secret ARN in SSM at ${SSM_PROD_APP_DB_SECRET_ARN} ..."
aws ssm put-parameter \
    --profile  "$PROFILE" \
    --region   "$REGION" \
    --name     "$SSM_PROD_APP_DB_SECRET_ARN" \
    --value    "$APP_SECRET_ARN" \
    --type     "String" \
    --overwrite \
    --description "ARN of the vaultmtg_app Secrets Manager secret for production BFF" \
    > /dev/null

echo "[create-production-db] SSM parameter written."

# ---------------------------------------------------------------------------
# Locate the EC2 instance ID (the instance running the BFF).
# ---------------------------------------------------------------------------
EC2_INSTANCE_ID=$(aws ec2 describe-instances \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --filters "Name=tag:Name,Values=${EC2_INSTANCE_TAG}" "Name=instance-state-name,Values=${EC2_INSTANCE_STATE}" \
    --query   "Reservations[0].Instances[0].InstanceId" \
    --output  text)

if [[ -z "$EC2_INSTANCE_ID" || "$EC2_INSTANCE_ID" == "None" ]]; then
    echo "[create-production-db] ERROR: Could not find EC2 instance tagged '${EC2_INSTANCE_TAG}' in running state."
    exit 1
fi

echo "[create-production-db] Using EC2 instance: $EC2_INSTANCE_ID"

# ---------------------------------------------------------------------------
# Build the psql commands to run on the EC2 host.
# The DO-block idempotency guard converges the role on repeated runs.
# ---------------------------------------------------------------------------
SQL_COMMANDS=$(cat <<ENDSQL
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${DB_APP_ROLE}') THEN
        CREATE ROLE ${DB_APP_ROLE}
            WITH LOGIN
                 PASSWORD '${APP_PASSWORD}'
                 CONNECTION LIMIT 10;
    ELSE
        ALTER ROLE ${DB_APP_ROLE} WITH LOGIN PASSWORD '${APP_PASSWORD}' CONNECTION LIMIT 10;
    END IF;
END
\$\$;

GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_APP_ROLE};
ENDSQL
)

SCHEMA_SQL=$(cat <<ENDSQL
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT  CREATE ON SCHEMA public TO ${DB_APP_ROLE};
GRANT  USAGE  ON SCHEMA public TO ${DB_APP_ROLE};
ENDSQL
)

REMOTE_SCRIPT=$(cat <<'ENDBASH'
set -euo pipefail
DB_ENDPOINT="__DB_ENDPOINT__"
DB_NAME="__DB_NAME__"
MASTER_USER="__MASTER_USER__"
export PGPASSWORD="__MASTER_PASSWORD__"

echo "[remote] Running role creation SQL..."
psql -h "$DB_ENDPOINT" -U "$MASTER_USER" -d postgres -v ON_ERROR_STOP=1 <<SQL
__SQL_COMMANDS__
SQL

echo "[remote] Running schema-level grant SQL on ${DB_NAME}..."
psql -h "$DB_ENDPOINT" -U "$MASTER_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 <<SQL
__SCHEMA_SQL__
SQL

echo "[remote] Verifying role privilege boundary..."
psql -h "$DB_ENDPOINT" -U "$MASTER_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -c "
SELECT rolname, rolsuper, rolcreatedb, rolcreaterole, rolcanlogin
FROM pg_roles WHERE rolname = '__DB_APP_ROLE__';
"

echo "[remote] Done. ${DB_APP_ROLE} is provisioned and ready."
ENDBASH
)

# Substitute placeholders.
REMOTE_SCRIPT="${REMOTE_SCRIPT//__DB_ENDPOINT__/$DB_ENDPOINT}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__DB_NAME__/$DB_NAME}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__MASTER_USER__/$MASTER_USER}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__MASTER_PASSWORD__/$MASTER_PASSWORD}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__SQL_COMMANDS__/$SQL_COMMANDS}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__SCHEMA_SQL__/$SCHEMA_SQL}"
REMOTE_SCRIPT="${REMOTE_SCRIPT//__DB_APP_ROLE__/$DB_APP_ROLE}"

unset APP_PASSWORD MASTER_PASSWORD

echo "[create-production-db] Sending SSM Run Command to $EC2_INSTANCE_ID ..."

COMMAND_ID=$(aws ssm send-command \
    --profile      "$PROFILE" \
    --region       "$REGION" \
    --instance-ids "$EC2_INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters   "commands=[\"$(echo "$REMOTE_SCRIPT" | base64 | tr -d '\n' | xargs -I{} echo 'echo {} | base64 -d | bash')\"]" \
    --comment      "create vaultmtg_app production role for #2223" \
    --query        "Command.CommandId" \
    --output       text)

echo "[create-production-db] SSM Command ID: $COMMAND_ID"
echo "[create-production-db] Waiting for command to complete..."

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
    echo "[create-production-db] SUCCESS. vaultmtg_app is provisioned."
    echo ""
    echo "Next step: run scripts/deploy/provision-db-url.sh to write the app"
    echo "  credentials into the BFF env file, then restart the BFF service."
else
    echo "[create-production-db] FAILED (status: $STATUS). Fetching output..."
    aws ssm get-command-invocation \
        --profile     "$PROFILE" \
        --region      "$REGION" \
        --command-id  "$COMMAND_ID" \
        --instance-id "$EC2_INSTANCE_ID" \
        --query       "StandardErrorContent" \
        --output      text
    exit 1
fi
