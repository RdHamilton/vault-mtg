# Customer Success Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — [Task name]
**Type**: [feedback synthesis / bug triage / support doc / NPS / loop closure]
**Output**: [file path or GitHub issue numbers]
**Key insight**: [the one thing PM most needs to know]
-->

## 2026-05-06 — Pre-beta daemon support knowledge base
**Type**: support doc
**Output**: docs/support/daemon-installation.md, docs/support/daemon-troubleshooting.md, docs/support/daemon-uninstall.md, docs/support/faq.md (replaced)
**Key insight**: The top predicted support volume during beta is the Gatekeeper/SmartScreen unsigned-installer friction — both install and troubleshooting articles lead with that flow; having these docs live before invites go out should deflect the majority of first-session support tickets

## 2026-05-06 — Beta access model recommendation (PRD Open Question #4)
**Type**: feedback synthesis
**Output**: Inline recommendation delivered to multi-agent PRD planning session
**Key insight**: Recommend invite-only (25 users) with explicit numeric gates (80% daemon connection success) before any batch expansion; fully open beta is not viable until silent daemon failure has a visible error state and self-serve troubleshooting doc exists

## 2026-05-06 — Beta user expectations research
**Type**: feedback synthesis
**Output**: Delivered inline report to multi-agent planning session
**Key insight**: Users will accept rough UI in beta but will churn instantly if match data is inaccurate or if the daemon setup fails silently — data trust and onboarding success rate are the two gates that must clear before any beta invite goes out
