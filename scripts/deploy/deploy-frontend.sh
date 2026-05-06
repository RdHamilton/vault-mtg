#!/bin/sh
# deploy-frontend.sh
# Pulls the React SPA tarball from S3 and atomically swaps it into the nginx webroot.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Required environment variables (passed by the GitHub Actions deploy step):
#   DEPLOY_BUCKET - S3 bucket name
#   DEPLOY_SHA    - Git SHA used as the S3 key prefix

set -e

: "${DEPLOY_BUCKET:?DEPLOY_BUCKET must be set}"
: "${DEPLOY_SHA:?DEPLOY_SHA must be set}"

DEPLOY_DIR=/var/www/mtga-companion
STAGING_DIR=/var/www/mtga-companion-staging

rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"

aws s3 cp "s3://${DEPLOY_BUCKET}/frontend/${DEPLOY_SHA}/frontend-dist.tar.gz" /tmp/frontend-dist.tar.gz
tar -xzf /tmp/frontend-dist.tar.gz -C "$STAGING_DIR"

rm -rf "${DEPLOY_DIR}.old"
if [ -d "$DEPLOY_DIR" ]; then
  mv "$DEPLOY_DIR" "${DEPLOY_DIR}.old"
fi
mv "$STAGING_DIR" "$DEPLOY_DIR"

chown -R nginx:nginx "$DEPLOY_DIR"
chmod -R 755 "$DEPLOY_DIR"
nginx -t && systemctl reload nginx

rm -f /tmp/frontend-dist.tar.gz
echo "Frontend deploy complete: ${DEPLOY_SHA}"
