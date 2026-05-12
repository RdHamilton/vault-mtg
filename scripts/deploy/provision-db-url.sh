#!/bin/sh
# provision-db-url.sh
# Writes DB_SECRET_ARN and a credential-free DATABASE_URL into
# /etc/mtga-companion/env.  Runs ON the EC2 instance via SSM RunShellScript.
#
# The BFF binary reads DB_SECRET_ARN at startup and fetches the current
# credentials from Secrets Manager, so the env file never holds a password
# that can go stale after an RDS rotation.
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

DATABASE_URL="postgresql://${DB_ENDPOINT}:5432/${DB_NAME}?sslmode=require"

mkdir -p /etc/mtga-companion

# Upsert DB_SECRET_ARN.
if grep -q '^DB_SECRET_ARN=' "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^DB_SECRET_ARN=.*|DB_SECRET_ARN=${DB_SECRET_ARN}|" "$ENV_FILE"
else
  printf 'DB_SECRET_ARN=%s\n' "$DB_SECRET_ARN" >> "$ENV_FILE"
fi

# Upsert credential-free DATABASE_URL (credentials resolved at BFF startup).
if grep -q '^DATABASE_URL=' "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^DATABASE_URL=.*|DATABASE_URL=${DATABASE_URL}|" "$ENV_FILE"
else
  printf 'DATABASE_URL=%s\n' "$DATABASE_URL" >> "$ENV_FILE"
fi

chmod 600 "$ENV_FILE"
echo "DB_SECRET_ARN and DATABASE_URL provisioned (credentials resolved at startup)."
