#!/usr/bin/env bash
#
# staging-auth-smoke.sh — end-to-end smoke test of BFF + SPA auth wiring.
#
# Purpose: catch the auth misconfiguration we hit on 2026-05-12 — SPA bundle's
# Clerk publishable key not matching the BFF's Clerk secret — without needing a
# real browser session. Validates the BFF reaches Clerk's JWKS, the SPA bundle
# has the expected pk_*, and protected routes actually reject missing/invalid
# Bearer tokens (i.e. middleware is wired, not a noop).
#
# Run before claiming any auth-related change is "done".
#
# Usage (staging — default):
#   ./scripts/test/staging-auth-smoke.sh
#
# Usage (production):
#   SPA_HOST=https://app.vaultmtg.app \
#   BFF_HOST=https://api.vaultmtg.app \
#   SSM_CLERK_PK_PARAM=/mtga-companion/prod/CLERK_PUBLISHABLE_KEY \
#   SSM_CLERK_FRONTEND_PARAM=/mtga-companion/prod/CLERK_FRONTEND_API \
#   EC2_INSTANCE_ID=i-<prod-instance-id> \
#   BFF_ENV_FILE=/etc/mtga-companion/env \
#   BFF_SYSTEMD_UNIT=mtga-bff \
#   ./scripts/test/staging-auth-smoke.sh
#
# Requires:
#   - curl
#   - jq
#   - aws CLI configured with the "personal" profile (for SSM reads)

set -euo pipefail

# ── config ────────────────────────────────────────────────────────────────────
SPA_HOST="${SPA_HOST:-https://stg-app.vaultmtg.app}"
BFF_HOST="${BFF_HOST:-https://staging-api.vaultmtg.app}"
AWS_PROFILE_NAME="${AWS_PROFILE_NAME:-personal}"
SSM_REGION="${SSM_REGION:-us-east-1}"

# SSM parameter names for Clerk keys.
SSM_CLERK_PK_PARAM="${SSM_CLERK_PK_PARAM:-/mtga-companion/staging/CLERK_PUBLISHABLE_KEY}"
SSM_CLERK_FRONTEND_PARAM="${SSM_CLERK_FRONTEND_PARAM:-/mtga-companion/staging/CLERK_FRONTEND_API}"

# EC2 instance and BFF runtime paths.
EC2_INSTANCE_ID="${EC2_INSTANCE_ID:-i-065351fbb99da2d22}"
BFF_ENV_FILE="${BFF_ENV_FILE:-/etc/mtga-companion-staging/env}"
BFF_SYSTEMD_UNIT="${BFF_SYSTEMD_UNIT:-mtga-bff-staging}"

# ── coloring ──────────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[0;33m'; NC=$'\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; NC=''
fi

pass_count=0
fail_count=0
fail_lines=()

ok()    { printf "%s✓%s %s\n" "$GREEN" "$NC" "$1"; pass_count=$((pass_count+1)); }
fail()  { printf "%s✗%s %s\n" "$RED"   "$NC" "$1"; fail_count=$((fail_count+1)); fail_lines+=("$1"); }
info()  { printf "%s→%s %s\n" "$YELLOW" "$NC" "$1"; }

# ── helpers ───────────────────────────────────────────────────────────────────
http_status() {
  local url="$1"; shift
  curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$@" "$url" || echo "000"
}

expect_status() {
  local desc="$1" url="$2" want="$3"; shift 3
  local got
  got=$(http_status "$url" "$@")
  if [[ "$got" == "$want" ]]; then
    ok "$desc (got $got)"
  else
    fail "$desc — expected $want, got $got — url=$url"
  fi
}

# ── tests ─────────────────────────────────────────────────────────────────────

info "Pulling Clerk SSM parameters for cross-check"
SSM_OUT=$(aws ssm get-parameters \
  --names "$SSM_CLERK_PK_PARAM" \
          "$SSM_CLERK_FRONTEND_PARAM" \
  --region "$SSM_REGION" --profile "$AWS_PROFILE_NAME" \
  --query "Parameters[*].{Name:Name,Value:Value}" --output json)

BFF_PK=$(echo "$SSM_OUT"     | jq -r '.[] | select(.Name | endswith("CLERK_PUBLISHABLE_KEY")) | .Value')
BFF_FRONTEND=$(echo "$SSM_OUT" | jq -r '.[] | select(.Name | endswith("CLERK_FRONTEND_API"))   | .Value')

if [[ -z "$BFF_PK" || -z "$BFF_FRONTEND" ]]; then
  fail "Could not read Clerk SSM params (got empty values)"
  exit 1
fi
ok "BFF SSM CLERK_PUBLISHABLE_KEY: ${BFF_PK:0:24}…"
ok "BFF SSM CLERK_FRONTEND_API:    $BFF_FRONTEND"

# ── 1. SPA bundle has matching pk_ ────────────────────────────────────────────
info "Test 1 — SPA bundle uses the same Clerk publishable key as BFF SSM"
BUNDLE_PATH=$(curl -sL "$SPA_HOST/" | grep -oE 'assets/index-[A-Za-z0-9_-]+\.js' | head -1)
if [[ -z "$BUNDLE_PATH" ]]; then
  fail "Could not find bundle reference in $SPA_HOST/"
else
  ok "Found bundle: $BUNDLE_PATH"
  SPA_PKS=$(curl -sL "$SPA_HOST/$BUNDLE_PATH" | grep -oE 'pk_(test|live)_[A-Za-z0-9_-]+' | sort -u || true)
  if [[ -z "$SPA_PKS" ]]; then
    fail "No pk_ key found in bundle (Clerk SDK won't initialize)"
  elif echo "$SPA_PKS" | grep -qx "$BFF_PK"; then
    ok "SPA bundle pk matches BFF SSM pk"
  else
    fail "MISMATCH — SPA has [$SPA_PKS], BFF expects [$BFF_PK]"
  fi
fi

# ── 2. BFF /healthz is reachable (sanity) ─────────────────────────────────────
info "Test 2 — BFF /healthz returns 200"
expect_status "GET $BFF_HOST/healthz" "$BFF_HOST/healthz" "200"

# ── 3. CORS preflight succeeds with the SPA origin ────────────────────────────
info "Test 3 — CORS preflight from SPA origin is accepted"
CORS_HDR=$(curl -s -D - -o /dev/null --max-time 10 \
  -X OPTIONS \
  -H "Origin: $SPA_HOST" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: Authorization,Content-Type" \
  "$BFF_HOST/api/v1/health/daemon" | tr -d '\r' | grep -i "Access-Control-Allow-Origin:" | head -1)
if echo "$CORS_HDR" | grep -qi "$SPA_HOST"; then
  ok "CORS preflight returns ACAO with $SPA_HOST"
else
  fail "CORS preflight missing/incorrect ACAO header (got: '$CORS_HDR')"
fi

# ── 4. Clerk-protected route rejects missing Authorization ────────────────────
info "Test 4 — Clerk middleware rejects missing Authorization (401, not 200)"
expect_status "GET /api/v1/health/daemon no-auth"           "$BFF_HOST/api/v1/health/daemon" "401"
expect_status "POST /api/v1/matches no-auth"                "$BFF_HOST/api/v1/matches"       "401" -X POST -H 'Content-Type: application/json' --data '{}'
expect_status "POST /api/v1/matches/stats no-auth"          "$BFF_HOST/api/v1/matches/stats" "401" -X POST -H 'Content-Type: application/json' --data '{}'

# ── 5. Clerk-protected route rejects garbage Bearer ───────────────────────────
info "Test 5 — Clerk middleware rejects garbage Bearer (still 401, not 500)"
expect_status "GET /api/v1/health/daemon garbage-bearer" "$BFF_HOST/api/v1/health/daemon" "401" \
  -H "Authorization: Bearer not-a-real-jwt"

# ── 6. Daemon API-key route rejects garbage key ───────────────────────────────
info "Test 6 — /api/v1/ingest/events rejects garbage api_key with 401"
expect_status "POST /api/v1/ingest/events garbage-key" "$BFF_HOST/api/v1/ingest/events" "401" \
  -X POST -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer not-a-real-api-key' \
  --data '{"events":[]}'

# ── 7. /api/v1/daemon/register rejects missing OAuth token ────────────────────
info "Test 7 — /api/v1/daemon/register rejects unauth registration"
expect_status "POST /api/v1/daemon/register no-auth" "$BFF_HOST/api/v1/daemon/register" "401" \
  -X POST -H 'Content-Type: application/json' \
  --data '{"device_id":"smoke","platform":"darwin","daemon_ver":"smoke"}'

# ── 8. BFF can actually reach Clerk's JWKS (proves frontend_api is correct) ───
info "Test 8 — Clerk Frontend API JWKS endpoint is reachable from public internet"
JWKS_URL="${BFF_FRONTEND%/}/.well-known/jwks.json"
expect_status "GET $JWKS_URL" "$JWKS_URL" "200"

# ── 9. Schema drift — daemon_events must have every column the BFF writes ─────
# Catches the 2026-05-12 bug where migration 069 added `sequence` but the
# staging table was missing it, so every /api/v1/ingest/events returned 202
# while silently failing to persist (BFF logs "ERROR persisting event").
# Requires DB access via SSM RunCommand on the BFF host.
info "Test 9 — daemon_events schema matches BFF Insert call signature"
SCHEMA_CHECK=$(aws ssm send-command \
  --document-name "AWS-RunShellScript" \
  --instance-ids "$EC2_INSTANCE_ID" \
  --region "$SSM_REGION" --profile "$AWS_PROFILE_NAME" \
  --parameters "commands=[\"set -a && source $BFF_ENV_FILE && set +a && psql \\\"\\\$DATABASE_URL\\\" -tAc \\\"SELECT string_agg(column_name, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'daemon_events';\\\"\"]" \
  --query "Command.CommandId" --output text 2>&1)
sleep 5
SCHEMA_RESULT=$(aws ssm get-command-invocation \
  --command-id "$SCHEMA_CHECK" --instance-id "$EC2_INSTANCE_ID" \
  --region "$SSM_REGION" --profile "$AWS_PROFILE_NAME" \
  --query "StandardOutputContent" --output text 2>&1 | tr -d '\n')

# Columns the BFF's IngestHandler writes into daemon_events. Update this list
# whenever a migration adds a column the handler depends on.
REQUIRED_COLS=("user_id" "account_id" "event_type" "payload" "occurred_at" "event_id" "sequence")
missing=""
for col in "${REQUIRED_COLS[@]}"; do
  if [[ ",$SCHEMA_RESULT," != *",$col,"* ]]; then
    missing="$missing $col"
  fi
done
if [[ -z "$missing" ]]; then
  ok "daemon_events has all required columns (${#REQUIRED_COLS[@]} checked)"
else
  fail "daemon_events MISSING columns:$missing — ingest will 202 but silently drop events"
fi

# ── 10. Daemon-event ingest is actually persisting (last 5 min) ───────────────
# Catches "ingest accepts but doesn't write" silent failures even when the
# schema check above passes — e.g. permission errors, FK violations.
info "Test 10 — recent daemon ingest persistence is healthy"
INGEST_CHECK=$(aws ssm send-command \
  --document-name "AWS-RunShellScript" \
  --instance-ids "$EC2_INSTANCE_ID" \
  --region "$SSM_REGION" --profile "$AWS_PROFILE_NAME" \
  --parameters "commands=[\"journalctl -u $BFF_SYSTEMD_UNIT --since '5 min ago' --no-pager | grep -c 'ERROR persisting event' || true\"]" \
  --query "Command.CommandId" --output text 2>&1)
sleep 5
ERR_COUNT=$(aws ssm get-command-invocation \
  --command-id "$INGEST_CHECK" --instance-id "$EC2_INSTANCE_ID" \
  --region "$SSM_REGION" --profile "$AWS_PROFILE_NAME" \
  --query "StandardOutputContent" --output text 2>&1 | tr -d '[:space:]')
if [[ "$ERR_COUNT" == "0" ]]; then
  ok "no 'ERROR persisting event' lines in last 5 min"
else
  fail "$ERR_COUNT 'ERROR persisting event' lines in last 5 min — schema drift or DB error"
fi

# ── 11. SSE /api/v1/events rejects unauthenticated connection ────────────────
# Regression check: no auth + no cookie + no ?token= must always 401.  Catches
# any future middleware misconfiguration that would let the SSE stream open
# without verifying the Clerk JWT.
info "Test 11 — GET /api/v1/events with no auth returns 401"
expect_status "GET /api/v1/events no-auth" "$BFF_HOST/api/v1/events" "401"

# ── 12. SSE /api/v1/events accepts ?token= extractor (Issue #1904 wiring) ─────
# Sends a deliberately invalid JWT in ?token=.  The BFF middleware should
# parse the query parameter, attempt verification, and return 401 — proving
# that (a) the ?token= extractor is wired into RequireClerkAuthForSSE and
# (b) failures still return 401, never 5xx.  A 200 here would be a serious
# regression (auth bypass), and a 5xx would indicate the extractor crashes
# on bad input.  See services/bff/internal/api/middleware/clerk_auth.go.
info "Test 12 — GET /api/v1/events?token=<garbage> returns 401 (not 5xx)"
expect_status "GET /api/v1/events ?token=garbage" "$BFF_HOST/api/v1/events?token=not.a.real.jwt" "401"

# ── summary ───────────────────────────────────────────────────────────────────
echo
echo "──────────────────────────────────────────────"
echo "  passed: $pass_count"
echo "  failed: $fail_count"
if (( fail_count > 0 )); then
  echo
  echo "Failures:"
  for line in "${fail_lines[@]}"; do
    echo "  - $line"
  done
  exit 1
fi
echo "  staging auth wiring looks healthy."
