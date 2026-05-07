# UX Designer Changelog

<!-- Entries are appended newest-first via consolidate.py. Format:
## YYYY-MM-DD — [Task name]
**Type**: [brand doc / component spec / wireframe / design review]
**Output**: [file path]
**Key decisions**: [palette rationale, font choices, notable tradeoffs]
-->

## 2026-05-06 — VaultMTG comprehensive design system spec
**Type**: design system / component spec
**Output**: docs/design/vaultmtg-design-system.md
**Key decisions**: Built on top of the existing vaultmtg-brand.md palette (Vault Amber #F5A623, cool-slate surfaces, Sora/Inter/JetBrains Mono stack). Audited all 30+ component CSS files in frontend/src/components/ — found the existing app uses entirely ad-hoc hex values (#4a9eff blue, #1e1e1e/#2d2d2d gray surfaces, no variables) which confirm there is no competing design system to reconcile. Added surface-sunken (#0A0E16) for inputs and code blocks — a fourth surface tier not in the brand doc, necessary for dark inputs on dark cards. Added secondary accent (Indigo #6366F1) to provide an alternative interactive color for non-primary elements. Full component specs for Button (4 variants + icon), Card (3 variants), Badge/Tag (tier, format, pip, win-rate), Table, Nav/Sidebar, Input/Select, Toast (4 variants), Empty State, Loading Skeleton, and Draft Pick Card. CSS custom properties block ready to drop into index.css. Tailwind config extension includes overridden font sizes (11/13/15/17/20/26/34/48 instead of Tailwind defaults), all color tokens, shadow tokens, z-index scale, and shimmer/toast/fade keyframes. Migration notes prioritize the high-frequency replacements (#4a9eff → primary, #aaaaaa → text-secondary) to guide front-engineer through the token swap.

## 2026-05-05 — rhamiltoneng.com brand design document
**Type**: brand doc
**Output**: docs/design/rhamiltoneng-brand.md
**Key decisions**: Chose indigo (#4F46E5, 6.0:1 contrast) as sole brand accent — restrained and WCAG AA on light surfaces. Font pairing: Plus Jakarta Sans (display, 700-800) + Inter (body) — distinct enough to give the site voice, common enough to render everywhere. Surface base is Slate 50 (#F8FAFC) rather than pure white — warmer, less clinical. All text colors verified WCAG AA: text-primary 14.0:1, text-secondary 7.2:1, text-muted 4.5:1 (exactly meeting threshold). Aesthetic intentionally inverts VaultMTG: daylight surfaces, single accent, generous whitespace, no decorative chrome.

## 2026-05-05 — VaultMTG brand design document
**Type**: brand doc
**Output**: docs/design/vaultmtg-brand.md
**Key decisions**: Primary accent chosen as Vault Amber (#F5A623) — warm gold-amber evokes mastery and premium feel without copying MTG IP. Surface palette uses cool-neutral dark slate (#0D1117 base) with a blue-indigo tint for a polished, gaming-appropriate dark theme. Font stack: Sora (display) for geometric precision, Inter (body) for screen readability and tabular numerals, JetBrains Mono (data) for stat alignment. All text colors verified WCAG AA compliant.
