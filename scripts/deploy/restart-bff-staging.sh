#!/bin/sh
# restart-bff-staging.sh
# Restarts the mtga-bff-staging systemd service.
# Runs ON the EC2 instance via SSM RunShellScript.

set -e

systemctl restart mtga-bff-staging
echo "mtga-bff-staging service restarted."
