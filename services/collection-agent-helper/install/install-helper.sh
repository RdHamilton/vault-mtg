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

# Unload existing daemon if running (support both legacy and modern launchctl).
launchctl bootout system/"$LABEL" 2>/dev/null || \
    launchctl unload "$PLIST_DEST" 2>/dev/null || true

cp -f "$HELPER_BINARY" "$DEST_DIR/collection-helper"
chmod 755 "$DEST_DIR/collection-helper"
chown root:wheel "$DEST_DIR/collection-helper"
# Clear quarantine attribute so Gatekeeper does not block the binary.
xattr -cr "$DEST_DIR/collection-helper" 2>/dev/null || true

cp -f "$PLIST_SRC" "$PLIST_DEST"
chmod 644 "$PLIST_DEST"
chown root:wheel "$PLIST_DEST"

# Use the modern bootstrap API (macOS Ventura+); fall back to legacy load.
launchctl bootstrap system "$PLIST_DEST" 2>/dev/null || \
    launchctl load "$PLIST_DEST"

echo "VaultMTG collection helper installed and started."
