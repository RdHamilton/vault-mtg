# VaultMTG Feature Opportunities — Market Research Synthesis
**Date**: 2026-05-08
**Sources**: Business Analyst (format-feature-market-research.md) + Growth Marketing research

---

## Context

Both the BA and Growth agents independently researched format popularity, competitor gaps, and acquisition opportunities. Three features emerged with consensus across both reports. All are scoped for v0.4.0+ (post-beta launch July 7, 2026).

Addressable market: MTG Arena has ~5–7M MAU, with an estimated 250K–1M users who would use a companion app. Reaching 50K MAU is achievable with focused execution on any one of these.

---

## Feature 1: Brawl Commander Analytics
**Consensus ranking**: BA #1 / Growth #3 — strongest agreement across both agents

### Opportunity
Brawl surpassed Historic as Arena's #2 format in late 2024 and is still growing. Its Commander-style gameplay attracts the paper MTG Commander crowd (the largest paper format segment globally). No companion app — including Untapped.gg — provides personal Brawl stats at the individual player level.

### What to build
- Personal win rate by commander
- Matchup records against specific opposing commanders
- Session history per commander
- "Best commanders in your collection" recommendations

### Why now
- Zero direct competition at the personal stats level
- Adjacent to existing match tracking pipeline — just needs to tag commander identities
- Content about it will rank easily (low supply of competing tools)
- Subreddit surface area: r/MagicArena regularly discusses Brawl with no tool to point to

---

## Feature 2: Free Win Rate Dashboard + Shareable Stats Profile
**Consensus ranking**: BA #3 / Growth #1+#2 — full agreement, two complementary features

### 2a — Free Constructed Win Rate Dashboard

### Opportunity
"Arena doesn't show my stats" is the #1 complaint across all Arena player segments. Untapped.gg charges $4–8/month for per-deck win rates and opponent archetype breakdowns. Standard alone is ~50% of all Arena sessions.

### What to build
- Per-deck win rate across Standard, Explorer, Historic Brawl
- Opponent archetype breakdown (what are you losing to?)
- Win rate trend over time
- Positioning: "the free alternative to Untapped.gg Premium"

### Keyword targets
- "MTG Arena win rate tracker free"
- "untapped.gg alternative"
- "MTG Arena Standard tracker"

### 2b — Shareable Stats Profile (Viral Acquisition Loop)

### Opportunity
No MTG Arena tool has a shareable profile. Every share from a user reaches a warm audience of MTG players by definition. This is the single highest-ROI acquisition feature VaultMTG could ship.

### What to build
- Public profile URL: `vaultmtg.app/u/[username]`
- Shows: current-set draft record, all-time match record, collection completion %
- One-click share image for X and Discord (think: Spotify Wrapped, chess.com stats card)
- "Compare your stats" CTA that drives new registrations

### Why this is the viral loop
The Wizards official Companion app launched "Player Profiles" in February 2026, validating the appetite. VaultMTG can do it better with real, granular stats.

---

## Feature 3: Arena Collection Export / Moxfield Integration
**Unique BA finding** — no competing deck builder recommended; close the gap instead

### Opportunity
VaultMTG already parses the Arena collection from the log. The single most-upvoted feature request on Moxfield's public feedback board is Arena collection sync. Rather than building a competing deck builder (Moxfield dominates — don't compete), expose collection data in a way that feeds Moxfield/Archidekt.

### What to build
- Collection export in Moxfield/Archidekt-compatible format
- Read-only API endpoint for collection data (owned cards + quantities)
- Wildcard calculator: "how many wildcards do I need to build this deck?"

### Why this is different from building a deck builder
You're not competing with Moxfield — you're becoming the data layer they don't have. This opens a partnership/referral angle: Moxfield recommends VaultMTG to their users as the Arena sync solution, you recommend Moxfield as the deck builder. Both win.

---

## What NOT to Build (for acquisition)
- **Alchemy-specific features** — low player interest, divisive format, small audience
- **Full standalone deck builder** — Moxfield dominates; you'd need to be dramatically better to displace them
- **Paper MTG collection tracking** — different product category, different user entirely

---

## Recommended Sequencing
Given the July 7 beta launch and v0.3.0 completion timeline:

1. **Brawl Commander Analytics** — start in v0.4.0 Wave 1. Most differentiated, no competition, adjacent to existing pipeline.
2. **Free Win Rate Dashboard** — v0.4.0 Wave 1 alongside Brawl. Match tracking already exists; surfacing constructed analytics is primarily a frontend + query effort.
3. **Shareable Stats Profile** — v0.4.0 Wave 2. Depends on dashboard data being stable. Ship before first major event appearance (SCG CON Dallas, Sep 4–6).
4. **Collection Export / Moxfield Integration** — v0.4.0 Wave 2 or 3. Lower acquisition urgency but high goodwill with the deck-building community.

---

## Full Research Reports
- BA deep dive: `docs/analytics/format-feature-market-research.md`
- Convention/event plan: `docs/marketing/convention-event-plan.md`
