# Business Analyst Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — [Task name]
**Type**: [weekly metrics / feature adoption / A/B test / competitive / ad-hoc]
**Output**: [file path]
**Key finding**: [the one insight that changes how PM should think about something]
-->

## 2026-05-06 — PostHog event taxonomy finalization
**Type**: ad-hoc
**Output**: /Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/docs/analytics/event-taxonomy.md
**Key finding**: Zero PostHog instrumentation exists in main — PR #1463 is on a branch not checked out here; all 31 events defined in the taxonomy are net-new work. The activation funnel has 7 events, all unimplemented, meaning beta will launch with no funnel visibility unless the frontend ticket is prioritized immediately.

## 2026-05-06 — Beta success metrics and funnel to 50K MAU
**Type**: ad-hoc
**Output**: returned inline to product-manager planning session
**Key finding**: Untapped.gg pulls ~2–3M monthly visits against an estimated 1–3M MAU player base that uses companion tools; reaching 50K MAU requires ~500K–750K monthly organic visits with a 2–5% visit-to-signup rate and 25–35% D30 retention — the retention rate is the dominant lever, not acquisition volume.
