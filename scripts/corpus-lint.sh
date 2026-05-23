#!/usr/bin/env bash
# corpus-lint.sh — VaultMTG agent corpus linter
#
# Usage:  bash scripts/corpus-lint.sh [--agents-dir <path>]
#
# Checks the .claude/agents/ corpus for invariant violations.
# Exit 0 = clean. Exit 1 = one or more violations found.
#
# Invariants enforced here (from .claude/agents/INVARIANTS.md):
#   I-01  Stale branch-creation pattern (git checkout main && git pull)
#   I-13  Stale repo references (MTGA-Companion / mtga-companion-web)
#   SIZE  Persona file over 13 KB (prep for P-07 slim-down)
#   DUP   Re-duplicated rule family: Local Verification defined outside _shared.md / orchestration.md
#   DUP   Re-duplicated rule family: standalone "GitHub shows you as RdHamilton" outside _shared.md
#   DUP   Re-duplicated rule family: Task Scope enforcement defined outside _shared.md (P-03)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

AGENTS_DIR="${AGENTS_DIR:-/Users/ramonehamilton/Documents/Personal Projects/.claude/agents}"
MAX_FILE_SIZE_BYTES=13312   # 13 KB
SKIP_IF_MISSING=0

# Override agents dir via --agents-dir flag
while [[ $# -gt 0 ]]; do
  case "$1" in
    --agents-dir)
      AGENTS_DIR="$2"
      shift 2
      ;;
    --skip-if-missing)
      SKIP_IF_MISSING=1
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ ! -d "$AGENTS_DIR" ]]; then
  if [[ $SKIP_IF_MISSING -eq 1 ]]; then
    echo "corpus-lint: agents dir not found at $AGENTS_DIR — skipping (no agents checked into this repo)"
    echo "corpus-lint: PASS (skipped — no agents dir)"
    exit 0
  fi
  echo "ERROR: agents dir not found: $AGENTS_DIR" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

VIOLATIONS=0

fail() {
  local check="$1"
  local file="$2"
  local detail="$3"
  echo "FAIL  [$check]  $file"
  echo "      $detail"
  VIOLATIONS=$(( VIOLATIONS + 1 ))
}

pass() {
  local check="$1"
  echo "PASS  [$check]"
}

# ---------------------------------------------------------------------------
# Check I-13: Stale repo references — RdHamilton/MTGA-Companion
# Exclude: _team.md (carries the "formerly MTGA-Companion" note for context),
#          pam.md (carries the same historical note),
#          orchestration.md (documents legacy worktree paths for context)
# ---------------------------------------------------------------------------

check_stale_repo_refs() {
  local found=0
  while IFS= read -r -d '' file; do
    local base
    base=$(basename "$file")
    # These files are allowed to mention MTGA-Companion for historical context
    if [[ "$base" == "_team.md" || "$base" == "pam.md" || "$base" == "orchestration.md" ]]; then
      continue
    fi
    if grep -qE 'RdHamilton/MTGA-Companion|mtga-companion-web' "$file"; then
      local lines
      lines=$(grep -nE 'RdHamilton/MTGA-Companion|mtga-companion-web' "$file" | head -3)
      fail "stale-repo-ref" "$file" "contains stale repo reference (use RdHamilton/vault-mtg):$( echo; echo "$lines" | sed 's/^/        /')"
      found=1
    fi
  done < <(find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -not -name "INVARIANTS.md" -print0)
  if [[ $found -eq 0 ]]; then
    pass "stale-repo-ref"
  fi
}

# ---------------------------------------------------------------------------
# Check I-01: Stale branch-creation pattern
# Pattern: "git checkout main && git pull origin main"
# Correct:  "git fetch origin && git checkout -b <name> origin/main"
# The old pattern is visible in lee.md Rule #4 (sync note) — allowed context.
# We flag only if it appears as an *instruction* (imperative form), not as a
# descriptive back-reference. Heuristic: flag if it appears outside a
# "running checks on a stale branch" commentary line.
# ---------------------------------------------------------------------------

check_branch_pattern() {
  local found=0
  while IFS= read -r -d '' file; do
    local base
    base=$(basename "$file")
    # Search for the old imperative pattern
    if grep -qE 'git checkout main &&' "$file"; then
      # Allow if the line also contains the explanatory "running checks on a stale branch" text
      # (that is lee.md Rule #4 contextual note, not an instruction)
      while IFS= read -r line; do
        if echo "$line" | grep -qE 'git checkout main &&'; then
          if ! echo "$line" | grep -qE 'running checks on a stale branch|stale branch produces false'; then
            fail "branch-pattern" "$file" "found old branch pattern 'git checkout main && git pull' — use 'git fetch origin && git checkout -b <name> origin/main': $(echo "$line" | sed 's/^[[:space:]]*//')"
            found=1
          fi
        fi
      done < "$file"
    fi
  done < <(find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -not -name "INVARIANTS.md" -print0)
  if [[ $found -eq 0 ]]; then
    pass "branch-pattern"
  fi
}

# ---------------------------------------------------------------------------
# Check SIZE: Persona file over 13 KB
# Excludes support files (_shared, _team, BROADCAST, orchestration, INVARIANTS)
# ---------------------------------------------------------------------------

check_file_sizes() {
  local found=0
  while IFS= read -r -d '' file; do
    local base
    base=$(basename "$file")
    # Only check named persona files (not shared/infra files)
    case "$base" in
      _shared.md|_team.md|BROADCAST.md|orchestration.md|INVARIANTS.md)
        continue
        ;;
    esac
    local size
    size=$(wc -c < "$file")
    if [[ $size -gt $MAX_FILE_SIZE_BYTES ]]; then
      local size_kb
      size_kb=$(awk "BEGIN { printf \"%.1f\", $size / 1024 }")
      fail "persona-size" "$file" "file is ${size_kb} KB (limit: 13 KB). Slim down or split per P-07."
      found=1
    fi
  done < <(find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -print0)
  if [[ $found -eq 0 ]]; then
    pass "persona-size"
  fi
}

# ---------------------------------------------------------------------------
# Check DUP-LV: Re-duplicated Local Verification definition
# A persona file (not _shared.md, not orchestration.md) contains a
# multi-sentence standalone definition of Local Verification — i.e., it
# defines the rule rather than just referencing it.
#
# Heuristic: more than one sentence (≥2 sentences in a paragraph) on a line
# that defines Local Verification policy. We look for phrases that spell out
# the rule ("must be a pasted", "not prose", "copied from the terminal").
# ---------------------------------------------------------------------------

check_dup_local_verification() {
  local found=0
  while IFS= read -r -d '' file; do
    local base
    base=$(basename "$file")
    # _shared.md and orchestration.md are the canonical homes — skip them
    if [[ "$base" == "_shared.md" || "$base" == "orchestration.md" || "$base" == "INVARIANTS.md" ]]; then
      continue
    fi
    # Check for a multi-sentence definition (defining phrases, not just a pointer).
    # Require the longer canonical phrases that appear only in a full definition,
    # not in a brief pointer like "real transcript, not prose — see _shared.md §6".
    if grep -qE 'must be a pasted real transcript|copied from the terminal|written from expectation' "$file"; then
      local lines
      lines=$(grep -nE 'must be a pasted real transcript|copied from the terminal|written from expectation' "$file" | head -3)
      fail "dup-local-verification" "$file" "contains a standalone Local Verification definition — point to _shared.md §6 instead:$( echo; echo "$lines" | sed 's/^/        /')"
      found=1
    fi
  done < <(find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -print0)
  if [[ $found -eq 0 ]]; then
    pass "dup-local-verification"
  fi
}

# ---------------------------------------------------------------------------
# Check DUP-GH: Re-duplicated "GitHub shows you as RdHamilton" explanation
# A persona file (other than _shared.md) contains a multi-sentence standalone
# explanation of why the Agent field is required due to GitHub showing RdHamilton.
# Heuristic: look for the defining phrases side-by-side (same paragraph context).
# ---------------------------------------------------------------------------

check_dup_github_author() {
  local found=0
  while IFS= read -r -d '' file; do
    local base
    base=$(basename "$file")
    if [[ "$base" == "_shared.md" || "$base" == "INVARIANTS.md" ]]; then
      continue
    fi
    # The smell: explaining both "GitHub shows RdHamilton" AND "Agent field is the only signal"
    # in a paragraph that reads like a definition (not just a reference to _shared.md)
    if grep -qE 'GitHub shows every PR as authored by.*RdHamilton' "$file" && \
       grep -qE 'Agent field is the only.*signal|only reliable signal' "$file"; then
      local lines
      lines=$(grep -nE 'GitHub shows every PR as authored by.*RdHamilton|Agent field is the only.*signal|only reliable signal' "$file" | head -4)
      fail "dup-github-author" "$file" "contains a standalone 'GitHub shows RdHamilton' authorship explanation — point to _shared.md §6 instead:$( echo; echo "$lines" | sed 's/^/        /')"
      found=1
    fi
  done < <(find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -print0)
  if [[ $found -eq 0 ]]; then
    pass "dup-github-author"
  fi
}

# ---------------------------------------------------------------------------
# Check DUP-TS: Re-duplicated Task Scope enforcement block
# The canonical definition lives in _shared.md §1.
# BROADCAST.md is also excluded (it may carry a one-line reminder pointing
# to _shared.md §1, but must NOT host the full multi-bullet rule block).
# Heuristic: look for the defining phrases that only appear in a full
# re-statement of the rule, not in a brief one-line pointer.
# ---------------------------------------------------------------------------

check_dup_task_scope() {
  local found=0
  while IFS= read -r -d '' file; do
    local base
    base=$(basename "$file")
    # _shared.md is the canonical home — skip it
    if [[ "$base" == "_shared.md" || "$base" == "INVARIANTS.md" ]]; then
      continue
    fi
    # Detect a full re-statement: long phrase only found when the rule is
    # spelled out in full, not in a one-line pointer.
    if grep -qE 'MUST perform ONLY the work explicitly described in your assigned instruction' "$file"; then
      local lines
      lines=$(grep -nE 'MUST perform ONLY the work explicitly described' "$file" | head -3)
      fail "dup-task-scope" "$file" "contains a standalone Task Scope definition — replace with a one-line pointer to _shared.md §1:$( echo; echo "$lines" | sed 's/^/        /')"
      found=1
    fi
  done < <(find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -print0)
  if [[ $found -eq 0 ]]; then
    pass "dup-task-scope"
  fi
}

# ---------------------------------------------------------------------------
# Run all checks
# ---------------------------------------------------------------------------

echo "corpus-lint: scanning $AGENTS_DIR"
echo ""

check_stale_repo_refs
check_branch_pattern
check_file_sizes
check_dup_local_verification
check_dup_github_author
check_dup_task_scope

echo ""
if [[ $VIOLATIONS -gt 0 ]]; then
  echo "corpus-lint: FAIL — $VIOLATIONS violation(s) found"
  exit 1
else
  echo "corpus-lint: PASS — corpus is clean"
  exit 0
fi
