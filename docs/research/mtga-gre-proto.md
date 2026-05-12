# MTGA Game Rules Engine — Protobuf Reference

`mtga-gre.proto` contains the reverse-engineered protobuf message definitions for
MTGA's internal Game Rules Engine (GRE). Source: HearthSim `proto-extractor` tool,
extracted from the Untapped.gg companion app.

## What it covers

- **GRE message envelope** — `GREMessage`, `GREMessageType`
- **Match state** — `GameStateMessage`, zones, permanents, players, life totals
- **Player actions** — `ActionsAvailableReq`, `SubmitAttackersReq`, `CastSpellReq`, etc.
- **Annotations** — game events emitted by the rules engine (damage, draw, trigger, etc.)
- **Draft** — `DraftPickReq`, `DraftPickResp`, booster pack contents

## What it does NOT cover

- `PlayerInventory` / collection data — separate subsystem, not in this proto
- Account / auth messages
- Store / economy messages

## Transport

MTGA connects via TCP to:
```text
frontdoor-mtga-production-<id>.w2.mtgarena.com:30010
```
Messages are length-prefixed protobuf. The outer envelope is `ClientMessage` /
`ServerMessage` containing one or more `GREMessage` payloads.

## Relevance to VaultMTG

| Feature | Uses GRE proto? |
|---------|----------------|
| Collection agent (Phase 1–5) | No — reads process memory directly |
| Live match overlay | Yes — would need GRE message interception |
| Real-time draft assistant | Yes — `DraftPickReq`/`Resp` messages |
| Post-game stats (current) | No — uses Player.log parsing |

The proto is committed here as a reference for future live-game features.
Do not check the Untapped companion's extracted copy into version control;
use this copy instead so it stays versioned with the codebase.
