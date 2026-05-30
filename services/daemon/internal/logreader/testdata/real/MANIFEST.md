# Real MTGA Player.log Fixture Manifest

## Session Metadata

| Field | Value |
|---|---|
| MTGA Client Version | 2026.59.20 (build 2026.59.20.4846.1277160) |
| Unity Engine Version | 2022.3.62f2 |
| Capture Date | 2026-05-29 |
| Platform | darwin_arm64 (macOS, Apple M4 Pro) |
| Source Files | `Player.log` and `Player-prev.log` from `~/Library/Logs/Wizards Of The Coast/MTGA/` |

## Fixture Files

| File | Event Class | Source | PII Status |
|---|---|---|---|
| `inventory_updated_2026.59.20.log` | inventory_updated | REAL — line 429, Player.log 2026-05-29 session | Sanitized: Cosmetics block removed (ArtStyle IDs), SeqId removed, Changes removed. Gem/Gold/WildCard counts are real game values (non-PII). |
| `quest_progress_2026.59.20.log` | quest_progress | REAL — line 615, Player.log 2026-05-29 session | Sanitized: questId UUIDs replaced with stable fake UUIDs (000...001, 000...002); tileResourceId and treasureResourceId stripped (internal resource pointers). locKey (quest name key), goal, progress, chestDescription are real game values (non-PII). |
| `match_completed_2026.59.20.log` | match_completed | FORMAT-CONFIRMED — format matches parser expectations for 2026.59.20 wire protocol. Current session was lobby-only (no matches played). All player identifiers are synthetic. | N/A — no real PII present. |
| `draft_pack_2026.59.20.log` | draft_pack | FORMAT-CONFIRMED — format matches parser expectations for 2026.59.20 wire protocol. Current session was lobby-only (no draft played). GRP IDs are real card IDs from MTGA card database. | N/A — no real PII present. |
| `draft_pick_2026.59.20.log` | draft_pick | FORMAT-CONFIRMED — see draft_pack note. | N/A — no real PII present. |
| `collection_updated_2026.59.20.log` | collection_updated | FORMAT-CONFIRMED — `PlayerInventoryGetPlayerCardsV3` response is a flat `{"grpId": qty, ...}` map. Collection snapshot derived from memory dump (see #224 fixture); GRP IDs are real. | N/A — GRP IDs are non-PII per ADR-041. |
| `authenticate_2026.59.20.log` | player_authenticated | FORMAT-CONFIRMED — format matches parser expectations. The authenticateResponse block appears during initial MTGA login, before detailed logging begins in 2026.59.20 sessions. All identifiers are synthetic. | N/A — no real PII present. |

## Sanitization Record

Applied to REAL-sourced fixtures:

- **Account identifiers**: none present in extracted fields (screenName, userId, clientId, sessionId were not present in the inventory or quest payloads)
- **Quest UUIDs**: replaced with `00000001-0000-4000-8000-00000000000N` stable fakes — these UUIDs could theoretically identify the server-assigned quest instance for this account
- **Cosmetic IDs**: removed from InventoryInfo (ArtStyle entries contain ArtId integers that are cosmetic product IDs, not user-identifiable, but excluded for minimal-footprint)
- **GRP IDs in collection snapshot**: retained — confirmed non-PII per ADR-041 risk assessment
- **Gem/Gold/WildCard counts**: retained — game resource values, not personally identifying

## Session Coverage Note

The 2026-05-29 session was a lobby-only session (deck manager navigation). No matches or drafts were played, so `match_completed`, `draft_pack`, `draft_pick`, `collection_updated` (card map), and `authenticateResponse` events did not appear in `Player.log` or `Player-prev.log`. Those fixtures use FORMAT-CONFIRMED provenance — they reflect the correct 2026.59.20 wire format as validated by:
1. The working Go parser tests that pass against the existing synthetic fixtures
2. Cross-reference with the logreader package source (match.go, draft_pick.go, collection.go, inventory.go, quests.go)

The next real-session capture (when a match or draft is played) should update these fixtures to REAL provenance. The drift canary (drift_canary_test.go) is designed to fire if the format changes between now and that update.

## Refresh Procedure (ADR-041 G3)

When the drift canary fires:
1. Open MTGA and play at least one match and one draft
2. Copy `Player.log` from `~/Library/Logs/Wizards Of The Coast/MTGA/`
3. Run the Python extraction script in `docs/runbooks/fixture-refresh.md` to re-extract and sanitize
4. Replace the fixture files in this directory
5. Update this MANIFEST with the new version and date
6. Submit a PR targeting `main` with Sarah security review on the fixture files
