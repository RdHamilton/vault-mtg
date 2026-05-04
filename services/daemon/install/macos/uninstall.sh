#!/usr/bin/env bash
# uninstall.sh — removes the MTGA Companion daemon from macOS.
#
# Usage:
#   bash uninstall.sh
#
# Steps:
#   1. Unloads and disables the launchd job.
#   2. Removes the plist from ~/Library/LaunchAgents/.
#   3. Removes the binary from /usr/local/bin/.

set -euo pipefail

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="mtga-companion-daemon"
PLIST_LABEL="com.mtga-companion.daemon"
PLIST_PATH="${HOME}/Library/LaunchAgents/${PLIST_LABEL}.plist"

# ---------------------------------------------------------------------------
# Unload the launchd job.
# -w removes the Disabled key from the launch database so the job does not
# reload on next login even if the plist is present.
# We use `|| true` because `launchctl unload` exits non-zero when the job
# was never loaded (e.g. running uninstall twice).
# ---------------------------------------------------------------------------
if [[ -f "${PLIST_PATH}" ]]; then
  echo "Unloading launchd job ${PLIST_LABEL}..."
  launchctl unload -w "${PLIST_PATH}" 2>/dev/null || true
  echo "Removing plist: ${PLIST_PATH}"
  rm -f "${PLIST_PATH}"
else
  echo "Plist not found (${PLIST_PATH}), skipping launchd unload."
fi

# ---------------------------------------------------------------------------
# Remove the binary.
# sudo is needed because /usr/local/bin is owned by root on stock macOS.
# ---------------------------------------------------------------------------
BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"
if [[ -f "${BINARY_PATH}" ]]; then
  echo "Removing binary: ${BINARY_PATH} (may prompt for sudo)..."
  sudo rm -f "${BINARY_PATH}"
else
  echo "Binary not found (${BINARY_PATH}), skipping."
fi

echo ""
echo "MTGA Companion daemon uninstalled."
echo "Log file (${HOME}/Library/Logs/mtga-companion-daemon.log) was NOT removed."
echo "Config file (~/.config/mtga-companion/daemon.yaml) was NOT removed."
echo "Remove those manually if desired."
