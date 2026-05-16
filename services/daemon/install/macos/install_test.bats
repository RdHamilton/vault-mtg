#!/usr/bin/env bats
# install_test.bats — unit tests for services/daemon/install/macos/install.sh
#
# Run with:
#   bats services/daemon/install/macos/install_test.bats
#
# All tests use stubs so they never download anything, call sudo, or touch
# launchctl.  The DRY_RUN=1 guard (added in the same PR) prevents real
# system mutations even if a stub is missed.

INSTALL_SH="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)/install.sh"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Create a minimal stub directory that is prepended to PATH.
# Stubs replace curl, sudo, launchctl, and read so the script can run
# non-interactively in CI without any network or privilege access.
_make_stub_dir() {
  local stub_dir
  stub_dir="$(mktemp -d)"

  # curl — write an empty file so TMP_BIN is created but empty
  cat > "${stub_dir}/curl" <<'EOF'
#!/usr/bin/env bash
# Absorb all flags; write empty content to -o <file> if present.
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) shift; touch "$1" ;;
  esac
  shift
done
EOF
  chmod +x "${stub_dir}/curl"

  # sudo — record that it was called; do NOT execute the real command
  cat > "${stub_dir}/sudo" <<'EOF'
#!/usr/bin/env bash
echo "stub-sudo: $*" >&2
SUDO_CALLED_FILE="${BATS_TEST_TMPDIR}/sudo_called"
echo 1 > "${SUDO_CALLED_FILE}"
EOF
  chmod +x "${stub_dir}/sudo"

  # launchctl — record that it was called
  cat > "${stub_dir}/launchctl" <<'EOF'
#!/usr/bin/env bash
echo "stub-launchctl: $*" >&2
LAUNCHCTL_CALLED_FILE="${BATS_TEST_TMPDIR}/launchctl_called"
echo 1 > "${LAUNCHCTL_CALLED_FILE}"
EOF
  chmod +x "${stub_dir}/launchctl"

  echo "${stub_dir}"
}

# ---------------------------------------------------------------------------
# 1. Architecture detection — arm64
# ---------------------------------------------------------------------------
@test "arch detection: arm64 maps to darwin-arm64 asset suffix" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  # Override uname so the script sees arm64
  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "arm64"
EOF
  chmod +x "${stub_dir}/uname"

  # Provide a pre-set RELEASE_TAG so the script skips the GitHub API call.
  # HOME is pointed at a temp dir so no real files are touched.
  local fake_home
  fake_home="$(mktemp -d)"

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    bash "${INSTALL_SH}" <<< $'https://api.example.com\nfake-token\n'

  echo "output: ${output}"
  [[ "${output}" == *"darwin-arm64"* ]]
}

# ---------------------------------------------------------------------------
# 2. Architecture detection — x86_64
# ---------------------------------------------------------------------------
@test "arch detection: x86_64 maps to darwin-amd64 asset suffix" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "x86_64"
EOF
  chmod +x "${stub_dir}/uname"

  local fake_home
  fake_home="$(mktemp -d)"

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    bash "${INSTALL_SH}" <<< $'https://api.example.com\nfake-token\n'

  echo "output: ${output}"
  [[ "${output}" == *"darwin-amd64"* ]]
}

# ---------------------------------------------------------------------------
# 3. Architecture detection — unknown arch exits 1
# ---------------------------------------------------------------------------
@test "arch detection: unknown architecture exits with status 1" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "riscv64"
EOF
  chmod +x "${stub_dir}/uname"

  local fake_home
  fake_home="$(mktemp -d)"

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    bash "${INSTALL_SH}"

  [ "${status}" -eq 1 ]
  [[ "${output}" == *"Unsupported architecture"* ]]
}

# ---------------------------------------------------------------------------
# 4. Asset name construction — download URL contains correct asset name
# ---------------------------------------------------------------------------
@test "asset name: RELEASE_TAG=daemon/v0.1.0 on arm64 produces correct download URL" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  # Capture the URL curl is called with
  cat > "${stub_dir}/curl" <<'EOF'
#!/usr/bin/env bash
# Print every arg so we can inspect the URL in the test output.
echo "stub-curl-args: $*" >&2
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) shift; touch "$1" ;;
  esac
  shift
done
EOF
  chmod +x "${stub_dir}/curl"

  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "arm64"
EOF
  chmod +x "${stub_dir}/uname"

  local fake_home
  fake_home="$(mktemp -d)"

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    bash "${INSTALL_SH}" <<< $'https://api.example.com\nfake-token\n'

  echo "output: ${output}"
  # The download URL must contain the versioned tag and correct asset suffix
  [[ "${output}" == *"daemon/v0.1.0"* ]]
  [[ "${output}" == *"mtga-companion-daemon-darwin-arm64"* ]]
}

# ---------------------------------------------------------------------------
# 5. jq fallback — python3 fallback produces valid JSON with required keys
# ---------------------------------------------------------------------------
@test "jq fallback: python3 fallback writes JSON with cloud_api_url and api_key" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "arm64"
EOF
  chmod +x "${stub_dir}/uname"

  # Remove jq from the stub dir entirely so command -v jq returns false
  # and the script falls through to the python3 path.
  # (A stub that exits 127 is still found by command -v, causing set -e to
  # abort the script before the fallback branch is reached.)
  rm -f "${stub_dir}/jq"

  local fake_home
  fake_home="$(mktemp -d)"

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    bash "${INSTALL_SH}" <<< $'https://api.example.com\nmy-secret-token\n'

  echo "output: ${output}"
  [ "${status}" -eq 0 ]

  local config_file="${fake_home}/.mtga-companion/daemon.json"
  [ -f "${config_file}" ]

  # The config must contain both required keys
  python3 -c "
import json, sys
with open('${config_file}') as f:
    data = json.load(f)
assert 'cloud_api_url' in data, 'missing cloud_api_url'
assert 'api_key' in data, 'missing api_key'
assert data['cloud_api_url'] == 'https://api.example.com', 'wrong url'
assert data['api_key'] == 'my-secret-token', 'wrong api_key'
"
}

# ---------------------------------------------------------------------------
# 6. Idempotency — existing config is not overwritten
# ---------------------------------------------------------------------------
@test "idempotency: existing config file is not overwritten" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "arm64"
EOF
  chmod +x "${stub_dir}/uname"

  local fake_home
  fake_home="$(mktemp -d)"

  # Pre-populate the config so the script should skip the prompt
  local config_dir="${fake_home}/.mtga-companion"
  mkdir -p "${config_dir}"
  local config_file="${config_dir}/daemon.json"
  echo '{"cloud_api_url":"https://original.example.com","api_key":"original-token"}' > "${config_file}"
  local original_content
  original_content="$(cat "${config_file}")"

  # stdin is /dev/null — if the script prompts, it will fail because there is
  # no input to read, causing the test to surface a bug immediately.
  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    bash "${INSTALL_SH}" < /dev/null

  echo "output: ${output}"
  [ "${status}" -eq 0 ]

  # Config content must be unchanged
  local new_content
  new_content="$(cat "${config_file}")"
  [ "${new_content}" = "${original_content}" ]

  # Script must report the skip message
  [[ "${output}" == *"Config already exists, skipping"* ]]
}

# ---------------------------------------------------------------------------
# 7. DRY_RUN mode — sudo and launchctl are NOT called when DRY_RUN=1
# ---------------------------------------------------------------------------
@test "DRY_RUN=1: sudo and launchctl are not invoked" {
  local stub_dir
  stub_dir="$(_make_stub_dir)"

  cat > "${stub_dir}/uname" <<'EOF'
#!/usr/bin/env bash
echo "arm64"
EOF
  chmod +x "${stub_dir}/uname"

  local fake_home
  fake_home="$(mktemp -d)"

  # Use BATS_TEST_TMPDIR so stub scripts can write sentinel files
  export BATS_TEST_TMPDIR

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    RELEASE_TAG="daemon/v0.1.0" \
    DRY_RUN=1 \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${INSTALL_SH}" <<< $'https://api.example.com\nfake-token\n'

  echo "output: ${output}"
  [ "${status}" -eq 0 ]

  # Sentinel files must NOT exist (stub sudo/launchctl write them when called)
  [ ! -f "${BATS_TEST_TMPDIR}/sudo_called" ]
  [ ! -f "${BATS_TEST_TMPDIR}/launchctl_called" ]

  # DRY_RUN output messages must be present
  [[ "${output}" == *"[DRY_RUN] would install binary"* ]]
  [[ "${output}" == *"[DRY_RUN] would run: launchctl"* ]]
}
