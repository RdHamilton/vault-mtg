#!/bin/sh
# restart-bff-staging.sh
# Restarts the staging BFF systemd service.
# Runs ON the EC2 instance via SSM RunShellScript (as root).
#
# Service name is sourced from infra/config/deploy-env.sh (BFF_STAGING_SERVICE).

set -e

# Source canonical deploy facts.  deploy-env.sh is downloaded alongside
# this script from S3 into /tmp/ before execution.
. /tmp/deploy-env.sh

SERVICE="$BFF_STAGING_SERVICE"
UNIT_FILE="/etc/systemd/system/${SERVICE}.service"

# Guard: verify the systemd unit exists before attempting restart.
# If the unit is missing the EC2 instance was never bootstrapped — surface
# a clear error rather than a cryptic "Unit not found" from systemctl.
if [ ! -f "$UNIT_FILE" ]; then
    echo "[restart-bff-staging] ERROR: systemd unit not found at $UNIT_FILE"
    echo "  The EC2 instance has not been bootstrapped for the staging service."
    echo "  Run infra/scripts/install-staging-service.sh on the instance first,"
    echo "  or re-run the infra CloudFormation bootstrap stack."
    exit 1
fi

systemctl daemon-reload
systemctl enable "$SERVICE"
systemctl restart "$SERVICE"

echo "[restart-bff-staging] ${SERVICE} restarted successfully."
systemctl status "$SERVICE" --no-pager --lines=5 || true
