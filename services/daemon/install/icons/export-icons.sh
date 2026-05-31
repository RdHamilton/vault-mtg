#!/usr/bin/env bash
# export-icons.sh — Reproducibly regenerate vaultmtg.icns, vaultmtg.ico, and the
# tray icon from the canonical SVG master in vault-mtg-docs.
#
# Usage:
#   SVG_PATH=<path/to/logo-vaultmtg-app-icon.svg> bash export-icons.sh
#
# Or, if vault-mtg-docs is cloned alongside vault-mtg:
#   bash export-icons.sh   # uses default SVG_PATH below
#
# Requirements:
#   - rsvg-convert  (librsvg — brew install librsvg)
#   - iconutil      (macOS built-in, /usr/bin/iconutil)
#   - magick        (ImageMagick 7 — brew install imagemagick)
#
# Outputs (relative to this script's directory):
#   vaultmtg.icns                            macOS app icon (full iconset 16→512@2x)
#   vaultmtg.ico                             Windows multi-res icon (16/32/48/256 px)
#   ../../internal/tray/assets/icon.png      Tray icon (32×32 RGBA PNG)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../../.." && pwd)"

# Default SVG location: vault-mtg-docs cloned alongside vault-mtg.
DOCS_ASSETS="${REPO_ROOT}/../vault-mtg-docs/engineering/design/rebranding/Ray Hamilton Engineering Design System/assets"
DEFAULT_SVG="${DOCS_ASSETS}/logo-vaultmtg-app-icon.svg"
SVG_PATH="${SVG_PATH:-${DEFAULT_SVG}}"

if [[ ! -f "${SVG_PATH}" ]]; then
  echo "ERROR: SVG not found at: ${SVG_PATH}" >&2
  echo "Set SVG_PATH= to the canonical logo-vaultmtg-app-icon.svg." >&2
  exit 1
fi

# Verify required tools.
for tool in rsvg-convert iconutil magick; do
  if ! command -v "${tool}" &>/dev/null; then
    echo "ERROR: ${tool} not found in PATH." >&2
    echo "  rsvg-convert: brew install librsvg" >&2
    echo "  iconutil:     macOS built-in (/usr/bin/iconutil)" >&2
    echo "  magick:       brew install imagemagick" >&2
    exit 1
  fi
done

echo "[export-icons] SVG source: ${SVG_PATH}"

# ---------------------------------------------------------------------------
# macOS .icns — full iconset (16→512 plus @2x retina variants)
# ---------------------------------------------------------------------------
ICONSET_DIR="$(mktemp -d)/vaultmtg.iconset"
mkdir -p "${ICONSET_DIR}"

echo "[export-icons] generating .icns iconset PNGs ..."
for size in 16 32 128 256 512; do
  rsvg-convert -w "${size}"         -h "${size}"         "${SVG_PATH}" -o "${ICONSET_DIR}/icon_${size}x${size}.png"
  rsvg-convert -w "$((size * 2))"   -h "$((size * 2))"   "${SVG_PATH}" -o "${ICONSET_DIR}/icon_${size}x${size}@2x.png"
done

iconutil -c icns "${ICONSET_DIR}" -o "${SCRIPT_DIR}/vaultmtg.icns"
echo "[export-icons] vaultmtg.icns written ($(du -sh "${SCRIPT_DIR}/vaultmtg.icns" | cut -f1))"

# ---------------------------------------------------------------------------
# Windows .ico — multi-resolution (16 / 32 / 48 / 256 px)
# ---------------------------------------------------------------------------
echo "[export-icons] generating .ico PNGs ..."
ICO_TMP="$(mktemp -d)"
for size in 16 32 48 256; do
  rsvg-convert -w "${size}" -h "${size}" "${SVG_PATH}" -o "${ICO_TMP}/icon_${size}.png"
done

magick \
  "${ICO_TMP}/icon_16.png" \
  "${ICO_TMP}/icon_32.png" \
  "${ICO_TMP}/icon_48.png" \
  "${ICO_TMP}/icon_256.png" \
  "${SCRIPT_DIR}/vaultmtg.ico"
echo "[export-icons] vaultmtg.ico written ($(du -sh "${SCRIPT_DIR}/vaultmtg.ico" | cut -f1))"

# ---------------------------------------------------------------------------
# Tray icon — 32×32 RGBA PNG (systray; used by the CGO tray build)
# ---------------------------------------------------------------------------
TRAY_ASSET="${SCRIPT_DIR}/../../internal/tray/assets/icon.png"
echo "[export-icons] generating tray icon.png (32×32) ..."
rsvg-convert -w 32 -h 32 "${SVG_PATH}" -o "${TRAY_ASSET}"
echo "[export-icons] icon.png written: ${TRAY_ASSET}"

# Cleanup temp dirs.
rm -rf "$(dirname "${ICONSET_DIR}")" "${ICO_TMP}"

echo "[export-icons] done — Result: SUCCESS"
