#!/bin/sh
# stage-binary-staging.sh
# Downloads the staging BFF binary from S3 and atomically replaces the staged binary.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Required environment variables (exported inline in the SSM command string):
#   DEPLOY_BUCKET - S3 bucket name (vaultmtg-deploy-artifacts-staging)
#   DEPLOY_SHA    - Git SHA used as the S3 key prefix

set -e

: "${DEPLOY_BUCKET:?DEPLOY_BUCKET must be set}"
: "${DEPLOY_SHA:?DEPLOY_SHA must be set}"

aws s3 cp "s3://${DEPLOY_BUCKET}/staging/${DEPLOY_SHA}/mtga-bff-staging" /usr/local/bin/mtga-bff-staging.next
chmod +x /usr/local/bin/mtga-bff-staging.next
mv /usr/local/bin/mtga-bff-staging.next /usr/local/bin/mtga-bff-staging

echo "Staging binary staged: ${DEPLOY_SHA}"
