#!/bin/sh
# restart-bff.sh
# Restarts the BFF systemd service on the EC2 host.
# Runs ON the EC2 instance via SSM RunShellScript.
#
# Service name: mtga-companion  (unit file: mtga-companion-infra/systemd/mtga-companion.service)
# Do NOT use "mtga-bff" — that unit does not exist on the host and will exit code 5.

set -e

systemctl restart mtga-companion
echo "mtga-companion service restarted."
