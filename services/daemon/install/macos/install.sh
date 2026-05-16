#!/usr/bin/env bash
# install.sh — macOS installer for the MTGA Companion daemon
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/install.sh | bash
#
# The script:
#   1. Detects the host architecture (arm64 or amd64).
#   2. Downloads the correct release binary from GitHub Releases.
#   3. Installs the binary to /usr/local/bin/.
#   4. Writes a launchd plist to ~/Library/LaunchAgents/ and loads it.
#
# DRY_RUN mode (for CI validation):
#   Set DRY_RUN=1 to skip the sudo install and launchctl load steps.
#   The script will still validate architecture detection, tag resolution,
#   config/plist generation, and output all paths — but will not write to
#   system directories or touch launchd.
#   Example:  DRY_RUN=1 bash install.sh

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration — edit these for a specific release.
# ---------------------------------------------------------------------------
GITHUB_REPO="RdHamilton/MTGA-Companion"
# RELEASE_TAG is the daemon release tag, e.g. "daemon/v0.2.0".
# Override with:  RELEASE_TAG=daemon/v0.1.0 bash install.sh
RELEASE_TAG="${RELEASE_TAG:-}"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="mtga-companion-daemon"
PLIST_LABEL="com.mtga-companion.daemon"
PLIST_PATH="${HOME}/Library/LaunchAgents/${PLIST_LABEL}.plist"
CONFIG_DIR="${HOME}/.mtga-companion"
CONFIG_FILE="${CONFIG_DIR}/daemon.json"

# ---------------------------------------------------------------------------
# Detect architecture.
# ---------------------------------------------------------------------------
ARCH="$(uname -m)"
case "${ARCH}" in
  arm64)  ASSET_SUFFIX="darwin-arm64" ;;
  x86_64) ASSET_SUFFIX="darwin-amd64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

ASSET_NAME="${BINARY_NAME}-${ASSET_SUFFIX}"

# ---------------------------------------------------------------------------
# Resolve the release tag: use the provided tag or fetch the latest daemon
# release from the GitHub API.
# ---------------------------------------------------------------------------
if [[ -z "${RELEASE_TAG}" ]]; then
  echo "Fetching latest daemon release tag..."
  # The GitHub API returns releases sorted by creation date newest-first.
  # We filter to tags that start with "daemon/" to avoid picking a wrong tag.
  RELEASE_TAG="$(
    curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases" \
      | python3 -c "
import json, sys
releases = json.load(sys.stdin)
for r in releases:
    tag = r.get('tag_name', '')
    if tag.startswith('daemon/'):
        print(tag)
        break
"
  )"
  if [[ -z "${RELEASE_TAG}" ]]; then
    echo "Could not determine latest daemon release. Set RELEASE_TAG and retry." >&2
    exit 1
  fi
fi

echo "Installing MTGA Companion daemon ${RELEASE_TAG} (${ASSET_SUFFIX})..."

# ---------------------------------------------------------------------------
# Build the download URL and fetch the binary.
# GitHub Releases asset URL format:
#   https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>
# ---------------------------------------------------------------------------
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}/${ASSET_NAME}"
TMP_BIN="$(mktemp)"

echo "Downloading ${DOWNLOAD_URL}..."
curl -fsSL --progress-bar -o "${TMP_BIN}" "${DOWNLOAD_URL}"

# ---------------------------------------------------------------------------
# Install the binary.
# /usr/local/bin typically requires sudo on a stock macOS install.
# ---------------------------------------------------------------------------
chmod +x "${TMP_BIN}"
echo "Installing binary to ${INSTALL_DIR}/${BINARY_NAME} (may prompt for sudo)..."
if [[ -z "${DRY_RUN:-}" ]]; then
  sudo install -m 755 "${TMP_BIN}" "${INSTALL_DIR}/${BINARY_NAME}"
else
  echo "[DRY_RUN] skipping: sudo install -m 755 ${TMP_BIN} ${INSTALL_DIR}/${BINARY_NAME}"
fi
rm -f "${TMP_BIN}"

echo "Binary installed: ${INSTALL_DIR}/${BINARY_NAME}"

# ---------------------------------------------------------------------------
# Write the JSON config file.
# Key names must match the json struct tags in
# services/daemon/internal/config/config.go.
# Default path matches main.go: ~/.mtga-companion/daemon.json
#
# jq is used to produce safe JSON — values are escaped properly even if they
# contain quotes, backslashes, or newlines.  python3 is the fallback because
# macOS ships it but not jq by default.
# ---------------------------------------------------------------------------
mkdir -p "${CONFIG_DIR}"

if [[ ! -f "${CONFIG_FILE}" ]]; then
  # Prompt for values only on a fresh install.
  echo ""
  printf "Enter BFF URL (e.g. https://api.yourdomain.com): "
  read -r BFF_URL
  printf "Enter daemon auth token (daemon JWT from first registration): "
  read -r DAEMON_AUTH_TOKEN

  if command -v jq >/dev/null 2>&1; then
    jq -n --arg cloud "${BFF_URL}" --arg key "${DAEMON_AUTH_TOKEN}" \
      '{"cloud_api_url":$cloud,"api_key":$key}' > "${CONFIG_FILE}"
  else
    python3 -c "
import json, sys
print(json.dumps({'cloud_api_url': sys.argv[1], 'api_key': sys.argv[2]}, indent=2))
" "${BFF_URL}" "${DAEMON_AUTH_TOKEN}" > "${CONFIG_FILE}"
  fi

  chmod 600 "${CONFIG_FILE}"
  echo "Config written: ${CONFIG_FILE}"
else
  echo "Config already exists, skipping: ${CONFIG_FILE}"
fi

# ---------------------------------------------------------------------------
# Write the launchd plist.
# RunAtLoad=true  — start the daemon when the user logs in.
# KeepAlive=true  — relaunch the daemon if it exits unexpectedly.
# ---------------------------------------------------------------------------
mkdir -p "${HOME}/Library/LaunchAgents"

cat > "${PLIST_PATH}" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- Unique identifier for this launchd job. -->
    <key>Label</key>
    <string>${PLIST_LABEL}</string>

    <!-- The daemon binary and its arguments. -->
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>-config</string>
        <string>${CONFIG_FILE}</string>
    </array>

    <!-- Start automatically when the user logs in. -->
    <key>RunAtLoad</key>
    <true/>

    <!-- Restart the daemon if it exits for any reason. -->
    <key>KeepAlive</key>
    <true/>

    <!-- Write stdout/stderr to the system log directory. -->
    <key>StandardOutPath</key>
    <string>${HOME}/Library/Logs/mtga-companion-daemon.log</string>
    <key>StandardErrorPath</key>
    <string>${HOME}/Library/Logs/mtga-companion-daemon.log</string>
</dict>
</plist>
PLIST

echo "launchd plist written: ${PLIST_PATH}"

# ---------------------------------------------------------------------------
# Load (and enable) the launchd job.
# -w flag persists the job across reboots by writing to the LaunchAgents DB.
# ---------------------------------------------------------------------------
if [[ -z "${DRY_RUN:-}" ]]; then
  launchctl load -w "${PLIST_PATH}"
else
  echo "[DRY_RUN] skipping: launchctl load -w ${PLIST_PATH}"
fi

echo ""
echo "MTGA Companion daemon installed and running."
echo "  Binary : ${INSTALL_DIR}/${BINARY_NAME}"
echo "  Config : ${CONFIG_FILE}"
echo "  plist  : ${PLIST_PATH}"
echo "  Logs   : ${HOME}/Library/Logs/mtga-companion-daemon.log"
echo ""
echo "To change the BFF URL or rotate the auth token, edit: ${CONFIG_FILE}"
