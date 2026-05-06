#!/usr/bin/env bash
# sync-ssm-params.sh
# Reads CloudFormation stack outputs for vaultmtg-app-cdn and rhamiltoneng-cdn
# and writes them to SSM Parameter Store.
#
# Usage: ./infrastructure/scripts/sync-ssm-params.sh [--profile <aws-profile>] [--region <aws-region>]
#
# Defaults:
#   --profile  personal
#   --region   us-east-1
#
# Run this after deploying or updating either CDN stack so that CI/CD workflows
# can resolve bucket names and distribution IDs from SSM without hardcoding.

set -euo pipefail

PROFILE="personal"
REGION="us-east-1"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile) PROFILE="$2"; shift 2 ;;
    --region)  REGION="$2";  shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

aws_cf_output() {
  local stack="$1"
  local key="$2"
  aws cloudformation describe-stacks \
    --stack-name "$stack" \
    --region "$REGION" \
    --profile "$PROFILE" \
    --query "Stacks[0].Outputs[?OutputKey=='${key}'].OutputValue" \
    --output text
}

ssm_put() {
  local name="$1"
  local value="$2"
  echo "  Writing $name"
  aws ssm put-parameter \
    --name "$name" \
    --value "$value" \
    --type String \
    --overwrite \
    --region "$REGION" \
    --profile "$PROFILE" \
    --output none
}

echo "==> Pulling vaultmtg-app-cdn outputs..."
MARKETING_BUCKET=$(aws_cf_output "vaultmtg-app-cdn" "MarketingBucketName")
SPA_BUCKET=$(aws_cf_output "vaultmtg-app-cdn" "SPABucketName")
MARKETING_DIST_ID=$(aws_cf_output "vaultmtg-app-cdn" "MarketingDistributionId")
SPA_DIST_ID=$(aws_cf_output "vaultmtg-app-cdn" "SPADistributionId")

echo "==> Writing vaultmtg SSM parameters..."
ssm_put "/vaultmtg/production/marketing-bucket-name"     "$MARKETING_BUCKET"
ssm_put "/vaultmtg/production/spa-bucket-name"           "$SPA_BUCKET"
ssm_put "/vaultmtg/production/marketing-distribution-id" "$MARKETING_DIST_ID"
ssm_put "/vaultmtg/production/spa-distribution-id"       "$SPA_DIST_ID"

echo "==> Pulling rhamiltoneng-cdn outputs..."
SITE_BUCKET=$(aws_cf_output "rhamiltoneng-cdn" "SiteBucketName")
DIST_ID=$(aws_cf_output "rhamiltoneng-cdn" "DistributionId")

echo "==> Writing rhamiltoneng SSM parameters..."
ssm_put "/rhamiltoneng/production/site-bucket-name" "$SITE_BUCKET"
ssm_put "/rhamiltoneng/production/distribution-id"  "$DIST_ID"

echo ""
echo "==> Verifying..."
aws ssm get-parameters-by-path \
  --path "/vaultmtg/production" \
  --region "$REGION" \
  --profile "$PROFILE" \
  --query "Parameters[].{Name:Name,Value:Value}" \
  --output table

aws ssm get-parameters-by-path \
  --path "/rhamiltoneng/production" \
  --region "$REGION" \
  --profile "$PROFILE" \
  --query "Parameters[].{Name:Name,Value:Value}" \
  --output table

echo "==> Done."
