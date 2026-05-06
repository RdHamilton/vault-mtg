#!/bin/sh
# stage-binary.sh
# Downloads the BFF binary from S3 and atomically replaces the running binary.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Required environment variables (set by the GitHub Actions deploy step via SSM
# --environment-variables or by exporting before calling this script):
#   DEPLOY_BUCKET - S3 bucket name
#   DEPLOY_SHA    - Git SHA used as the S3 key prefix

set -e

: "${DEPLOY_BUCKET:?DEPLOY_BUCKET must be set}"
: "${DEPLOY_SHA:?DEPLOY_SHA must be set}"

aws s3 cp "s3://${DEPLOY_BUCKET}/releases/${DEPLOY_SHA}/mtga-bff" /usr/local/bin/mtga-bff.next
chmod +x /usr/local/bin/mtga-bff.next
mv /usr/local/bin/mtga-bff.next /usr/local/bin/mtga-bff

echo "Binary staged: ${DEPLOY_SHA}"
