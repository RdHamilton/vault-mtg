# Waitlist Research — VaultMTG Beta Launch

**Research date**: 2026-05-09 (refreshed 2026-05-09 with additional web research)
**Deadline**: August 1, 2026 (waitlist opens)
**Owner**: growth-marketing agent

---

## 1. SaaS Beta Waitlist Best Practices

### Lead time

- Ship a waitlist landing page in **week 1**; set up referral mechanics by
  **week 2**; spend weeks 3–8 publishing-in-public on 2–3 channels; amplify
  via Product Hunt, Hacker News, and niche communities in weeks 9–12.
- With an August 1 open date and today being May 9, we have **84 days** —
  just enough for the full playbook if we start immediately.
- Referral programs set up after the first 200 signups lose the opportunity
  to have those early adopters share. **The referral loop must be live at launch.**

### Landing page conversion

- High-converting waitlist pages achieve **20–40% visitor-to-signup** rates.
- The headline does 80% of conversion work. Test: can a stranger read it and
  immediately state (a) who it's for and (b) what they get?
- Recommended form fields (3 max to keep >60% conversion): email (required),
  MTG format preference (Limited / Constructed / Both), how they heard about us.

### Email nurture

- Send a confirmation email immediately on signup with: their spot number,
  what to expect, and links to follow on social.
- Follow up every 2 weeks with product progress updates ("build-in-public"
  posts) — these keep list warm and reduce unsubscribes before launch day.
- Launch-day email to the full list has the highest open rate of any email
  you'll ever send — treat it as your most important marketing asset.

### Referral mechanics

- Top-performing programs achieve **22–25% referral rates** (1 in 4 signups
  brings another).
- Referred users show **37% higher retention** and are **4x more likely to
  refer others**.
- Tiered rewards outperform single-sided rewards: "Get 5 friends in, skip
  the line entirely" is more motivating than a flat "move up 10 spots per
  referral."
- Reward ideas for VaultMTG: early access priority (queue position), exclusive
  card-back cosmetic on beta launch, extra draft analyses, or lifetime discount.

---

## 2. MTG / Gaming App Waitlist Precedents

- **DeckSage** (MTG Arena companion) currently runs a waitlist for early
  access — confirms the format works in this niche. No public data on
  conversion rates.
- **Wizards of the Coast / MTG Companion** went straight to open beta in 2019;
  not a useful model for a closed beta with capacity limits.
- **General gaming app trend**: desktop-referred users now outperform mobile
  for the first time (desktop took lead in 2026 over mobile's previous 57%
  share). For a desktop daemon tool like VaultMTG, desktop referral channels
  (Reddit, Discord, Twitter/X) are the right focus — not mobile-first
  channels.
- **Key insight**: The MTG community is highly concentrated in a small number
  of forums (r/lrcast, r/spikes, r/magicTCG, LimitedResources Discord,
  draftsim Discord). Seeding in these specific communities outperforms any
  broad social campaign.

---

## 3. Tooling Recommendation: Custom Form vs. Waitlist Platform

### Current plan: custom form + manual Clerk invite

**Assessment**: Adequate for <500 signups with no referral mechanic. Does not
scale past that without significant manual effort (tracking who referred whom,
sending position-update emails, managing priority queue). For a product where
referral virality is a goal, the custom form misses the viral coefficient
entirely.

### Option A: Prefinery ($49–$499/mo as of 2026)

**Best fit for VaultMTG.** API-first, built for beta program management, tracks
referral source, conversion rates, demographics, and form responses. Supports
automated Clerk invite triggers (webhook → Clerk invite). Manages the
priority queue automatically. Segment users by MTG format for targeted launch
waves.

- Price: $49/mo (Lite) covers up to 1,000 waitlist members; $199/mo (Essentials) for more
- Integration: webhook on signup → Clerk invitation API
- Pros: no custom dev needed, analytics included, beta-management native, developer-friendly API
- Cons: monthly cost (trivial at this stage); pricing increased since original research

### Option B: GetWaitlist ($15–$50/mo)

**Cheaper option** with referral mechanics and basic analytics. Referral features
gated to the $50/mo plan. Less sophisticated than Prefinery for segmentation and
beta management. Works well if budget is the constraint. Embeds easily in Webflow/custom HTML.

### Option C: Beehiiv

**Newsletter-first.** Best if VaultMTG wants to build a content-marketing
newsletter alongside the product (e.g., a weekly MTG draft tips email). Not
the right tool if the primary goal is pure waitlist management and invite
orchestration.

### Option D: Viral Loops ($49–$299/mo)

**Longest referral analytics track record.** Templates modeled on Dropbox/Harry's/Robinhood.
Fraud detection trained on 7 years of referral data (detects same-IP duplicate signups,
disposable email domains). At $299 for 10,000 signups, overkill for a 500-person closed
beta — but worth considering if the referral flywheel is the primary growth lever.

### Recommendation

**Use Prefinery.** It handles the full lifecycle — waitlist signup, referral
tracking, priority queue, beta invite dispatch — and integrates with Clerk via
webhook. At $49/mo (updated pricing), cost is immaterial against the risk of a
broken custom referral system. Cancel after beta invites are fully dispatched.

**Fraud protection note**: If using a custom form with referral codes, add
basic fraud detection (block same-IP multiple signups, reject disposable email
domains). Viral Loops and Prefinery handle this automatically.

**Migration plan if staying custom**: at minimum, add `?ref=CODE` tracking to
the signup form URL and store it in the submission. Even if we don't automate
rewards, we need referral attribution data.

---

## 4. What Growth-Marketing Should Do RIGHT NOW (May 9 → August 1)

### 84-day action plan

**Week 1–2 (by May 23)**

1. **Ship the waitlist landing page** at `vaultmtg.app` with the confirmed
   August 1 open date. Headline must pass the "stranger test." Three fields:
   email, MTG format, referral source. Prefinery integration live on day 1.
2. **Set up referral mechanic** before driving any traffic. Tiered rewards:
   - 1 referral → move up 50 spots
   - 5 referrals → guaranteed Wave 1 access (first 100 invitees)
   - 10 referrals → beta tester credit + lifetime 20% discount on GA pricing

**Week 3–6 (May 24 – June 21)**

3. **Seed niche communities** with genuine, non-promotional posts:
   - r/lrcast: "building a draft tracker for Arena — what data do you wish
     you had?" (build-in-public post, no link spam)
   - r/spikes: same angle for constructed metrics
   - LimitedResources Discord: direct outreach to moderators for a community
     partner slot
   - Draftsim Discord: same
   - MTG Arena subreddit: "what companion tools do you actually use?"
   - Post waitlist link only in comments when organically asked; lead with
     value not promotion
4. **Publish 4 build-in-public posts** on Twitter/X and Reddit — feature
   previews, draft analysis screenshots, "this is what we track" data posts.
   Each post ends with a CTA to the waitlist.

**Week 7–10 (June 22 – July 20)**

5. **Marvel Super Heroes draft season content** (Arena set drop ~June 23):
   - Publish "Marvel Super Heroes draft tier list — what VaultMTG recommends
     for top archetypes" targeting seasonal high-volume keyword
   - Draft analytics screenshot thread on Twitter/X
   - Reddit post in r/lrcast with early archetype data
   - This is the highest-leverage content window before the August 1 open

**Week 11–12 (July 21 – August 1)**

- Send "opening soon" email to full list 7 days before (July 25)
- Send "we open tomorrow" email on July 31
- August 1: send "waitlist is open" email + post across all channels
- Prioritize Wave 1 invites to highest referrers first

---

## 5. Concrete Action Items for Growth-Marketing (Start Now)

1. **Decision needed by May 16**: choose Prefinery or custom form with ref
   tracking. If Prefinery: sign up, configure Clerk webhook, get link live.
2. **Landing page copy live by May 23**: headline, 3-field form, August 1
   date prominent, referral reward tiers visible.
3. **Community seeding plan drafted by May 16**: list of 5 target
   communities, post angle per community, posting schedule. No link drops
   until the page is live.
4. **Marvel Set content brief by June 1**: identify 2–3 data angles
   VaultMTG can uniquely provide (e.g., color pair win rates from internal
   testing data). Draft the post before the set drops.
5. **Email nurture sequence written by May 30**: (a) confirmation email,
   (b) week-2 product update, (c) week-4 "here's what the data shows",
   (d) 7-days-before, (e) launch day. Five emails, all written before the
   first signup arrives.

---

---

## 6. Community Channel Strategy (MTG-Specific)

Based on 2025–2026 research on gaming app community growth:

- **Reddit posts with visuals generate 650% more engagement** than text-only.
  Every community post should include a screenshot of the product in action.
- **Discord users spend ~1.5 hours/day on platform** (73% aged 16–34).
  The MTG Draft Discord communities (LimitedResources, Draftsim) are the
  highest-density audiences of VaultMTG's exact user persona.
- **Development update posts outperform announcement posts** in indie gaming
  communities. Frame posts as "here's what I'm building" not "here's a product."
- **Cross-promotion with other servers**: identify 3–5 adjacent MTG tool
  servers (Moxfield, MTGO communities) for cross-promotion opportunities.
- **44% of successful indie developers** attribute primary growth to community
  engagement (not paid ads). This validates the community-first approach.

---

## Key Risk

**84 days is enough — but only if week 1–2 tasks start this week.** A
landing page that goes live June 1 instead of May 23 loses two weeks of
compounding referral growth. The referral flywheel needs time to spin up
before the August 1 open date.

**Sources consulted (2026-05-09)**:
- Waitlister.me — Best Waitlist Software 2026 comparison
- Prefinery.com — SaaS waitlist strategy and tooling guide
- GetLaunchList.com — 0 → 1,000 Beta Users in 90 Days playbook
- Viral-Loops.com — Referral waitlist mechanics and fraud detection
- CloutBoost — Marketing on Reddit: 2025 Guide for Game Developers
- TicketFairy — Mastering Reddit & Discord for niche community promotion 2026
