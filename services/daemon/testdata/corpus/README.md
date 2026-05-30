# ADR-042 Golden Fixture Corpus

## Overview

This corpus provides the canonical golden fixtures for the ADR-042 data-pipeline regression test architecture. It is the Layer 1 foundation: all downstream test layers (Layer 2 contract gate, Layer 3 projection integration, Layer 4 staging rehearsal) consume fixtures from this directory.

## MTGA Version

| Field | Value |
|---|---|
| MTGA Client Version | 2026.59.20 (build 2026.59.20.4846.1277160) |
| Capture Date | 2026-05-30 |
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

### Fixtures requiring promotion to REAL

The following fixtures are currently FORMAT-CONFIRMED and must be promoted to REAL on the next
live MTGA session (match played + draft played). See follow-up issue for tracking:

- `player-log/match-completed.log`
- `player-log/draft-pack.log`
- `player-log/draft-pick.log`
- `player-log/collection-updated.log`
- `player-log/player-authenticated.log`
- `daemon-emit/match-completed.json`
- `daemon-emit/match-completed-empty-format.json`
- `daemon-emit/match-completed-missing-id.json`
- `daemon-emit/draft-pack.json`
- `daemon-emit/draft-pick.json`

## PII Sanitisation

All fixtures follow ADR-041 G3 rules (applied programmatically by `tools/fixture-extractor/extract.py --sanitize`):

| Data type | Treatment |
|---|---|
| Match IDs | Stable fake UUIDs: `00000000-0000-4000-8000-0000000000NN` |
| Account / client / session IDs | `test-account-001`, stable fake UUIDs |
| Player / screen names | `LocalPlayer#00001`, `Opponent#00002` |
| Quest UUIDs | Stable fakes: `00000001-0000-4000-8000-0000000000NN` |
| Deck IDs | Stable fakes: `33333333-0000-4000-8000-0000000000NN` |
| Draft IDs | Stable fakes: `44444444-0000-0000-0000-0000000000NN` |
| GRP IDs / Arena card IDs | Retained (non-PII per ADR-041 risk assessment) |
| Gem / Gold / WildCard counts | Retained (game resource values, not personally identifying) |
| Cosmetic IDs | Stripped (excluded for minimal footprint) |

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
