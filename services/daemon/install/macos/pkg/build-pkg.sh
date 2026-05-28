#!/usr/bin/env bash
# build-pkg.sh — Build the macOS .pkg installer and wrap it in a .dmg.
#
# Usage:
#   BINARY_PATH=bin/vaultmtg-daemon \
#   VERSION=0.3.1 \
#   TEAM_ID=<Apple Team ID> \
#   bash services/daemon/install/macos/pkg/build-pkg.sh
#
# Required environment variables:
#   BINARY_PATH   Path to the darwin universal binary (already codesigned).
#   VERSION       Semver string (e.g. "0.3.1") — no leading "v".
#   TEAM_ID       Apple Developer Team ID for signing (omit to skip signing).
#
# Outputs (in the current directory):
#   vaultmtg-daemon-darwin-universal.pkg
#   vaultmtg-daemon-darwin-universal.dmg
#
# This script is intended to be called from the GoReleaser hooks or the
# daemon-release CI workflow after the binary has been built and signed.

set -euo pipefail

# DRY_RUN=1 — print what would be done and exit 0.  Used by CI to assert the
# pkg-root layout and by developers to preview actions without needing pkgbuild
# or hdiutil installed.  Pass --dry-run as the first argument for the same effect.
if [[ "${1:-}" == "--dry-run" ]]; then DRY_RUN=1; fi
DRY_RUN="${DRY_RUN:-0}"

BINARY_PATH="${BINARY_PATH:?BINARY_PATH is required}"
VERSION="${VERSION:?VERSION is required}"
TEAM_ID="${TEAM_ID:-}"

BINARY_NAME="vaultmtg-daemon"
PKG_ID="com.vaultmtg.daemon"
PKG_NAME="vaultmtg-daemon-darwin-universal.pkg"
DMG_NAME="vaultmtg-daemon-darwin-universal.dmg"

# Share directory — where the uninstall script is installed on the target system.
# Referenced here (to populate the package root) and in postinstall (echo to user).
# Single constant — never copy-paste this path.
SHARE_DIR="/usr/local/share/vaultmtg"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG_ROOT="$(mktemp -d)/pkg-root"
DMG_STAGING="$(mktemp -d)/dmg-staging"

echo "[build-pkg] building .pkg version ${VERSION}"
# Always emit PKG_ROOT so callers can inspect the layout without parsing mktemp output.
echo "PKG_ROOT=${PKG_ROOT}"

# ---------------------------------------------------------------------------
# Populate the package root.
# The binary lives at /usr/local/bin/<name> — pkgbuild uses the directory
# structure under pkg-root as the install tree (--install-location /).
# ---------------------------------------------------------------------------
mkdir -p "${PKG_ROOT}/usr/local/bin"
cp "${BINARY_PATH}" "${PKG_ROOT}/usr/local/bin/${BINARY_NAME}"
chmod 755 "${PKG_ROOT}/usr/local/bin/${BINARY_NAME}"

# Install the uninstall script to the share directory so users have a clean
# removal path after install. The path is echoed by postinstall so users see
# it immediately after installation completes.
mkdir -p "${PKG_ROOT}${SHARE_DIR}"
cp "${SCRIPT_DIR}/../uninstall.sh" "${PKG_ROOT}${SHARE_DIR}/uninstall.sh"
chmod 755 "${PKG_ROOT}${SHARE_DIR}/uninstall.sh"

# DRY_RUN: pkg-root is now populated — print layout and exit without
# calling pkgbuild or hdiutil (neither is required for layout assertions).
if [[ "${DRY_RUN}" == "1" ]]; then
  echo "[build-pkg] DRY_RUN — pkg-root layout (no .pkg or .dmg produced):"
  find "${PKG_ROOT}" -type f | sort
  echo "[build-pkg] DRY_RUN complete — Result: PASS"
  exit 0
fi

# ---------------------------------------------------------------------------
# Build the .pkg using the postinstall script for LaunchAgent setup.
# ---------------------------------------------------------------------------
PKGBUILD_ARGS=(
  --root "${PKG_ROOT}"
  --scripts "${SCRIPT_DIR}"
  --identifier "${PKG_ID}"
  --version "${VERSION}"
  --install-location /
)

if [[ -n "${TEAM_ID}" ]]; then
  PKGBUILD_ARGS+=(--sign "Developer ID Installer: ${TEAM_ID}" --timestamp)
fi

pkgbuild "${PKGBUILD_ARGS[@]}" "${PKG_NAME}"

echo "[build-pkg] .pkg built: ${PKG_NAME}"

# ---------------------------------------------------------------------------
# Wrap the .pkg in a .dmg.
# The .dmg gives users a familiar "drag to install" surface and allows the
# release to be distributed as a single downloadable file.
# ---------------------------------------------------------------------------
cp "${PKG_NAME}" "${DMG_STAGING}/"

hdiutil create \
  -volname "MTGA Companion Daemon ${VERSION}" \
  -srcfolder "${DMG_STAGING}" \
  -ov \
  -format UDZO \
  "${DMG_NAME}"

echo "[build-pkg] .dmg built: ${DMG_NAME}"

# ---------------------------------------------------------------------------
# Clean up temp dirs.
# ---------------------------------------------------------------------------
rm -rf "$(dirname "${PKG_ROOT}")" "$(dirname "${DMG_STAGING}")"

echo "[build-pkg] done"
echo "  pkg : ${PKG_NAME}"
echo "  dmg : ${DMG_NAME}"
