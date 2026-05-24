#!/usr/bin/env bash
# tests/deploy-chain/deploy-chain-integration-test.sh
#
# Deploy-chain integration test — exercises the full provision → stage-binary →
# run-migrations → grant → restart → healthcheck sequence against a throwaway
# Docker Postgres environment so that cross-script contract mismatches are caught
# in CI, not in production.
#
# What is exercised and how:
#
#   provision-env.sh / provision-db-url.sh
#     SSM calls are replaced by STUB_* env vars injected by this harness.
#     The script logic (file write, upsert, chmod) runs for real.
#
#   stage-binary.sh
#     AWS S3 call is stubbed: harness writes a fake binary to the expected path
#     so the atomic swap (cp / chmod / mv) exercises the real script body.
#
#   infra/scripts/run-migrations.sh (migration + grant path)
#     Runs golang-migrate and psql against a real Dockerised Postgres.
#     AWS SSM / Secrets Manager calls are replaced by env-var stubs.
#     Validates: dirty-state recovery logic, idempotent re-run.
#
#   restart-bff.sh
#     systemctl calls are stubbed via a shell function; the script's guard
#     logic and exit-code contract are exercised.
#
#   healthcheck-bff.sh
#     A tiny HTTP server (nc) responds to /healthz — verifies the poll/retry
#     loop and exit-zero path.
#
# Idempotency re-run check:
#   After the first full-pass the migration+grant step is run a SECOND time
#   from the same environment. All steps must exit 0 with no state explosion.
#
# Contract mismatch detection:
#   Variables required by downstream scripts (DEPLOY_BUCKET, DEPLOY_SHA,
#   DATABASE_URL etc.) are validated at the boundaries between phases so a
#   missing export or renamed var causes an explicit FAIL before the next
#   script runs.
#
# Usage:
#   bash tests/deploy-chain/deploy-chain-integration-test.sh
#
# CI: triggered by .github/workflows/deploy-script-integration-test.yml

set -euo pipefail

# ---- colour helpers --------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}PASS${NC}  $*"; }
fail() { echo -e "${RED}FAIL${NC}  $*"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC}  $*"; }

# ---- repo root -------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# ---- scratch space ---------------------------------------------------------
SCRATCH="$(mktemp -d)"
cleanup() {
    local code=$?
    if [[ -n "${HTTP_PID:-}" ]]; then
        kill "$HTTP_PID" 2>/dev/null || true
    fi
    if [[ -n "${PG_CONTAINER:-}" ]]; then
        info "Tearing down Postgres container..."
        docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
    fi
    rm -rf "$SCRATCH"
    exit $code
}
trap cleanup EXIT

info "Scratch dir: $SCRATCH"
info "Repo root  : $REPO_ROOT"

# ---------------------------------------------------------------------------
# Install deploy-env.sh at /tmp/deploy-env.sh — every EC2-side script sources
# it from that path (it is downloaded from S3 alongside each script before
# execution on EC2).  The test harness places a patched copy here so that
# BFF_ENV_FILE / BFF_ENV_DIR point into the scratch space rather than the
# real /etc/mtga-companion paths that require root access.
#
# DB_PORT and DB_SSL_MODE are also overridden so run-migrations.sh (which
# reads these vars from deploy-env.sh) connects to the throwaway Docker
# Postgres on the test port without TLS.
# ---------------------------------------------------------------------------
PG_PORT=15432  # declared here so the sed patches below can reference it

STUB_ENV_DIR="${SCRATCH}/etc/mtga-companion"
STUB_ENV_FILE="${SCRATCH}/etc/mtga-companion/env"
mkdir -p "$STUB_ENV_DIR"

DEPLOY_ENV_STUB="/tmp/deploy-env.sh"
sed \
    -e "s|BFF_ENV_DIR=\"/etc/mtga-companion\"|BFF_ENV_DIR=\"${STUB_ENV_DIR}\"|" \
    -e "s|BFF_ENV_FILE=\"/etc/mtga-companion/env\"|BFF_ENV_FILE=\"${STUB_ENV_FILE}\"|" \
    -e 's|DB_PORT="5432"|DB_PORT="'"${PG_PORT}"'"|' \
    -e 's|DB_SSL_MODE="sslmode=require"|DB_SSL_MODE="sslmode=disable"|' \
    "${REPO_ROOT}/infra/config/deploy-env.sh" > "$DEPLOY_ENV_STUB"
info "Installed patched deploy-env.sh at $DEPLOY_ENV_STUB (env paths, port, sslmode redirected for local test)"

# ===========================================================================
# Critical value assertions — infra/config/deploy-env.sh (single source of truth)
#
# The v0.3.1 deploy incident (post-mortem Defect 1) was caused by an undetected
# wrong systemd unit name; it was only caught because an engineer noticed a
# prose mismatch in the PR description.  These assertions convert that class
# of silent misconfiguration into a hard CI failure.
#
# Each assertion validates a critical fact in infra/config/deploy-env.sh.  If
# any of these values is renamed or accidentally blanked, the deploy-chain
# integration test must fail loudly with a descriptive message.
#
# The original deploy-env.sh (NOT the patched /tmp copy) is sourced here so
# we validate the source-of-truth values that ship to production, not the
# test-local overrides.
# ===========================================================================
info "Asserting critical deploy-env.sh values (source of truth)..."

# Source the original deploy-env.sh in a subshell so its variables do not
# pollute the rest of the harness (which uses the patched /tmp copy via the
# scripts under test).  We capture each value via a one-shot subshell echo.
# shellcheck source=infra/config/deploy-env.sh
ASSERT_BFF_SERVICE=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${BFF_SERVICE:-}")
# shellcheck source=infra/config/deploy-env.sh
ASSERT_BFF_STAGING_SERVICE=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${BFF_STAGING_SERVICE:-}")
# shellcheck source=infra/config/deploy-env.sh
ASSERT_BFF_BINARY=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${BFF_BINARY:-}")
# shellcheck source=infra/config/deploy-env.sh
ASSERT_BFF_STAGING_BINARY=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${BFF_STAGING_BINARY:-}")
# shellcheck source=infra/config/deploy-env.sh
ASSERT_DB_APP_ROLE=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${DB_APP_ROLE:-}")
# shellcheck source=infra/config/deploy-env.sh
ASSERT_DB_STAGING_APP_ROLE=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${DB_STAGING_APP_ROLE:-}")
# shellcheck source=infra/config/deploy-env.sh
ASSERT_DB_STAGING_NAME=$(. "${REPO_ROOT}/infra/config/deploy-env.sh" && printf '%s' "${DB_STAGING_NAME:-}")

assert_eq() {
    # assert_eq <var-name> <actual> <expected>
    local name="$1"; local actual="$2"; local expected="$3"
    if [[ -z "$actual" ]]; then
        fail "ASSERT FAILED: ${name} is empty/unset in infra/config/deploy-env.sh"
    fi
    if [[ "$actual" != "$expected" ]]; then
        fail "ASSERT FAILED: ${name} expected '${expected}', got '${actual}' (infra/config/deploy-env.sh)"
    fi
}

# infra/config/deploy-env.sh → BFF_SERVICE (systemd unit name for production BFF)
assert_eq "BFF_SERVICE" "$ASSERT_BFF_SERVICE" "mtga-companion"
# infra/config/deploy-env.sh → BFF_STAGING_SERVICE (systemd unit name for staging BFF)
assert_eq "BFF_STAGING_SERVICE" "$ASSERT_BFF_STAGING_SERVICE" "vault-mtg-bff-staging"
# infra/config/deploy-env.sh → BFF_BINARY (production binary basename in /usr/local/bin)
assert_eq "BFF_BINARY" "$ASSERT_BFF_BINARY" "mtga-bff"
# infra/config/deploy-env.sh → BFF_STAGING_BINARY (staging binary basename in /usr/local/bin)
assert_eq "BFF_STAGING_BINARY" "$ASSERT_BFF_STAGING_BINARY" "mtga-bff-staging"
# infra/config/deploy-env.sh → DB_APP_ROLE (production Postgres role)
assert_eq "DB_APP_ROLE" "$ASSERT_DB_APP_ROLE" "vaultmtg_app"
# infra/config/deploy-env.sh → DB_STAGING_APP_ROLE (staging Postgres role)
assert_eq "DB_STAGING_APP_ROLE" "$ASSERT_DB_STAGING_APP_ROLE" "vaultmtg_staging_app"
# infra/config/deploy-env.sh → DB_STAGING_NAME (staging Postgres database name)
assert_eq "DB_STAGING_NAME" "$ASSERT_DB_STAGING_NAME" "vaultmtg_staging"

pass "Critical deploy-env.sh values match expected constants (BFF_SERVICE, BFF_STAGING_SERVICE, BFF_BINARY, BFF_STAGING_BINARY, DB_APP_ROLE, DB_STAGING_APP_ROLE, DB_STAGING_NAME)"

# ===========================================================================
# Dependency check
# ===========================================================================
for dep in docker psql migrate curl nc; do
    if ! command -v "$dep" &>/dev/null; then
        fail "Required tool not found: $dep  (install it before running this test)"
    fi
done

# ===========================================================================
# Phase 0: Spin up throwaway Postgres
# ===========================================================================
info "Phase 0 — starting throwaway Postgres container..."

PG_CONTAINER="deploy-chain-test-pg-$$"
PG_PORT=15432
PG_USER=deploy_test
PG_PASSWORD=deploy_test
PG_DB=deploy_test_db
APP_ROLE=vaultmtg_app

docker run -d \
    --name "$PG_CONTAINER" \
    -e POSTGRES_USER="$PG_USER" \
    -e POSTGRES_PASSWORD="$PG_PASSWORD" \
    -e POSTGRES_DB="$PG_DB" \
    -p "${PG_PORT}:5432" \
    pgvector/pgvector:pg16 \
    >/dev/null

# Wait for Postgres to be ready (up to 30 s).
READY=0
for _i in $(seq 1 30); do
    if PGPASSWORD="$PG_PASSWORD" psql -h 127.0.0.1 -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -c "SELECT 1" >/dev/null 2>&1; then
        READY=1
        break
    fi
    sleep 1
done
[[ "$READY" -eq 1 ]] || fail "Postgres container did not become ready within 30 s"
pass "Phase 0 — Postgres container ready on port $PG_PORT"

# Pre-create the application role that the grant script references.
PGPASSWORD="$PG_PASSWORD" psql -h 127.0.0.1 -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -c \
    "DO \$\$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${APP_ROLE}') THEN CREATE ROLE ${APP_ROLE} LOGIN PASSWORD 'test'; END IF; END \$\$;" >/dev/null
pass "Phase 0 — app role '${APP_ROLE}' created"

MIGRATE_DB_URL="postgres://${PG_USER}:${PG_PASSWORD}@127.0.0.1:${PG_PORT}/${PG_DB}?sslmode=disable"
MIGRATIONS_DIR="${REPO_ROOT}/services/bff/internal/storage/migrations/postgres"

# Shared deploy vars (used by stage-binary and migrate steps).
DEPLOY_BUCKET="deploy-chain-test-bucket"
DEPLOY_SHA="abc1234"

# ===========================================================================
# Phase 1: provision-env.sh + provision-db-url.sh (EC2/SSM stubbed)
# ===========================================================================
info "Phase 1 — provision-env (SSM stubbed, writes env file to scratch)..."

# Build a stub 'aws' CLI at the front of PATH so all scripts that call
# `aws ssm get-parameter`, `aws sts assume-role`,
# `aws sts get-caller-identity`, or `aws secretsmanager get-secret-value`
# receive canned local responses.  This is written once here and
# overwritten in Phase 3 to also handle `aws s3` calls needed by
# run-migrations.sh.
#
# The sts stub persists the assumed session name to a scratch file so
# that the subsequent get-caller-identity call returns an ARN matching
# the session-name regex provision-db-url.sh's identity-verify gate
# checks for (case "$CALLER_ARN" in *":assumed-role/<role>/${SESSION_NAME}"). Without
# this state-passing, the gate would reject the call and the script
# would exit 1.
STUB_AWS="${SCRATCH}/aws"
STUB_AWS_STATE_DIR="${SCRATCH}/aws-stub-state"
mkdir -p "$STUB_AWS_STATE_DIR"
export STUB_AWS_STATE_DIR
cat > "$STUB_AWS" <<'AWSSTUB_PHASE1'
#!/usr/bin/env bash
# Phase 1 aws CLI stub — handles the read+assume-role+get-secret flow that
# provision-env.sh and provision-db-url.sh exercise.  Session-name state is
# persisted under $STUB_AWS_STATE_DIR so the identity-verify gate sees an
# ARN matching the role-session-name returned by assume-role.

if [[ "$1" == "ssm" && "$2" == "get-parameter" ]]; then
    NAME=""
    while [[ $# -gt 0 ]]; do
        case "$1" in --name) NAME="$2"; shift 2 ;; *) shift ;; esac
    done
    case "$NAME" in
        */ALLOWED_ORIGINS)   echo "http://localhost:3000" ;;
        */CLERK_SECRET_KEY)  echo "sk_test_stub" ;;
        */db-secret-arn)     echo "arn:aws:secretsmanager:us-east-1:000000000000:secret:stub" ;;
        */db-endpoint)       echo "127.0.0.1" ;;
        */db-name)           echo "deploy_test_db" ;;
        *)                   echo "stub_value" ;;
    esac
    exit 0
fi

if [[ "$1" == "sts" && "$2" == "assume-role" ]]; then
    # provision-db-url.sh invokes:
    #   aws sts assume-role --role-arn <arn> --role-session-name <session>
    #     --duration-seconds 900 --region <r>
    #     --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]'
    #     --output text
    # which produces three tab-separated tokens (no JSON wrapper).
    SESSION=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --role-session-name) SESSION="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    if [[ -z "$SESSION" ]]; then
        echo "STUB(phase1): sts assume-role missing --role-session-name" >&2
        exit 1
    fi
    # Persist for the get-caller-identity call that follows.
    printf '%s' "$SESSION" > "${STUB_AWS_STATE_DIR}/last_session_name"
    # Tab-separated AccessKeyId, SecretAccessKey, SessionToken (valid shape).
    printf 'ASIA_STUB_ACCESS_KEY_ID_XXXXXXXX\tstub_secret_access_key_value_XXXXXXXXXXXXXXXXXXXX\tstub_session_token_value_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX\n'
    exit 0
fi

if [[ "$1" == "sts" && "$2" == "get-caller-identity" ]]; then
    # provision-db-url.sh invokes:
    #   aws sts get-caller-identity --query Arn --output text
    # and matches: *":assumed-role/vaultmtg-staging-deploy-provisioner/${SESSION_NAME}"
    SESSION=""
    if [[ -f "${STUB_AWS_STATE_DIR}/last_session_name" ]]; then
        SESSION=$(cat "${STUB_AWS_STATE_DIR}/last_session_name")
    fi
    if [[ -z "$SESSION" ]]; then
        echo "STUB(phase1): sts get-caller-identity called before assume-role (no session-name state)" >&2
        exit 1
    fi
    printf 'arn:aws:sts::901347789205:assumed-role/vaultmtg-staging-deploy-provisioner/%s\n' "$SESSION"
    exit 0
fi

if [[ "$1" == "secretsmanager" && "$2" == "get-secret-value" ]]; then
    # provision-db-url.sh invokes:
    #   aws secretsmanager get-secret-value --secret-id <arn> --region <r>
    #     --query SecretString --output text
    # which returns the raw SecretString (JSON blob) -- not the wrapping
    # GetSecretValueResponse envelope.
    # Return the throwaway Postgres credentials (deploy_test:deploy_test) so
    # that the env file written by provision-db-url.sh carries a DATABASE_URL
    # that run-migrations.sh can source directly against the real container in
    # Phase 3. Credentials are hardcoded to match PG_USER/PG_PASSWORD set
    # below (single-quoted heredoc cannot expand outer-scope variables).
    printf '{"username":"deploy_test","password":"deploy_test"}\n'
    exit 0
fi

echo "STUB(phase1): unhandled aws command: $*" >&2
exit 1
AWSSTUB_PHASE1
chmod +x "$STUB_AWS"
export PATH="${SCRATCH}:${PATH}"

# Validate the provision-env.sh contract: the script must accept
# (ENV_KEY SSM_PARAM_NAME [--with-decryption]) and write KEY=VALUE to
# the BFF env file.  BFF_ENV_FILE / BFF_ENV_DIR are resolved from the
# patched /tmp/deploy-env.sh installed above — no further path overrides needed.
PROVISION_ENV="${SCRATCH}/provision-env-test.sh"
cp "${REPO_ROOT}/scripts/deploy/provision-env.sh" "$PROVISION_ENV"
chmod +x "$PROVISION_ENV"

bash "$PROVISION_ENV" ALLOWED_ORIGINS /mtga-companion/production/ALLOWED_ORIGINS
bash "$PROVISION_ENV" CLERK_SECRET_KEY /mtga-companion/production/CLERK_SECRET_KEY --with-decryption

# Contract check: provision-db-url.sh must write DATABASE_URL and DB_SECRET_ARN.
PROVISION_DB="${SCRATCH}/provision-db-url-test.sh"
cp "${REPO_ROOT}/scripts/deploy/provision-db-url.sh" "$PROVISION_DB"
chmod +x "$PROVISION_DB"

bash "$PROVISION_DB"

# Contract assertions — the env file must contain the keys downstream scripts depend on.
# Per #2461 / contract-test C5, DB_SECRET_ARN and BFF_DB_RESOLVE_FROM_SM must NOT be
# written -- the BFF reads inline credentials from DATABASE_URL and the runtime SM
# call is gated off. Asserting the absence here mirrors the C5 contract at the
# integration-test layer so a regression that re-introduces either key is caught.
for key in ALLOWED_ORIGINS DATABASE_URL AWS_DEFAULT_REGION; do
    if ! grep -q "^${key}=" "$STUB_ENV_FILE"; then
        fail "Phase 1 — contract violation: ${key} not written to env file (downstream scripts will break)"
    fi
done
for forbidden in DB_SECRET_ARN BFF_DB_RESOLVE_FROM_SM; do
    if grep -q "^${forbidden}=" "$STUB_ENV_FILE"; then
        fail "Phase 1 — contract violation (C5): env file MUST NOT contain ${forbidden}= -- re-enables runtime SM, reproduces #2461 crash-loop"
    fi
done
# Verify the SM splice happened: DATABASE_URL must carry the inline credentials
# returned by the stubbed secretsmanager get-secret-value call (PG_USER:PG_PASSWORD
# so that Phase 3 run-migrations.sh can use the env file directly against the
# throwaway Postgres container without a separate credential override).
if ! grep -q "^DATABASE_URL=postgresql://${PG_USER}:${PG_PASSWORD}@" "$STUB_ENV_FILE"; then
    fail "Phase 1 — contract violation: DATABASE_URL did not splice ${PG_USER}:${PG_PASSWORD} from the stubbed Secrets Manager response (rendered: $(grep ^DATABASE_URL= "$STUB_ENV_FILE"))"
fi
pass "Phase 1 — provision-env + provision-db-url ran; required keys present, DB_SECRET_ARN/BFF_DB_RESOLVE_FROM_SM absent (C5), DATABASE_URL spliced from SM"

# ===========================================================================
# Phase 2: stage-binary.sh (S3 stubbed via aws PATH overlay)
# ===========================================================================
info "Phase 2 — stage-binary (S3 stubbed with fake binary)..."

FAKE_BINARY_SRC="${SCRATCH}/fake-mtga-bff"
FAKE_BIN_DIR="${SCRATCH}/usr/local/bin"
mkdir -p "$FAKE_BIN_DIR"

cat > "$FAKE_BINARY_SRC" <<'BINSTUB'
#!/bin/sh
echo "stub mtga-bff"
exit 0
BINSTUB
chmod +x "$FAKE_BINARY_SRC"

# Patch stage-binary.sh in two passes:
# 1. Redirect /usr/local/bin/mtga-bff → scratch dir (all occurrences)
# 2. Replace the aws s3 cp line with a direct cp of our fake binary.
#    (This must come AFTER the path redirect so the already-redirected
#    destination is not double-substituted.)
STAGE_SCRIPT="${SCRATCH}/stage-binary-test.sh"
sed "s|/usr/local/bin/mtga-bff|${FAKE_BIN_DIR}/mtga-bff|g" \
    "${REPO_ROOT}/scripts/deploy/stage-binary.sh" \
    | sed "s|aws s3 cp \"s3://\${DEPLOY_BUCKET}/releases/\${DEPLOY_SHA}/mtga-bff\" ${FAKE_BIN_DIR}/mtga-bff.next|cp \"${FAKE_BINARY_SRC}\" \"${FAKE_BIN_DIR}/mtga-bff.next\"|" \
    > "$STAGE_SCRIPT"
chmod +x "$STAGE_SCRIPT"

DEPLOY_BUCKET="$DEPLOY_BUCKET" DEPLOY_SHA="$DEPLOY_SHA" bash "$STAGE_SCRIPT"

# Contract assertions.
[[ -x "${FAKE_BIN_DIR}/mtga-bff" ]] || \
    fail "Phase 2 — staged binary not found/executable at expected path"
[[ ! -f "${FAKE_BIN_DIR}/mtga-bff.next" ]] || \
    fail "Phase 2 — atomic swap left .next file (partial deploy risk)"

pass "Phase 2 — binary staged atomically; no .next artifact left behind"

# ===========================================================================
# Phase 3: run-migrations.sh (env-file credential model, real Postgres)
# ===========================================================================
info "Phase 3 — run-migrations against real Postgres (env-file credential model)..."

# run-migrations.sh now reads DATABASE_URL from $BFF_ENV_FILE (PR #2461 fix).
# The env file was written in Phase 1 by provision-db-url.sh with inline
# credentials using the throwaway Postgres user/password (deploy_test:deploy_test),
# so Phase 3 can source it directly without a separate credential override.
# Shape: postgresql://deploy_test:deploy_test@127.0.0.1:15432/deploy_test_db?sslmode=disable
#
# No SSM or Secrets Manager calls are made by run-migrations.sh; only S3 is
# needed (migrations download + grant SQL download).

# Phase 3 aws stub — S3 only (no SSM/SM needed; creds come from the env file).
cat > "$STUB_AWS" <<AWSSTUB_PHASE3
#!/usr/bin/env bash
if [[ "\$1" == "s3" ]]; then
    case "\$2" in
        sync)
            # aws s3 sync <s3-src> <local-dest> [--option value ...]
            # Positional: \$3 = s3 src, \$4 = local dest (before flags).
            DEST="\$4"
            mkdir -p "\$DEST"
            cp "${MIGRATIONS_DIR}"/*.sql "\$DEST/"
            exit 0
            ;;
        cp)
            # aws s3 cp <src> <dest> [--option value ...]
            # Positional: \$3 = src, \$4 = dest.
            SRC="\$3"; DEST="\$4"
            case "\$SRC" in
                */grant-production-tables.sql)
                    cp "${REPO_ROOT}/infra/db/grant-production-tables.sql" "\$DEST"
                    ;;
                *)
                    echo "STUB(phase3): unknown s3 cp src: \$SRC" >&2
                    exit 1
                    ;;
            esac
            exit 0
            ;;
    esac
fi
echo "STUB(phase3): unhandled aws command: \$*" >&2
exit 1
AWSSTUB_PHASE3
chmod +x "$STUB_AWS"

# Patch run-migrations.sh for local test execution:
#   - BFF_ENV_FILE / DB_PORT / DB_SSL_MODE already redirected via the patched
#     /tmp/deploy-env.sh (BFF_ENV_FILE → scratch env file; DB_PORT → 15432;
#     DB_SSL_MODE → sslmode=disable).
#   - psql call: inject -p PG_PORT so the grant step connects to the right port.
#   - remove dnf install block (psql already on PATH in CI).
MIGRATE_SCRIPT="${SCRATCH}/run-migrations-test.sh"
sed \
    -e 's|dnf install -y postgresql15||g' \
    -e "s|PGPASSWORD=\"\$MASTER_PASSWORD\" psql|PGPASSWORD=\"\$MASTER_PASSWORD\" psql -p ${PG_PORT}|g" \
    "${REPO_ROOT}/infra/scripts/run-migrations.sh" > "$MIGRATE_SCRIPT"
chmod +x "$MIGRATE_SCRIPT"

DEPLOY_BUCKET="$DEPLOY_BUCKET" AWS_REGION=us-east-1 bash "$MIGRATE_SCRIPT"

# Contract assertions: migrations applied, schema_migrations clean.
CURRENT_VER=$(migrate \
    -path "$MIGRATIONS_DIR" \
    -database "$MIGRATE_DB_URL" \
    version 2>&1 || true)
if echo "$CURRENT_VER" | grep -qi "dirty"; then
    fail "Phase 3 — migrations left database in dirty state: $CURRENT_VER"
fi
[[ -n "$CURRENT_VER" ]] || fail "Phase 3 — migrate version returned empty output (no migrations applied?)"

# Contract: grant-production-tables.sql must have given vaultmtg_app table access.
TABLE_COUNT=$(PGPASSWORD="$PG_PASSWORD" psql -h 127.0.0.1 -p "$PG_PORT" \
    -U "$PG_USER" -d "$PG_DB" -t -c \
    "SELECT count(*) FROM information_schema.role_table_grants WHERE grantee = '${APP_ROLE}';" 2>&1 | tr -d ' ')
[[ "$TABLE_COUNT" -ge 1 ]] || \
    fail "Phase 3 — grant step did not grant any tables to ${APP_ROLE} (contract: grant-production-tables.sql must run after migrate)"

pass "Phase 3 — migrations applied (version: $CURRENT_VER); ${TABLE_COUNT} table grants applied to ${APP_ROLE}"

# ===========================================================================
# Phase 4: restart-bff.sh (systemctl stubbed via PATH overlay)
# ===========================================================================
info "Phase 4 — restart-bff (systemctl stubbed)..."

STUB_SYSTEMCTL="${SCRATCH}/systemctl"
cat > "$STUB_SYSTEMCTL" <<'SVCSTUB'
#!/usr/bin/env bash
CMD="$1"; SERVICE="${2:-}"
echo "[stub-systemctl] $CMD $SERVICE"
if [[ "$CMD" == "restart" && "$SERVICE" != "mtga-companion" ]]; then
    echo "ERROR: wrong unit name — expected 'mtga-companion', got '${SERVICE}'" >&2
    exit 1
fi
exit 0
SVCSTUB
chmod +x "$STUB_SYSTEMCTL"

bash "${REPO_ROOT}/scripts/deploy/restart-bff.sh"

pass "Phase 4 — restart-bff.sh invoked correct systemd unit name (mtga-companion)"

# ===========================================================================
# Phase 5: healthcheck-bff.sh (stub HTTP server)
# ===========================================================================
info "Phase 5 — healthcheck-bff (stub HTTP server on port 8080)..."

# Start a minimal HTTP 200 server.
HTTP_PID=""
{
    while true; do
        printf 'HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK' \
            | nc -l 8080 2>/dev/null || true
    done
} &
HTTP_PID=$!

# Give the server a moment to bind.
sleep 1

bash "${REPO_ROOT}/scripts/deploy/healthcheck-bff.sh"

kill "$HTTP_PID" 2>/dev/null || true
HTTP_PID=""

pass "Phase 5 — healthcheck-bff.sh polled /healthz and received 200"

# ===========================================================================
# Idempotency re-run (Phase 3 repeated — the most stateful step)
# ===========================================================================
info "Idempotency check — re-running migration+grant step against same DB..."

DEPLOY_BUCKET="$DEPLOY_BUCKET" AWS_REGION=us-east-1 bash "$MIGRATE_SCRIPT"

CURRENT_VER_2=$(migrate \
    -path "$MIGRATIONS_DIR" \
    -database "$MIGRATE_DB_URL" \
    version 2>&1 || true)
if echo "$CURRENT_VER_2" | grep -qi "dirty"; then
    fail "Idempotency — second run left database dirty: $CURRENT_VER_2"
fi
[[ "$CURRENT_VER" == "$CURRENT_VER_2" ]] || \
    fail "Idempotency — migration version changed on re-run: was '$CURRENT_VER', now '$CURRENT_VER_2'"

TABLE_COUNT_2=$(PGPASSWORD="$PG_PASSWORD" psql -h 127.0.0.1 -p "$PG_PORT" \
    -U "$PG_USER" -d "$PG_DB" -t -c \
    "SELECT count(*) FROM information_schema.role_table_grants WHERE grantee = '${APP_ROLE}';" 2>&1 | tr -d ' ')
[[ "$TABLE_COUNT_2" -ge "$TABLE_COUNT" ]] || \
    fail "Idempotency — grant count dropped on re-run ($TABLE_COUNT → $TABLE_COUNT_2)"

pass "Idempotency — second run is a no-op; DB clean at version $CURRENT_VER_2"

# ===========================================================================
# Dirty-state recovery simulation
# ===========================================================================
info "Dirty-state recovery — injecting dirty flag and verifying auto-recovery..."

CLEAN_VER=$(echo "$CURRENT_VER_2" | grep -oE '[0-9]+' | head -1)
DIRTY_VER=$((CLEAN_VER + 1))

PGPASSWORD="$PG_PASSWORD" psql -h 127.0.0.1 -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" \
    -c "UPDATE schema_migrations SET version=${DIRTY_VER}, dirty=true;" >/dev/null

DIRTY_CHECK=$(migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DB_URL" version 2>&1 || true)
if ! echo "$DIRTY_CHECK" | grep -qi "dirty"; then
    fail "Dirty-state setup — expected dirty flag, but migrate version did not report dirty"
fi

# Run the migration script again — it should detect and recover.
DEPLOY_BUCKET="$DEPLOY_BUCKET" AWS_REGION=us-east-1 bash "$MIGRATE_SCRIPT"

RECOVERED_VER=$(migrate -path "$MIGRATIONS_DIR" -database "$MIGRATE_DB_URL" version 2>&1 || true)
if echo "$RECOVERED_VER" | grep -qi "dirty"; then
    fail "Dirty-state recovery — script did not clear dirty flag: $RECOVERED_VER"
fi

pass "Dirty-state recovery — script detected dirty state and recovered to clean version"

# ===========================================================================
# Summary
# ===========================================================================
echo ""
echo -e "${GREEN}===========================================${NC}"
echo -e "${GREEN} Deploy-chain integration test: ALL PASS  ${NC}"
echo -e "${GREEN}===========================================${NC}"
echo ""
echo "  Phase 0 — throwaway Postgres container"
echo "  Phase 1 — provision-env + provision-db-url (SSM stubbed)"
echo "  Phase 2 — stage-binary (S3 stubbed)"
echo "  Phase 3 — run-migrations + grant (real Postgres)"
echo "  Phase 4 — restart-bff (systemctl stubbed)"
echo "  Phase 5 — healthcheck-bff (stub HTTP server)"
echo "  Idempotency — second full run is a no-op"
echo "  Dirty-state recovery — auto-recovery from mid-flight failure"
echo ""
