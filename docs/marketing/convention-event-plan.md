# Convention & Event Promotion Plan — VaultMTG
**Created**: 2026-05-08
**Updated**: 2026-05-08
**Window**: September–December 2026
**Goal**: Drive installs and active user growth in the post-beta period; convert in-person attendees to retained VaultMTG users

---

## Travel Constraint

Ray cannot travel to conventions before September 2026. The following previously-identified events are off the table:

- SCG CON Las Vegas — June 26–28, 2026 (removed)
- MagicCon: Amsterdam — July 17–19, 2026 (removed)

**Pre-September strategy**: Focus entirely on online channels — streaming outreach, Reddit, Discord, and content marketing. See Section 6 for the online-only plan covering May–August.

---

## Key Dates

| Date | Milestone |
|---|---|
| August 1, 2026 | Waitlist opens |
| August 18, 2026 | Closed beta launch |
| September 4–6, 2026 | SCG CON Dallas (first in-person opportunity) |
| September 11–13, 2026 | SCG CON Baltimore |
| September 25, 2026 | Reality Fracture prerelease |
| October 2, 2026 | Reality Fracture full release (Arena drop) |
| October 9–11, 2026 | SCG CON Los Angeles |
| October 23–25, 2026 | SCG CON Hartford |
| November 13–15, 2026 | MagicCon: Atlanta / Magic World Championship 32 |
| November 13, 2026 | MTG x Star Trek set release |

---

## 1. Target Events (September–December 2026)

### MagicCon Events

**MagicCon: Atlanta — November 13–15, 2026** *(PRIMARY IN-WINDOW TARGET)*
- Georgia World Congress Center, Atlanta, GA
- Hosts Magic World Championship 32 (livestreamed on twitch.tv/magic and YouTube)
- Largest competitive MTG event of Q4; World Championship brings the highest density of enfranchised competitive players of any event in the year
- Coincides with MTG x Star Trek set release on November 13 — draft content will be highly relevant in the days immediately following
- Exhibit hall, Mana Stage panels, Community Panel Room
- This is domestic travel and the prestige event of the fall season — attend if beta traction is strong enough to justify the trip

### SCG CON Events

**SCG CON Dallas — September 4–6, 2026** *(FIRST AVAILABLE IN-PERSON OPPORTUNITY)*
- Fort Worth Convention Center, Fort Worth, TX
- Flagship event: Magic Spotlight format (competitive-heavy)
- First event in the eligible travel window — use it as a trial run for the demo setup before higher-stakes events
- Reality Fracture prerelease is September 25, so the Dallas audience will be active in the current competitive metagame (Marvel Super Heroes / older formats)
- Estimated attendance: 1,500–3,000 competitive players

**SCG CON Baltimore — September 11–13, 2026**
- Baltimore Convention Center, Baltimore, MD
- One week after Dallas — do not attempt to staff both unless partner can cover one solo
- Competitive-heavy; Regional Championship qualifier format at many SCG CONs makes for high-engagement attendees
- Strong option if Dallas goes well and team has bandwidth

**SCG CON Los Angeles — October 9–11, 2026**
- Los Angeles Convention Center, Los Angeles, CA
- Western US reach — different geographic audience than Dallas/Baltimore
- Timing is ideal: Reality Fracture releases October 2, so attendees will be actively drafting the new set and pick recommendations will be directly relevant
- This timing makes LA one of the highest-ROI events in the window

**SCG CON Hartford — October 23–25, 2026**
- Connecticut Convention Center, Hartford, CT
- Northeast US coverage
- Features Magic Spotlight: Reality Fracture — competitive players focused on the new set
- Strong pairing with any content VaultMTG has published on Reality Fracture draft picks by this point

### FNM / Local Game Store (LGS) Events

FNMs run every Friday nationwide. Two high-traffic LGS windows in this period:
- Reality Fracture prerelease (September 25 week)
- Star Trek prerelease (late October / early November — confirm date when spoiled)

**Recommended approach:**
- Do not staff individual FNMs — too distributed for travel
- Deploy the LGS Ambassador micro-kit (printable flyer + QR code) for both prerelease weekends
- Post to r/MagicArena and r/DraftMTG asking community members to post a flyer at their local store — non-promotional framing
- Frame the Reality Fracture kit around: "New set just dropped — here's a tool that shows you pick grades in real time"

---

## 2. Convention Demo Setup

### Hardware

- **Primary machine**: Laptop running VaultMTG with a live MTG Arena session open
- **Secondary display**: USB-C travel monitor if possible — demo is significantly more compelling on a larger display than a 13" laptop
- **Hotspot**: Do not rely on convention WiFi. Bring a dedicated mobile hotspot. MTG Arena + the app require a reliable connection.
- **Backup phone**: Have the app download URL loaded in browser, ready to hand to someone who wants to sign up from their own device

### Software / App State

Before arriving at any event, pre-load:
- A live or recent draft session showing pick recommendations for the current set
- Match history view with win-rate statistics visible
- Collection page if populated
- App download / landing page open in a browser tab (vaultmtg.app)

Do a full dry run at home the evening before any event — Arena updates drop without warning and can break the daemon connection at the worst possible moment.

### Elevator Pitch (30 seconds)

> "VaultMTG is a companion app for MTG Arena. While you're drafting, it shows you real-time pick recommendations based on your actual card pool — not just a static tier list. After games, it tracks your win rates by deck, archetype, and format. It runs in the background; you don't have to change how you play."

Key points to hit based on who you're talking to:
- **Drafters**: lead with pick recommendations and grade overlays
- **Constructed/ladder players**: lead with match history, win rate by archetype, metagame breakdown
- **Casual players who are hesitant**: "It's free, and it never touches your account — it reads your screen, it doesn't log in as you"

### Leave-Behind Materials

Print before every event:

1. **Business card** (3.5" x 2"):
   - Front: VaultMTG logo, tagline ("Your MTG Arena companion — draft smarter, track everything")
   - Back: QR code linking to download page, URL printed below QR code, "Free — install in 2 minutes"

2. **Half-sheet flyer** (5.5" x 8.5", single-sided):
   - Screenshot of the draft pick overlay with current-set cards
   - 3 bullet features (pick grades, match tracking, collection view)
   - QR code + URL
   - "Free companion app for MTG Arena"

Print quantities: 200 business cards and 50 flyers per event. Business cards are the priority — they travel home in a pocket; flyers get left at the venue.

Print source: Vistaprint or local FedEx Office 48 hours before the event.

---

## 3. Waitlist and Install Funnel

### QR Code Flow

1. QR code on all print materials links to: `https://vaultmtg.app/?utm_source=event&utm_medium=print&utm_campaign=<event-slug>`
   - Use UTM slugs defined in `docs/marketing/utm-naming-convention.md`
   - Event slugs: `scgcon-dallas-2026`, `scgcon-baltimore-2026`, `scgcon-la-2026`, `scgcon-hartford-2026`, `magiccon-atl-2026`, `fnm-fracture-2026`
2. By September the beta is live (launched August 18) — QR code links directly to the app download page, not a waitlist
3. Landing page conversion goal shifts from "join waitlist" to "install now"

### On-the-Spot Signup Incentive

If app is live at time of event (expected from August 18 beta onward):
> "It's free — you can install it right now from your phone if you want. Takes about two minutes."

If someone seems especially engaged:
> "We're also actively looking for feedback from competitive players. If you find anything useful or broken, there's a Discord where we respond to everything."

This drives Discord joins alongside installs, which improves retention.

### Follow-Up Email Sequence (Mailchimp)

For any email collected at events, trigger a 3-email onboarding sequence:

| Email | Timing | Subject | Content |
|---|---|---|---|
| 1 — Welcome | Immediate | "VaultMTG is ready to install" | Direct download link, 1 getting-started tip, Discord link |
| 2 — Feature highlight | Day 3 | "What VaultMTG shows you during a draft" | Screenshots of pick overlay in current set, link to guide |
| 3 — Re-engage | Day 10 | "Your VaultMTG stats after your first week" | If they've logged in: personalized stats hook. If not: "Still haven't tried it? Here's how to get started in 2 minutes" |

Keep emails short — 3 paragraphs max. One CTA button per email. Mobile-readable.

---

## 4. Staffing — Ray + Partner Role Split

At a two-person booth or demo table, avoid both people doing the same thing at once.

### Ray — Technical Demos

- Controls the laptop/demo
- Handles technical questions about how the app works, the daemon, data sourcing
- Engages competitive players who want to talk about pick theory, win-rate methodology
- Signs up interested players on the spot (laptop open to download/install page)

### Partner — Engagement and Distribution

- Stands slightly off to the side (not behind the table) — more approachable body language
- Opens conversations with passersby: "Have you tried any Arena companion tools?"
- Hands out business cards to anyone who slows down
- Collects emails on a phone (mobile browser open to install or contact form) when Ray is occupied with a demo
- Tracks rough head count of conversations and signups (tally in Notes app)

### When traffic is slow

Ray should walk the floor and engage at side event tables. Competitive players waiting for pairings are a captive audience — a 2-minute demo conversation is easy to start. Partner holds the demo station.

---

## 5. Streamer Outreach

Streamer partnerships are not gated on in-person events. This work should start in May–June and run continuously.

**Target profile**: MTG Arena draft streamers with 1,000–10,000 concurrent viewers on Twitch or YouTube Live. This tier is large enough to matter but small enough that an individual outreach message gets read.

**Outreach offer**: Early access to VaultMTG (available from August 18 beta onward), plus direct line to the dev for feedback. Do not offer money — the app value proposition should be sufficient at this stage.

**Pitch angle**:
> "Your viewers watch you draft and want to learn. VaultMTG shows pick grades and deck-building stats in real time. We think it makes for better stream content and helps your viewers follow along. We'd like to give you early access — no strings attached."

**Target list (to be built)**:
- Identify 5–10 streamers via Twitch MTG Arena category, sorted by average concurrent viewers
- Look for streamers who focus on Limited / draft specifically — they align directly with the pick overlay feature
- Check if any are already VaultMTG users (via Discord or direct search)
- Prioritize streamers who interact with their chat during picks — the overlay creates a natural viewer engagement moment

**Coordination with MagicCon Atlanta**: If a target streamer is attending MagicCon Atlanta as a content creator or community guest, a brief in-person intro at the event is more effective than a cold DM. Cross-reference the streamer list against the MagicCon Atlanta content creator guest list when it is published (typically 4–6 weeks before the event).

---

## 6. Pre-September Online Channel Plan (May–August)

Since no in-person events are viable before September, all pre-September promotion runs through online channels only. This is not a downgrade — the beta launch is July 7, and online channels are the right medium for a digital-only app.

### Reddit (r/MagicArena, r/DraftMTG)

| Timing | Post type | Topic |
|---|---|---|
| August 1 | Announcement | Waitlist is open — non-promotional framing |
| June 23 | Content | "Marvel Super Heroes draft picks — what VaultMTG recommends for the top archetypes" |
| August 18 | Launch | "We launched VaultMTG beta — free companion app for MTG Arena" |
| July 17–19 | Pro Tour weekend | "Watching Pro Tour this weekend? Here's how VaultMTG would break down the featured draft picks" |
| August | Ongoing | Respond to any "MTG Arena tracker" or "draft helper" threads with a genuine, non-spammy mention |

### Discord

- Post in #announcements for waitlist open (August 1) and beta launch (August 18)
- Engage in r/MagicArena and MTG-adjacent Discord servers as a community member, not a promoter
- Share the Reality Fracture tier list article in relevant servers once published in late September

### X (Twitter)

- Schedule posts around each major date above via Buffer
- During Pro Tour Amsterdam (July 17–19), post commentary and engage with #MagicArena and #ProTour hashtags (pre-beta awareness)
- Use #MTGArena and #MagicTheGathering on all app-related posts

### Content Marketing

Two high-value articles to publish before September:
1. **"MTG Arena companion apps compared"** — targets "best MTG Arena companion app" keyword; publishes mid-June
2. **"Marvel Super Heroes draft tier list"** — targets seasonal high-volume keyword; publishes June 23 (Arena drop day)

Both pieces should be live and indexed before the September events so there is SEO traction to point to.

---

## 7. Event Priority Ranking (September–December 2026)

Updated to reflect new travel window. Ranked by expected install ROI.

### Priority 1 — SCG CON Los Angeles (October 9–11)

**Why**: Reality Fracture drops October 2, so attendees are actively drafting the new set the week of the event. VaultMTG's pick recommendations are maximally relevant. Domestic travel. Western US geographic reach not covered by other events. LA Convention Center is a well-run venue with reliable exhibitor infrastructure.

**Target**: 50–150 direct installs over the weekend from floor demos; secondary reach via social posts from attendees.

**Action required**: Check scgcon.starcitygames.com for exhibitor/vendor space availability. Guerrilla floor approach is acceptable if booth cost is prohibitive — attend as players and demo at side event tables.

### Priority 2 — MagicCon: Atlanta (November 13–15)

**Why**: Largest competitive event of the fall. Magic World Championship 32 brings the highest-profile players and streaming audience of any Q4 event. Star Trek set drops the same weekend, so draft content is fresh. Exhibit hall provides a fixed demo location. Secondary benefit: World Championship broadcast reaches tens of thousands of viewers — any organic social posts from the floor get amplified.

**Attend if**: Beta has reached 500+ MAU by November 1 (signal that the product is ready for this level of exposure). MagicCon has structured exhibitor/vendor options — evaluate cost vs. expected ROI closer to the event.

**Target**: 100–300 installs from direct demos plus secondary social lift.

### Priority 3 — SCG CON Dallas (September 4–6)

**Why**: First available event in the travel window. Treat as a low-stakes trial run to test the demo setup, elevator pitch, and leave-behind materials before higher-stakes events. Dallas/Fort Worth is a large competitive scene. Any signups are a bonus; the primary goal is operational rehearsal.

**Target**: 25–75 installs. Keep expectations calibrated — Reality Fracture has not dropped yet, so the pitch relies on current-set relevance.

### Priority 4 — SCG CON Hartford (October 23–25)

**Why**: Features Magic Spotlight: Reality Fracture — directly relevant to VaultMTG's draft picks. Northeast US coverage. However, it falls two weeks after LA, making back-to-back travel demanding. Attend only if LA goes well and bandwidth exists.

### Lower priority — SCG CON Baltimore (September 11–13)

Immediately follows Dallas (one week later). Staffing both in a single week is aggressive. Consider attending Baltimore as players (no demo table) to scope the venue and audience for a future year, or skip and recover between Dallas and the October events.

---

## Pre-Event Checklist

Run this before any event:

- [ ] QR code tested on 3 different phones — links to correct UTM-tagged URL
- [ ] App install flow functional end-to-end (download → install → first launch)
- [ ] Business cards and flyers printed and packed (current-set screenshots on flyer)
- [ ] Laptop charged, Arena + daemon pre-loaded and running with current set
- [ ] Mobile hotspot data plan confirmed active
- [ ] Mailchimp confirmation email tested (send to ray.hamilton@stablekernel.com)
- [ ] Partner briefed on elevator pitch and install flow
- [ ] UTM parameter for this specific event is live in the analytics dashboard
- [ ] Reality Fracture tier list article live on site (for October+ events)

---

## Budget Estimates

| Item | Per Event | Notes |
|---|---|---|
| Vendor/exhibitor space | $500–$2,000 | Guerrilla option: $0 with attendee badge |
| Badge (if not exhibiting) | $50–$100 | Standard attendee badge; SCG CON entry is free |
| Business cards (200) | $20–$30 | Vistaprint |
| Flyers (50) | $15–$25 | FedEx Office, full color |
| Hotspot data add-on | $15–$30 | One-time or monthly |
| Travel | Varies | Ray estimates own costs per city |

Guerrilla approach (no vendor table, floor demos only): total hard cost under $200 excluding travel. Recommended starting point for Dallas as the trial event.

---

## Open Questions / Decisions Required

1. **Exhibitor vs. attendee at SCG CON Dallas**: Decide by August 1 — space at smaller SCG CONs sells out 4–6 weeks out.
2. **MagicCon Atlanta exhibitor space**: Early registration typically opens 3–4 months before the event (August). Set a calendar reminder to check availability and pricing then.
3. **Streamer outreach list**: Build the target list of 5–10 draft streamers by June 15 so outreach begins well before beta launches on August 18.
4. **Reality Fracture tier list**: Must be published on or before October 2 (Arena drop) to capture search traffic ahead of SCG CON LA on October 9.
5. **LGS Ambassador kit**: Create printable PDF flyer for Reality Fracture prerelease (September 25 week). Asset due by September 18.
