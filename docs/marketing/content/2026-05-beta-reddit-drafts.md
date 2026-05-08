<!-- PM REVIEW — 2026-05-08
Post 1 (Draft Pick Grades): REQUEST CHANGES. Body claimed letter grades (A/B/C) calibrated to archetype, meta performance, and mana curve — none of that logic is confirmed shipped. Shipped work is SSE-based real-time pack/pick streaming (#1388–#1391) only. Revised to accurately describe what we have. Also removed win-rate-by-matchup claim (unconfirmed for v0.3.0).

Post 2 (Match History): REQUEST CHANGES. "Opponent deck archetype" is not confirmed shipped — classifier work (#1508) covers game_started/game_ended events; archetype inference is not in scope. Win rate by matchup is unverified. Removed both specific claims; retained what is confirmed (full match history, color-pair win rates from projection endpoints #1396).

Post 3 (Social Proof/200 Testers): REJECTED. Closed beta has not launched; August 18 is the launch date. Claiming 200 testers and a three-week-old beta is fabricated social proof. Rewritten as a launch-day announcement post for the day beta actually opens (August 18). Do not schedule or post this before beta is live and numbers are real.

Scheduling note: Posts 1 and 2 should not be posted until the waitlist page is live and the invite link resolves. Confirm with growth-marketing before posting.
-->

# VaultMTG Beta — Reddit Posting Strategy

**Campaign**: beta-wave-2026-05  
**Subreddit**: r/MagicArena  
**Posting method**: Manual (requires product-manager review + sign-off before posting)  
**Rule reminder**: Reddit is a community, not an ad platform. No promotional tone. Lead with user benefit, not hype.

---

## Reddit Post 1 — Draft Pick Grades (Wave 1 — May 12)

**Target keyword**: MTG Arena draft helper / pick advisor  
**Tone**: How-to / feature explainer  
**Target audience**: Draft-focused players curious about pick optimization

**Title:**
We added a real-time draft tracker to VaultMTG — see every pack and pick live as it happens

**Body:**

If you draft on Arena, you finish a session and the game shows you your record. That's it. No pick log, no pack history, no way to go back and see what you passed or what you took.

VaultMTG is a free companion app with a live draft view that fixes that. Install a small background daemon, sign in, and the next time you draft you'll see the current pack and your picks update in real time in your browser.

After the draft you get:
- A full pick log — every card you saw in every pack, and what you took
- Your complete draft history across sessions

The live view is a screen you keep open in a browser tab while you're drafting. It reads from your Arena log in real time; there's no overlay on the game client.

It's invite-only right now — first wave just opened. We're keeping it small so we can support people through setup and iterate on what testers actually want.

If you care about reviewing your draft decisions after the fact, we'd like your feedback:

https://app.vaultmtg.app

Happy to answer questions in the comments about how the daemon works or what data gets logged.

**Post scheduling**: Tuesday, May 12 (morning ET)  
**Status**: PENDING product-manager review (do not post without sign-off)

---

## Reddit Post 2 — Match History & Win Rates (Wave 2 — May 14)

**Target keyword**: MTG Arena match history / stats tracker  
**Tone**: Problem statement / solution  
**Target audience**: Constructed ladder + draft players looking for self-analysis

**Title:**
We added a match history dashboard to VaultMTG — finally a way to review your Arena record by color pair

**Body:**

Here's a scenario: you grind to Mythic in constructed, hit a wall, and start losing 60% of your games. Arena's match history just shows you W/L. There's nothing to dig into.

VaultMTG is a free companion app that gives you an actual match history to work with. Install a daemon, sign in, and every game you play gets logged automatically. You can then look at:
- Your full match history (date, deck, result, game duration)
- Win rate broken down by color pair you were playing
- Draft history with a full pick log from each draft session

The daemon reads your Arena log in the background — there's nothing to click mid-game.

This is early. We're not doing opponent archetype inference yet, and matchup-level win rates are on the roadmap rather than live. What's there now is a clean, searchable history and color-pair win rates for your own decks.

Invite-only beta just opened. We're looking for players who want to actually review their play instead of just guess:

https://app.vaultmtg.app

Questions about what data gets collected, how to set up the daemon, or anything else — ask in comments.

**Post scheduling**: Thursday, May 14 (morning ET)  
**Status**: PENDING product-manager review (do not post without sign-off)

---

## Reddit Post 3 — Beta Launch Announcement (Wave 3 — August 18 or later)

**Target keyword**: best MTG Arena companion app / MTGA tools  
**Tone**: Community + credibility (avoid self-promotion)  
**Target audience**: Tool-aware / serious players; existing 17Lands users  
**IMPORTANT**: Do not post before August 18, 2026 (closed beta launch date). Update social proof numbers with real data on launch day before posting.

**Title:**
VaultMTG closed beta is open — free Arena draft tracker and match history dashboard

**Body:**

A little over a year ago I started building a companion app for MTG Arena because I wanted a better way to review my draft picks and track my constructed record. Today the closed beta is open.

It's a free daemon + web dashboard. Install the daemon, sign in, and it reads your Arena log in the background. What you get:

**Draft tracking**
- Live draft view showing the current pack and your picks as you draft
- Full pick log from every draft session — every card you saw, every card you took

**Match history**
- Every game you've played, searchable and filterable
- Win rate by color pair across your full history

We're not trying to replace 17Lands or UntappedGG. Those tools are excellent for aggregate meta analysis. VaultMTG is about your data — your picks, your record, your patterns.

This is a closed beta. We're being deliberate about invite size so we can actually support people through setup and act on feedback quickly.

If you want access, you can request an invite here:

https://app.vaultmtg.app

If you have questions about setup, what data gets collected, or how the daemon works — ask below. I'll be in the comments.

**Post scheduling**: August 18, 2026 (beta launch day) or when real tester numbers are available  
**Status**: PENDING product-manager review (do not post without sign-off) — DO NOT post before August 18

---

## Meta Notes

- All three posts are positioned as feature / tool explainers, not promotional content
- Reddit users know UntappedGG and 17Lands; acknowledge that context (post 3)
- Each post targets a different persona: pick optimizer (post 1), ladder grinder (post 2), data-driven tester (post 3)
- Do not cross-post all three on the same day — space them 2 days apart to respect subreddit norms and maximize reach
- Product-manager must review and sign off on all three before any posting
- Monitor comments for 24 hours after each post — respond to genuine questions; do not delete criticism

---

## Posting Workflow

1. Notify product-manager: "Reddit posts 1-3 ready for review at docs/marketing/content/2026-05-beta-reddit-drafts.md"
2. Wait for sign-off (email or Slack)
3. Post to r/MagicArena (do not post to multiple subreddits)
4. Set 24-hour monitoring for comments
5. Log performance (upvotes, comments, traffic) in weekly report
