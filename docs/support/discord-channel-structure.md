# VaultMTG Discord Server — Channel Structure

## Overview

This document defines the recommended channel layout for the VaultMTG Discord server. Ray creates the server; channel ownership and SLAs are defined here so each agent knows what they are responsible for.

---

## Channel Layout

### Category: INFO

| Channel | Purpose | Owner |
|---|---|---|
| #announcements | Release notes, beta invites, planned downtime | growth-marketing |
| #rules | Server rules and code of conduct | growth-marketing |
| #roadmap | Public-facing roadmap snippets; no internal dates | growth-marketing |

### Category: COMMUNITY

| Channel | Purpose | Owner |
|---|---|---|
| #general | Off-topic, MTG chat, community banter | growth-marketing (moderation) |
| #show-your-stats | Users share screenshots of their VaultMTG data | growth-marketing |
| #draft-chat | Draft strategy, pick orders, set discussion | unowned (community-driven) |

### Category: SUPPORT

| Channel | Purpose | Owner |
|---|---|---|
| #help | Installation issues, daemon problems, data questions | customer-success |
| #feedback | Feature requests and general product feedback | customer-success |
| #bugs | Reproducible bug reports; a GitHub issue is created for each one | customer-success |

### Category: BETA (visible to beta role only)

| Channel | Purpose | Owner |
|---|---|---|
| #beta-announcements | Beta-specific release notes | growth-marketing |
| #beta-feedback | Structured feedback requests (surveys, specific flows) | customer-success |
| #beta-bugs | Beta-specific bugs before public release | customer-success |

---

## Pinned Message Template — #help

Pin this message in #help when the server launches:

---

**Welcome to #help**

Having trouble with VaultMTG? You're in the right place.

**Before posting, check:**
- Our FAQ: https://vaultmtg.app/help (placeholder until site is live)
- Pinned messages in this channel
- Previous messages — your issue may already be answered

**When posting a bug or issue, include:**
1. Your operating system (Mac / Windows) and version
2. VaultMTG app version (shown in Settings)
3. What you were doing when the problem happened
4. What you expected vs. what actually happened
5. A screenshot if you have one

**For live chat**, use the chat icon on vaultmtg.app.

The team monitors this channel and responds within 24 hours on weekdays.

---

## Response SLA Targets

| Channel | Target first response | Notes |
|---|---|---|
| #help | 24 hours (weekdays) | Acknowledge within 24h even if resolution takes longer |
| #bugs | 48 hours (weekdays) | Acknowledge + create GitHub issue; do not promise fix timeline |
| #feedback | 72 hours (weekdays) | Thank the user; no commitment to build timeline |
| #beta-bugs | 24 hours (weekdays) | Higher priority during active beta window |

Weekend messages are acknowledged on the following business day. Users should be notified of this policy in #rules.

---

## Ownership Summary

| Owner | Channels |
|---|---|
| growth-marketing | #announcements, #rules, #roadmap, #general, #show-your-stats, #beta-announcements |
| customer-success | #help, #feedback, #bugs, #beta-feedback, #beta-bugs |
| community-driven | #draft-chat |

---

## Launch Checklist (for Ray)

- [ ] Create Discord server named "VaultMTG"
- [ ] Create all categories and channels above
- [ ] Set #announcements to read-only for everyone except growth-marketing role
- [ ] Set #beta-* channels to visible only to users with the "Beta Tester" role
- [ ] Pin the #help message template above
- [ ] Invite customer-success team as moderators of #help, #feedback, #bugs
- [ ] Post server invite link to vaultmtg.app (coordinate with front-engineer)
