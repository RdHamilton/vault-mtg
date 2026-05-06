#!/bin/sh
# provision-env.sh
# Generic upsert helper: reads a value from SSM Parameter Store and writes a
# KEY=VALUE line into /etc/mtga-companion/env.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Usage: provision-env.sh <ENV_KEY> <SSM_PARAM_NAME> [--with-decryption]
#
#   ENV_KEY        - The environment variable name to upsert (e.g. ALLOWED_ORIGINS)
#   SSM_PARAM_NAME - Full SSM parameter path to read the value from
#   --with-decryption (optional) - Pass for SecureString parameters
#
# Example:
#   provision-env.sh ALLOWED_ORIGINS /mtga-companion/production/ALLOWED_ORIGINS
#   provision-env.sh CLERK_SECRET_KEY /mtga-companion/production/CLERK_SECRET_KEY --with-decryption

set -e

ENV_KEY="${1:?Usage: provision-env.sh ENV_KEY SSM_PARAM_NAME [--with-decryption]}"
SSM_PARAM_NAME="${2:?Usage: provision-env.sh ENV_KEY SSM_PARAM_NAME [--with-decryption]}"
DECRYPT_FLAG="${3:-}"

REGION=us-east-1
ENV_FILE=/etc/mtga-companion/env

# Build the get-parameter command, optionally with decryption for SecureString.
if [ "$DECRYPT_FLAG" = "--with-decryption" ]; then
  ENV_VALUE=$(aws ssm get-parameter \
    --name "$SSM_PARAM_NAME" \
    --with-decryption \
    --region "$REGION" \
    --query Parameter.Value \
    --output text)
else
  ENV_VALUE=$(aws ssm get-parameter \
    --name "$SSM_PARAM_NAME" \
    --region "$REGION" \
    --query Parameter.Value \
    --output text)
fi

if [ -z "$ENV_VALUE" ]; then
  echo "ERROR: SSM parameter ${SSM_PARAM_NAME} is empty." >&2
  exit 1
fi

mkdir -p /etc/mtga-companion

# Upsert: replace existing line or append if absent.
if grep -q "^${ENV_KEY}=" "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^${ENV_KEY}=.*|${ENV_KEY}=${ENV_VALUE}|" "$ENV_FILE"
else
  printf '%s=%s\n' "$ENV_KEY" "$ENV_VALUE" >> "$ENV_FILE"
fi

chmod 600 "$ENV_FILE"
echo "${ENV_KEY} provisioned."
