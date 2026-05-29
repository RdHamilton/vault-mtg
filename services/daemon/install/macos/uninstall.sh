#!/usr/bin/env bash
# uninstall.sh — removes the VaultMTG daemon from macOS.
#
# Usage:
#   bash uninstall.sh [--purge]
#
# Options:
#   --purge   Also delete the daemon's API key from the macOS Keychain.
#             By default the keychain entry (service: com.vaultmtg.daemon,
#             account: api-key) is retained for downgrade safety so that a
#             reinstall does not require re-authenticating.
#
# Steps (ADR-022 Phase 2):
#   1. Unloads and disables the new launchd job (com.vaultmtg.daemon).
#   2. Unloads and disables the legacy launchd job (com.mtga-companion.daemon)
#      if still present — handles the upgrade-then-uninstall scenario.
#   3. Removes both plists from ~/Library/LaunchAgents/.
#   4. Removes the binary from /usr/local/bin/.
#   5. Removes the legacy binary (mtga-companion-daemon) if present (upgrader path).
#   6. (--purge only) Deletes the API key from the macOS Keychain.

set -euo pipefail

# ---------------------------------------------------------------------------
# Parse arguments.
# ---------------------------------------------------------------------------
PURGE=0
for arg in "$@"; do
  case "${arg}" in
    --purge) PURGE=1 ;;
    *) echo "Unknown argument: ${arg}" >&2; exit 1 ;;
  esac
done

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="vaultmtg-daemon"
# ADR-022 Phase 2: legacy binary name — removed on the upgrader path.
BINARY_NAME_LEGACY="mtga-companion-daemon"

# ADR-022 Phase 2: new label.
PLIST_LABEL="com.vaultmtg.daemon"
# Legacy label — also unloaded when present.
PLIST_LABEL_LEGACY="com.mtga-companion.daemon"

PLIST_PATH="${HOME}/Library/LaunchAgents/${PLIST_LABEL}.plist"
PLIST_PATH_LEGACY="${HOME}/Library/LaunchAgents/${PLIST_LABEL_LEGACY}.plist"

# ---------------------------------------------------------------------------
# Unload the new launchd job (com.vaultmtg.daemon).
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
# CRITICAL (ADR-022 Constraint 1): Unload the legacy launchd job if present.
# This handles the case where a user had the old daemon installed and never
# ran the new installer — the legacy label may still be registered.
# Failures are non-fatal (|| true) — a fresh install has no legacy label.
# ---------------------------------------------------------------------------
if [[ -f "${PLIST_PATH_LEGACY}" ]]; then
  echo "Found legacy plist (${PLIST_PATH_LEGACY}) — unloading and removing..."
  launchctl unload -w "${PLIST_PATH_LEGACY}" 2>/dev/null || true
  rm -f "${PLIST_PATH_LEGACY}"
  echo "Legacy launchd job removed."
elif launchctl list "${PLIST_LABEL_LEGACY}" >/dev/null 2>&1; then
  # Label is loaded but plist is gone — use label-based bootout.
  echo "Found legacy launchd label ${PLIST_LABEL_LEGACY} (no plist) — booting out..."
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL_LEGACY}" 2>/dev/null || true
else
  echo "Legacy launchd label (${PLIST_LABEL_LEGACY}) not found, skipping."
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

# ---------------------------------------------------------------------------
# Remove the legacy binary (upgrader path — vault-mtg-tickets#48).
# Mirrors the pattern above. The guard ensures sudo is only invoked when the
# file is actually present — a fresh install (no legacy binary) skips cleanly.
# ---------------------------------------------------------------------------
LEGACY_BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME_LEGACY}"
if [[ -f "${LEGACY_BINARY_PATH}" ]]; then
  echo "Found legacy binary: ${LEGACY_BINARY_PATH} — removing..."
  sudo rm -f "${LEGACY_BINARY_PATH}"
  echo "Legacy binary removed."
else
  echo "Legacy binary not found (${LEGACY_BINARY_PATH}), skipping."
fi

# ---------------------------------------------------------------------------
# Keychain entry (com.vaultmtg.daemon / api-key).
# Default behaviour: RETAIN the entry for downgrade safety — a user who
# reinstalls the daemon will not need to re-authenticate.
# --purge: delete the entry via security(1) so no credential remains on disk.
# Failure (entry already absent) is non-fatal — security exits 44 in that case.
# ---------------------------------------------------------------------------
KEYCHAIN_SERVICE="com.vaultmtg.daemon"
KEYCHAIN_ACCOUNT="api-key"

if [[ "${PURGE}" -eq 1 ]]; then
  echo "Removing keychain entry (${KEYCHAIN_SERVICE} / ${KEYCHAIN_ACCOUNT})..."
  security delete-generic-password \
    -s "${KEYCHAIN_SERVICE}" \
    -a "${KEYCHAIN_ACCOUNT}" 2>/dev/null || true
  echo "Keychain entry removed (or was already absent)."
fi

echo ""
echo "VaultMTG daemon uninstalled."
echo "Log file (${HOME}/Library/Logs/vaultmtg-daemon.log) was NOT removed."
echo "Config file (~/.vaultmtg/daemon.json) was NOT removed."
echo "Remove those manually if desired."
if [[ "${PURGE}" -eq 0 ]]; then
  echo "API key retained in keychain for downgrade safety. Run with --purge to remove all data."
fi
