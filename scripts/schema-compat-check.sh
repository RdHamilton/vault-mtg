#!/usr/bin/env bash
# schema-compat-check.sh — Schema-compatibility gate (vault-mtg-tickets#207)
#
# Usage (local):
#   BASE_SHA=<merge-base> HEAD_SHA=<pr-head> bash scripts/schema-compat-check.sh
#
# Usage (CI):
#   Set by the calling workflow — see .github/workflows/schema-compat.yml.
#
# What it does:
#   1. Lists .up.sql migration files changed between BASE_SHA and HEAD_SHA.
#   2. Extracts column names from DROP COLUMN and RENAME COLUMN ... TO statements
#      in those migrations.  The "old" name (before rename) is extracted for
#      RENAME COLUMN because that column disappears from the schema.
#   3. For each extracted column name, performs a word-boundary grep across all
#      *_repo.go files under services/bff/internal/storage/repository/.
#   4. Exits non-zero (1) if any dropped/renamed column is still referenced by
#      repository code.
#
# Limitations (v0.3.4 scope — vault-mtg-tickets#207):
#   - Grep-based: does not track which TABLE a column belongs to.  A column name
#     that appears in repo code against a DIFFERENT table will produce a false
#     positive.  Use the override procedure in the runbook when this occurs.
#   - Does not handle inline SQL comments that obscure the keyword.
#   - Does not catch struct-tag references, only raw SQL strings in *_repo.go.
#   Full ORM-level validation is deferred to v0.3.5.
#
# Override procedure:
#   vault-mtg-docs/engineering/runbooks/schema-compat-gate.md
#
# Exit codes:
#   0 — clean (no dropped/renamed columns are referenced in repository code)
#   1 — one or more dropped/renamed columns found in repository code
#   2 — script error (bad args, git failure)

set -uo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "ERROR: Not inside a git repository." >&2
  exit 2
}

REPO_DIR="${REPO_ROOT}/services/bff/internal/storage/repository"

BASE_SHA="${BASE_SHA:-}"
HEAD_SHA="${HEAD_SHA:-}"

if [[ -z "$BASE_SHA" || -z "$HEAD_SHA" ]]; then
  echo "ERROR: BASE_SHA and HEAD_SHA must be set." >&2
  echo "  export BASE_SHA=\$(git merge-base origin/main HEAD)" >&2
  echo "  export HEAD_SHA=HEAD" >&2
  exit 2
fi

# ---------------------------------------------------------------------------
# Step 1: collect changed .up.sql files in the PR diff
# ---------------------------------------------------------------------------

echo "=== Schema-Compat Gate ==="
echo "BASE: ${BASE_SHA}"
echo "HEAD: ${HEAD_SHA}"
echo ""

# Use a temp file to collect migration paths (avoids subshell + newline issues)
CHANGED_MIGRATIONS_FILE=$(mktemp)
trap 'rm -f "$CHANGED_MIGRATIONS_FILE"' EXIT

git diff --name-only "${BASE_SHA}" "${HEAD_SHA}" 2>/dev/null \
  | grep -E 'services/bff/internal/storage/migrations/postgres/[0-9]+_.*\.up\.sql$' \
  > "$CHANGED_MIGRATIONS_FILE" \
  || true

if [[ ! -s "$CHANGED_MIGRATIONS_FILE" ]]; then
  echo "No changed .up.sql migration files — gate SKIP (pass)."
  exit 0
fi

echo "Changed migration(s):"
while IFS= read -r f; do
  echo "  $f"
done < "$CHANGED_MIGRATIONS_FILE"
echo ""

# ---------------------------------------------------------------------------
# Step 2: extract dropped/renamed column names from changed migrations
# Use a temp file to deduplicate column names.
# ---------------------------------------------------------------------------

COLUMNS_FILE=$(mktemp)
trap 'rm -f "$CHANGED_MIGRATIONS_FILE" "$COLUMNS_FILE"' EXIT

while IFS= read -r rel_path; do
  abs_path="${REPO_ROOT}/${rel_path}"
  [[ ! -f "$abs_path" ]] && continue  # file deleted in this PR

  # DROP COLUMN [IF EXISTS] col_name
  # Handles single-line and multi-drop (comma-separated) statements.
  grep -ioE 'DROP[[:space:]]+COLUMN[[:space:]]+(IF[[:space:]]+EXISTS[[:space:]]+)?[a-zA-Z_][a-zA-Z0-9_]*' "$abs_path" \
    | grep -oiE '[a-zA-Z_][a-zA-Z0-9_]*$' \
    | tr '[:upper:]' '[:lower:]' \
    >> "$COLUMNS_FILE" \
    || true

  # RENAME COLUMN old_name TO new_name — capture old_name only
  grep -ioE 'RENAME[[:space:]]+COLUMN[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]+[[:space:]]+TO[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]+' "$abs_path" \
    | grep -oiE 'COLUMN[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]+' \
    | grep -oiE '[a-zA-Z_][a-zA-Z0-9_]+$' \
    | tr '[:upper:]' '[:lower:]' \
    >> "$COLUMNS_FILE" \
    || true

done < "$CHANGED_MIGRATIONS_FILE"

# Deduplicate
DEDUPED_FILE=$(mktemp)
trap 'rm -f "$CHANGED_MIGRATIONS_FILE" "$COLUMNS_FILE" "$DEDUPED_FILE"' EXIT
sort -u "$COLUMNS_FILE" > "$DEDUPED_FILE"

if [[ ! -s "$DEDUPED_FILE" ]]; then
  echo "No DROP COLUMN or RENAME COLUMN statements found in changed migrations — gate PASS."
  exit 0
fi

echo "Dropped/renamed column(s) detected:"
while IFS= read -r col; do
  echo "  '${col}'"
done < "$DEDUPED_FILE"
echo ""

# ---------------------------------------------------------------------------
# Step 3: grep *_repo.go files for each column name
# ---------------------------------------------------------------------------

if [[ ! -d "$REPO_DIR" ]]; then
  echo "ERROR: Repository directory not found: $REPO_DIR" >&2
  exit 2
fi

VIOLATIONS=0

while IFS= read -r col; do
  [[ -z "$col" ]] && continue

  # Word-boundary grep (GNU grep -w works on both Linux and macOS).
  MATCHES=$(
    grep -rn --include="*_repo.go" -iw "${col}" "$REPO_DIR" 2>/dev/null \
      || true
  )

  if [[ -n "$MATCHES" ]]; then
    echo "FAIL: column '${col}' is referenced in repository code:"
    while IFS= read -r line; do echo "  $line"; done <<< "$MATCHES"
    echo ""
    VIOLATIONS=$(( VIOLATIONS + 1 ))
  else
    echo "OK:   column '${col}' — no reference in *_repo.go files."
  fi
done < "$DEDUPED_FILE"

echo ""

# ---------------------------------------------------------------------------
# Result
# ---------------------------------------------------------------------------

if [[ $VIOLATIONS -gt 0 ]]; then
  echo "=== Gate FAIL: ${VIOLATIONS} column(s) dropped/renamed while still referenced in repository code. ==="
  echo ""
  echo "Action required:"
  echo "  1. Update the *_repo.go file(s) shown above to remove or update the reference."
  echo "  2. If this is a known false positive (same column name on a different table),"
  echo "     follow the override procedure in:"
  echo "     vault-mtg-docs/engineering/runbooks/schema-compat-gate.md"
  exit 1
fi

echo "=== Gate PASS: all dropped/renamed columns are safe to remove. ==="
exit 0
