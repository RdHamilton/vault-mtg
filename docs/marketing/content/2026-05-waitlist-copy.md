# Waitlist Page Copy

**Target keyword**: MTG Arena companion app
**Secondary keywords**: MTG Arena win rate tracker, MTG Arena draft tracker, best MTG Arena companion app
**Publish date**: 2026-05-06
**Distribution**: vaultmtg.app/waitlist, vaultmtg.app/download (waitlist section)

---

## /waitlist — Standalone Page

### Meta Description (SEO, under 160 chars)

Track every MTG Arena draft and match. VaultMTG shows your win rates, card performance, and deck stats — so you can actually improve.

---

### H1

Track Every Draft. Know Your Win Rates. Improve Your Game.

### Subheading

VaultMTG is the MTG Arena companion app built for players who want more than a match history. Sign up to be notified when beta opens.

### Feature Bullets

- **Win rate by card, deck, and archetype** — see exactly what is winning for you in draft and constructed, not just the field average
- **Full draft and match history** — every pick, every game, with the context to understand what went wrong (or right)
- **Metagame and opponent tracking** — know what decks you are facing on the ladder and how your builds perform against them

### Form Label

Your email address

### CTA Button Text

Join the Waitlist

### Confirmation Message (shown inline after successful submit)

You are on the list. We will email you at [email] when the VaultMTG beta opens.

---

## /download — Waitlist Section Copy

Use this copy for the waitlist module embedded on the /download page (shown when the app is not yet available for download).

### Section Heading

Beta Coming Soon

### Section Body

VaultMTG is in active development. Leave your email and we will notify you the moment the beta opens — no spam, one email when it matters.

### Form Label

Email address

### CTA Button Text

Notify Me

### Confirmation Message

Got it — you are on the list. We will reach out when the beta is ready.

---

## Mailchimp Setup Notes (completed 2026-05-06)

- Audience name: **VaultMTG Waitlist**
- Tag applied to all form signups: `waitlist`
- Welcome/confirmation email:
  - Subject: **You're on the VaultMTG waitlist**
  - Body: "Thanks for signing up. We're building VaultMTG — an MTG Arena companion app for draft and constructed players who want real data on their games. We'll email you when beta opens. That's it — one email, no noise."
- Automated welcome email is triggered on list subscription (single-opt-in)

> Note: Mailchimp audience and automation were configured manually in the Mailchimp dashboard. API credentials are stored in SSM — see waitlist-form-spec.md.
