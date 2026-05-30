#!/usr/bin/env bats
# postinstall_test.bats — tests for services/daemon/install/macos/pkg/postinstall
#
# These tests verify the LaunchAgent plist written by postinstall contains all
# required keys, including the env vars introduced in issue #2127.
#
# Strategy:
#   - We produce a test-variant of postinstall using sed that:
#     (a) replaces build-time __PLACEHOLDER__ values with real-looking values
#     (b) overrides PLIST_DIR and CONFIG_DIR to point at BATS_TEST_TMPDIR
#         so we can inspect written files without touching the real ~
#   - OS-level privileged calls (xattr, launchctl, install -o) are stubbed.
#   - SUDO_USER is set to the real invoking user so tilde expansion in
#     "eval echo ~$REAL_USER" resolves to a real path (then overridden).
#
# Run with:
#   bats services/daemon/install/macos/pkg/postinstall_test.bats

POSTINSTALL_SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)/postinstall"
REAL_USER="$(whoami)"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Build a stub directory whose executables replace privileged OS tools.
_make_stub_dir() {
  local stub_dir
  stub_dir="$(mktemp -d)"

  # xattr — no-op (cannot clear quarantine in test)
  cat > "${stub_dir}/xattr" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${stub_dir}/xattr"

  # launchctl — record calls, always succeed
  cat > "${stub_dir}/launchctl" <<'EOF'
#!/usr/bin/env bash
echo "stub-launchctl: $*" >&2
touch "${BATS_TEST_TMPDIR}/launchctl_called"
exit 0
EOF
  chmod +x "${stub_dir}/launchctl"

  # install — strip -o <owner> (requires root); perform operation unprivileged.
  # Supported forms used by postinstall:
  #   install -d -m 755 -o <user> <dir>
  #   install -m 600 -o <user> /dev/null <file>
  #   install -m 644 -o <user> /dev/null <file>
  cat > "${stub_dir}/install" <<'EOF'
#!/usr/bin/env bash
positional=()
skip_next=0
is_dir=0
for arg in "$@"; do
  if [[ "${skip_next}" == "1" ]]; then
    skip_next=0
    continue
  fi
  case "${arg}" in
    -o) skip_next=1 ;;
    -m) skip_next=1 ;;
    -d) is_dir=1 ;;
    *)  positional+=("${arg}") ;;
  esac
done
count="${#positional[@]}"
last_idx=$(( count - 1 ))
target="${positional[${last_idx}]}"
if [[ "${is_dir}" == "1" ]]; then
  mkdir -p "${target}"
else
  mkdir -p "$(dirname "${target}")"
  touch "${target}"
fi
EOF
  chmod +x "${stub_dir}/install"

  # curl — default stub returns a healthy /health response so existing tests
  # (which test plist content, not health-check behaviour) continue to pass.
  # Health-check-specific tests override this stub in their own setup.
  cat > "${stub_dir}/curl" <<'EOF'
#!/usr/bin/env bash
echo '{"status":"ok","account_id":"user_stub","auth_status":"authenticated"}'
EOF
  chmod +x "${stub_dir}/curl"

  # sleep — no-op so tests do not incur the real delay between retries.
  cat > "${stub_dir}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${stub_dir}/sleep"

  # pkill — no-op (no real daemon process to kill in tests)
  cat > "${stub_dir}/pkill" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${stub_dir}/pkill"

  echo "${stub_dir}"
}

# Produce a test-variant of postinstall with:
#   - all __PLACEHOLDER__ values substituted
#   - PLIST_DIR and CONFIG_DIR redirected to BATS_TEST_TMPDIR subdirs
# so the plist and config files are written to an inspectable location.
_make_test_script() {
  local dest="$1"
  local test_dir="$2"
  local cloud_url="${3:-https://staging-api.vaultmtg.app/api/v1}"
  local clerk_api="${4:-https://settled-martin-99.clerk.accounts.dev}"
  local clerk_key="${5:-pk_test_abc123}"
  local clerk_client="${6:-oauth_testclient}"

  local plist_dir="${test_dir}/LaunchAgents"
  local config_dir="${test_dir}/.vaultmtg"

  sed \
    -e "s|__VAULTMTG_CLOUD_API_URL__|${cloud_url}|g" \
    -e "s|__CLERK_FRONTEND_API__|${clerk_api}|g" \
    -e "s|__CLERK_PUBLISHABLE_KEY__|${clerk_key}|g" \
    -e "s|__CLERK_OAUTH_CLIENT_ID__|${clerk_client}|g" \
    -e "s|PLIST_DIR=\"\${REAL_HOME}/Library/LaunchAgents\"|PLIST_DIR=\"${plist_dir}\"|g" \
    -e "s|CONFIG_DIR=\"\${REAL_HOME}/.vaultmtg\"|CONFIG_DIR=\"${config_dir}\"|g" \
    "${POSTINSTALL_SCRIPT}" > "${dest}"
  chmod +x "${dest}"
}

# ---------------------------------------------------------------------------
# Shared setup
# ---------------------------------------------------------------------------

setup() {
  export BATS_TEST_TMPDIR
  TEST_DIR="${BATS_TEST_TMPDIR}/postinstall-$$"
  mkdir -p "${TEST_DIR}"
  TMP_SCRIPT="${BATS_TEST_TMPDIR}/postinstall-subst-$$"
  STUB_DIR="$(_make_stub_dir)"
  _make_test_script "${TMP_SCRIPT}" "${TEST_DIR}"

  PLIST_PATH="${TEST_DIR}/LaunchAgents/com.vaultmtg.daemon.plist"
  CONFIG_FILE="${TEST_DIR}/.vaultmtg/daemon.json"
}

# ---------------------------------------------------------------------------
# 1. Plist contains VAULTMTG_DAEMON_CLOUD_API_URL (issue #2127 + #2564 regression test).
#    The canonical env var name is VAULTMTG_DAEMON_*; the daemon's EnvWithFallback
#    shim still reads MTGA_DAEMON_* for existing legacy installs, but new
#    installs must inject the canonical name (#2564).
# ---------------------------------------------------------------------------
@test "plist: VAULTMTG_DAEMON_CLOUD_API_URL key is present with correct value" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ -f "${PLIST_PATH}" ]

  grep -q "VAULTMTG_DAEMON_CLOUD_API_URL" "${PLIST_PATH}"
  grep -q "staging-api.vaultmtg.app/api/v1" "${PLIST_PATH}"
  # Guard: must not perpetuate the legacy name in new installs (#2564).
  ! grep -q "<key>MTGA_DAEMON_CLOUD_API_URL</key>" "${PLIST_PATH}"
}

# ---------------------------------------------------------------------------
# 2. Plist contains ThrottleInterval (issue #2127 — prevent restart storm)
# ---------------------------------------------------------------------------
@test "plist: ThrottleInterval key is present with value 10" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ -f "${PLIST_PATH}" ]

  grep -q "ThrottleInterval" "${PLIST_PATH}"
  grep -q "<integer>10</integer>" "${PLIST_PATH}"
}

# ---------------------------------------------------------------------------
# 3. Plist contains all Clerk EnvironmentVariables keys
# ---------------------------------------------------------------------------
@test "plist: all Clerk EnvironmentVariables keys are present" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ -f "${PLIST_PATH}" ]

  grep -q "CLERK_FRONTEND_API" "${PLIST_PATH}"
  grep -q "CLERK_PUBLISHABLE_KEY" "${PLIST_PATH}"
  grep -q "CLERK_OAUTH_CLIENT_ID" "${PLIST_PATH}"
}

# ---------------------------------------------------------------------------
# 4. Plist contains KeepAlive=true and RunAtLoad=true
# ---------------------------------------------------------------------------
@test "plist: KeepAlive and RunAtLoad are set to true" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ -f "${PLIST_PATH}" ]

  grep -q "KeepAlive" "${PLIST_PATH}"
  grep -q "RunAtLoad" "${PLIST_PATH}"
}

# ---------------------------------------------------------------------------
# 5. Placeholder validation — script exits 1 when substitution did not happen
# ---------------------------------------------------------------------------
@test "placeholder check: exits 1 when build-time placeholders are not replaced" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${POSTINSTALL_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 1 ]
  [[ "${output}" == *"build-time substitution did not run"* ]]
}

# ---------------------------------------------------------------------------
# 6. daemon.json is written on first install with cloud_api_url
# ---------------------------------------------------------------------------
@test "daemon.json: written on fresh install with cloud_api_url" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ -f "${CONFIG_FILE}" ]
  grep -q "staging-api.vaultmtg.app/api/v1" "${CONFIG_FILE}"
}

# ---------------------------------------------------------------------------
# 7. daemon.json: same-env reinstall — existing fields preserved, no overwrite
#
#    When the existing cloud_api_url matches the baked-in value (same-env
#    reinstall), the config is not overwritten and other fields are retained.
# ---------------------------------------------------------------------------
@test "daemon.json: same-env reinstall preserves existing fields" {
  # Pre-create config with the SAME URL that TMP_SCRIPT bakes in so this is a
  # same-env reinstall (no mismatch).  Extra fields must be preserved.
  # _make_test_script defaults to https://staging-api.vaultmtg.app/api/v1.
  mkdir -p "${TEST_DIR}/.vaultmtg"
  python3 -c "
import json
print(json.dumps({
    'cloud_api_url': 'https://staging-api.vaultmtg.app/api/v1',
    'api_key': 'my-existing-key',
    'sync_enabled': True
}, indent=2))
" > "${CONFIG_FILE}"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ -f "${CONFIG_FILE}" ]

  # cloud_api_url is unchanged (same-env path).
  python3 -c "
import json
with open('${CONFIG_FILE}') as f:
    d = json.load(f)
assert d['cloud_api_url'] == 'https://staging-api.vaultmtg.app/api/v1', \
    'FAIL: cloud_api_url changed on same-env reinstall'
assert d.get('api_key') == 'my-existing-key', \
    'FAIL: api_key not preserved: ' + repr(d.get('api_key'))
assert d.get('sync_enabled') == True, \
    'FAIL: sync_enabled not preserved'
print('PASS: same-env reinstall preserved all fields')
"
  [[ "${output}" == *"cloud_api_url unchanged"* ]]
}

# ---------------------------------------------------------------------------
# 8. Reinstall: bootout is attempted before bootstrap (stop before reload)
# ---------------------------------------------------------------------------
@test "reinstall: bootout is called before bootstrap on reinstall" {
  # Run the script twice in sequence to simulate a reinstall.
  # After both runs, launchctl should have been called at least twice:
  # once for bootout and once for bootstrap. We verify the stub was invoked
  # and that both "bootout" and "bootstrap" appear in its call log.

  local call_log="${BATS_TEST_TMPDIR}/launchctl_calls"

  # Override the launchctl stub to log every invocation with its arguments.
  cat > "${STUB_DIR}/launchctl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >> "${BATS_TEST_TMPDIR}/launchctl_calls"
exit 0
EOF
  chmod +x "${STUB_DIR}/launchctl"

  # First install
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"
  [ "${status}" -eq 0 ]

  # Second install (reinstall)
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"
  [ "${status}" -eq 0 ]

  # The call log must contain both "bootout" and "bootstrap" invocations.
  grep -q "bootout" "${call_log}"
  grep -q "bootstrap" "${call_log}"

  # bootout must appear before the final bootstrap in the log.
  local bootout_line bootstrap_line
  bootout_line=$(grep -n "bootout" "${call_log}" | tail -1 | cut -d: -f1)
  bootstrap_line=$(grep -n "bootstrap" "${call_log}" | tail -1 | cut -d: -f1)
  [ "${bootout_line}" -lt "${bootstrap_line}" ]
}

# ---------------------------------------------------------------------------
# 9. Postinstall echoes the uninstall path using the SHARE_DIR constant
# ---------------------------------------------------------------------------
@test "postinstall: output contains uninstall echo referencing /usr/local/share/vaultmtg/uninstall.sh" {
  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [[ "${output}" == *"To uninstall: sudo /usr/local/share/vaultmtg/uninstall.sh"* ]]
}

# ---------------------------------------------------------------------------
# Health-check tests (issue #2131) — verify poll_daemon_health behaviour.
#
# Strategy: add a curl stub to STUB_DIR that echoes a configurable JSON body
# or simulates a timeout.  We override STUB_DIR's curl for each test.
# ---------------------------------------------------------------------------

# 10. Health-check passes when daemon responds with a non-empty account_id.
@test "health check: exits 0 when daemon responds with non-empty account_id" {
  # curl stub returns a healthy JSON body immediately.
  cat > "${STUB_DIR}/curl" <<'EOF'
#!/usr/bin/env bash
echo '{"status":"ok","account_id":"user_abc123","auth_status":"authenticated"}'
EOF
  chmod +x "${STUB_DIR}/curl"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [[ "${output}" == *"daemon healthy"* ]]
  [[ "${output}" == *"post-install health check passed"* ]]
}

# 11. Health-check fails when daemon never responds (curl always fails).
@test "health check: exits 1 when daemon never responds within retry limit" {
  # curl stub always exits non-zero (connection refused simulation).
  # Also stub sleep so the test does not actually wait 15 s.
  cat > "${STUB_DIR}/curl" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
  chmod +x "${STUB_DIR}/curl"

  cat > "${STUB_DIR}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${STUB_DIR}/sleep"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 1 ]
  [[ "${output}" == *"daemon did not respond"* ]]
}

# 12. Health-check fails when daemon responds but account_id is empty.
@test "health check: exits 1 when daemon responds but account_id is empty" {
  # curl stub returns a JSON body without account_id (setup_required state).
  cat > "${STUB_DIR}/curl" <<'EOF'
#!/usr/bin/env bash
echo '{"status":"ok","auth_status":"setup_required"}'
EOF
  chmod +x "${STUB_DIR}/curl"

  cat > "${STUB_DIR}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${STUB_DIR}/sleep"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 1 ]
  [[ "${output}" == *"daemon did not respond"* ]]
}

# 13. Health-check retries the correct number of times before giving up.
@test "health check: makes exactly HEALTH_MAX_ATTEMPTS curl calls before failing" {
  local call_count_file="${BATS_TEST_TMPDIR}/curl_calls"

  cat > "${STUB_DIR}/curl" <<EOF
#!/usr/bin/env bash
count=\$(cat "${call_count_file}" 2>/dev/null || echo 0)
count=\$(( count + 1 ))
echo "\$count" > "${call_count_file}"
exit 1
EOF
  chmod +x "${STUB_DIR}/curl"

  cat > "${STUB_DIR}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${STUB_DIR}/sleep"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  [ "${status}" -eq 1 ]
  local calls
  calls=$(cat "${call_count_file}")
  echo "curl call count: ${calls}"
  [ "${calls}" -eq 5 ]
}

# ---------------------------------------------------------------------------
# Tests for cross-env reinstall guard (vault-mtg-tickets#194)
# ---------------------------------------------------------------------------

# 14. Cross-env reinstall: when existing cloud_api_url differs from the baked-in
#     value, the keychain entry is cleared and cloud_api_url is updated in-place
#     while other fields (api_key, sync_enabled) are preserved.
@test "cross-env reinstall: keychain cleared and cloud_api_url updated when URL changes" {
  local security_calls="${BATS_TEST_TMPDIR}/security_calls_14"

  # Override security stub to record calls.
  cat > "${STUB_DIR}/security" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "${security_calls}"
exit 0
EOF
  chmod +x "${STUB_DIR}/security"

  # Pre-create config with an OLD URL and extra fields that must be preserved.
  mkdir -p "${TEST_DIR}/.vaultmtg"
  python3 -c "
import json
data = {
  'cloud_api_url': 'https://old-staging-api.example.com/api/v1',
  'api_key': 'my-existing-api-key',
  'sync_enabled': True,
  'account_id': 'user_abc123'
}
print(json.dumps(data, indent=2))
" > "${CONFIG_FILE}"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]

  # Security delete-generic-password must have been called (keychain cleared).
  [ -f "${security_calls}" ]
  grep -q "delete-generic-password" "${security_calls}"
  grep -q "com.vaultmtg.daemon" "${security_calls}"
  grep -q "api-key" "${security_calls}"

  # cloud_api_url must now be the new (baked-in) value.
  python3 -c "
import json
with open('${CONFIG_FILE}') as f:
    d = json.load(f)
assert d['cloud_api_url'] == 'https://staging-api.vaultmtg.app/api/v1', \
    'FAIL: cloud_api_url not updated: ' + repr(d['cloud_api_url'])
print('PASS: cloud_api_url updated to new value')
"

  # Other fields must be preserved by name.
  python3 -c "
import json
with open('${CONFIG_FILE}') as f:
    d = json.load(f)
assert d.get('api_key') == 'my-existing-api-key', \
    'FAIL: api_key changed: ' + repr(d.get('api_key'))
assert d.get('sync_enabled') == True, \
    'FAIL: sync_enabled changed: ' + repr(d.get('sync_enabled'))
assert d.get('account_id') == 'user_abc123', \
    'FAIL: account_id changed: ' + repr(d.get('account_id'))
print('PASS: api_key, sync_enabled, account_id all preserved after cross-env reinstall')
"

  [[ "${output}" == *"cross-env reinstall detected"* ]]
  [[ "${output}" == *"keychain entry cleared"* ]]
  [[ "${output}" == *"cloud_api_url updated"* ]]
}

# 15. Same-env reinstall: when cloud_api_url is already correct, keychain is
#     NOT cleared and no fields are modified.
#
# 16. ADR-011-C guard: same-env reinstall leaves daemon.json byte-for-byte
#     identical — postinstall must not call json.dump or any write on the
#     same-env path.  SHA256 comparison is the authoritative check.
@test "ADR-011-C: same-env reinstall is a daemon.json byte-exact no-op" {
  # Pre-create config with the same URL TMP_SCRIPT bakes in so this is a
  # same-env reinstall.  Use a fixed JSON string so the SHA is deterministic.
  mkdir -p "${TEST_DIR}/.vaultmtg"
  python3 -c "
import json
# Produce compact, deterministic JSON matching what a previous install wrote.
data = {
    'cloud_api_url': 'https://staging-api.vaultmtg.app/api/v1',
    'api_key': 'preserve-me',
    'account_id': 'user_adr011c',
    'sync_enabled': True
}
print(json.dumps(data, indent=2))
" > "${CONFIG_FILE}"

  local sha_before
  sha_before="$(shasum -a 256 "${CONFIG_FILE}" | cut -d' ' -f1)"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]

  local sha_after
  sha_after="$(shasum -a 256 "${CONFIG_FILE}" | cut -d' ' -f1)"

  if [ "${sha_before}" != "${sha_after}" ]; then
    echo "FAIL: daemon.json SHA256 changed on same-env reinstall (ADR-011-C violated)"
    echo "  before: ${sha_before}"
    echo "  after:  ${sha_after}"
    echo "  file contents:"
    cat "${CONFIG_FILE}"
    false
  fi
  echo "PASS: daemon.json SHA256 unchanged on same-env reinstall (ADR-011-C satisfied)"
  [[ "${output}" == *"cloud_api_url unchanged"* ]]
}

@test "same-env reinstall: keychain NOT cleared when cloud_api_url is unchanged" {
  local security_calls="${BATS_TEST_TMPDIR}/security_calls_15"

  cat > "${STUB_DIR}/security" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "${security_calls}"
exit 0
EOF
  chmod +x "${STUB_DIR}/security"

  # Pre-create config with the SAME URL that TMP_SCRIPT will bake in.
  # _make_test_script uses https://staging-api.vaultmtg.app/api/v1 by default.
  mkdir -p "${TEST_DIR}/.vaultmtg"
  python3 -c "
import json
data = {
  'cloud_api_url': 'https://staging-api.vaultmtg.app/api/v1',
  'api_key': 'original-key',
  'sync_enabled': False
}
print(json.dumps(data, indent=2))
" > "${CONFIG_FILE}"
  local original_content
  original_content="$(cat "${CONFIG_FILE}")"

  run env \
    PATH="${STUB_DIR}:${PATH}" \
    SUDO_USER="${REAL_USER}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${TMP_SCRIPT}"

  echo "status: ${status}"
  echo "output: ${output}"
  [ "${status}" -eq 0 ]

  # Security must NOT have been called for a keychain delete.
  if [ -f "${security_calls}" ]; then
    run grep "delete-generic-password" "${security_calls}"
    [ "${status}" -ne 0 ]
  fi

  # Config must be unchanged.
  local new_content
  new_content="$(cat "${CONFIG_FILE}")"

  python3 -c "
import json
with open('${CONFIG_FILE}') as f:
    d = json.load(f)
assert d.get('api_key') == 'original-key', \
    'FAIL: api_key changed on same-env reinstall: ' + repr(d.get('api_key'))
assert d.get('sync_enabled') == False, \
    'FAIL: sync_enabled changed on same-env reinstall: ' + repr(d.get('sync_enabled'))
print('PASS: api_key and sync_enabled unchanged on same-env reinstall')
"

  [[ "${output}" == *"cloud_api_url unchanged"* ]]
}
