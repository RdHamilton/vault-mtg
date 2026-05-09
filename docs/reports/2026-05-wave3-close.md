# Wave 3 Close Report — v0.3.0 Telemetry Parity

**Date**: 2026-05-08
**Milestone**: v0.3.0
**Waves completed**: 0, 1, 2, 3 (of 4)

---

## Shipped

### Wave 0 — Prerequisites
- #1387 SSE auth spike: EventSource auth via Clerk session cookie confirmed (PR #1561)
- #1392 Nginx proxy_read_timeout set to 300s, SSE buffering disabled (PR #1558)
- #1501 Log sample spike: Player.log samples captured, legacy parser compatibility validated (PR #1535)

### Wave 1 — Extraction and Schema
- #1502 pkg/logparse: extracted internal/mtga/logreader to shared pkg/logparse module (PR #1535)
- #1503 account_id backfill: added account_id column to quests, quest_session_tracking, inventory_tracking, game_plays, life_change_tracking (PR #1529)
- #1509 Contract: sequence uint64 field added to DaemonEvent per ADR-013 (PR #1528)
- #1521 DBA: sequence column added to daemon_events table (PR #1530)
- #1523 Desktop: deleted dead internal/mtga/logreader consumer (ADR-014 gate) (PR #1535)

### Wave 2 — Daemon Classifiers and SSE Consumer
- #1504 Daemon: inventory.updated classifier and payload parser wired (PR #1536)
- #1505 Daemon: quest.progress / quest.completed classifier and payload parser wired (PR #1537)
- #1506 Daemon: collection.updated classifier and payload parser wired (PR #1539)
- #1507 Daemon: deck.updated classifier and payload parser wired (PR #1544)
- #1508 Daemon: match.game_started / match.game_ended classifiers wired (PR #1545)
- #1388 Frontend: useDraftEventStream SSE consumer hook (PR #1564)
- #1389 Frontend: draft session state machine — pack/pick tracking (PR #1563)
- #1391 BFF: draft.pack/draft.pick events wired from IngestHandler to SSE broker (PR #1559)
- #1532 BFF: account_id hashed in gap detection log lines (PR #1559)

### Wave 3 — BFF Projection Layer v2 and Live Draft Page
- #1510 BFF: projection layer v2 — inventory, quest, deck, and collection projectors (PR #1552)
- #1511 BFF: projection layer v2 — collection delta projector (included in PR #1552)
- #1512 BFF: projection layer v2 — game-play (GRE) projector (PR #1557)
- #1522 BFF: daemon event gap detection logging and PostHog instrumentation (PR #1531)
- #1390 Frontend: /draft/live page with real-time pack and card grade display (included via #1563/#1564/#1561 chain)

---

## Deferred to Wave 4

| Ticket | Title | Reason |
|--------|-------|--------|
| #1393 | docs: user guide for /draft/live | Docs work; non-blocking for engineering exit gate |
| #1513 | feat(bff): DeckPerformance, WinRateTrend, FormatDistribution endpoints | Wave 4 scope |
| #1514 | feat(bff): DraftAnalytics, RankProgression, ResultBreakdown, Collection endpoints | Wave 4 scope |
| #1515 | feat(frontend): Settings / user profile page | Wave 4 scope |
| #1516 | chore(bff): pagination/filtering/sorting standard | Wave 4 scope |
| #1517 | feat(infra): CSP and security headers on CloudFront | Wave 4 scope |
| #1519 | feat(daemon): GRE session flush threshold config and stale-buffer sweep | Wave 4 scope |
| #1520 | chore(dba): partial column for game_plays incomplete GRE sessions | Wave 4 scope |
| #1524 | chore(ci): update CI pipeline for pkg/logparse extraction | Wave 4 scope |
| #1495 | test(e2e): EnvBadge visibility smoke assertion | Wave 4 scope |

---

## Metrics

- Tickets completed: 22/31 (Wave 4 tickets deferred by design)
- PRs merged: 16 (Waves 0–3 work)
- Open bugs at close: 0

---

## Wave 4 green light

YES — ready to kick off

Wave 4 entry condition satisfied: all projectors write correctly scoped rows; SSE pipeline is end-to-end verified; /draft/live page renders live pack data. Wave 4 can proceed with analytics endpoints (#1513, #1514), Settings page (#1515), pagination standard (#1516), CloudFront security headers (#1517), GRE flush threshold (#1519), game_plays partial column (#1520), CI pipeline update (#1524), and user guide (#1393).
