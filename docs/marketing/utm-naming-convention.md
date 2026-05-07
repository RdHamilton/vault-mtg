# UTM Naming Convention — VaultMTG Beta Launch

**Version**: 1.0
**Created**: 2026-05-06
**Owner**: Growth & Marketing

---

## Guiding Rules

1. All values lowercase, words separated by hyphens. No underscores except where GA4 requires them.
2. utm_campaign is the only value that changes per launch wave. utm_source and utm_medium are fixed vocabularies — never invent new values without updating this doc.
3. Every public link leaving the VaultMTG team (Reddit, Discord, X, email, DMs) must carry UTM parameters. Direct traffic without UTMs is unattributable.
4. Build URLs using the generator at the bottom of this doc. Do not assemble by hand.

---

## Standard Parameter Values

### utm_source — Where the click originates

| Value | When to use |
|---|---|
| `reddit` | Any post or comment on Reddit |
| `discord` | Any link posted in a Discord server (VaultMTG server or external) |
| `x` | Posts and replies on X (Twitter) |
| `email` | Mailchimp campaigns, transactional emails, invite emails |
| `direct` | QR codes, physical materials, links with no referrer context |

### utm_medium — How the content reaches the audience

| Value | When to use |
|---|---|
| `social` | Organic posts on X |
| `community` | Reddit posts/comments, Discord messages — conversational placements |
| `organic` | Blog content, SEO landing pages linked from within the site |
| `referral` | Shared links between users (future referral program) |
| `email` | All email campaigns and transactional messages |

### utm_campaign — Which initiative this link belongs to

Pattern: `{initiative}-{YYYY-MM}` or `{initiative}-{wave}`

| Value | Use case |
|---|---|
| `beta-launch-2026-05` | All links in the initial beta announcement wave (May 2026) |
| `wave1-invite` | Direct invite emails sent to the first cohort of beta users |
| `wave2-invite` | Second cohort invite emails |
| `waitlist-confirm` | Automated confirmation email on waitlist signup |
| `set-launch-{setcode}` | Links tied to a new MTG set release (e.g., `set-launch-dsk`) |
| `weekly-digest` | Recurring weekly metagame email |
| `reengagement-2026-06` | Re-engagement email to inactive users |

### utm_content — A/B variant or placement within a campaign

Used to distinguish two links in the same campaign pointing to the same destination.

| Pattern | Example |
|---|---|
| `cta-{variant}` | `cta-apply` vs. `cta-learn-more` |
| `placement-{location}` | `placement-top` vs. `placement-footer` |
| `copy-{variant}` | `copy-a` vs. `copy-b` |
| `format-{type}` | `format-text` vs. `format-image` |

Leave utm_content blank if there is only one link variant in the placement.

---

## Pre-Built UTM URL Lookup Table

Base URL: `https://vaultmtg.app`

| # | Placement | URL |
|---|---|---|
| 1 | Reddit — r/magicTCG beta announcement post (body link) | `https://vaultmtg.app?utm_source=reddit&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-body` |
| 2 | Reddit — r/magicTCG beta announcement post (comment CTA link) | `https://vaultmtg.app?utm_source=reddit&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-comment` |
| 3 | Reddit — r/lrcast beta announcement post (body link) | `https://vaultmtg.app?utm_source=reddit&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-lrcast-body` |
| 4 | Reddit — r/MagicArena beta announcement post (body link) | `https://vaultmtg.app?utm_source=reddit&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-magicArena-body` |
| 5 | Discord — VaultMTG #announcements (primary CTA) | `https://vaultmtg.app?utm_source=discord&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=cta-apply` |
| 6 | Discord — VaultMTG #announcements (secondary learn more link) | `https://vaultmtg.app?utm_source=discord&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=cta-learn-more` |
| 7 | Discord — External MTG server link drop | `https://vaultmtg.app?utm_source=discord&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-external` |
| 8 | X (Twitter) — First tweet in thread | `https://vaultmtg.app?utm_source=x&utm_medium=social&utm_campaign=beta-launch-2026-05&utm_content=placement-tweet1` |
| 9 | X (Twitter) — Thread follow-up CTA tweet | `https://vaultmtg.app?utm_source=x&utm_medium=social&utm_campaign=beta-launch-2026-05&utm_content=placement-tweet-cta` |
| 10 | Email — Wave 1 invite (header CTA button) | `https://vaultmtg.app?utm_source=email&utm_medium=email&utm_campaign=wave1-invite&utm_content=cta-button` |
| 11 | Email — Wave 1 invite (footer text link) | `https://vaultmtg.app?utm_source=email&utm_medium=email&utm_campaign=wave1-invite&utm_content=placement-footer` |
| 12 | Email — Waitlist confirmation email | `https://vaultmtg.app?utm_source=email&utm_medium=email&utm_campaign=waitlist-confirm&utm_content=cta-learn-more` |
| 13 | Email — Wave 2 invite (CTA button) | `https://vaultmtg.app?utm_source=email&utm_medium=email&utm_campaign=wave2-invite&utm_content=cta-button` |
| 14 | Discord — VaultMTG #announcements (daemon install guide link) | `https://vaultmtg.app/install?utm_source=discord&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-install-guide` |
| 15 | Reddit — r/lrcast comment reply (when answering questions) | `https://vaultmtg.app?utm_source=reddit&utm_medium=community&utm_campaign=beta-launch-2026-05&utm_content=placement-comment-reply` |

---

## Generating New UTMs

Use this formula for any new link not in the table above:

```
https://vaultmtg.app[/path]
  ?utm_source=[source]
  &utm_medium=[medium]
  &utm_campaign=[campaign]
  &utm_content=[content]   ← omit if only one variant
```

Steps:
1. Pick utm_source from the standard values table. If none fit, discuss with the team before inventing a new value.
2. Pick utm_medium from the standard values table.
3. Pick or create a utm_campaign. If the campaign doesn't exist yet, add it to the campaign table above.
4. Add utm_content only if you have two or more links going to the same URL in the same campaign.
5. Add the new URL to the lookup table above with a description of its placement.
6. Verify the link resolves correctly before publishing.

---

## GA4 Verification

After any campaign goes live, verify attribution is landing correctly:
- GA4 > Reports > Acquisition > Traffic acquisition
- Filter by Session campaign = `beta-launch-2026-05` (or relevant value)
- Confirm utm_source, utm_medium, and utm_content dimensions are populated

If a link shows as `(not set)`, the UTM parameters were either missing or malformed — check for encoding issues (spaces must be `%20` or use hyphens).
