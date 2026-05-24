#!/usr/bin/env bash
# provision-db-url.sh
# Renders DATABASE_URL into the production env file with credentials spliced
# inline from Secrets Manager. Runs ON the EC2 instance via SSM RunShellScript.
#
# As of #2223 (ADR-024 implementation), this script reads the vaultmtg_app
# application secret ARN from SSM_PROD_APP_DB_SECRET_ARN and splices the
# vaultmtg_app credentials into DATABASE_URL. The BFF binary connects as the
# least-privilege vaultmtg_app role, not the master superuser.
#
# IMPORTANT: run-migrations.sh independently reads SSM_PROD_DB_SECRET_ARN
# (the master/RDS-managed secret) to run DDL migrations and GRANT statements.
# The two scripts deliberately use different SSM parameter names:
#   provision-db-url.sh -> SSM_PROD_APP_DB_SECRET_ARN (vaultmtg_app DML role)
#   run-migrations.sh   -> SSM_PROD_DB_SECRET_ARN     (master/DDL role)
# This distinction is enforced by deploy-chain contract test C4/C5.
#
# Credential model (mirror of provision-staging-env.sh per #2461, prod half):
#   1. The EC2 instance role (mtga-companion-ec2-role-production) reads the
#      /vaultmtg/app/production/{app-db-secret-arn,db-endpoint,db-name} SSM
#      parameters first; the instance role's VaultmtgAppProductionNamespace
#      statement (ec2.yml) covers these reads.
#   2. The script then sts:AssumeRoles into vaultmtg-staging-deploy-provisioner
#      -- the only role in the account that holds the secretsmanager:GetSecretValue
#      grant on the RDS-managed credential (arn:...:secret:rds!db-12c647a0-*,
#      see staging-deploy-role.yml::StagingProvisioningDBSecretRead +
#      StagingProvisioningKMSDecrypt for the KMS Decrypt half). The EC2
#      instance role's StagingDeployProvisionerAssumeRole policy already
#      grants sts:AssumeRole on this exact ARN; the provisioner role's
#      EC2InstanceRoleBridge trust statement permits the assume.
#      The role's name is "staging-deploy-provisioner" for historical reasons
#      (PR #187 extended its grant onto the RDS secret, which is the ONLY
#      rds!db-* secret in the account and is shared between prod and staging
#      databases). Renaming the role to vaultmtg-deploy-provisioner -- or
#      splitting into per-env provisioners -- is tracked as follow-up tech
#      debt; mid-incident the existing grant is the correct lever.
#      NOTE: The provisioner role's GetSecretValue grant covers BOTH the
#      RDS-managed master secret AND the vaultmtg-production-app-db secret
#      created by create-production-db.sh (#2223). Both are needed:
#      this script fetches the app secret; run-migrations.sh fetches the master.
#   3. The temporary credentials returned by AssumeRole are exported as
#      AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN so the
#      subsequent aws secretsmanager get-secret-value call runs under the
#      provisioner role.
#   4. The JSON secret is parsed with jq, the username/password are
#      URL-encoded via jq @uri, and a credential-laden DATABASE_URL is
#      written into the env file. DB_SECRET_ARN is deliberately NOT written;
#      BFF_DB_RESOLVE_FROM_SM is deliberately NOT written (defaults OFF so
#      the BFF binary never constructs its SM client). This is the prod
#      mirror of #2539 (staging) -- both env files are now symmetric.
#   5. An EXIT trap unsets the env vars and clears DB_SECRET_JSON /
#      DB_PASSWORD / DB_USERNAME after the env file is written, so no
#      leftover creds remain in the SSM shell environment.
#
# Rotation impact: when the vaultmtg_app secret (vaultmtg-production-app-db)
# is rotated, re-run this script (provision-db-url.sh) to pick up the new
# password. Rotation is manual until S-19 (automated rotation re-emission) is
# wired. The secret Description carries an explicit note: "Rotation: manual
# until S-19; re-run provision-db-url.sh to pick up new password."
#
# SSM parameter names and the env file path are sourced from
# infra/config/deploy-env.sh -- do NOT hardcode them here.

set -e

# Source canonical deploy facts.  deploy-env.sh is downloaded alongside
# this script from S3 into /tmp/ before execution.
. /tmp/deploy-env.sh

REGION="$DEPLOY_REGION"
ENV_FILE="$BFF_ENV_FILE"

# ---------------------------------------------------------------------------
# Step 1: Read production SSM parameters under the EC2 instance role.
#
# Read BEFORE the assume-role because the provisioner role's SSM grant is
# scoped to /vaultmtg/staging/* + /vaultmtg/app/staging/* only and does not
# include /vaultmtg/app/production/*. The instance role already has read
# access to /vaultmtg/app/production/* via the VaultmtgAppProductionNamespace
# policy in ec2.yml.
#
# As of #2223: reads SSM_PROD_APP_DB_SECRET_ARN (vaultmtg_app app secret) so
# the BFF connects as the least-privilege role, not the master superuser.
# run-migrations.sh independently reads SSM_PROD_DB_SECRET_ARN (master secret)
# for DDL migrations and GRANT statements.
# ---------------------------------------------------------------------------
DB_SECRET_ARN_VALUE=$(aws ssm get-parameter \
  --name "$SSM_PROD_APP_DB_SECRET_ARN" \
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

if [ -z "$DB_SECRET_ARN_VALUE" ] || [ -z "$DB_ENDPOINT" ] || [ -z "$DB_NAME" ]; then
  echo "ERROR: one or more production DB SSM parameters returned empty." >&2
  echo "  DB_SECRET_ARN_VALUE (from ${SSM_PROD_APP_DB_SECRET_ARN}): '${DB_SECRET_ARN_VALUE}'" >&2
  echo "  DB_ENDPOINT (from ${SSM_PROD_DB_ENDPOINT}): '${DB_ENDPOINT}'" >&2
  echo "  DB_NAME (from ${SSM_PROD_DB_NAME}): '${DB_NAME}'" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Step 2: Assume the scoped provisioner role.
#
# Calls aws sts assume-role using the EC2 instance role (the SSM session's
# default credentials) as the calling principal. Exports the returned
# temporary credentials so the subsequent aws secretsmanager call runs as
# vaultmtg-staging-deploy-provisioner.
#
# 900s == 15 minutes, the minimum allowed by IAM. The script completes in
# under 15s in practice, so the short TTL is fine and reduces blast radius
# if the credentials leak.
# ---------------------------------------------------------------------------
PROVISIONER_ROLE_ARN="arn:aws:iam::901347789205:role/vaultmtg-staging-deploy-provisioner"
SESSION_NAME="prod-db-url-$(date +%s)"

# Defense in depth: clear temporary credentials AND any secret-bearing
# variables on any exit (success or failure) so the SSM shell environment
# never carries them past this script.
cleanup_creds() {
  unset AWS_ACCESS_KEY_ID
  unset AWS_SECRET_ACCESS_KEY
  unset AWS_SESSION_TOKEN
  unset DB_SECRET_JSON DB_USERNAME DB_PASSWORD DB_USERNAME_ENC DB_PASSWORD_ENC
}
trap cleanup_creds EXIT

echo "Assuming role ${PROVISIONER_ROLE_ARN} as session ${SESSION_NAME}..."
ASSUME_OUTPUT=$(aws sts assume-role \
  --role-arn "$PROVISIONER_ROLE_ARN" \
  --role-session-name "$SESSION_NAME" \
  --duration-seconds 900 \
  --region "$REGION" \
  --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' \
  --output text)

if [ -z "$ASSUME_OUTPUT" ]; then
  echo "ERROR: aws sts assume-role returned empty credentials." >&2
  exit 1
fi

# Tab-separated by --output text; split into the three variables.
AWS_ACCESS_KEY_ID=$(echo "$ASSUME_OUTPUT" | awk '{print $1}')
AWS_SECRET_ACCESS_KEY=$(echo "$ASSUME_OUTPUT" | awk '{print $2}')
AWS_SESSION_TOKEN=$(echo "$ASSUME_OUTPUT" | awk '{print $3}')

if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ] || [ -z "$AWS_SESSION_TOKEN" ]; then
  echo "ERROR: aws sts assume-role returned incomplete credentials." >&2
  exit 1
fi

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY
export AWS_SESSION_TOKEN

# Verify the assumed identity before proceeding -- guards against any silent
# fallback to instance-role credentials.
CALLER_ARN=$(aws sts get-caller-identity --query Arn --output text)
case "$CALLER_ARN" in
  *":assumed-role/vaultmtg-staging-deploy-provisioner/${SESSION_NAME}")
    echo "Assumed role identity confirmed: ${CALLER_ARN}"
    ;;
  *)
    echo "ERROR: caller identity ${CALLER_ARN} is not the provisioner role -- refusing to continue." >&2
    exit 1
    ;;
esac

# ---------------------------------------------------------------------------
# Step 3: Fetch RDS credentials from Secrets Manager under the provisioner role.
# ---------------------------------------------------------------------------
DB_SECRET_JSON=$(aws secretsmanager get-secret-value \
  --secret-id "$DB_SECRET_ARN_VALUE" \
  --region "$REGION" \
  --query SecretString \
  --output text)
DB_USERNAME=$(printf '%s' "$DB_SECRET_JSON" | jq -r '.username // empty')
DB_PASSWORD=$(printf '%s' "$DB_SECRET_JSON" | jq -r '.password // empty')
if [ -z "$DB_USERNAME" ] || [ -z "$DB_PASSWORD" ]; then
  echo "ERROR: RDS secret JSON missing username or password." >&2
  exit 1
fi
# URL-encode credentials so any special characters (`@`, `:`, `/`, `?`,
# `#`, `%`, etc.) in the rotated password do not break URL parsing.
DB_USERNAME_ENC=$(jq -rn --arg v "$DB_USERNAME" '$v|@uri')
DB_PASSWORD_ENC=$(jq -rn --arg v "$DB_PASSWORD" '$v|@uri')
DATABASE_URL=$(printf 'postgresql://%s:%s@%s:%s/%s?%s' \
  "$DB_USERNAME_ENC" "$DB_PASSWORD_ENC" "$DB_ENDPOINT" "$DB_PORT" "$DB_NAME" "$DB_SSL_MODE")
unset DB_SECRET_JSON DB_USERNAME DB_PASSWORD DB_USERNAME_ENC DB_PASSWORD_ENC

# ---------------------------------------------------------------------------
# Step 4: Write the env file. Upsert AWS_DEFAULT_REGION and DATABASE_URL only;
# do NOT write DB_SECRET_ARN or BFF_DB_RESOLVE_FROM_SM -- those would re-enable
# the BFF's runtime SM client and reintroduce the #2461 crash-loop.
# ---------------------------------------------------------------------------
mkdir -p "$BFF_ENV_DIR"

# Upsert AWS_DEFAULT_REGION so the BFF's AWS clients (Sentry, PostHog, etc.)
# resolve the right endpoint. DATABASE_URL no longer triggers an SM call but
# AWS_DEFAULT_REGION is still required by other SDK users in the binary.
if grep -q '^AWS_DEFAULT_REGION=' "$ENV_FILE" 2>/dev/null; then
  sed -i "s|^AWS_DEFAULT_REGION=.*|AWS_DEFAULT_REGION=${REGION}|" "$ENV_FILE"
else
  printf 'AWS_DEFAULT_REGION=%s\n' "$REGION" >> "$ENV_FILE"
fi

# Drop any stale DB_SECRET_ARN or BFF_DB_RESOLVE_FROM_SM lines left over from
# the pre-migration env file. Using sed -i to delete in-place. Both keys are
# documented as intentionally-not-written above; the BFF defaults
# BFF_DB_RESOLVE_FROM_SM=false so an unset value is the safe state.
if grep -q '^DB_SECRET_ARN=' "$ENV_FILE" 2>/dev/null; then
  sed -i '/^DB_SECRET_ARN=/d' "$ENV_FILE"
fi
if grep -q '^BFF_DB_RESOLVE_FROM_SM=' "$ENV_FILE" 2>/dev/null; then
  sed -i '/^BFF_DB_RESOLVE_FROM_SM=/d' "$ENV_FILE"
fi

# Upsert inline-credential DATABASE_URL. Use printf rather than echo to
# preserve any special characters (jq @uri keeps them URL-safe already, but
# echo can interpret backslash sequences on some shells).
if grep -q '^DATABASE_URL=' "$ENV_FILE" 2>/dev/null; then
  # Use a sed delimiter (|) that does not appear in postgresql:// URLs.
  sed -i "s|^DATABASE_URL=.*|DATABASE_URL=${DATABASE_URL}|" "$ENV_FILE"
else
  printf 'DATABASE_URL=%s\n' "$DATABASE_URL" >> "$ENV_FILE"
fi

# DATABASE_URL goes out of scope at script exit; explicit unset for parity
# with the staging script's defensive cleanup.
unset DATABASE_URL

chmod 600 "$ENV_FILE"
echo "DATABASE_URL provisioned (credentials spliced from Secrets Manager under provisioner role)."
