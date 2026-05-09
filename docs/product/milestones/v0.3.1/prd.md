# PRD: v0.3.1 "Packaging"

**Owner: Najah (Product Manager)**
**Status**: ACTIVE — started 2026-05-09
**Board**: #33 (Project ID: `PVT_kwHOABsZ684BXMn-`)
**Milestone**: v0.3.1 — ships before v0.4.0
**Last updated**: 2026-05-09

---

## Theme

**Daemon Packaging + Component Library Foundation**

v0.3.1 completes two parallel tracks before v0.4.0 begins:

1. **Daemon Packaging** — Signed and notarized macOS daemon binary, Windows installer, PKCE browser auth flow, and tag-guarded release workflows. This is the prerequisite for beta users to install the daemon without terminal access.
2. **Component Library Foundation** — Storybook 8 installed, Chromatic baseline captured, and visual regression CI in place before beta users see the UI. Visual regressions caught pre-beta are significantly cheaper to fix than those surfaced post-launch.

Engineering does **not** begin v0.4.0 Wave 0 until v0.3.1 closes and PM issues a GO.

---

## Problem Statement

VaultMTG's daemon ships as raw unsigned binaries installed via shell scripts. Beta users cannot be expected to use a terminal. The product cannot move past alpha without a double-clickable installer and a PKCE browser login flow.

In parallel, the React component library has no visual regression baseline. Beta is the first external audience; without Storybook and Chromatic in place before beta, visual regressions will surface in production with no automated safety net.

---

## Target Users

- **Beta invitees**: MTG Arena players who are not engineers — they need a standard installer UX.
- **Engineering team**: Storybook enables UX reviews without a running environment; Chromatic gates PRs on visual changes before they ship.

---

## Success Metrics

- **Primary (packaging)**: ≥80% of users who download the installer complete daemon pairing within 10 minutes, measured via PostHog `daemon_paired` event.
- **Primary (component library)**: Chromatic baseline captured from main and all stories approved by ux-designer before v0.3.1 closes.
- **Secondary**: Zero support tickets attributable to "could not install" in the first two weeks of beta; no visual regressions ship without a Chromatic review.

---

## Wave Structure

### Wave 1 — Daemon Packaging (Engineering Track)

Detailed ACs are in `docs/prd/0001-daemon-packaging.md`.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1601 | GoReleaser pipeline — macOS DMG + Windows NSIS | infrastructure + backend-engineer | M |
| #1602 | macOS .pkg per-user LaunchAgent install | backend-engineer | M |
| #1603 | Windows NSIS installer with Scheduled Task | backend-engineer | M |
| #1604 | PKCE first-run auth flow (daemon → browser → keychain) | backend-engineer | M |
| #1605 | BFF `POST /v1/daemon/register` endpoint | backend-engineer | S |
| #1606 | SPA `/setup` page — download link, installer guide, status indicator | front-engineer | S |
| #1607 | CI tag-guarded release workflow | infrastructure | S |

**Exit gate**: DMG mounts and installs silently on macOS 13+; NSIS installs silently on Windows 10+; PKCE flow completes in <60 seconds; PostHog `daemon_paired` event fires after first install.

---

### Wave 2 — CI Hardening

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1610 | Upgrade CI to Node.js 22 LTS across all jobs | infrastructure | S |
| #1611 | Pin all GitHub Actions to SHA-pinned versions | infrastructure | XS |
| #1612 | Add `govulncheck` to Go CI jobs | infrastructure | XS |
| #1613 | Add `npm audit --audit-level=high` to frontend CI | infrastructure | XS |

**Exit gate**: All CI jobs green; no high-severity vulnerabilities in `npm audit` or `govulncheck`.

---

### Wave 3 — Staging Validation

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1615 | Staging deploy pipeline smoke test | infrastructure | S |
| #1616 | BFF `/healthz` verified on staging | backend-engineer | XS |
| #1617 | Playwright staging smoke suite runs clean | ui-tester | S |

**Exit gate**: Staging deploy completes clean from scratch; all smoke tests pass; BFF `/healthz` returns 200.

---

### Wave 4 — Pre-Release Packaging Validation

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1618 | macOS notarization readiness review (GA scope note) | backend-engineer | XS |
| #1619 | Windows EV signing readiness review (GA scope note) | backend-engineer | XS |
| #1620 | Full release checklist run against v0.3.1 tag | lead-engineer | XS |

**Exit gate**: v0.3.1 release tag cut; artifacts published to GitHub Releases; checksums verified.

---

### Wave 5 — Auth + API Key UX (Deferred from v0.2.0)

Ticket #1314 (API key issuance, list, revoke, rotate UI) and the daemon onboarding documented path were deferred from v0.2.0. They belong in v0.3.1 as packaging prerequisites.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1314 | API key issuance, list, revoke, rotate UI | front-engineer + backend-engineer | M |
| TBD | Daemon onboarding documented path (SPA `/setup` step-by-step) | front-engineer | S |

**Exit gate**: A new user can complete Clerk sign-up → API key copy → daemon install → first event without developer assistance.

---

### Wave 6 — Component Library Foundation

Beta users will see the UI before any other external audience. Visual regressions caught pre-beta are significantly cheaper to fix than those surfaced during or after closed beta. Storybook also serves as living component documentation that enables UX reviews without requiring a running environment.

**Theme**: Establish visual regression baseline before beta launch.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1621 | Discovery spike: Storybook + Chromatic setup on React 19 + Vite | front-engineer | XS (2 days) |
| #1622 | feat(frontend): install and configure Storybook 8 with Vite builder | front-engineer | S |
| #1625 | feat(frontend): capture Chromatic baseline snapshots + approve initial set | front-engineer + ux-designer | XS |

**ACs:**

- **#1621 — Discovery spike**: Spike doc written in `docs/engineering/reference/storybook-spike.md`; confirms Storybook 8 + `@storybook/react-vite` builder works with React 19; documents any known incompatibilities; GO/NO-GO recommendation included.
- **#1622 — Install + configure**: `npx storybook dev` runs without errors; `.storybook/` config committed; existing Vitest suite still passes; TypeScript has no new errors; Storybook deployed to Chromatic.
- **#1625 — Chromatic baseline**: Chromatic project linked in repo; `chromatic --exit-zero-on-changes` runs on every PR in CI; Chromatic build URL posted as a PR check; Chromatic baseline captured from main; all stories reviewed and accepted by ux-designer.

**Definition of Done (Wave 6):**
- [ ] Chromatic baseline approved by ux-designer
- [ ] Storybook deployed to Chromatic (accessible without a local dev environment)
- [ ] CI passes with Chromatic visual diff gate active
- [ ] No Vitest or TypeScript regressions introduced by Storybook installation

---

### Release Gate Wave — v0.3.1 Close

All of the following must be true before PM issues a GO for v0.4.0:

- [ ] DMG and NSIS artifacts published to GitHub Releases with checksums
- [ ] PKCE flow tested end-to-end on macOS and Windows (at least one successful pair each)
- [ ] CI green on main — no CI red allowed at release
- [ ] Staging deploy clean from scratch; BFF `/healthz` 200; Playwright smoke passes
- [ ] Chromatic baseline captured and approved by ux-designer
- [ ] Storybook deployed to Chromatic with CI visual diff gate active
- [ ] API key UX (#1314) live in SPA
- [ ] v0.3.1 tag cut and release notes published

---

## Out of Scope (v0.3.1)

- Apple Developer Program enrollment and notarization — GA milestone
- Azure Trusted Signing for Windows — GA milestone
- System tray / menubar icon — post-GA
- MSI installer (Windows enterprise) — post-GA if requested
- Linux packaging — unsupported platform
- Homebrew cask — secondary channel, post-GA
- Automatic daemon auto-updater — post-GA
- Storybook stories for all components (full story library) — v0.4.0 Wave 1 follow-on
- Chromatic CI visual diff gate integrated into PR blocking (full blocking gate) — v0.4.0 Wave 1 follow-on after baseline is captured

---

## Open Questions

1. **Ephemeral port range** — fixed port (e.g., 51423) first for UX consistency, or fully random? Fixed port simplifies firewall instructions. Decision needed before ADR-020 implementation.
2. **API key scoping** — per-machine or per-user-session? Per-machine is simpler but means reinstall requires re-pairing. Needs backend-engineer input.
3. **Key revocation on reinstall** — should the old key be revoked automatically on reinstall? Needs a call before the BFF registration endpoint is scoped.

---

## RICE Score

| Initiative | Reach | Impact | Confidence | Effort (pw) | Score |
|---|---|---|---|---|---|
| Daemon packaging (installer + PKCE) | 500 (all beta invitees) | 3 (enabling — no beta without this) | 95% | 6 | 237.5 |
| Component library (Storybook + Chromatic) | 500 (beta users see UI) | 2 (regression prevention) | 90% | 1 | 900 |

Storybook + Chromatic has a high RICE score because effort is minimal (XS + S + XS) and confidence is high — the toolchain is mature. The impact on regression prevention compounds over time; catching a visual bug pre-beta versus post-beta is a significant cost multiplier.

---

## Dependencies

- ADR-011 — daemon distribution strategy
- ADR-020 — PKCE auth acquisition (supersedes ADR-011 first-run section)
- ADR-009 — Clerk auth (JWT on BFF, `clerk-sdk-go v2`)
- `go-keyring` — OS keychain library (add to `go.mod`)
- GoReleaser open-source — build orchestrator
- `pkgbuild` / `productbuild` — macOS system tools (available on macOS GitHub Actions runner)
- NSIS — Windows installer compiler (via `chocolatey install nsis` on Windows runner)
- Storybook 8 + `@storybook/react-vite` builder — confirmed compatible with React 19 (pending spike #1621)
- Chromatic — visual testing SaaS (free tier covers pre-beta volume)

---

## Sequencing Note

v0.3.1 is a blocking prerequisite for v0.4.0. Engineering does not begin v0.4.0 Wave 0 until PM issues a formal GO after all v0.3.1 Release Gate items are green.

Storybook tickets (#1621, #1622, #1625) are v0.3.1 scope, not v0.4.0. They are assigned to Wave 6 of this milestone and must complete before the Release Gate wave closes.
