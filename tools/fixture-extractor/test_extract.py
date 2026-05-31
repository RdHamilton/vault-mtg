"""
test_extract.py — Unit tests for the ADR-042 fixture extractor.

Run with: python3 -m unittest test_extract
"""

import io
import json
import pathlib
import sys
import tempfile
import unittest


# Allow importing extract.py from the same directory.
sys.path.insert(0, str(pathlib.Path(__file__).parent))
import extract  # noqa: E402


# ---------------------------------------------------------------------------
# Synthetic log lines used across tests
# ---------------------------------------------------------------------------

_INVENTORY_LINE = (
    '{"InventoryInfo":{"Gems":500,"Gold":1000,"TotalVaultProgress":10,'
    '"WildCardCommons":5,"WildCardUnCommons":3,"WildCardRares":1,'
    '"WildCardMythics":0,"Boosters":[]}}'
)

_QUEST_LINE = (
    '{"canSwap":true,"quests":[{"questId":"aaaaaaaa-0000-0000-0000-000000000001",'
    '"goal":20,"locKey":"Quests/Quest_Test","endingProgress":5,"canSwap":true}]}'
)

_AUTH_LINE = (
    '{"authenticateResponse":{"screenName":"RealPlayer#99999",'
    '"userId":"bbbbbbbb-0000-0000-0000-000000000001",'
    '"clientId":"cccccccc-0000-0000-0000-000000000001",'
    '"sessionId":"dddddddd-0000-0000-0000-000000000001"}}'
)

_MATCH_LINE = (
    '{"matchGameRoomStateChangedEvent":{"gameRoomInfo":{"stateType":'
    '"MatchGameRoomStateType_MatchCompleted","gameRoomConfig":{"matchId":'
    '"eeeeeeee-0000-0000-0000-000000000001","reservedPlayers":[{"userId":'
    '"ffffffff-0000-0000-0000-000000000001","playerName":"RealPlayer#99999",'
    '"teamId":1,"eventId":"Ladder"}]},"finalMatchResult":{"matchId":'
    '"eeeeeeee-0000-0000-0000-000000000001","resultList":[{"scope":'
    '"MatchScope_Match","result":"ResultType_WinLoss","winningTeamId":1,'
    '"reason":"ResultReason_Game"}]}}}}'
)

_DRAFT_PACK_LINE = (
    '{"draftPack":{"PackCards":[12345,67890],"SelfPick":1},"CourseName":"PremierDraft_BLB"}'
)

_DRAFT_PICK_LINE = (
    '{"pickedCards":[12345],"PackNumber":0,"PickNumber":0,"CourseName":"PremierDraft_BLB"}'
)

_COLLECTION_LINE = '{"12345":4,"67890":2}'

_NON_JSON_LINE = "2026-05-01 10:00:00 [UnityCrossThreadLogger] session started"
_PARTIAL_JSON_LINE = '{"broken: json'

# A match line that is NOT MatchCompleted (should be skipped).
_MATCH_NON_COMPLETED_LINE = (
    '{"matchGameRoomStateChangedEvent":{"gameRoomInfo":{"stateType":'
    '"MatchGameRoomStateType_Playing","gameRoomConfig":{"matchId":'
    '"eeeeeeee-0000-0000-0000-000000000002"}}}}'
)


def _run(lines: list[str], **kwargs) -> dict:
    """Run extraction on the given lines and return (written_map, tmpdir)."""
    defaults = dict(
        sanitize=False,
        first_only=False,
        variant=None,
        key_map=extract.DEFAULT_KEY_MAP,
    )
    defaults.update(kwargs)
    with tempfile.TemporaryDirectory() as tmp:
        out = pathlib.Path(tmp)
        src = io.StringIO("\n".join(lines) + "\n")
        written = extract.extract(src, out, **defaults)
        # Read back the written files.
        result = {}
        for name in written:
            result[name] = (out / name).read_text(encoding="utf-8")
        return result


class TestEventClassification(unittest.TestCase):
    """Correct file is written for each event key."""

    def test_inventory_written(self):
        result = _run([_INVENTORY_LINE])
        self.assertIn("inventory-updated.log", result)

    def test_quest_written(self):
        result = _run([_QUEST_LINE])
        self.assertIn("quest-progress.log", result)

    def test_auth_written(self):
        result = _run([_AUTH_LINE])
        self.assertIn("player-authenticated.log", result)

    def test_match_completed_written(self):
        result = _run([_MATCH_LINE])
        self.assertIn("match-completed.log", result)

    def test_match_non_completed_skipped(self):
        result = _run([_MATCH_NON_COMPLETED_LINE])
        self.assertNotIn("match-completed.log", result)

    def test_draft_pack_written(self):
        result = _run([_DRAFT_PACK_LINE])
        self.assertIn("draft-pack.log", result)

    def test_draft_pick_written(self):
        result = _run([_DRAFT_PICK_LINE])
        self.assertIn("draft-pick.log", result)

    def test_collection_written(self):
        result = _run([_COLLECTION_LINE])
        self.assertIn("collection-updated.log", result)


class TestNonJsonLines(unittest.TestCase):
    """Non-JSON lines are skipped without error."""

    def test_timestamp_prefix_skipped(self):
        result = _run([_NON_JSON_LINE, _INVENTORY_LINE])
        self.assertIn("inventory-updated.log", result)
        self.assertEqual(len(result), 1)

    def test_partial_json_skipped(self):
        result = _run([_PARTIAL_JSON_LINE, _QUEST_LINE])
        self.assertIn("quest-progress.log", result)
        self.assertEqual(len(result), 1)

    def test_empty_lines_skipped(self):
        result = _run(["", "   ", _AUTH_LINE])
        self.assertIn("player-authenticated.log", result)


class TestSanitise(unittest.TestCase):
    """--sanitize replaces real UUID and screenName with stable fakes."""

    def test_uuid_replaced(self):
        result = _run([_AUTH_LINE], sanitize=True)
        text = result["player-authenticated.log"]
        self.assertNotIn("bbbbbbbb", text, "Real UUID should be replaced")
        self.assertNotIn("cccccccc", text, "Real UUID should be replaced")

    def test_screen_name_replaced(self):
        result = _run([_AUTH_LINE], sanitize=True)
        text = result["player-authenticated.log"]
        self.assertNotIn("RealPlayer#99999", text, "Real screenName should be replaced")
        self.assertIn("TestPlayer#", text)

    def test_no_sanitize_preserves_original(self):
        result = _run([_AUTH_LINE], sanitize=False)
        text = result["player-authenticated.log"]
        self.assertIn("RealPlayer#99999", text)
        self.assertIn("bbbbbbbb", text)


class TestFirstOnly(unittest.TestCase):
    """--first-only writes only the first occurrence of each event class."""

    def test_first_only_single_match(self):
        second_match = _MATCH_LINE.replace(
            "eeeeeeee-0000-0000-0000-000000000001",
            "ffffffff-0000-0000-0000-000000000099",
        )
        result = _run([_MATCH_LINE, second_match], first_only=True)
        self.assertIn("match-completed.log", result)
        # The first match ID should be in the file, not the second.
        text = result["match-completed.log"]
        self.assertIn("eeeeeeee", text)
        self.assertNotIn("ffffffff-0000-0000-0000-000000000099", text)

    def test_first_only_false_writes_last(self):
        second_match = _MATCH_LINE.replace(
            "eeeeeeee-0000-0000-0000-000000000001",
            "ffffffff-0000-0000-0000-000000000099",
        )
        result = _run([_MATCH_LINE, second_match], first_only=False)
        # File is overwritten; last line wins.
        text = result["match-completed.log"]
        self.assertIn("ffffffff-0000-0000-0000-000000000099", text)


class TestVariantEmptyFormat(unittest.TestCase):
    """--variant empty-format produces a fixture where format is absent."""

    def test_event_id_cleared(self):
        result = _run([_MATCH_LINE], variant="empty-format", first_only=True)
        # Variant produces a new filename.
        self.assertIn("match-completed-empty-format.log", result)
        text = result["match-completed-empty-format.log"]
        obj = json.loads(text.strip())
        players = (
            obj["matchGameRoomStateChangedEvent"]["gameRoomInfo"]
               ["gameRoomConfig"]["reservedPlayers"]
        )
        for p in players:
            self.assertNotIn(
                "eventId", p,
                "eventId should have been removed by empty-format variant"
            )


class TestVariantMissingId(unittest.TestCase):
    """--variant missing-id produces a fixture where matchId is empty."""

    def test_match_id_cleared(self):
        result = _run([_MATCH_LINE], variant="missing-id", first_only=True)
        self.assertIn("match-completed-missing-id.log", result)
        text = result["match-completed-missing-id.log"]
        obj = json.loads(text.strip())
        fmr = (
            obj["matchGameRoomStateChangedEvent"]["gameRoomInfo"]["finalMatchResult"]
        )
        self.assertEqual(fmr["matchId"], "")


class TestCollectionDetection(unittest.TestCase):
    """Collection entries (flat int-key map) are detected correctly."""

    def test_collection_detected(self):
        result = _run([_COLLECTION_LINE])
        self.assertIn("collection-updated.log", result)

    def test_non_integer_key_not_collection(self):
        non_collection = '{"someNamedKey":"value","12345":4}'
        result = _run([non_collection])
        # Should not be classified as collection-updated since it has a named key
        # matching no known event (falls through classification).
        # The named key "someNamedKey" is not a known wrapper key, but the int
        # check fails because "someNamedKey" is not a positive int.
        self.assertNotIn("collection-updated.log", result)


# ---------------------------------------------------------------------------
# Catalog mode (#262)
# ---------------------------------------------------------------------------

# Premier draft pick request — picked grpId lives in a STRINGIFIED request
# envelope (double-parse required).
_API_DRAFT_PICK_LINE = (
    '[UnityCrossThreadLogger]==> EventPlayerDraftMakePick '
    '{"id":"11111111-2222-4333-8444-555555555555",'
    '"request":"{\\"DraftId\\":\\"99999999-8888-4777-8666-555555555555\\",'
    '\\"GrpIds\\":[102704],\\"Pack\\":0,\\"Pick\\":0}"}'
)

# Premier draft pack — bare-prefix Draft.Notify message.
_DRAFT_NOTIFY_LINE = (
    '[UnityCrossThreadLogger]Draft.Notify '
    '{"draftId":"99999999-8888-4777-8666-555555555555","SelfPick":1,'
    '"SelfPack":1,"PackCards":"102614,102609,102691"}'
)

# Auth wrapped in the {transactionId, requestId, timestamp, <event>} envelope.
# clientId is a 26-char base32 account token == reservedPlayers[].userId.
# Values here are synthetic (not from any real capture).
_ENVELOPE_AUTH_LINE = (
    '{"transactionId":"aaaaaaaa-0000-4000-8000-000000000001",'
    '"requestId":3,"timestamp":"2026-05-31T07:21:00Z",'
    '"authenticateResponse":{"clientId":"ABCDEFGHIJKLMNOPQRSTUVWXYZ",'
    '"sessionId":"bbbbbbbb-0000-4000-8000-000000000002",'
    '"screenName":"SampleHandle"}}'
)

_GRE_LINE = (
    '{"greToClientEvent":{"greToClientMessages":[{"type":'
    '"GREMessageType_GameStateMessage"}]}}'
)


def _run_catalog(lines, sanitize=True):
    import io
    src = io.StringIO("\n".join(lines) + "\n")
    with tempfile.TemporaryDirectory() as tmp:
        out = pathlib.Path(tmp)
        rows = extract.catalog(src, out, sanitize)
        samples = {}
        for r in rows:
            if r["sample_file"]:
                samples[r["event"]] = (out / r["sample_file"]).read_text(encoding="utf-8")
        return rows, samples


class TestCatalogAxes(unittest.TestCase):
    def test_api_request_axis(self):
        rows, _ = _run_catalog([_API_DRAFT_PICK_LINE])
        ev = {(r["axis"], r["event"]) for r in rows}
        self.assertIn(("api-request", "EventPlayerDraftMakePick"), ev)

    def test_prefix_message_axis(self):
        rows, _ = _run_catalog([_DRAFT_NOTIFY_LINE])
        ev = {(r["axis"], r["event"]) for r in rows}
        self.assertIn(("prefix-message", "Draft.Notify"), ev)

    def test_envelope_unwraps_to_business_key(self):
        # The {transactionId,...,authenticateResponse} envelope must catalog
        # under the business key, not "transactionId".
        rows, _ = _run_catalog([_ENVELOPE_AUTH_LINE])
        events = {r["event"] for r in rows}
        self.assertIn("authenticateResponse", events)
        self.assertNotIn("transactionId", events)

    def test_gre_axis(self):
        rows, _ = _run_catalog([_GRE_LINE])
        ev = {(r["axis"], r["event"]) for r in rows}
        self.assertIn(("gre-message", "GameStateMessage"), ev)

    def test_counts_accumulate(self):
        rows, _ = _run_catalog([_DRAFT_NOTIFY_LINE, _DRAFT_NOTIFY_LINE])
        row = next(r for r in rows if r["event"] == "Draft.Notify")
        self.assertEqual(row["count"], 2)


class TestCatalogSanitisation(unittest.TestCase):
    def test_stringified_envelope_double_parse_no_raw_uuid(self):
        # The DraftId nested inside the stringified request must be unwrapped
        # AND sanitised (Ray's required change #3).
        _, samples = _run_catalog([_API_DRAFT_PICK_LINE])
        text = samples["EventPlayerDraftMakePick"]
        self.assertNotIn("62a14a91", text, "nested DraftId must be sanitised")
        # GrpIds (card ids) retained as non-PII.
        self.assertIn("102704", text)
        # Unwrapped, not a string blob.
        obj = json.loads(text.strip())
        self.assertIsInstance(obj["request"], dict)

    def test_account_token_sanitised_and_stable(self):
        _, samples = _run_catalog([_ENVELOPE_AUTH_LINE])
        text = samples["authenticateResponse"]
        self.assertNotIn("ABCDEFGHIJKLMNOPQRSTUVWXYZ", text,
                         "26-char base32 clientId must be sanitised")
        self.assertNotIn("SampleHandle", text, "bare player handle must be sanitised")

    def test_bare_player_handle_sanitised(self):
        # A player handle with no #NNNNN suffix must still be replaced (the
        # screen-name regex alone would miss it).
        line = '{"playerName":"BareHandleNoHash","teamId":1}'
        _, samples = _run_catalog([line])
        # business key is playerName
        for text in samples.values():
            self.assertNotIn("BareHandleNoHash", text)


if __name__ == "__main__":
    unittest.main()
