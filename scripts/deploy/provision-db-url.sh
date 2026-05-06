#!/bin/sh
# provision-db-url.sh
# Fetches RDS credentials from Secrets Manager and writes DATABASE_URL into
# /etc/mtga-companion/env.  Runs ON the EC2 instance via SSM RunShellScript.
#
# All inputs are read from SSM Parameter Store so no secrets ever touch the
# GitHub Actions runner or appear in workflow logs.
#
# Required SSM parameters (must exist before the deploy workflow runs):
#   /mtga-companion/production/db-secret-arn  - ARN of the Secrets Manager secret
#   /mtga-companion/production/db-endpoint    - RDS instance hostname
#   /mtga-companion/production/db-name        - PostgreSQL database name

set -e

REGION=us-east-1
ENV_FILE=/etc/mtga-companion/env

DB_SECRET_ARN=$(aws ssm get-parameter \
  --name /mtga-companion/production/db-secret-arn \
  --region "$REGION" \
  --query Parameter.Value \
  --output text)

DB_ENDPOINT=$(aws ssm get-parameter \
  --name /mtga-companion/production/db-endpoint \
  --region "$REGION" \
  --query Parameter.Value \
  --output text)

DB_NAME=$(aws ssm get-parameter \
  --name /mtga-companion/production/db-name \
  --region "$REGION" \
  --query Parameter.Value \
  --output text)

# Fetch the JSON secret from Secrets Manager.
SECRET=$(aws secretsmanager get-secret-value \
  --secret-id "$DB_SECRET_ARN" \
  --region "$REGION" \
  --query SecretString \
  --output text)

# Extract username and URL-encode the password so special characters are safe.
DB_USER=$(printf '%s' "$SECRET" | python3 -c 'import sys,json; print(json.load(sys.stdin)["username"])')
DB_PASS=$(printf '%s' "$SECRET" | python3 -c 'import sys,json,urllib.parse; print(urllib.parse.quote(json.load(sys.stdin)["password"], safe=""))')

DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@${DB_ENDPOINT}:5432/${DB_NAME}"

mkdir -p /etc/mtga-companion

# Upsert: replace existing line or append if absent.
if grep -q '^DATABASE_URL=' "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^DATABASE_URL=.*|DATABASE_URL=${DATABASE_URL}|" "$ENV_FILE"
else
  printf 'DATABASE_URL=%s\n' "$DATABASE_URL" >> "$ENV_FILE"
fi

chmod 600 "$ENV_FILE"
echo "DATABASE_URL provisioned."
