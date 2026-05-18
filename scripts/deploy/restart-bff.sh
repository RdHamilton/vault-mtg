#!/bin/sh
# restart-bff.sh
# Restarts the production BFF systemd service.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Service name is sourced from infra/config/deploy-env.sh (BFF_SERVICE).

set -e

# Source canonical deploy facts.  deploy-env.sh is downloaded alongside
# this script from S3 into /tmp/ before execution.
. /tmp/deploy-env.sh

systemctl restart "$BFF_SERVICE"
echo "${BFF_SERVICE} service restarted."
