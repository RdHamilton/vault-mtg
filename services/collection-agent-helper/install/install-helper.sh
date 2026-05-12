#!/bin/bash
# Installs the VaultMTG collection helper as a root launchd daemon.
# Must be run as root (invoked by the tray via osascript with admin privileges).
set -euo pipefail

HELPER_BINARY="${1:?usage: install-helper.sh <helper-binary-path>}"
DEST_DIR="/Library/Application Support/VaultMTG"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PLIST_SRC="$SCRIPT_DIR/com.vaultmtg.collection-helper.plist"

# Validate plist exists and is a regular file before using it.
if [[ ! -f "$PLIST_SRC" ]]; then
    echo "error: plist not found: $PLIST_SRC" >&2
    exit 1
fi
PLIST_DEST="/Library/LaunchDaemons/com.vaultmtg.collection-helper.plist"
LOG_DIR="/Library/Logs/VaultMTG"
LABEL="com.vaultmtg.collection-helper"

mkdir -p "$DEST_DIR"
mkdir -p "$LOG_DIR"

# Unload existing daemon if running
if launchctl list "$LABEL" &>/dev/null; then
    launchctl unload "$PLIST_DEST" 2>/dev/null || true
fi

cp -f "$HELPER_BINARY" "$DEST_DIR/collection-helper"
chmod 755 "$DEST_DIR/collection-helper"
chown root:wheel "$DEST_DIR/collection-helper"

cp -f "$PLIST_SRC" "$PLIST_DEST"
chmod 644 "$PLIST_DEST"
chown root:wheel "$PLIST_DEST"

launchctl load "$PLIST_DEST"

echo "VaultMTG collection helper installed and started."
