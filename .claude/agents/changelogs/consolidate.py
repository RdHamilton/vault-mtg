#!/usr/bin/env python3
"""
Merge pending changelog entries into their target changelog files.

Pending files live in .pending/ and follow the naming convention:
  YYYYMMDD-HHMMSS-RAND-{changelog-name}.md

Each pending file starts with a target header:
  target: {changelog-name}
  ---
  ## YYYY-MM-DD — ...  (the actual entry)

Entries are inserted newest-first, immediately after the HTML comment block.
This script is idempotent and safe to run concurrently — each file is processed
atomically (read pending → write target → delete pending).
"""

import os
import glob
import sys

CHANGELOGS_DIR = os.path.dirname(os.path.abspath(__file__))
PENDING_DIR = os.path.join(CHANGELOGS_DIR, ".pending")


def insert_after_comment(existing: str, entry: str) -> str:
    """Insert entry immediately after the closing --> of the HTML comment block."""
    idx = existing.find("-->")
    if idx >= 0:
        newline_after = existing.find("\n", idx)
        insert_pos = newline_after + 1 if newline_after >= 0 else idx + 3
        return existing[:insert_pos] + "\n" + entry.strip() + "\n" + existing[insert_pos:]
    # No comment block found — prepend
    return entry.strip() + "\n\n" + existing


def process_pending_file(path: str) -> bool:
    try:
        with open(path) as f:
            raw = f.read()
    except FileNotFoundError:
        return True  # already processed by another process

    lines = raw.split("\n")
    if not lines or not lines[0].startswith("target:"):
        print(f"  skip {os.path.basename(path)}: missing 'target:' header", file=sys.stderr)
        return False

    target_name = lines[0].split(":", 1)[1].strip()
    # Entry starts after "target: X\n---\n"
    sep = next((i for i, l in enumerate(lines) if l.strip() == "---"), 1)
    entry = "\n".join(lines[sep + 1:]).strip()

    if not entry:
        os.remove(path)
        return True

    target_file = os.path.join(CHANGELOGS_DIR, f"{target_name}.md")
    if not os.path.isfile(target_file):
        print(f"  warning: target file not found: {target_file}", file=sys.stderr)
        return False

    existing = open(target_file).read()
    updated = insert_after_comment(existing, entry)

    with open(target_file, "w") as f:
        f.write(updated)

    os.remove(path)
    print(f"  merged {os.path.basename(path)} → {target_name}.md")
    return True


def main():
    if not os.path.isdir(PENDING_DIR):
        return

    # Sort by filename (timestamp prefix) so oldest entries go in first,
    # producing correct newest-first order in the final changelog.
    pending = sorted(glob.glob(os.path.join(PENDING_DIR, "*.md")))

    if not pending:
        return

    print(f"consolidate: {len(pending)} pending entries")
    for path in pending:
        process_pending_file(path)


if __name__ == "__main__":
    main()
