#!/usr/bin/env bats
# uninstall_test.bats — regression tests for services/daemon/install/macos/uninstall.sh
#
# Run with:
#   bats services/daemon/install/macos/uninstall_test.bats
#
# These tests pin the regression fixed by #2143 (uninstall.sh was scoped to the
# pre-rebrand binary name `mtga-companion-daemon` and silently skipped binary
# removal). They cover:
#
#   1. Binary removal at INSTALL_DIR/vaultmtg-daemon
#   2. Idempotent uninstall (run twice → same end state)
#   3. Current plist (com.vaultmtg.daemon) removal
#   4. Legacy plist (com.mtga-companion.daemon) removal (ADR-022 upgrader path)
#   5. Exit-zero / no-op when nothing is installed
#   6. Defensive: log file is NOT removed (covers sibling #2144)
#   7. Defensive: config dir is NOT removed (covers sibling #2145)
#   8. INSTALL_DIR env override (#2143 plan — single-source-of-truth, ADR-036 I-4)
#
# All tests stub `sudo` and `launchctl` so they never require privileges or
# touch real launchd state. HOME is pointed at a per-test temp dir so plist
# and log path assertions are hermetic.

UNINSTALL_SH="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)/uninstall.sh"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Create a minimal stub directory that is prepended to PATH.
# Stubs replace sudo and launchctl so the script can run non-interactively
# without any privilege escalation or real launchd mutations.
#
# The sudo stub is the load-bearing piece: uninstall.sh removes the binary
# via `sudo rm -f`. The stub forwards to the real `rm` so the on-disk binary
# is actually deleted under HOME (which is a temp dir per test) — that lets
# us assert the post-state directly.
_make_stub_dir() {
  local stub_dir
  stub_dir="$(mktemp -d)"

  # sudo — forward `sudo rm <args>` to plain `rm <args>` so test fixtures
  # under HOME can be removed without elevation. Record invocation for the
  # tests that need to assert sudo was reached.
  cat > "${stub_dir}/sudo" <<'EOF'
#!/usr/bin/env bash
echo "stub-sudo: $*" >&2
if [[ -n "${BATS_TEST_TMPDIR:-}" ]]; then
  echo 1 > "${BATS_TEST_TMPDIR}/sudo_called"
fi
# Forward to the real command (skip the "sudo" arg).
exec "$@"
EOF
  chmod +x "${stub_dir}/sudo"

  # launchctl — record that it was called; never call the real launchctl.
  # `launchctl list <label>` is also used by the script as a probe — exit 1
  # so the "label loaded but plist gone" branch is not taken by default.
  cat > "${stub_dir}/launchctl" <<'EOF'
#!/usr/bin/env bash
echo "stub-launchctl: $*" >&2
if [[ -n "${BATS_TEST_TMPDIR:-}" ]]; then
  echo 1 > "${BATS_TEST_TMPDIR}/launchctl_called"
fi
# `launchctl list <label>` — return 1 so the "loaded but plist gone" path
# is only reached when a test explicitly overrides this stub.
if [[ "$1" == "list" ]]; then
  exit 1
fi
exit 0
EOF
  chmod +x "${stub_dir}/launchctl"

  echo "${stub_dir}"
}

# Create a fake HOME with optional fixtures.
#   $1 — install_dir (binary lives at <install_dir>/vaultmtg-daemon)
#   $2 — "with-binary" | "no-binary"
#   $3 — "with-current-plist" | "no-current-plist"
#   $4 — "with-legacy-plist" | "no-legacy-plist"
#   $5 — "with-log" | "no-log"
#   $6 — "with-config" | "no-config"
_make_fake_home() {
  local install_dir="$1"
  local binary_flag="$2"
  local current_plist_flag="$3"
  local legacy_plist_flag="$4"
  local log_flag="$5"
  local config_flag="$6"

  local fake_home
  fake_home="$(mktemp -d)"

  mkdir -p "${install_dir}"
  if [[ "${binary_flag}" == "with-binary" ]]; then
    echo "fake binary" > "${install_dir}/vaultmtg-daemon"
    chmod +x "${install_dir}/vaultmtg-daemon"
  fi

  mkdir -p "${fake_home}/Library/LaunchAgents"
  if [[ "${current_plist_flag}" == "with-current-plist" ]]; then
    echo "<plist>current</plist>" > "${fake_home}/Library/LaunchAgents/com.vaultmtg.daemon.plist"
  fi
  if [[ "${legacy_plist_flag}" == "with-legacy-plist" ]]; then
    echo "<plist>legacy</plist>" > "${fake_home}/Library/LaunchAgents/com.mtga-companion.daemon.plist"
  fi

  mkdir -p "${fake_home}/Library/Logs"
  if [[ "${log_flag}" == "with-log" ]]; then
    echo "fake log content" > "${fake_home}/Library/Logs/vaultmtg-daemon.log"
  fi

  if [[ "${config_flag}" == "with-config" ]]; then
    mkdir -p "${fake_home}/.vaultmtg"
    echo '{"cloud_api_url":"https://api.example.com","api_key":"token"}' > "${fake_home}/.vaultmtg/daemon.json"
  fi

  echo "${fake_home}"
}

# ---------------------------------------------------------------------------
# 1. Binary removal — the regression fixed by #2143
# ---------------------------------------------------------------------------
@test "binary removal: vaultmtg-daemon is deleted from INSTALL_DIR" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    with-binary no-current-plist no-legacy-plist no-log no-config)"

  [ -f "${install_dir}/vaultmtg-daemon" ]

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ ! -f "${install_dir}/vaultmtg-daemon" ]
  [[ "${output}" == *"Removing binary"* ]]
  [[ "${output}" == *"vaultmtg-daemon"* ]]
}

# ---------------------------------------------------------------------------
# 2. Idempotency — running uninstall twice produces the same end state
# ---------------------------------------------------------------------------
@test "idempotency: two consecutive runs leave the same end state and exit 0" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    with-binary with-current-plist with-legacy-plist no-log no-config)"

  # First run — full uninstall.
  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"
  echo "first-output: ${output}"
  [ "${status}" -eq 0 ]
  [ ! -f "${install_dir}/vaultmtg-daemon" ]
  [ ! -f "${fake_home}/Library/LaunchAgents/com.vaultmtg.daemon.plist" ]
  [ ! -f "${fake_home}/Library/LaunchAgents/com.mtga-companion.daemon.plist" ]

  # Second run — must be a clean no-op, no errors, status 0.
  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"
  echo "second-output: ${output}"
  [ "${status}" -eq 0 ]
  [ ! -f "${install_dir}/vaultmtg-daemon" ]
  [ ! -f "${fake_home}/Library/LaunchAgents/com.vaultmtg.daemon.plist" ]
  [ ! -f "${fake_home}/Library/LaunchAgents/com.mtga-companion.daemon.plist" ]
  # Second run should report skips for the missing artifacts.
  [[ "${output}" == *"skipping"* ]]
}

# ---------------------------------------------------------------------------
# 3. Current plist removal — com.vaultmtg.daemon.plist is deleted
# ---------------------------------------------------------------------------
@test "current plist: com.vaultmtg.daemon.plist is removed from ~/Library/LaunchAgents" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    no-binary with-current-plist no-legacy-plist no-log no-config)"

  local plist="${fake_home}/Library/LaunchAgents/com.vaultmtg.daemon.plist"
  [ -f "${plist}" ]

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ ! -f "${plist}" ]
  [[ "${output}" == *"com.vaultmtg.daemon"* ]]
  [[ "${output}" == *"Removing plist"* ]]
}

# ---------------------------------------------------------------------------
# 4. Legacy plist removal — com.mtga-companion.daemon.plist is deleted
# (ADR-022 upgrader path: user installed pre-rename, then uninstalls post-rename)
# ---------------------------------------------------------------------------
@test "legacy plist: com.mtga-companion.daemon.plist is removed (ADR-022 upgrader path)" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    no-binary no-current-plist with-legacy-plist no-log no-config)"

  local legacy_plist="${fake_home}/Library/LaunchAgents/com.mtga-companion.daemon.plist"
  [ -f "${legacy_plist}" ]

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [ ! -f "${legacy_plist}" ]
  [[ "${output}" == *"legacy plist"* ]] || [[ "${output}" == *"Found legacy plist"* ]]
  [[ "${output}" == *"Legacy launchd job removed"* ]]
}

# ---------------------------------------------------------------------------
# 5. No-op uninstall — exit 0 when nothing is installed
# ---------------------------------------------------------------------------
@test "no-op: exits 0 cleanly when binary and plists are all absent" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    no-binary no-current-plist no-legacy-plist no-log no-config)"

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  [[ "${output}" == *"Binary not found"* ]]
  [[ "${output}" == *"Plist not found"* ]]
  [[ "${output}" == *"VaultMTG daemon uninstalled"* ]]

  # No sudo should have been invoked — nothing to elevate for.
  [ ! -f "${BATS_TEST_TMPDIR}/sudo_called" ]
}

# ---------------------------------------------------------------------------
# 6. Defensive: log file is NOT removed (intentional — sibling ticket #2144)
# ---------------------------------------------------------------------------
@test "log preservation: ~/Library/Logs/vaultmtg-daemon.log is preserved (not removed)" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    with-binary with-current-plist no-legacy-plist with-log no-config)"

  local log_path="${fake_home}/Library/Logs/vaultmtg-daemon.log"
  [ -f "${log_path}" ]

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  # Log file MUST still exist after uninstall.
  [ -f "${log_path}" ]
  # Script must call out the preserved log path so the user knows to clean it manually.
  [[ "${output}" == *"Log file"* ]]
  [[ "${output}" == *"NOT removed"* ]]
  # Pin the exact filename (#2144): message must reference vaultmtg-daemon.log,
  # not the pre-rebrand name mtga-companion-daemon.log.
  [[ "${output}" == *"vaultmtg-daemon.log"* ]]
}

# ---------------------------------------------------------------------------
# 7. Defensive: config dir is NOT removed (intentional — sibling ticket #2145)
# ---------------------------------------------------------------------------
@test "config preservation: ~/.vaultmtg/daemon.json is preserved (not removed)" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  local install_dir; install_dir="$(mktemp -d)"
  local fake_home; fake_home="$(_make_fake_home "${install_dir}" \
    with-binary with-current-plist no-legacy-plist no-log with-config)"

  local config_file="${fake_home}/.vaultmtg/daemon.json"
  local config_dir="${fake_home}/.vaultmtg"
  [ -f "${config_file}" ]

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  # Config file AND its parent dir must still exist.
  [ -f "${config_file}" ]
  [ -d "${config_dir}" ]
  # Script must call out the preserved config path.
  [[ "${output}" == *"Config file"* ]]
  [[ "${output}" == *"NOT removed"* ]]
}

# ---------------------------------------------------------------------------
# 8. INSTALL_DIR env override — confirms the single-line override added in #2143
# (ADR-036 Invariant I-4: single source of truth — env override is consistent
# with install.sh:22 DRY_RUN convention)
# ---------------------------------------------------------------------------
@test "INSTALL_DIR env override: custom install dir is honored" {
  local stub_dir; stub_dir="$(_make_stub_dir)"
  # Use a non-default install dir to prove the override is wired up.
  local custom_install_dir; custom_install_dir="$(mktemp -d)/opt/bin"
  mkdir -p "${custom_install_dir}"
  local fake_home; fake_home="$(_make_fake_home "${custom_install_dir}" \
    with-binary no-current-plist no-legacy-plist no-log no-config)"

  [ -f "${custom_install_dir}/vaultmtg-daemon" ]

  run env \
    PATH="${stub_dir}:${PATH}" \
    HOME="${fake_home}" \
    INSTALL_DIR="${custom_install_dir}" \
    BATS_TEST_TMPDIR="${BATS_TEST_TMPDIR}" \
    bash "${UNINSTALL_SH}"

  echo "output: ${output}"
  [ "${status}" -eq 0 ]
  # The binary at the custom path must be gone.
  [ ! -f "${custom_install_dir}/vaultmtg-daemon" ]
  # The output must reference the custom path, not /usr/local/bin.
  [[ "${output}" == *"${custom_install_dir}/vaultmtg-daemon"* ]]
  [[ "${output}" != *"/usr/local/bin/vaultmtg-daemon"* ]]
}
