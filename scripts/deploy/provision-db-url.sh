#!/bin/sh
# provision-db-url.sh
# Writes DB_SECRET_ARN and a credential-free DATABASE_URL into the
# production env file.  Runs ON the EC2 instance via SSM RunShellScript.
#
# The BFF binary reads DB_SECRET_ARN at startup and fetches the current
# credentials from Secrets Manager, so the env file never holds a password
# that can go stale after an RDS rotation.
#
# SSM parameter names and the env file path are sourced from
# infra/config/deploy-env.sh — do NOT hardcode them here.

set -e

# Source canonical deploy facts.  deploy-env.sh is downloaded alongside
# this script from S3 into /tmp/ before execution.
. /tmp/deploy-env.sh

REGION="$DEPLOY_REGION"
ENV_FILE="$BFF_ENV_FILE"

DB_SECRET_ARN=$(aws ssm get-parameter \
  --name "$SSM_PROD_DB_SECRET_ARN" \
  --region "$REGION" \
  --query Parameter.Value \
  --output text)

DB_ENDPOINT=$(aws ssm get-parameter \
  --name "$SSM_PROD_DB_ENDPOINT" \
  --region "$REGION" \
  --query Parameter.Value \
  --output text)

DB_NAME=$(aws ssm get-parameter \
  --name "$SSM_PROD_DB_NAME" \
  --region "$REGION" \
  --query Parameter.Value \
  --output text)

DATABASE_URL="postgresql://${DB_ENDPOINT}:${DB_PORT}/${DB_NAME}?${DB_SSL_MODE}"

mkdir -p "$BFF_ENV_DIR"

# Upsert AWS_DEFAULT_REGION so the BFF's Secrets Manager client resolves the endpoint.
if grep -q '^AWS_DEFAULT_REGION=' "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^AWS_DEFAULT_REGION=.*|AWS_DEFAULT_REGION=${REGION}|" "$ENV_FILE"
else
  printf 'AWS_DEFAULT_REGION=%s\n' "$REGION" >> "$ENV_FILE"
fi

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
