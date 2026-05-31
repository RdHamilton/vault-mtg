#!/usr/bin/env python3
"""
extract.py — ADR-042 Player.log fixture extractor.

Two modes:

1. Curated extraction (legacy, default): identifies MTGA log events by
   top-level JSON key, writes one sanitised fixture file per known event class
   to --output-dir. Drives the Layer 2/3 corpus fixtures.

2. Catalog mode (--catalog, #262): enumerates EVERY distinct event type across
   the four structural axes MTGA emits — API request/response markers
   (==> / <==), GREMessageType_*, ClientMessageType_*,
   MatchGameRoomStateType_*, and top-level single-line JSON keys — with no
   fixed allowlist. Emits catalog.json + catalog.md (each distinct event type →
   occurrence count → ONE sanitised sample) plus one sanitised sample file per
   distinct event type under samples/.

Stdlib only: json, pathlib, argparse, re, sys.

Usage:
    # Curated (legacy):
    python3 extract.py --input Player.log --output-dir ./corpus-raw --sanitize --first-only
    python3 extract.py --input Player.log --output-dir ./corpus-raw --sanitize --variant empty-format
    python3 extract.py --input Player.log --output-dir ./corpus-raw --sanitize --variant missing-id

    # Catalog (event-discovery audit, #262):
    python3 extract.py --input Player.log --output-dir ./catalog --catalog --sanitize
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


# MTGA account identifiers are 26-char uppercase base32 tokens (e.g. the
# clientId / reservedPlayers[].userId), NOT UUIDs — the UUID regex misses them.
# We bound the match to 26 chars to avoid clobbering set codes / card-name
# tokens. Stable-fake map keyed by first occurrence.
_ACCOUNT_ID_RE = re.compile(r'"[A-Z0-9]{26}"')
_ACCOUNT_ID_MAP: dict = {}

# Keys whose string VALUE is PII and must be replaced with a stable fake,
# regardless of the value's textual shape (catches base32 account ids, ISO
# timestamps, integer request ids, etc. that the UUID/screen-name regexes miss).
# Applied during a recursive JSON walk so nested + stringified envelopes are
# covered after the envelope is unwrapped.
_PII_VALUE_KEYS = {
    "clientId":      ("account", "ACCOUNTID0000000000000000A"),
    "userId":        ("account", None),   # same slot as clientId so they stay equal
    "requestId":     ("request", "0"),
    "timestamp":     ("ts", "2026-01-01T00:00:00Z"),
    "_dailyRewardResetTimestamp":  ("ts", "2026-01-01T00:00:00Z"),
    "_weeklyRewardResetTimestamp": ("ts", "2026-01-01T00:00:00Z"),
    "ServerTime":    ("ts", "2026-01-01T00:00:00Z"),
    # Player handles are PII regardless of textual shape — MTGA handles can be
    # bare ("SomeHandle"), classic ("Name#12345"), or malformed, so the
    # screen-name *regex* alone is insufficient. Replace by key.
    "playerName":    ("name", None),
    "screenName":    ("name", None),
    "displayName":   ("name", None),
}
# Stable player-name fakes, deterministic by first sight.
_NAME_VALUE_MAP: dict = {}


def _fake_name(real: str) -> str:
    """Return a stable fake player handle for a real one."""
    if real not in _NAME_VALUE_MAP:
        n = len(_NAME_VALUE_MAP) + 1
        _NAME_VALUE_MAP[real] = f"TestPlayer#{n:05d}"
    return _NAME_VALUE_MAP[real]
# Stable account-token fakes (26-char base32, deterministic by first sight).
_ACCOUNT_TOKEN_MAP: dict = {}


def _fake_account_token(real: str) -> str:
    """Return a stable 26-char base32 fake for a real account token."""
    if real not in _ACCOUNT_TOKEN_MAP:
        n = len(_ACCOUNT_TOKEN_MAP) + 1
        # 26-char uppercase base32-ish: TESTACCOUNT + zero-padded index.
        _ACCOUNT_TOKEN_MAP[real] = ("TESTACCOUNT" + f"{n:015d}")[:26]
    return _ACCOUNT_TOKEN_MAP[real]


def _sanitize_value_keys(obj):
    """Recursively replace PII values keyed by field name, in place.

    Also unwraps stringified-JSON envelope fields (request/Payload/PackCards)
    so account ids nested inside a JSON string get scanned too.
    """
    if isinstance(obj, dict):
        for k, v in list(obj.items()):
            if k in _PII_VALUE_KEYS and isinstance(v, (str, int)):
                slot, _ = _PII_VALUE_KEYS[k]
                if slot == "account":
                    obj[k] = _fake_account_token(str(v))
                elif slot == "request":
                    obj[k] = 0
                elif slot == "ts":
                    obj[k] = "2026-01-01T00:00:00Z"
                elif slot == "name":
                    obj[k] = _fake_name(str(v))
            else:
                _sanitize_value_keys(v)
    elif isinstance(obj, list):
        for item in obj:
            _sanitize_value_keys(item)


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

    # Replace 26-char base32 account tokens (clientId / userId) with stable
    # fakes. Runs after UUID replacement so it cannot touch fake UUIDs.
    def _replace_account(m):
        return f'"{_fake_account_token(m.group(0).strip(chr(34)))}"'

    text = _ACCOUNT_ID_RE.sub(_replace_account, text)

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
# Catalog mode (#262): enumerate EVERY distinct event type across four axes.
# ---------------------------------------------------------------------------

# API request/response marker: "==> Method(txnId) {body}" or "<== Method(txnId)".
# The optional [UnityCrossThreadLogger] prefix is stripped first.
_API_MARKER_RE = re.compile(
    r'^(==>|<==)\s+(\w+)(?:\(([0-9a-fA-F-]+)\))?\s*(.*)$'
)
# Single-line message-type tokens we enumerate as their own axes.
_GRE_RE = re.compile(r'GREMessageType_(\w+)')
_CLIENT_MSG_RE = re.compile(r'ClientMessageType_(\w+)')
_MATCH_ROOM_RE = re.compile(r'MatchGameRoomStateType_(\w+)')
_LOGGER_PREFIX_RE = re.compile(r'^\[UnityCrossThreadLogger\]')
# A bare-prefix message line: "Draft.Notify {json}" / "Event.X {json}" — no
# ==> / <== arrow. Captured as its own axis so Premier Draft.Notify packs and
# other notification messages aren't lost.
_PREFIX_MSG_RE = re.compile(r'^([A-Za-z][\w.]*\.[A-Za-z][\w.]*)\s+(\{.*\})\s*$')
# Envelope wrapper keys that are NOT the business event — when a single-line
# JSON object carries one of these wrappers, classify by the FIRST non-wrapper
# key so distinct business events aren't collapsed under "transactionId".
_ENVELOPE_KEYS = frozenset({
    "transactionId", "requestId", "timestamp",
})


def _business_key(obj: dict) -> str:
    """Return the most representative top-level key of a single-line event.

    Skips the {transactionId, requestId, timestamp} envelope so the underlying
    business event (authenticateResponse, matchGameRoomStateChangedEvent,
    quests, …) becomes the catalog entry rather than the wrapper.
    """
    non_env = [k for k in obj if k not in _ENVELOPE_KEYS]
    if non_env:
        return non_env[0]
    return next(iter(obj))


def _unwrap_stringified(obj):
    """Recursively parse any string value that is itself JSON, in place.

    Real MTGA envelopes nest the true payload inside a stringified field
    (request / Payload). Unwrapping lets the PII scanner and the catalog see
    the real structure. Returns the (possibly mutated) object.
    """
    if isinstance(obj, dict):
        for k, v in list(obj.items()):
            if isinstance(v, str):
                s = v.strip()
                if s.startswith("{") or s.startswith("["):
                    try:
                        obj[k] = _unwrap_stringified(json.loads(v))
                        continue
                    except (json.JSONDecodeError, ValueError):
                        pass
            else:
                _unwrap_stringified(v)
    elif isinstance(obj, list):
        for i, item in enumerate(obj):
            obj[i] = _unwrap_stringified(item)
    return obj


def _sanitize_catalog_obj(obj, sanitize: bool) -> str:
    """Unwrap stringified envelopes, key-sanitise, then text-sanitise.

    The double pass (key-walk + text-regex) ensures account ids nested inside
    formerly-stringified envelopes are caught (Ray's required change #3).
    """
    obj = _unwrap_stringified(obj)
    if sanitize:
        _sanitize_value_keys(obj)
    return _sanitize_obj(obj, sanitize)


def catalog(source, output_dir: pathlib.Path, sanitize: bool):
    """Enumerate every distinct event type across the four structural axes.

    Writes catalog.json, catalog.md, and one sample per distinct event type
    under samples/. Returns the catalog list (axis, event, count, sample_file).
    """
    samples_dir = output_dir / "samples"
    samples_dir.mkdir(parents=True, exist_ok=True)

    # (axis, event) -> {"count": int, "sample": str|None}
    counts: dict = {}
    # txnId -> request body text (so a <== response can borrow the request body
    # when its own body is on the following line / absent).
    pending_request: dict = {}
    lines_iter = iter(source)
    prev_marker = None  # (axis, event, txn) awaiting a body on the next line

    def _bump(axis, event, sample_text=None):
        key = (axis, event)
        slot = counts.setdefault(key, {"count": 0, "sample": None})
        slot["count"] += 1
        if slot["sample"] is None and sample_text is not None:
            slot["sample"] = sample_text

    for raw_line in lines_iter:
        line = raw_line.rstrip("\n")
        stripped = _LOGGER_PREFIX_RE.sub("", line.strip())

        # If a previous marker is awaiting its body and this line is a JSON
        # object/array, attach it as that marker's sample.
        if prev_marker is not None:
            if stripped.startswith("{") or stripped.startswith("["):
                axis, event, _txn = prev_marker
                try:
                    body = json.loads(stripped)
                    sample = _sanitize_catalog_obj(body, sanitize)
                    # backfill the sample if not yet set
                    slot = counts.get((axis, event))
                    if slot is not None and slot["sample"] is None:
                        slot["sample"] = sample
                except json.JSONDecodeError:
                    pass
            prev_marker = None

        # --- Axis 1: API request/response markers ---
        m = _API_MARKER_RE.match(stripped)
        if m:
            arrow, method, txn, body = m.group(1), m.group(2), m.group(3), m.group(4).strip()
            axis = "api-request" if arrow == "==>" else "api-response"
            sample_text = None
            if body and (body.startswith("{") or body.startswith("[")):
                try:
                    sample_text = _sanitize_catalog_obj(json.loads(body), sanitize)
                    if arrow == "==>" and txn:
                        pending_request[txn] = sample_text
                except json.JSONDecodeError:
                    pass
            elif arrow == "<==" and txn and txn in pending_request:
                # Response marker with no inline body: borrow request sample
                # only if its own next-line body is absent.
                sample_text = None
            _bump(axis, method, sample_text)
            if sample_text is None:
                prev_marker = (axis, method, txn)
            continue

        # --- Axes 2-4: message-type tokens (may appear inside larger lines) ---
        matched_token = False
        for axis, rx in (
            ("gre-message", _GRE_RE),
            ("client-message", _CLIENT_MSG_RE),
            ("match-room-state", _MATCH_ROOM_RE),
        ):
            tokens = rx.findall(stripped)
            if tokens:
                matched_token = True
                # Sample only when the whole line is a parseable JSON object.
                sample_text = None
                if stripped.startswith("{"):
                    try:
                        sample_text = _sanitize_catalog_obj(json.loads(stripped), sanitize)
                    except json.JSONDecodeError:
                        sample_text = None
                for tok in set(tokens):
                    _bump(axis, tok, sample_text)
        if matched_token:
            continue

        # --- Axis 5: bare-prefix notification messages (e.g. Draft.Notify) ---
        pm = _PREFIX_MSG_RE.match(stripped)
        if pm:
            method, body = pm.group(1), pm.group(2)
            try:
                sample_text = _sanitize_catalog_obj(json.loads(body), sanitize)
                _bump("prefix-message", method, sample_text)
                continue
            except json.JSONDecodeError:
                pass

        # --- Axis 6: single-line top-level JSON keys (envelope-unwrapped) ---
        if stripped.startswith("{"):
            try:
                obj = json.loads(stripped)
            except json.JSONDecodeError:
                continue
            if isinstance(obj, dict) and obj:
                biz_key = _business_key(obj)
                sample_text = _sanitize_catalog_obj(obj, sanitize)
                _bump("json-key", biz_key, sample_text)

    # Build the catalog list + write per-event sample files.
    catalog_list = []
    for (axis, event), slot in sorted(counts.items()):
        safe_event = re.sub(r'[^A-Za-z0-9_.-]', "_", event)
        sample_file = f"samples/{axis}__{safe_event}.json"
        sample_text = slot["sample"]
        if sample_text is not None:
            (output_dir / sample_file).write_text(sample_text + "\n", encoding="utf-8")
        else:
            sample_file = None
        catalog_list.append({
            "axis": axis,
            "event": event,
            "count": slot["count"],
            "sample_file": sample_file,
        })

    # catalog.json (machine).
    (output_dir / "catalog.json").write_text(
        json.dumps(catalog_list, indent=2) + "\n", encoding="utf-8"
    )

    # catalog.md (human table, grouped by axis).
    md = ["# Player.log Event Catalog", "",
          "Generated by `tools/fixture-extractor/extract.py --catalog`.",
          "All samples are PII-sanitised (ADR-041). Raw log never committed.", ""]
    by_axis: dict = {}
    for row in catalog_list:
        by_axis.setdefault(row["axis"], []).append(row)
    total = len(catalog_list)
    md.append(f"**Total distinct event types: {total}** across {len(by_axis)} axes.")
    md.append("")
    for axis in sorted(by_axis):
        rows = sorted(by_axis[axis], key=lambda r: (-r["count"], r["event"]))
        md.append(f"## Axis: `{axis}` ({len(rows)} distinct)")
        md.append("")
        md.append("| event | count | sample |")
        md.append("|---|---|---|")
        for r in rows:
            sample = f"`{r['sample_file']}`" if r["sample_file"] else "—"
            md.append(f"| `{r['event']}` | {r['count']} | {sample} |")
        md.append("")
    (output_dir / "catalog.md").write_text("\n".join(md) + "\n", encoding="utf-8")

    return catalog_list


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
        "--catalog",
        action="store_true",
        help=(
            "Catalog mode (#262): enumerate EVERY distinct event type across "
            "all four structural axes (no allowlist). Emits catalog.json, "
            "catalog.md, and one sanitised sample per event type under samples/."
        ),
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

    if args.catalog:
        if args.input:
            with args.input.open(encoding="utf-8", errors="replace") as f:
                rows = catalog(f, args.output_dir, args.sanitize)
        else:
            rows = catalog(sys.stdin, args.output_dir, args.sanitize)
        axes = sorted({r["axis"] for r in rows})
        print(f"  cataloged {len(rows)} distinct event types across {len(axes)} axes")
        print(f"  wrote {args.output_dir / 'catalog.json'} + catalog.md + samples/")
        if not rows:
            print("WARNING: no events cataloged. Check --input path or log content.",
                  file=sys.stderr)
        return 0

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
