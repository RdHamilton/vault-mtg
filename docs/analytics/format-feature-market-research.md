# Format & Feature Market Research
**Date**: 2026-05-08
**Purpose**: Inform VaultMTG product roadmap for expanding beyond draft/limited players
**Method**: Web search synthesis — qualitative; not hard internal Wizards data

---

## 1. MTG Arena Format Popularity

Source: Wizards of the Coast "State of the Formats 2024" (official) and community synthesis.

| Format | Relative share | Notes |
|---|---|---|
| Standard | ~50%+ of play | Far ahead of all others; 2x the next format |
| Brawl (Historic) | ~20–25% | Surpassed Historic in Nov 2024; fastest-growing |
| Historic | ~15–20% | Declining relative share vs. Brawl |
| Alchemy | ~8–12% | Stable niche; polarizing among players |
| Explorer | ~4–7% | Small but loyal competitive playerbase |
| Timeless | ~3–5% | New format (2024); roughly matches Explorer |
| Standard Brawl | ~2–3% | ~1/9th the play rate of Historic Brawl |
| Draft/Limited | Separate queue | No direct % comparison in official data |

**Note**: Wizards does not publish exact MAU per format. Figures above are relative population estimates synthesized from the 2024 State of the Formats article and community reporting. They represent game session share, not unique player counts.

**Key takeaway**: Standard is the dominant constructed format by a large margin. Brawl has overtaken Historic and is the #2 format overall. Draft sits in its own competitive queue ecosystem (Premier Draft, Quick Draft, Sealed) and skews toward more invested players.

---

## 2. Brawl — Size, Trajectory, and Tool Gaps

**Growth signal**: Brawl was ~Alchemy-tier in 2023. By late 2024 it surpassed Historic to become the #2 format. Community commentary suggests it continues growing in 2025–2026.

**Why it's growing**: Brawl is MTG Arena's closest analog to Commander (the #1 paper format globally). It is a singleton 60-card format with a commander, making it feel familiar to the ~50%+ of paper players who identify as Commander players. Commander cards are progressively being added to Arena, increasing Brawl's card pool appeal.

**What Brawl players currently have**:
- Untapped.gg: Historic Brawl meta stats and card win rates (premium-gated)
- AetherHub: Brawl metagame decks, commander archetype tracking
- MTGAAssistant: Basic Brawl meta data
- No tool offers: commander-specific synergy analysis, personal win rate vs. specific commanders, or deck-building that filters "only cards I own" filtered to Brawl legality

**Gap**: No companion app provides a commander-centric Brawl experience — synergy scoring, matchup win rates by opposing commander, or a "build around your commander with what you own" deck wizard. This is table stakes in the Commander-paper world (Moxfield, EDHREC) but absent for Arena's Brawl format.

---

## 3. Standard / Constructed Deck Builder Gap

**The core problem**: MTG Arena does not natively display win rates, match history statistics, or deck performance analytics. Players who want this data must install a third-party tracker.

**Current tool landscape**:
- Untapped.gg: Best-in-class constructed tracker; deck win rates, meta share, card stats. Premium subscription required for full analytics. Windows-only companion app.
- MTG Arena Tool: Open source, broader feature set, less polished UX
- Moxfield/Archidekt: Best-in-class deck builders, but no live Arena collection sync. Players must manually export collection from Arena (CSV) and import; real-time "what do I own?" filtering requires a companion app that reads the log.
- 17Lands: Draft-only, does not track constructed play at all.

**Identified gap — Arena-native collection-aware deck building**: Moxfield is the dominant deck builder but cannot natively read a player's Arena collection. A player building a new Standard deck on Moxfield must separately check Arena to know which cards they own. Multiple high-upvote Moxfield feature requests ask for Arena collection sync (the top request on Moxfield's feedback board). A companion app that reads the Arena log and exposes collection data — either natively or via API — closes this gap.

**Identified gap — cross-format personal analytics**: Untapped.gg covers this but is paywalled beyond basic stats. Community complaint is consistent: the free tier shows aggregate meta data but personal win rates, deck-specific stats, and opponent archetype breakdowns are premium. A competitively-priced or free alternative with deeper personal analytics could attract a segment of players who churn from Untapped.gg's free tier.

---

## 4. Competitor Landscape Summary

| Tool | Formats covered | Key strengths | Gaps |
|---|---|---|---|
| 17Lands | Draft/Limited only | Best draft analytics globally; card grades, archetype win rates, replay | Zero constructed coverage; no overlay; Windows-only log client |
| Untapped.gg | All formats (constructed + Brawl) | Polished overlay; real-time deck tracking; meta dashboards | Meaningful analytics paywalled; Brawl analytics shallow vs. draft depth; no Brawl-specific commander tools |
| MTG Arena Tool | All formats | Open source; collection + deck tracker | Rough UX; slower to update after Arena patches; small team |
| Moxfield | Deck building (all formats, paper+digital) | Best-in-class deck builder UX; enormous community | No live Arena collection sync; no in-game session tracking |
| Archidekt | Deck building + collection | Collection tracking with Arena import | Manual import only; not a companion app |
| MTGAAssistant | All formats | Feature-complete extension; draft + constructed | Extension model (browser); UX less polished than Untapped.gg |

**Structural gap**: No tool combines (a) live Arena collection sync, (b) Brawl-specific commander tools, and (c) cross-format personal win rate tracking in a single, well-designed free or low-cost app.

---

## 5. Community Pain Points (Qualitative)

Sources: Community aggregation from MTG Arena Zone, Draftsim, Reddit synthesis, Moxfield feedback board.

1. **"Arena doesn't show my stats"** — The single most recurring complaint. Players do not know their win rate by format, deck, or time period without a third-party app. Any app that surfaces personal performance data clearly earns immediate goodwill.

2. **"I can't build decks with what I own without switching apps"** — Moxfield users who also play Arena frequently describe a painful context-switch: build on Moxfield, check Arena for card ownership, back to Moxfield. Arena-native collection awareness is a top wishlist item on Moxfield's public feedback board.

3. **"Brawl has no good tools"** — Brawl players note that while Untapped.gg provides some data, it lacks the commander-centric analytics that EDHREC and Commander-focused tools provide for paper play. No tool shows: "what commanders do I frequently lose to?" or "given my collection, what's the optimal commander for a fun/competitive Brawl deck?"

4. **"Tracker apps are too complex or paywalled"** — Recurring complaint that Untapped.gg's free tier is insufficient and its subscription ($4–8/mo) feels steep for casual players. Players who don't draft heavily (the core Untapped.gg value prop) feel the premium tier is not worth it for their use case.

5. **"No iOS/mobile option"** — MTG Arena on iPad exists but no companion app can access Arena's log on iOS due to sandboxing. Players who use iPad as their primary Arena device have zero access to companion analytics. This is a platform constraint, not a competitor gap — but it is an unserved segment worth monitoring as mobile companion architectures evolve.

---

## 6. Market Size Estimates

| Estimate | Figure | Source / confidence |
|---|---|---|
| MTG Arena registered accounts | 13M+ | Hasbro-reported; low confidence on recency |
| Estimated MTG Arena MAU | 5–7M | Extrapolated from registered base; low confidence |
| Current companion app penetration | ~5–15% of MAU | Industry estimate; no public data |
| Addressable users for a companion app | 250K–1M | 5–15% of 5–7M MAU estimate |
| Brawl players (estimated MAU equiv) | 1–1.5M | ~20–25% of estimated MAU |
| Standard players (estimated MAU equiv) | 2.5–3.5M | ~50% of estimated MAU |

**Caution**: All MAU figures are estimates synthesized from publicly available data. Wizards does not publish format-level MAU. These numbers are directional, not decision-grade.

---

## 7. Opportunity Matrix

| Opportunity | Format segment | Competitor coverage | VaultMTG effort to serve |
|---|---|---|---|
| Brawl commander analytics | Brawl (2nd largest format) | Weak — Untapped.gg shallow, no commander-centric tools | Medium — requires commander matchup tracking logic |
| Arena-native collection-aware deck builder | Standard + all formats | Gap — Moxfield/Archidekt require manual sync | Low-Medium — log-reading is already built for draft |
| Free personal win rate analytics across formats | All constructed formats | Paywalled at Untapped.gg | Low — extends existing match tracking to more formats |
| Draft pick advisor (current VaultMTG core) | Limited | 17Lands (dominant), Untapped.gg draft overlay | Already in roadmap |
| Constructed meta metagame dashboards | Standard, Explorer | Untapped.gg (strong), AetherHub | High — requires aggregate data at scale |

---

## Top 3 Opportunities (Summary)

### Opportunity 1: Brawl Commander Analytics
**Signal**: Brawl is the #2 format on Arena, surpassed Historic in late 2024, and is growing. The Commander-style gameplay has a massive paper fanbase migrating to Arena. No tool provides commander-centric analytics: personal win rate vs. specific opposing commanders, collection-filtered "build around this commander" deck wizard, or synergy scoring.

**Why VaultMTG**: Collection data is already read from the Arena log (core feature). Extending that to Brawl commander synergy analysis requires card relationship data (which is manageable to source) and matchup logging (already done for draft). This is an adjacent feature, not a new product.

**Risk**: Smaller absolute user count than Standard players; Brawl games have longer queues, so session data accumulates more slowly.

---

### Opportunity 2: Collection-Aware Deck Builder (All Formats)
**Signal**: Moxfield has the #1 feedback request for Arena collection sync. Archidekt requires manual CSV import. Players building Standard, Explorer, and Brawl decks want to see — in real time — which cards they own. VaultMTG already reads the Arena log for draft; collection export is a byproduct of that same data stream.

**Why VaultMTG**: This is the lowest-incremental-effort opportunity. The collection is already parsed. Exposing it through a deck builder UI — even a lightweight one — or providing an API that Moxfield could consume closes the most-requested gap in the ecosystem. Could also be positioned as a collection export/sync feature rather than a full deck builder, reducing scope.

**Risk**: Moxfield or Archidekt could build this themselves; dependency on their API willingness. Full deck builder UX is a significant investment.

---

### Opportunity 3: Free Cross-Format Personal Win Rate Analytics
**Signal**: "Arena doesn't show my stats" is the most common complaint across all Arena player segments. Untapped.gg covers this but meaningful analytics (per-deck win rate, opponent archetype breakdown, time-of-day performance) are paywalled. A free tier that meaningfully exceeds what Untapped.gg offers without payment is a direct acquisition lever.

**Why VaultMTG**: Match tracking for draft is core. Extending match tracking to log Standard, Brawl, and Explorer sessions with the same pipeline is an engineering lift but not a new product category. Win rate by deck and format could ship fast if the match logging pipeline is format-agnostic.

**Risk**: Differentiation erodes if Untapped.gg reduces its paywall. Building a competitive analytics product requires scale — aggregate meta stats (what opponents play) need many users contributing data to be meaningful.

---

## Sources
- [MTG Arena State of the Formats 2024 — Wizards of the Coast](https://magic.wizards.com/en/news/mtg-arena/mtg-arena-state-of-the-formats-2024)
- [State of Formats in MTG Arena — Wizards of the Coast](https://magic.wizards.com/en/news/mtg-arena/state-of-formats-in-mtg-arena)
- [MTG Arena Reveals Shocking Statistics Regarding Format Popularity — MTG Rocks](https://mtgrocks.com/mtg-arena-reveals-shocking-statistics-regarding-format-popularity/)
- [MTG Arena Shares Latest Format Popularity and Vision for Alchemy — MTG Arena Zone](https://mtgazone.com/mtg-arena-shares-latest-format-popularity-and-vision-for-alchemy/)
- [Exploring Brawl: MTG Arena's Casual Format Revolution in 2024 — MTG Circle](https://mtgcircle.com/articles/is-brawl-the-next-big-thing)
- [Brawl: Our Plans — Magic: The Gathering](https://magic.wizards.com/en/news/mtg-arena/brawl-our-plans)
- [Current State of MTG Arena Formats, Summarized — AetherHub](https://aetherhub.com/Article/Current-State-of-MTG-Arena-Formats-Summarized)
- [MTG Arena Player Count — Draftsim](https://draftsim.com/mtg-arena-player-count/)
- [Sync with Arena collection (Moxfield feedback board)](https://moxfield.nolt.io/1110)
- [How to Find Your Win Rate in MTG Arena — Draftsim](https://draftsim.com/mtg-arena-win-rate/)
- [Untapped.gg — Historic Brawl Meta](https://mtga.untapped.gg/meta?format=historicBrawl)
- [MTGA Tracker Apps — Gray Viking Games](https://www.grayvikinggames.com/blogs/gvg-blog/mtga-tracker-apps)
- [Magic: The Gathering Arena Tracker Apps — MTG Wiki Fandom](https://mtg.fandom.com/wiki/Magic:_The_Gathering_Arena/Tracker_Apps)
- [MTG Arena Pro Tracker](https://mtgarena.pro/mtga-pro-tracker/)

---

*All data is qualitative/synthesized from public sources. Format share percentages are directional estimates, not Wizards-certified figures. Treat as market signals, not measurement-grade data.*
