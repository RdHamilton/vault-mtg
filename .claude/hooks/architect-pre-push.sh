#!/bin/bash
# Architect pre-push review
# Fires on every Bash tool use; skips non-push commands in ~1ms

INPUT=$(cat)

CMD=$(echo "$INPUT" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('tool_input', {}).get('command', ''))
except Exception:
    print('')
" 2>/dev/null || true)

# Skip anything that isn't a git push
if ! echo "$CMD" | grep -q "git push"; then
    exit 0
fi

echo "🏛️  Architect review: scanning diff before push..." >&2

MERGE_BASE=$(git merge-base HEAD origin/main 2>/dev/null \
    || git merge-base HEAD main 2>/dev/null \
    || true)

if [ -z "$MERGE_BASE" ]; then
    echo "⚠️  Could not find merge base — skipping review" >&2
    exit 0
fi

DIFF=$(git diff "$MERGE_BASE"..HEAD 2>/dev/null || true)

if [ -z "$DIFF" ]; then
    echo "✅  No changes to review — push allowed" >&2
    exit 0
fi

DIFF_LINES=$(echo "$DIFF" | wc -l | tr -d ' ')
DIFF_TRUNCATED=$(echo "$DIFF" | head -400)
TRUNCATED_NOTE=""
if [ "$DIFF_LINES" -gt 400 ]; then
    TRUNCATED_NOTE=" (showing first 400 of $DIFF_LINES lines)"
fi

PROMPT="You are the MTGA Companion architect agent. A sub-agent is requesting pre-push approval. Review this git diff$TRUNCATED_NOTE for architectural concerns.

Check for:
1. Service boundary violations (daemon writing to DB directly, frontend bypassing adapters, etc.)
2. Missing account_id scoping on any user-data queries
3. go.work replace directives pointing to local filesystem paths (e.g. replace ... => ../)
4. ADR non-compliance: WebSocket usage (SSE only per ADR-001), fetch calls directly in React components (adapter pattern required)
5. Missing tests for changed functionality

Reply with EXACTLY one of:
- APPROVED
- BLOCKED: <specific issues that must be fixed>

First word must be APPROVED or BLOCKED. No preamble.

---
$DIFF_TRUNCATED"

echo "  Invoking architect..." >&2
REVIEW=$(claude -p "$PROMPT" 2>/dev/null || echo "REVIEW_FAILED")

if [ "$REVIEW" = "REVIEW_FAILED" ] || [ -z "$REVIEW" ]; then
    echo "⚠️  Architect review could not complete — allowing push" >&2
    exit 0
fi

if echo "$REVIEW" | head -1 | grep -q "^APPROVED"; then
    echo "✅  Architect review: APPROVED" >&2
    exit 0
else
    printf "\n⛔  Architect review blocked this push:\n\n%s\n\n" "$REVIEW" >&2
    echo "Fix the issues above, re-run pre-PR checks, then push again." >&2
    exit 2
fi
