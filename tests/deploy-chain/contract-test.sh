#!/usr/bin/env bash
# tests/deploy-chain/contract-test.sh
#
# Deploy-script contract test -- a STATIC analysis that asserts the deploy
# script set is internally consistent.  Catches the class of drift that
# caused v0.3.1 production bugs #2192 (wrong systemd unit name) and #2197
# (credential-free DATABASE_URL with no DB_SECRET_ARN) BEFORE it reaches
# production.
#
# Scope:  pure static checks -- no AWS, no docker, no network.  Runs in
# under 5 seconds on a clean checkout.
#
# Contracts asserted:
#
#   C1  deploy-env.sh is well-formed POSIX shell.
#
#   C2  Every variable that consumer scripts reference (regex-extracted
#       from `. /tmp/deploy-env.sh` sourcing scripts) is defined in
#       infra/config/deploy-env.sh.  No orphan references.
#
#   C3  SSM parameter NAMING consistency:
#         SSM_PROD_*               -> /vaultmtg/app/production/...
#         SSM_STAGING_*            -> /vaultmtg/app/staging/...
#         SSM_VAULTMTG_STAGING_*   -> /vaultmtg/app/staging/...
#
#   C4  DATABASE_URL construction:
#         Production (provision-db-url.sh) -- emits a credential-free
#         postgresql:// URL of the form postgresql://${HOST}:${PORT}/${DB}?${SSL}
#         and pairs it with DB_SECRET_ARN. Runtime SM resolution stays on
#         for production until #2461 migrates it.
#         Staging (provision-staging-env.sh) -- as of #2461 the provisioner
#         splices fresh RDS credentials into DATABASE_URL under its scoped
#         role and writes NO DB_SECRET_ARN, NO BFF_DB_RESOLVE_FROM_SM. This
#         removes the runtime SM dependency on the EC2 instance role.
#
#   C5  DB-credential model symmetry:
#         Production -- any DATABASE_URL writer for /etc/mtga-companion/env
#         (provision-db-url.sh) MUST also write DB_SECRET_ARN (the
#         legacy runtime-resolution path, kept for #2197 protection until
#         the prod-side migration ticket lands).
#         Staging -- provision-staging-env.sh MUST NOT write DB_SECRET_ARN
#         or BFF_DB_RESOLVE_FROM_SM, so the BFF's runtime SM path stays
#         dormant. Inverse of the C5 protection on prod; #2461 contract.
#
#   C6  Workflow <-> deploy-env.sh consistency: every SSM path literal in
#       .github/workflows/*.yml that matches the production/staging app
#       SSM prefix MUST equal the value of the corresponding constant in
#       deploy-env.sh.  No silently-divergent paths.
#
#   C7  Service-name discipline: scripts that systemctl-restart the BFF
#       MUST reference the BFF_SERVICE / BFF_STAGING_SERVICE variables
#       rather than hardcoding the unit name.  This was the #2192 bug.
#
#   C8  Provisioned-env <-> BFF-read symmetry: every KEY= written into a
#       BFF env file by provisioning scripts MUST correspond to an
#       os.Getenv("KEY") call in services/bff/.  (Allowlist below for
#       known infra-only keys like AWS_DEFAULT_REGION.)
#       Reported as a WARNING (non-blocking) because legitimate drift
#       can exist transiently during a rename; hard-failing here would
#       create a flaky gate.  Use the Cross-Component Contract Audit
#       runbook to address findings.
#
#   C9  Sourcing discipline: every consumer script that references a
#       deploy-env.sh constant MUST source deploy-env.sh first (either
#       directly via `. /tmp/deploy-env.sh` or via the local-path
#       fallback used by infra/scripts).
#
# Exit code:  0 if all HARD contracts (C1-C7, C9) hold; non-zero on any
#             violation, with a clear list of failures printed to stderr.

set -uo pipefail

# Allow callers to override the repo root (useful for testing) -- default
# to two levels up from this script's location.
REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

DEPLOY_ENV_SH="${REPO_ROOT}/infra/config/deploy-env.sh"
DEPLOY_SCRIPTS_DIR="${REPO_ROOT}/scripts/deploy"
INFRA_SCRIPTS_DIR="${REPO_ROOT}/infra/scripts"
WORKFLOWS_DIR="${REPO_ROOT}/.github/workflows"
BFF_SOURCE_DIR="${REPO_ROOT}/services/bff"

# ---- counters / output helpers --------------------------------------------
FAILS=0
WARNS=0

# Plain ASCII output -- no colour, no unicode -- so the transcript is safe
# to paste into PR bodies and renders identically in every terminal.
fail() { printf '[FAIL] %s\n' "$*" >&2; FAILS=$((FAILS + 1)); }
warn() { printf '[WARN] %s\n' "$*" >&2; WARNS=$((WARNS + 1)); }
pass() { printf '[ OK ] %s\n' "$*"; }
info() { printf '       %s\n' "$*"; }

# ---- discovery -------------------------------------------------------------
[[ -f "$DEPLOY_ENV_SH" ]] || {
  fail "Canonical config missing: $DEPLOY_ENV_SH"
  exit 1
}

# All consumer scripts that source deploy-env.sh.
# Use a portable while-read loop (mapfile requires bash 4+; macOS ships 3.2).
CONSUMER_SCRIPTS=()
while IFS= read -r line; do
  CONSUMER_SCRIPTS+=("$line")
done < <(
  grep -lE '\. (/tmp/deploy-env\.sh|.*infra/config/deploy-env\.sh)' \
    "$DEPLOY_SCRIPTS_DIR"/*.sh "$INFRA_SCRIPTS_DIR"/*.sh 2>/dev/null | sort -u
)

if [[ ${#CONSUMER_SCRIPTS[@]} -eq 0 ]]; then
  fail "No consumer scripts found that source deploy-env.sh."
  exit 1
fi

printf 'contract-test: scanning %d consumer script(s)\n' "${#CONSUMER_SCRIPTS[@]}"
for s in "${CONSUMER_SCRIPTS[@]}"; do info "- ${s#"$REPO_ROOT"/}"; done
echo

# ---- C1: deploy-env.sh is well-formed POSIX shell --------------------------
echo '== C1: deploy-env.sh syntax =='
if sh -n "$DEPLOY_ENV_SH" 2>/dev/null; then
  pass "deploy-env.sh parses cleanly under sh -n"
else
  fail "deploy-env.sh fails sh -n parse check"
  sh -n "$DEPLOY_ENV_SH" || true
fi
echo

# ---- extract the names defined in deploy-env.sh ---------------------------
# Anything of the form ^IDENT="..." at column 1 (ignoring shell builtins).
DEFINED_VARS=()
while IFS= read -r line; do
  DEFINED_VARS+=("$line")
done < <(
  grep -E '^[A-Z][A-Z0-9_]*=' "$DEPLOY_ENV_SH" \
    | sed -E 's/^([A-Z][A-Z0-9_]*)=.*/\1/' | sort -u
)
DEFINED_SET=" $(printf '%s ' "${DEFINED_VARS[@]}") "

# ---- C2: every ${VAR} or $VAR referenced by consumers is defined ----------
echo '== C2: consumer-referenced variables are defined in deploy-env.sh =='
UNDEFINED_REFS=()
for script in "${CONSUMER_SCRIPTS[@]}"; do
  # Extract all bare-or-braced variable references that match the
  # SHOUTY_SNAKE_CASE convention deploy-env.sh uses.  Excludes obvious
  # script-local locals like REGION, ENV_FILE, ENV_VALUE (they are
  # consumer-internal); cross-references those to the assignment side.
  refs=()
  while IFS= read -r r; do
    refs+=("$r")
  done < <(
    grep -oE '\$\{?[A-Z][A-Z0-9_]*\}?' "$script" \
      | sed -E 's/^\$\{?([A-Z][A-Z0-9_]*)\}?/\1/' \
      | sort -u
  )
  # Guard against an empty refs[] under set -u.
  [[ ${#refs[@]} -eq 0 ]] && continue
  for ref in "${refs[@]}"; do
    case "$ref" in
      # Skip variables that are clearly assigned inside the script itself.
      DEPLOY_BUCKET|DEPLOY_SHA|DEPLOY_REGION) continue;;
      DB_ENDPOINT|DB_NAME|DB_PORT) ;;  # checked separately below; fall through
    esac

    # The script-local case: skip if there is an assignment of the same name
    # in this script.  This avoids false positives on per-script working vars.
    if grep -qE "^[[:space:]]*(local[[:space:]]+|export[[:space:]]+)?${ref}=" "$script"; then
      continue
    fi
    # Skip if the ref is set via builtin (for-loop, read -r, etc.).
    if grep -qE "for[[:space:]]+${ref}[[:space:]]+in" "$script"; then
      continue
    fi
    if grep -qE "read[[:space:]]+[^|]*[[:space:]]${ref}([[:space:]]|\$)" "$script"; then
      continue
    fi
    # Skip positional / well-known shell vars.
    case "$ref" in
      PATH|HOME|PWD|IFS|UID|EUID|HOSTNAME|USER|BASH|BASH_SOURCE|SHELL|TERM|TMPDIR|LANG|LC_ALL|GITHUB_ACTIONS) continue;;
      AWS_REGION|AWS_DEFAULT_REGION|AWS_PROFILE) continue;;  # caller-injected
      ENV_KEY|ENV_VALUE|ENV_FILE|ENV_DIR|SERVICE|UNIT_FILE|SSM_PARAM_NAME|DECRYPT_FLAG) continue;;
      # Script-local working vars used by infra/scripts.
      RAW|STATUS|RC|SUCCEEDED|COMMAND_ID|VALUE|MIGRATIONS_DIR|MIGRATE_VERSION|MIGRATE_DB_URL|SECRET_ARN|SECRET_JSON|MASTER_USER|ENC_PASS|STAGING_PASSWORD|STAGING_DB_URL|PROFILE|REGION|TRUNCATE_ALL) continue;;
    esac

    # Allowed if defined in deploy-env.sh.
    if [[ "$DEFINED_SET" != *" $ref "* ]]; then
      UNDEFINED_REFS+=("${script#"$REPO_ROOT"/}: \$${ref}")
    fi
  done
done

if [[ ${#UNDEFINED_REFS[@]} -eq 0 ]]; then
  pass "All consumer variable references resolve to deploy-env.sh constants"
else
  for r in "${UNDEFINED_REFS[@]}"; do fail "Undefined reference: $r"; done
fi
echo

# ---- C3: SSM parameter naming consistency ---------------------------------
echo '== C3: SSM parameter naming convention =='
check_ssm_prefix() {
  local var_prefix="$1" expected_path_prefix="$2"
  local bad=0
  while IFS= read -r line; do
    name=$(echo "$line" | sed -E 's/^([A-Z_]+)=.*/\1/')
    value=$(echo "$line" | sed -E 's/^[A-Z_]+="([^"]+)"$/\1/')
    if [[ "$value" != "$expected_path_prefix"* ]]; then
      fail "SSM constant ${name}=${value} does not start with ${expected_path_prefix}"
      bad=1
    fi
  done < <(grep -E "^${var_prefix}[A-Z_]*=" "$DEPLOY_ENV_SH")
  return "$bad"
}

c3_ok=1
check_ssm_prefix "SSM_PROD_"              "/vaultmtg/app/production/" || c3_ok=0
check_ssm_prefix "SSM_STAGING_"           "/vaultmtg/app/staging/"    || c3_ok=0
check_ssm_prefix "SSM_VAULTMTG_STAGING_"  "/vaultmtg/app/staging/"    || c3_ok=0
[[ "$c3_ok" -eq 1 ]] && pass "All SSM_* constants use the correct /vaultmtg/app/{env}/ prefix"
echo

# ---- C4: DATABASE_URL construction (env-specific) -------------------------
echo '== C4: DATABASE_URL construction is correct per env =='
PROD_DBURL_FILE="${DEPLOY_SCRIPTS_DIR}/provision-db-url.sh"
STAGING_PROVISION_FILE="${DEPLOY_SCRIPTS_DIR}/provision-staging-env.sh"

check_credential_free_dburl() {
  local f="$1" label="$2"
  if [[ ! -f "$f" ]]; then
    fail "$label: missing file $f"
    return 1
  fi
  # 1. Has a postgresql:// URL construction.
  if ! grep -qE 'postgresql://' "$f"; then
    fail "$label: no postgresql:// URL constructed in $(basename "$f")"
    return 1
  fi
  # 2. The URL must NOT contain ${PASSWORD} or any user:pass@ segment.
  #    Look for ${ANYTHING}:${PASSWORD}@... or ${USER}:$PASS@ shapes inside
  #    a postgresql:// literal.
  if grep -E "postgresql://[A-Za-z0-9_\\\$\\{\\}]+:[A-Za-z0-9_\\\$\\{\\}]+@" "$f" >/dev/null 2>&1; then
    fail "$label: DATABASE_URL appears to embed credentials (user:pass@) in $(basename "$f")"
    return 1
  fi
  # 3. Must use one of the credential-free templates:
  #    (a) ${DB_ENDPOINT}:${DB_PORT}/${DB_NAME}?${DB_SSL_MODE} -- direct interpolation
  #    (b) postgresql://%s:%s/%s?%s with printf, where the arg list ends with
  #        DB_ENDPOINT, DB_PORT, DB_NAME, DB_SSL_MODE (any order is acceptable
  #        provided the host:port/dbname?ssl positions match below).
  if grep -qE "postgresql://\\\$\\{?DB_ENDPOINT\\}?:\\\$\\{?DB_PORT\\}?/\\\$\\{?DB_NAME\\}?\\?\\\$\\{?DB_SSL_MODE\\}?" "$f"; then
    return 0
  fi
  if grep -qE "postgresql://%s:%s/%s\\?%s" "$f" \
     && grep -qE 'DB_ENDPOINT'  "$f" \
     && grep -qE 'DB_PORT'      "$f" \
     && grep -qE 'DB_NAME'      "$f" \
     && grep -qE 'DB_SSL_MODE'  "$f"; then
    return 0
  fi
  fail "$label: DATABASE_URL shape in $(basename "$f") does not match credential-free template host:port/db?ssl"
  return 1
}

# Staging (post-#2461): the provisioner role splices fresh RDS credentials
# into DATABASE_URL via aws secretsmanager get-secret-value + jq, so the
# BFF binary never calls SM at startup. The URL shape is
# postgresql://%s:%s@%s:%s/%s?%s (user, pass, host, port, db, ssl) with the
# user/pass values URL-encoded via jq @uri. Verify both halves of that
# contract here.
check_staging_inline_dburl() {
  local f="$1"
  if [[ ! -f "$f" ]]; then
    fail "staging: missing file $f"
    return 1
  fi
  if ! grep -qE 'postgresql://%s:%s@%s:%s/%s\?%s' "$f"; then
    fail "staging: DATABASE_URL shape in $(basename "$f") is not the inline-credential template postgresql://%s:%s@%s:%s/%s?%s"
    return 1
  fi
  if ! grep -qE 'secretsmanager get-secret-value' "$f"; then
    fail "staging: $(basename "$f") emits inline-credential DATABASE_URL but never calls aws secretsmanager get-secret-value (#2461 splice broken)"
    return 1
  fi
  if ! grep -qE 'jq[[:space:]]+.*@uri' "$f"; then
    fail "staging: $(basename "$f") does not URL-encode the spliced username/password via jq @uri (#2461 splice unsafe)"
    return 1
  fi
  return 0
}

c4_ok=1
check_credential_free_dburl "$PROD_DBURL_FILE"  "production"  || c4_ok=0
check_staging_inline_dburl  "$STAGING_PROVISION_FILE"          || c4_ok=0
[[ "$c4_ok" -eq 1 ]] && pass "production DATABASE_URL is credential-free; staging DATABASE_URL is inline-credential per #2461"
echo

# ---- C5: DB-credential model symmetry (#2197 prod + #2461 staging) --------
echo '== C5: DB credential model -- prod paired with DB_SECRET_ARN; staging unpaired =='
c5_ok=1

# Production half (legacy contract -- #2197 protection):
# any DATABASE_URL writer must also write DB_SECRET_ARN, so the BFF can
# still resolve credentials at startup. This stays in force until the
# prod-side migration of #2461 lands.
if grep -qE "DATABASE_URL=" "$PROD_DBURL_FILE"; then
  if ! grep -qE "^[[:space:]]*(write_param[[:space:]]+DB_SECRET_ARN|printf[[:space:]].*DB_SECRET_ARN=|DB_SECRET_ARN=)" "$PROD_DBURL_FILE"; then
    fail "$(basename "$PROD_DBURL_FILE") writes DATABASE_URL but does not write DB_SECRET_ARN -- reproduces #2197"
    c5_ok=0
  fi
fi

# Staging half (new contract -- #2461):
# provision-staging-env.sh must NOT write DB_SECRET_ARN= or
# BFF_DB_RESOLVE_FROM_SM= into the env file. Either would re-enable the
# BFF's runtime SM call and reintroduce the crash-loop fixed by #2461.
# Match the actual write site (printf/echo into $ENV_FILE), NOT
# discussion of the key in comments. Reject the patterns:
#   write_param DB_SECRET_ARN ...
#   printf 'DB_SECRET_ARN=...'  >> "$ENV_FILE"
#   "DB_SECRET_ARN=$value"      >> "$ENV_FILE"
if grep -qE "^[[:space:]]*(write_param[[:space:]]+DB_SECRET_ARN|printf[[:space:]]+['\"][[:space:]]*DB_SECRET_ARN=|['\"]DB_SECRET_ARN=)" "$STAGING_PROVISION_FILE"; then
  fail "$(basename "$STAGING_PROVISION_FILE") writes DB_SECRET_ARN into the staging env file -- reproduces #2461 staging crash-loop"
  c5_ok=0
fi
if grep -qE "^[[:space:]]*(write_param[[:space:]]+BFF_DB_RESOLVE_FROM_SM|printf[[:space:]]+['\"][[:space:]]*BFF_DB_RESOLVE_FROM_SM=|['\"]BFF_DB_RESOLVE_FROM_SM=)" "$STAGING_PROVISION_FILE"; then
  fail "$(basename "$STAGING_PROVISION_FILE") writes BFF_DB_RESOLVE_FROM_SM into the staging env file -- re-enables runtime SM, reproduces #2461"
  c5_ok=0
fi

[[ "$c5_ok" -eq 1 ]] && pass "DB credential model symmetry holds (prod paired, staging unpaired)"
echo

# ---- C6: workflow <-> deploy-env.sh SSM-path agreement ----------------------
echo '== C6: workflow SSM-path literals match deploy-env.sh =='
# Extract every declared SSM path into a flat list (bash 3.2-compatible).
SSM_PATHS=()
while IFS= read -r line; do
  value=$(echo "$line" | sed -E 's/^[A-Z_]+="([^"]+)"$/\1/')
  SSM_PATHS+=("$value")
done < <(grep -E '^SSM_[A-Z_]+=' "$DEPLOY_ENV_SH")
ssm_path_set=" $(printf '%s ' "${SSM_PATHS[@]}") "

# For every literal /vaultmtg/app/{production|staging}/... appearing in a
# workflow, assert it is one of the declared SSM paths above (or document a
# legitimate workflow-only exception via the SSM_EXCEPTIONS allowlist).
SSM_EXCEPTIONS=(
  # Production SHA-tracking parameter -- written by deploy-bff.yml; readers
  # are ec2-bootstrap.sh (in mtga-companion-infra) and rollback tooling.
  # Not consumed by any in-repo deploy script.
  "/vaultmtg/app/production/latest-bff-sha"
)
exception_set=" $(printf '%s ' "${SSM_EXCEPTIONS[@]}") "

c6_ok=1
while IFS= read -r literal; do
  # Filter to app namespace only -- non-app /vaultmtg paths (spa-bucket-name,
  # prod/sentry-*) are out of scope of this contract.
  [[ "$literal" == /vaultmtg/app/* ]] || continue
  if [[ "$ssm_path_set" == *" $literal "* ]]; then
    continue
  fi
  if [[ "$exception_set" == *" $literal "* ]]; then
    continue
  fi
  fail "Workflow SSM literal not declared in deploy-env.sh: $literal"
  c6_ok=0
done < <(
  grep -hoE '/vaultmtg/app/[a-z]+/[A-Za-z0-9_./-]+' "$WORKFLOWS_DIR"/*.yml 2>/dev/null \
    | sort -u
)
[[ "$c6_ok" -eq 1 ]] && pass "Every /vaultmtg/app/* literal in workflows maps to a deploy-env.sh constant"
echo

# ---- C7: service-name discipline ------------------------------------------
echo '== C7: systemctl callers reference $BFF_SERVICE / $BFF_STAGING_SERVICE =='
c7_ok=1
for f in "$DEPLOY_SCRIPTS_DIR"/restart-bff*.sh; do
  [[ -f "$f" ]] || continue
  # Any literal systemctl <verb> mtga-companion or vault-mtg-bff would be a
  # smell -- we want only the variable form.  Allow systemctl daemon-reload.
  if grep -E 'systemctl[[:space:]]+(restart|enable|disable|start|stop|reload|status)[[:space:]]+(mtga-companion|vault-mtg-bff)' "$f" >/dev/null; then
    fail "$(basename "$f") hardcodes a systemd unit name -- reproduces #2192"
    c7_ok=0
  fi
  # Must reference at least one of the variable forms.
  if ! grep -qE '\$\{?BFF_(STAGING_)?SERVICE\}?' "$f"; then
    fail "$(basename "$f") does not reference \$BFF_SERVICE or \$BFF_STAGING_SERVICE"
    c7_ok=0
  fi
done
[[ "$c7_ok" -eq 1 ]] && pass "restart-bff*.sh use \$BFF_SERVICE / \$BFF_STAGING_SERVICE"
echo

# ---- C8: provisioned-env <-> BFF-read symmetry (WARNING) --------------------
echo '== C8: every provisioned env key is also read by the BFF (warning) =='
# Extract every KEY= that the provisioning scripts write into env files.
# Limited to `write_param KEY ...`, `printf 'KEY=%s\n'`, and direct
# `KEY=...>>"$ENV_FILE"` patterns.
PROVISIONED_KEYS=()
while IFS= read -r k; do
  PROVISIONED_KEYS+=("$k")
done < <(
  {
    # write_param KEY ... -- ignore comment lines and lines where the first
    # token is the literal "#".
    grep -hE '^[[:space:]]*write_param[[:space:]]+[A-Z_][A-Z0-9_]*' \
      "$STAGING_PROVISION_FILE" 2>/dev/null \
      | sed -E 's/^[[:space:]]*write_param[[:space:]]+([A-Z_][A-Z0-9_]*).*/\1/'
    # printf 'KEY=...' at the start of an executable line.
    grep -hE "^[[:space:]]*printf[[:space:]]+'[A-Z_][A-Z0-9_]*=" \
      "$STAGING_PROVISION_FILE" "$PROD_DBURL_FILE" 2>/dev/null \
      | sed -E "s/.*printf[[:space:]]+'([A-Z_][A-Z0-9_]*)=.*/\1/"
  } | sort -u
)

# BFF env-var allowlist: vars the BFF code actually reads.
BFF_VARS=()
while IFS= read -r v; do
  BFF_VARS+=("$v")
done < <(
  grep -rhE 'os\.Getenv\("[A-Z_][A-Z0-9_]*"\)' "$BFF_SOURCE_DIR" 2>/dev/null \
    | grep -v _test.go \
    | grep -oE '"[A-Z_][A-Z0-9_]*"' \
    | tr -d '"' \
    | sort -u
)
BFF_SET=" $(printf '%s ' "${BFF_VARS[@]}") "

# Infra-only env keys that exist for systemd / AWS SDK reasons, not for the
# BFF to consume directly.  Append-only -- every entry must be justified.
PROVISION_ALLOWLIST=(
  AWS_DEFAULT_REGION  # AWS SDK uses this; no Getenv in the BFF source.
  PORT                # See finding in PR description: written by
                      # provision-staging-env.sh (SSM_STAGING_PORT) but the
                      # BFF reads BFF_PORT instead.  Tracked separately;
                      # not blocking until rename lands.
  DISCORD_BOT_TOKEN
  DISCORD_GUILD_ID
  MAILCHIMP_API_KEY
  MAILCHIMP_LIST_ID
  CRISP_WEBSITE_ID
  RESEND_API_KEY
  CLERK_PUBLISHABLE_KEY  # Frontend-only; staging EC2 mounts it for parity.
)
allow_set=" $(printf '%s ' "${PROVISION_ALLOWLIST[@]}") "

if [[ ${#PROVISIONED_KEYS[@]} -gt 0 ]]; then
  for key in "${PROVISIONED_KEYS[@]}"; do
    if [[ "$BFF_SET" == *" $key "* ]]; then
      continue
    fi
    if [[ "$allow_set" == *" $key "* ]]; then
      continue
    fi
    warn "Provisioned env key '$key' is not read by services/bff/ and not on the allowlist"
  done
fi
if [[ "$WARNS" -eq 0 ]]; then
  pass "Every provisioned env key is read by services/bff/ or explicitly allowlisted"
fi
echo

# ---- C9: sourcing discipline ----------------------------------------------
echo '== C9: every consumer sources deploy-env.sh before use =='
# By construction CONSUMER_SCRIPTS already contains only scripts that source
# the file; this check exists to guard against future scripts that reference
# a deploy-env.sh variable WITHOUT sourcing.  We scan every .sh under
# scripts/deploy and infra/scripts that references a known SHOUTY_SNAKE_CASE
# constant from deploy-env.sh, and verify it sources.
c9_ok=1
all_sh=()
for f in "$DEPLOY_SCRIPTS_DIR"/*.sh "$INFRA_SCRIPTS_DIR"/*.sh; do
  [[ -f "$f" ]] && all_sh+=("$f")
done

if [[ ${#all_sh[@]} -eq 0 ]]; then
  warn "No .sh files found under scripts/deploy or infra/scripts"
fi
for f in "${all_sh[@]+"${all_sh[@]}"}"; do
  uses_constant=0
  for v in "${DEFINED_VARS[@]+"${DEFINED_VARS[@]}"}"; do
    if grep -qE "\\\$\\{?${v}\\}?" "$f"; then
      uses_constant=1
      break
    fi
  done
  if [[ "$uses_constant" -eq 1 ]]; then
    if ! grep -qE '\. (/tmp/deploy-env\.sh|.*infra/config/deploy-env\.sh)' "$f"; then
      fail "$(basename "$f") references a deploy-env.sh constant but does not source the file"
      c9_ok=0
    fi
  fi
done
[[ "$c9_ok" -eq 1 ]] && pass "Every script that references deploy-env.sh constants sources the file"
echo

# ---- summary ---------------------------------------------------------------
echo '== Summary =='
printf 'Failures: %d   Warnings: %d\n' "$FAILS" "$WARNS"
if [[ "$FAILS" -ne 0 ]]; then
  echo 'CONTRACT TEST FAILED' >&2
  exit 1
fi
echo 'CONTRACT TEST PASSED'
exit 0
