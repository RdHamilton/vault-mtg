#!/usr/bin/env python3
"""
extract.py — ADR-042 Player.log fixture extractor.

Reads a Player.log file (or stdin), identifies MTGA log events by top-level
JSON key, writes one sanitised fixture file per event class to --output-dir.

Stdlib only: json, pathlib, argparse, re, sys.

Usage:
    python3 extract.py --input Player.log --output-dir ./corpus-raw --sanitize --first-only
    python3 extract.py --input Player.log --output-dir ./corpus-raw --sanitize --variant empty-format
    python3 extract.py --input Player.log --output-dir ./corpus-raw --sanitize --variant missing-id
"""

import argparse
import json
import pathlib
import re
import sys


# ---------------------------------------------------------------------------
# Event-class detection: top-level JSON key -> output filename
# ---------------------------------------------------------------------------

# Default key map. Keys are checked in order; first match wins.
# Override or extend via --key-map KEY=FILE pairs.
DEFAULT_KEY_MAP = [
    ("matchGameRoomStateChangedEvent", "match-completed.log"),
    ("quests",                          "quest-progress.log"),
    ("canSwap",                         "quest-progress.log"),  # alt quest key (empty response)
    ("authenticateResponse",            "player-authenticated.log"),
    ("draftPack",                       "draft-pack.log"),
    ("pickedCards",                     "draft-pick.log"),
    ("InventoryInfo",                   "inventory-updated.log"),
    ("request",                         "deck-updated.log"),
]

# The collection snapshot is a flat {"grpId": count, ...} map — detected by
# the absence of any named wrapper key and all-integer keys.
_KNOWN_WRAPPER_KEYS = frozenset(
    k for k, _ in DEFAULT_KEY_MAP
) | frozenset(
    ["toSceneName", "fromSceneName", "CurrentEventState", "rankClass"]
)


def _is_collection_entry(obj: dict) -> bool:
    """Return True when every key in obj is a parseable positive integer."""
    if not obj:
        return True  # empty collection snapshot
    if _KNOWN_WRAPPER_KEYS.intersection(obj.keys()):
        return False
    return all(_parse_positive_int(k) is not None for k in obj)


def _parse_positive_int(s: str):
    """Return int(s) when s represents a positive integer, else None."""
    try:
        n = int(s)
        return n if n > 0 else None
    except (ValueError, TypeError):
        return None


# ---------------------------------------------------------------------------
# PII sanitisation (ADR-041 G3)
# ---------------------------------------------------------------------------

# Deterministic counter-based UUID replacement.
_COUNTER = {"match": 0, "account": 0, "session": 0, "quest": 0, "deck": 0, "draft": 0}

_UUID_RE = re.compile(
    r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
)

# Map from real UUID -> stable fake UUID (built during a single run).
_UUID_MAP: dict = {}
# Matches a quoted JSON string containing a MTGA screen name (e.g. "RealPlayer#99999").
# The MTGA format is: alphanumeric characters followed by # and digits, all within quotes.
_SCREEN_NAME_RE = re.compile(r'"[A-Za-z][A-Za-z0-9]*#\d+"')


def _next_fake_uuid(prefix: str) -> str:
    """Return the next deterministic fake UUID for the given slot prefix."""
    _COUNTER[prefix] += 1
    n = _COUNTER[prefix]
    prefix_bytes = {
        "match":   "00000000",
        "account": "11111111",
        "session": "22222222",
        "quest":   "00000001",
        "deck":    "33333333",
        "draft":   "44444444",
    }.get(prefix, "FFFFFFFF")
    return f"{prefix_bytes}-0000-4000-8000-{n:012d}"


def _sanitize_text(text: str) -> str:
    """Apply ADR-041 G3 PII-replacement rules to a raw JSON text string."""
    # Replace screen names (e.g. "RealPlayer#12345") with stable fakes.
    text = _SCREEN_NAME_RE.sub(lambda m: _replace_screen_name(m.group(0)), text)

    # Replace UUIDs with stable deterministic fakes, keyed by first occurrence.
    def _replace_uuid(m):
        real = m.group(0).lower()
        if real not in _UUID_MAP:
            # Heuristic: determine which slot based on surrounding context.
            # For simplicity, map every UUID to the account slot on first sight.
            _UUID_MAP[real] = _next_fake_uuid("account")
        return _UUID_MAP[real]

    text = _UUID_RE.sub(_replace_uuid, text)

    return text


_SEEN_NAMES: dict = {}


def _replace_screen_name(raw: str) -> str:
    """Return a stable fake screen name for a real one."""
    if raw not in _SEEN_NAMES:
        n = len(_SEEN_NAMES) + 1
        _SEEN_NAMES[raw] = f'"TestPlayer#0000{n}"'
    return _SEEN_NAMES[raw]


def _sanitize_obj(obj: dict, sanitize: bool) -> str:
    """Serialise obj to a JSON string and optionally sanitise PII."""
    text = json.dumps(obj, separators=(",", ":"))
    if sanitize:
        text = _sanitize_text(text)
    return text


# ---------------------------------------------------------------------------
# Variant post-processing
# ---------------------------------------------------------------------------

def _apply_variant(text: str, variant: str) -> str:
    """Post-process a match-completed fixture to produce a regression variant."""
    if variant == "empty-format":
        # Set the eventId / format field to empty string.
        obj = json.loads(text)
        _set_nested(obj, ["matchGameRoomStateChangedEvent", "gameRoomInfo",
                           "gameRoomConfig", "reservedPlayers"], _clear_event_id)
        return json.dumps(obj, separators=(",", ":"))
    elif variant == "missing-id":
        obj = json.loads(text)
        try:
            fmr = (obj["matchGameRoomStateChangedEvent"]["gameRoomInfo"]["finalMatchResult"])
            fmr["matchId"] = ""
        except (KeyError, TypeError):
            pass
        try:
            cfg = obj["matchGameRoomStateChangedEvent"]["gameRoomInfo"]["gameRoomConfig"]
            cfg["matchId"] = ""
        except (KeyError, TypeError):
            pass
        return json.dumps(obj, separators=(",", ":"))
    return text


def _clear_event_id(players):
    """Remove eventId from every entry in a reservedPlayers list."""
    if not isinstance(players, list):
        return players
    for p in players:
        if isinstance(p, dict) and "eventId" in p:
            p.pop("eventId")
    return players


def _set_nested(obj, path, fn):
    """Walk path and apply fn to the value at the final key."""
    cur = obj
    for key in path[:-1]:
        if not isinstance(cur, dict) or key not in cur:
            return
        cur = cur[key]
    final = path[-1]
    if isinstance(cur, dict) and final in cur:
        cur[final] = fn(cur[final])


# ---------------------------------------------------------------------------
# Main extraction logic
# ---------------------------------------------------------------------------

def _classify(obj: dict, key_map) -> str | None:
    """Return the output filename for the event class, or None."""
    for key, filename in key_map:
        if key in obj:
            return filename
    if _is_collection_entry(obj):
        return "collection-updated.log"
    return None


def _is_match_completed(obj: dict) -> bool:
    """Return True when the entry is a MatchGameRoomStateType_MatchCompleted event."""
    try:
        state = (obj["matchGameRoomStateChangedEvent"]
                   ["gameRoomInfo"]["stateType"])
        return state == "MatchGameRoomStateType_MatchCompleted"
    except (KeyError, TypeError):
        return False


def extract(
    source,
    output_dir: pathlib.Path,
    sanitize: bool,
    first_only: bool,
    variant: str | None,
    key_map,
):
    """
    Read Player.log lines from source (file-like), write fixture files to
    output_dir. Returns a dict mapping filename -> number of occurrences written.
    """
    output_dir.mkdir(parents=True, exist_ok=True)
    written: dict = {}

    for raw_line in source:
        line = raw_line.strip()
        if not line.startswith("{") and not line.startswith("["):
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(obj, dict):
            continue

        # For match events, require MatchGameRoomStateType_MatchCompleted.
        if "matchGameRoomStateChangedEvent" in obj and not _is_match_completed(obj):
            continue

        filename = _classify(obj, key_map)
        if filename is None:
            continue

        if first_only and filename in written:
            continue

        text = _sanitize_obj(obj, sanitize)

        if variant and filename == "match-completed.log":
            text = _apply_variant(text, variant)
            variant_stem, _, ext = filename.rpartition(".")
            filename = f"{variant_stem}-{variant}.{ext}"

        out_path = output_dir / filename
        out_path.write_text(text + "\n", encoding="utf-8")
        written[filename] = written.get(filename, 0) + 1

    return written


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def _parse_args(argv=None):
    p = argparse.ArgumentParser(
        description="Extract and sanitise MTGA Player.log event fixtures."
    )
    p.add_argument(
        "--input", "-i",
        type=pathlib.Path,
        default=None,
        help="Path to Player.log. Omit to read from stdin.",
    )
    p.add_argument(
        "--output-dir", "-o",
        type=pathlib.Path,
        default=pathlib.Path("./corpus-raw"),
        help="Directory to write fixture files (default: ./corpus-raw).",
    )
    p.add_argument(
        "--sanitize",
        action="store_true",
        help="Apply ADR-041 G3 PII-replacement rules.",
    )
    p.add_argument(
        "--first-only",
        action="store_true",
        help="Write only the first occurrence of each event class.",
    )
    p.add_argument(
        "--variant",
        choices=["empty-format", "missing-id"],
        default=None,
        help=(
            "Post-process the match-completed fixture to produce a regression "
            "variant: 'empty-format' clears eventId; 'missing-id' clears matchId."
        ),
    )
    p.add_argument(
        "--key-map",
        nargs="*",
        metavar="KEY=FILE",
        default=[],
        help=(
            "Additional KEY=FILE pairs appended to the default key map. "
            "Use this when MTGA renames a top-level log key."
        ),
    )
    return p.parse_args(argv)


def _build_key_map(extra: list) -> list:
    km = list(DEFAULT_KEY_MAP)
    for pair in extra:
        if "=" not in pair:
            print(f"WARNING: ignoring invalid --key-map entry: {pair!r}", file=sys.stderr)
            continue
        key, _, filename = pair.partition("=")
        km.append((key.strip(), filename.strip()))
    return km


def main(argv=None):
    args = _parse_args(argv)
    key_map = _build_key_map(args.key_map)

    if args.input:
        with args.input.open(encoding="utf-8", errors="replace") as f:
            written = extract(f, args.output_dir, args.sanitize, args.first_only,
                              args.variant, key_map)
    else:
        written = extract(sys.stdin, args.output_dir, args.sanitize, args.first_only,
                          args.variant, key_map)

    for filename, count in sorted(written.items()):
        print(f"  wrote {count}x → {args.output_dir / filename}")

    if not written:
        print("WARNING: no fixture lines extracted. Check --input path or log content.",
              file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
