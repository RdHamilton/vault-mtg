# ADR-042 Golden Fixture Corpus

## Overview

This corpus provides the canonical golden fixtures for the ADR-042 data-pipeline regression test architecture. It is the Layer 1 foundation: all downstream test layers (Layer 2 contract gate, Layer 3 projection integration, Layer 4 staging rehearsal) consume fixtures from this directory.

## MTGA Version

| Field | Value |
|---|---|
| MTGA Client Version | 2026.59.20 (build 2026.59.20.4846.1277160) |
| Capture Date | 2026-05-31 (Premier-draft session: Player_capture_premier_20260531T072112Z) |
| Refresh SLA | 24h after MTGA version bump triggers drift canary |

The full build string is in `mtga-version.txt`.

## Directory Layout

```
corpus/
  MANIFEST                  # pipe-delimited: file, event_class, provenance, pii_status, mtga_version
  README.md                 # this file
  mtga-version.txt          # MTGA build string
  player-log/               # raw Player.log snippets (one JSON line per event class)
  daemon-emit/              # contract.DaemonEvent wire JSON (daemon -> BFF)
  db-expected/              # expected DB row values after projection worker runs
  api-expected/             # expected BFF API response shapes
```

## Provenance

Fixtures use one of three provenance tags (recorded in MANIFEST):

- **REAL** — extracted from a live MTGA session capture, sanitised per ADR-041 G3.
- **REAL-DERIVED** — constructed from REAL data (e.g. daemon-emit/ built from REAL player-log/), sanitised.
- **FORMAT-CONFIRMED** — synthetic but validated against the 2026.59.20 wire protocol via working parser tests. Requires promotion to REAL on the next live-session capture.
- **SYNTHETIC** — constructed test data for projection/API assertions; no MTGA origin.

### Promotion status (#262 audit, 2026-05-31 Premier capture)

Promoted to REAL / REAL-DERIVED from the 2026-05-31 Premier-draft session capture:

- `player-log/match-completed.log` → REAL
- `player-log/player-authenticated.log` → REAL (CORRECTED `{clientId,sessionId,screenName}` shape; `clientId == reservedPlayers[].userId`; no invented `userId`/`accountId` key)
- `player-log/deck-updated.log` → REAL
- `daemon-emit/match-completed.json` (+ `-empty-format`, `-missing-id` variants) → REAL-DERIVED (run through `logreader.ParseMatchCompletedEntry`)
- `daemon-emit/deck-updated.json` → REAL-DERIVED (run through `logreader.ParseDeckEntry`)

Still FORMAT-CONFIRMED (could NOT be promoted from this capture):

- `player-log/collection-updated.log` + `daemon-emit/collection-updated.json` — the capture contains **no** `PlayerInventoryGetPlayerCardsV3` collection snapshot (player did not open the collection screen). Awaits a capture exercising the collection surface.
- `player-log/draft-pack.log` + `player-log/draft-pick.log` + `daemon-emit/draft-pack.json` + `daemon-emit/draft-pick.json` — **GATED on the Premier draft classifier/parser fix.** The Layer-2 contract gate parses the player-log draft fixtures through `ParseDraftPack`/`ParseDraftPick`, which require the top-level `draftPack`/`pickedCards` keys; the daemon classifier gates on those same keys. In the Premier capture those keys appear 0 times — the real Premier pack is `Draft.Notify {draftId,SelfPick,SelfPack,PackCards}` and the real pick is the `EventPlayerDraftMakePick` request — so neither the player-log nor the daemon-emit draft fixtures can be promoted without diverging from (or breaking) the current parser. They are intentionally left until the draft-parser fix (sibling of #336) lands. The real Premier draft shapes are captured under `catalog/samples/` and documented in the taxonomy report.

Note: BotDraft (QuickDraft) draft support is a separate daemon gap tracked by #337 — its raw shapes are catalogued in `tools/fixture-extractor` catalog output (axes `api-request/api-response BotDraftDraftPick`, `json-key DraftPack`), not promoted here.

## PII Sanitisation

All fixtures follow ADR-041 G3 rules (applied programmatically by `tools/fixture-extractor/extract.py --sanitize`):

| Data type | Treatment |
|---|---|
| UUIDs (match / session / transaction / deck / draft IDs) | Stable fake UUIDs, deterministic by first sight |
| Account tokens (`clientId` / `userId`, 26-char base32) | Stable fake `TESTACCOUNT…` tokens — the same real value maps to the same fake, so `clientId == reservedPlayers[].userId` is preserved |
| Player / screen names (`playerName` / `screenName` / `displayName`) | Replaced by **field key**, not regex — MTGA handles may be bare (`SomeHandle`), classic (`Name#12345`), or malformed; all → `TestPlayer#0000N` |
| `requestId` / timestamps / reward-reset timestamps | Zeroed / replaced with fixed epoch (`2026-01-01T00:00:00Z`) |
| GRP IDs / Arena card IDs (incl. `PackCards`, `GrpIds`) | Retained (non-PII per ADR-041 risk assessment) |
| Gem / Gold / WildCard counts | Retained (game resource values, not personally identifying) |
| Cosmetic IDs (`PreferredCosmetics`, sleeves, avatars) | Stripped |

PII is sanitised in two passes (catalog mode): a recursive key-walk that fixes
PII values **after** stringified envelopes (`request` / `Payload`) are unwrapped,
plus the legacy text-regex pass. This catches account tokens and handles nested
inside formerly-stringified envelopes that a naive top-level scan would miss.

Raw captures are never committed. Only sanitised output is committed.

## Corpus Refresh Procedure

1. Open MTGA and play at least one match and one draft.
2. Copy `Player.log` from `~/Library/Logs/Wizards Of The Coast/MTGA/`.
3. Run `python3 tools/fixture-extractor/extract.py --input Player.log --output-dir /tmp/corpus-raw --sanitize --first-only`.
4. Copy the sanitised output to replace the FORMAT-CONFIRMED fixtures in `player-log/`.
5. Rebuild the `daemon-emit/` fixtures from the new `player-log/` fixtures.
6. Update MANIFEST: change provenance from FORMAT-CONFIRMED to REAL, update mtga_version column.
7. Update `mtga-version.txt`.
8. Run `go test ./services/daemon/testdata/corpus/...` to verify corpus loads.
9. Submit a PR with Sarah security review on the fixture files (S-07 gate).

## Layer Consumption

| Layer | Consumes | Purpose |
|---|---|---|
| Layer 2 (contract gate) | `player-log/` + `daemon-emit/` | Assert parser output matches wire format |
| Layer 3 (projection integration) | `daemon-emit/` + `db-expected/` + `api-expected/` | Assert projection worker + API read correctness |
| Layer 4 (staging rehearsal) | `daemon-emit/` + `db-expected/` + `api-expected/` | Manual staging validation via SSM-authenticated psql |

## Related

- ADR-042: `vault-mtg-docs/engineering/architecture/ADR-042-data-pipeline-regression-test-architecture.md`
- ADR-041 (PII): `vault-mtg-docs/engineering/architecture/ADR-041-fixture-pii-sanitisation.md`
- Extraction tool: `tools/fixture-extractor/extract.py`
- Tickets: #243 (Layer 1), #244 (Layer 2), #245 (Layer 3a), #246 (Layer 3b), #247 (Layer 4)
