#!/bin/sh
# restart-bff.sh
# Restarts the mtga-bff systemd service.
# Runs ON the EC2 instance via SSM RunShellScript.

set -e

systemctl restart mtga-bff
echo "mtga-bff service restarted."
