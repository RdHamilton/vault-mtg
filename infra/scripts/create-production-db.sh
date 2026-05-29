#!/usr/bin/env bash
# create-production-db.sh
#
# Provisions the vaultmtg_app production application role on the shared RDS
# instance via SSM Run Command on the EC2 instance.
#
# This script is idempotent: the DO-block uses an IF NOT EXISTS guard that
# converges an existing role (including the manually-created NOLOGIN role from
# the v0.3.1 incident) without error on a second run.
#
# SSM parameter names and role/database names are sourced from
# infra/config/deploy-env.sh — do NOT hardcode them here.
#
# Prerequisites:
#   - AWS CLI configured with the `personal` profile
#   - EC2 instance has SSM agent installed and the instance role allows
#     ssm:SendCommand
#   - EC2 instance role has secretsmanager:GetSecretValue on
#     arn:aws:secretsmanager:us-east-1:*:secret:vaultmtg-production-app-db-*
#     (granted by IAM PR #224, merged 2026-05-25 02:58Z)
#   - psql is installed on the EC2 instance
#   - The RDS instance is NOT publicly accessible; all queries go through the
#     EC2 host over the private VPC subnet
#
# Usage (from your local machine):
#   AWS_PROFILE=personal bash infra/scripts/create-production-db.sh
#
#   Dry-run (no AWS calls — validates payload assembly only):
#   bash infra/scripts/create-production-db.sh --dry-run
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

# ---------------------------------------------------------------------------
# Flag parsing
# ---------------------------------------------------------------------------
DRY_RUN=0
for arg in "$@"; do
    if [[ "$arg" == "--dry-run" ]]; then
        DRY_RUN=1
    fi
done

PROFILE="${AWS_PROFILE:-personal}"
REGION="${AWS_REGION:-us-east-1}"

# Source canonical deploy facts from the repo root.
_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${_SCRIPT_DIR}/../../infra/config/deploy-env.sh"

# ---------------------------------------------------------------------------
# In dry-run mode, substitute real values with stubs so that payload
# assembly and validation can run without any AWS credentials or network.
# ---------------------------------------------------------------------------
if [[ "$DRY_RUN" == "1" ]]; then
    echo "[create-production-db] --dry-run mode: skipping all AWS API calls."
    DB_ENDPOINT="rds-dry-run.us-east-1.rds.amazonaws.com"
    DB_NAME="vaultmtg_dry"
    MASTER_USER="dryrunmaster"
    MASTER_PASSWORD="DryRunMasterPassword1"
    APP_PASSWORD="DryRunAppPassword000000000000000"
    APP_SECRET_ARN="arn:aws:secretsmanager:us-east-1:000000000000:secret:vaultmtg-production-app-db-dryrun"
    EC2_INSTANCE_ID="i-dryrun00000000000"
else
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

    # -------------------------------------------------------------------------
    # Generate a production app password and store it in Secrets Manager.
    # Uses openssl for a 32-char alphanumeric password (safe in psql connection
    # strings — no special chars that require quoting).
    # -------------------------------------------------------------------------
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

    # -------------------------------------------------------------------------
    # Locate the EC2 instance ID (the instance running the BFF).
    # -------------------------------------------------------------------------
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
fi

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
DB_APP_ROLE="__DB_APP_ROLE__"
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
FROM pg_roles WHERE rolname = '${DB_APP_ROLE}';
"

# ---------------------------------------------------------------------------
# Post-execution PG/SM verification (CR-1 from plan review).
# Reads the new app password from Secrets Manager via the EC2 instance role
# and opens a fresh psql connection as vaultmtg_app to confirm the password
# was actually applied.  If the connection fails or current_user does not
# match, the remote script exits non-zero — SSM reports Status=Failed and
# the orchestrator does NOT print SUCCESS.
#
# Prerequisite: EC2 instance role must have secretsmanager:GetSecretValue on
# arn:aws:secretsmanager:us-east-1:*:secret:vaultmtg-production-app-db-*
# (granted by IAM PR #224, merged 2026-05-25 02:58Z).
# ---------------------------------------------------------------------------
echo "[remote] Fetching app credentials from Secrets Manager for post-execution verification..."
APP_SECRET_JSON=$(aws secretsmanager get-secret-value \
    --region us-east-1 \
    --secret-id "vaultmtg-production-app-db" \
    --query "SecretString" \
    --output text)

APP_PASSWORD_VERIFY=$(echo "$APP_SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
unset APP_SECRET_JSON

echo "[remote] Verifying vaultmtg_app login with SM password..."
CURRENT_USER=$(PGPASSWORD="$APP_PASSWORD_VERIFY" psql \
    -X \
    -h "$DB_ENDPOINT" \
    -U "$DB_APP_ROLE" \
    -d "$DB_NAME" \
    -v ON_ERROR_STOP=1 \
    --tuples-only \
    --no-align \
    -c "SELECT current_user;" 2>&1)
unset APP_PASSWORD_VERIFY

CURRENT_USER_TRIMMED=$(echo "$CURRENT_USER" | tr -d '[:space:]')
if [[ "$CURRENT_USER_TRIMMED" != "$DB_APP_ROLE" ]]; then
    echo "[remote] ERROR: Post-execution verification FAILED. Expected current_user='${DB_APP_ROLE}', got '${CURRENT_USER_TRIMMED}'. SM and PG passwords are out of sync." >&2
    exit 1
fi

echo "[remote] Post-execution verification PASSED. current_user=${CURRENT_USER_TRIMMED}"
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

# Base64-encode the fully-substituted remote script for SSM payload delivery.
# Using --cli-input-json + a temp file avoids the xargs ARG_MAX ceiling that
# caused the 2026-05-25 P0 outage (xargs silently produced commands=[""],
# SSM accepted it as Status=Success, and no SQL ran).
ENCODED_SCRIPT=$(printf '%s' "$REMOTE_SCRIPT" | base64 | tr -d '\n')

# Minimum-length guard (CR-2): the real encoded payload is several KB.
# Anything under 1000 bytes indicates partial truncation or an empty expansion
# — abort before any AWS call rather than dispatch a no-op.
if [[ ${#ENCODED_SCRIPT} -lt 1000 ]]; then
    echo "[create-production-db] ERROR: Encoded remote script is suspiciously short (${#ENCODED_SCRIPT} bytes) — possible partial expansion. Aborting before SSM dispatch." >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# --dry-run mode: validate payload assembly without making any AWS calls.
# Runs three assertions:
#   1. Minimum-length guard fired correctly (payload >= 1000 bytes).
#   2. The assembled JSON temp file is valid JSON.
#   3. Parameters.commands[0] round-trips: base64-decode equals REMOTE_SCRIPT
#      byte-for-byte (catches JSON-escaping mangling of the payload).
# ---------------------------------------------------------------------------
if [[ "$DRY_RUN" == "1" ]]; then
    echo "[create-production-db] --dry-run: payload assembled (${#ENCODED_SCRIPT} bytes encoded)."
    echo "[create-production-db] --dry-run: Assertion 1 — minimum-length guard: PASS (${#ENCODED_SCRIPT} >= 1000)"

    # SECURITY: this dry-run JSON contains stub credentials only (no real secrets).
    # The trap still applies as a hygiene measure.
    TMP_JSON="/tmp/ssm-cmd-$$.json"
    trap 'rm -f "$TMP_JSON"' EXIT INT TERM HUP
    install -m 600 /dev/null "$TMP_JSON"
    cat > "$TMP_JSON" <<ENDJSON
{
  "DocumentName": "AWS-RunShellScript",
  "InstanceIds": ["${EC2_INSTANCE_ID}"],
  "Parameters": {
    "commands": ["echo ${ENCODED_SCRIPT} | base64 -d | bash"]
  },
  "Comment": "dry-run validation — no AWS call"
}
ENDJSON

    # Assertion 2: JSON validity
    if python3 -m json.tool "$TMP_JSON" > /dev/null 2>&1; then
        echo "[create-production-db] --dry-run: Assertion 2 — JSON validity: PASS"
    else
        echo "[create-production-db] --dry-run: Assertion 2 — JSON validity: FAIL" >&2
        exit 1
    fi

    # Assertion 3: extract Parameters.commands[0] from the written JSON, strip
    # the "echo ... | base64 -d | bash" wrapper, base64-decode, and compare
    # byte-for-byte against the original REMOTE_SCRIPT.
    # Uses printf (not echo) on both sides to avoid trailing-newline masking.
    EXTRACTED_B64=$(python3 -c "
import json, sys, re
data = json.load(open('$TMP_JSON'))
cmd = data['Parameters']['commands'][0]
m = re.match(r'^echo (\S+) \| base64 -d \| bash$', cmd)
if not m:
    print('PARSE_ERROR', end='')
    sys.exit(1)
print(m.group(1), end='')
")
    if [[ "$EXTRACTED_B64" == "PARSE_ERROR" ]]; then
        echo "[create-production-db] --dry-run: Assertion 3 — byte-compare: FAIL (could not parse commands[0])" >&2
        exit 1
    fi

    DECODED=$(printf '%s' "$EXTRACTED_B64" | base64 -d 2>/dev/null)
    if diff <(printf '%s' "$REMOTE_SCRIPT") <(printf '%s' "$DECODED") > /dev/null 2>&1; then
        echo "[create-production-db] --dry-run: Assertion 3 — byte-compare Parameters.commands[0] vs REMOTE_SCRIPT: PASS"
    else
        echo "[create-production-db] --dry-run: Assertion 3 — byte-compare Parameters.commands[0] vs REMOTE_SCRIPT: FAIL" >&2
        diff <(printf '%s' "$REMOTE_SCRIPT") <(printf '%s' "$DECODED") >&2 || true
        exit 1
    fi

    echo "[create-production-db] --dry-run: All assertions passed. No AWS calls made."
    exit 0
fi

# SECURITY: /tmp/ssm-cmd-$$.json contains the new role password in base64
# (embedded inside ENCODED_SCRIPT via __MASTER_PASSWORD__ substitution) and
# the generated app password.  Both the RDS master credentials and the new
# app secret are present in this file.  The trap below is the ONLY thing
# preventing it from persisting on disk after script exit or signal
# interruption.  Do not remove any signal from the trap.
TMP_JSON="/tmp/ssm-cmd-$$.json"
trap 'rm -f "$TMP_JSON"' EXIT INT TERM HUP
install -m 600 /dev/null "$TMP_JSON"

cat > "$TMP_JSON" <<ENDJSON
{
  "DocumentName": "AWS-RunShellScript",
  "InstanceIds": ["${EC2_INSTANCE_ID}"],
  "Parameters": {
    "commands": ["echo ${ENCODED_SCRIPT} | base64 -d | bash"]
  },
  "Comment": "create vaultmtg_app production role for #2583"
}
ENDJSON

echo "[create-production-db] Sending SSM Run Command to $EC2_INSTANCE_ID ..."

COMMAND_ID=$(aws ssm send-command \
    --profile         "$PROFILE" \
    --region          "$REGION" \
    --cli-input-json  "file://$TMP_JSON" \
    --query           "Command.CommandId" \
    --output          text)

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
