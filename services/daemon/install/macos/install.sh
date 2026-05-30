#!/usr/bin/env bash
# install.sh — macOS installer for the VaultMTG daemon
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/install.sh | bash
#
# The script:
#   1. Detects the host architecture (arm64 or amd64).
#   2. Downloads the correct release binary from GitHub Releases.
#   3. Installs the binary to /usr/local/bin/.
#   4. Detects and unloads the OLD com.mtga-companion.daemon launchd label if present
#      (CRITICAL: prevents two daemon instances running simultaneously — ADR-022).
#   5. Writes a launchd plist to ~/Library/LaunchAgents/ and loads it under the new
#      com.vaultmtg.daemon label.

set -euo pipefail

# ---------------------------------------------------------------------------
# DRY_RUN mode — set DRY_RUN=1 or pass --dry-run to skip sudo/launchctl.
# Used by automated tests (bats) and local verification to exercise the
# script safely without touching the system.
# ---------------------------------------------------------------------------
DRY_RUN="${DRY_RUN:-}"
for _arg in "$@"; do
  if [[ "${_arg}" == "--dry-run" ]]; then DRY_RUN=1; fi
done
unset _arg

# ---------------------------------------------------------------------------
# Configuration — edit these for a specific release.
# ---------------------------------------------------------------------------
GITHUB_REPO="RdHamilton/MTGA-Companion"
# RELEASE_TAG is the daemon release tag, e.g. "daemon/v0.3.2".
# Override with:  RELEASE_TAG=daemon/v0.3.2 bash install.sh
RELEASE_TAG="${RELEASE_TAG:-}"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="vaultmtg-daemon"

# ADR-022 Phase 2: new label.
PLIST_LABEL="com.vaultmtg.daemon"
# Legacy label — unloaded before registering the new one (prevents dual-daemon).
PLIST_LABEL_LEGACY="com.mtga-companion.daemon"

PLIST_PATH="${HOME}/Library/LaunchAgents/${PLIST_LABEL}.plist"
PLIST_PATH_LEGACY="${HOME}/Library/LaunchAgents/${PLIST_LABEL_LEGACY}.plist"

# ADR-022 Phase 2: new config dir.
CONFIG_DIR="${HOME}/.vaultmtg"
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

echo "Installing VaultMTG daemon ${RELEASE_TAG} (${ASSET_SUFFIX})..."

# ---------------------------------------------------------------------------
# Build the download URL and fetch the binary.
# GitHub Releases asset URL format:
#   https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>
# ---------------------------------------------------------------------------
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}/${ASSET_NAME}"
TMP_BIN="$(mktemp)"

if [[ -z "${DRY_RUN}" ]]; then
  echo "Downloading ${DOWNLOAD_URL}..."
  curl -fsSL --progress-bar -o "${TMP_BIN}" "${DOWNLOAD_URL}"
else
  echo "[DRY_RUN] would download ${DOWNLOAD_URL}"
  # mktemp (line above) already created the file; no placeholder needed.
fi

# ---------------------------------------------------------------------------
# Install the binary.
# /usr/local/bin typically requires sudo on a stock macOS install.
# ---------------------------------------------------------------------------
chmod +x "${TMP_BIN}"
echo "Installing binary to ${INSTALL_DIR}/${BINARY_NAME} (may prompt for sudo)..."
if [[ -z "${DRY_RUN}" ]]; then
  sudo install -m 755 "${TMP_BIN}" "${INSTALL_DIR}/${BINARY_NAME}"
else
  echo "[DRY_RUN] would install binary to ${INSTALL_DIR}/${BINARY_NAME}"
fi
rm -f "${TMP_BIN}"

echo "Binary installed: ${INSTALL_DIR}/${BINARY_NAME}"

# ---------------------------------------------------------------------------
# Write the JSON config file.
# Key names must match the json struct tags in
# services/daemon/internal/config/config.go.
# Default path matches main.go: ~/.vaultmtg/daemon.json
#
# jq is used to produce safe JSON — values are escaped properly even if they
# contain quotes, backslashes, or newlines.  python3 is the fallback because
# macOS ships it but not jq by default.
# ---------------------------------------------------------------------------
mkdir -p "${CONFIG_DIR}"

if [[ ! -f "${CONFIG_FILE}" ]]; then
  # Prompt for values only on a fresh install.
  echo ""
  printf "Enter BFF URL (e.g. https://api.vaultmtg.app/api/v1): "
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
  # Config exists (reinstall path) — always write cloud_api_url because the
  # installer is invoked for a specific environment (staging vs prod).
  # All other fields (api_key, account_id, daemon_jwt, etc.) are preserved.
  echo ""
  printf "Enter BFF URL (leave blank to keep existing): "
  read -r BFF_URL_INPUT || BFF_URL_INPUT=""
  if [[ -n "${BFF_URL_INPUT}" ]]; then
    python3 - "${CONFIG_FILE}" "${BFF_URL_INPUT}" <<'PYEOF'
import json, sys
path, new_url = sys.argv[1], sys.argv[2]
with open(path) as f:
    data = json.load(f)
data['cloud_api_url'] = new_url
with open(path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
PYEOF
    echo "Config updated (cloud_api_url): ${CONFIG_FILE}"
  else
    echo "Config already exists, skipping: ${CONFIG_FILE}"
  fi
fi

# ---------------------------------------------------------------------------
# CRITICAL (ADR-022 Constraint 1): Detect and unload the old launchd label
# BEFORE writing and loading the new one.  If both labels are loaded at the
# same time two daemon processes will run simultaneously, causing duplicate
# log ingestion and event duplication on the BFF.
#
# Steps:
#   1. Check whether the legacy label is currently loaded via `launchctl list`.
#   2. If loaded: bootout (modern, macOS 10.11+) or unload (fallback).
#   3. Remove the legacy plist so it is not re-loaded at next login.
#
# All failures are non-fatal (|| true) — a fresh install has no legacy label.
# ---------------------------------------------------------------------------
echo "Checking for legacy launchd job ${PLIST_LABEL_LEGACY}..."
LEGACY_LOADED=0
if [[ -z "${DRY_RUN}" ]] && launchctl list "${PLIST_LABEL_LEGACY}" >/dev/null 2>&1; then
  LEGACY_LOADED=1
fi

if [[ "${LEGACY_LOADED}" -eq 1 ]]; then
  echo "Found legacy daemon label ${PLIST_LABEL_LEGACY} — stopping and unloading..."
  if [[ -z "${DRY_RUN}" ]]; then
    # Prefer bootout (macOS 10.11+) which atomically stops + unregisters the job.
    launchctl bootout "gui/$(id -u)/${PLIST_LABEL_LEGACY}" 2>/dev/null || \
      launchctl unload -w "${PLIST_PATH_LEGACY}" 2>/dev/null || true
  else
    echo "[DRY_RUN] would run: launchctl bootout gui/$(id -u)/${PLIST_LABEL_LEGACY}"
  fi
  echo "Legacy daemon stopped."
fi

# Remove legacy plist (idempotent — ignore if already gone).
if [[ -f "${PLIST_PATH_LEGACY}" ]]; then
  echo "Removing legacy plist: ${PLIST_PATH_LEGACY}"
  rm -f "${PLIST_PATH_LEGACY}"
fi

# ---------------------------------------------------------------------------
# Write the launchd plist under the new label.
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
    <!-- Unique identifier for this launchd job (ADR-022 Phase 2). -->
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
    <string>${HOME}/Library/Logs/vaultmtg-daemon.log</string>
    <key>StandardErrorPath</key>
    <string>${HOME}/Library/Logs/vaultmtg-daemon.log</string>
</dict>
</plist>
PLIST

echo "launchd plist written: ${PLIST_PATH}"

# ---------------------------------------------------------------------------
# Load (and enable) the launchd job.
# -w flag persists the job across reboots by writing to the LaunchAgents DB.
# ---------------------------------------------------------------------------
if [[ -z "${DRY_RUN}" ]]; then
  launchctl load -w "${PLIST_PATH}"
else
  echo "[DRY_RUN] would run: launchctl load -w ${PLIST_PATH}"
fi

echo ""
echo "VaultMTG daemon installed and running."
echo "  Binary : ${INSTALL_DIR}/${BINARY_NAME}"
echo "  Config : ${CONFIG_FILE}"
echo "  plist  : ${PLIST_PATH}"
echo "  Logs   : ${HOME}/Library/Logs/vaultmtg-daemon.log"
echo ""
echo "To change the BFF URL or rotate the auth token, edit: ${CONFIG_FILE}"
