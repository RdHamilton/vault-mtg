# PRD: v0.3.1 "Packaging"

**Owner: Najah (Product Manager)**
**Status**: ACTIVE — started 2026-05-09
**Board**: #33 (Project ID: `PVT_kwHOABsZ684BXMn-`)
**Milestone**: v0.3.1 — ships before v0.4.0
**Last updated**: 2026-05-09 (revised — restored Component Library Foundation wave per Ray's direction; Storybook tickets #1621, #1622, #1625 are v0.3.1 scope)

---

## Theme

**Daemon Packaging**

v0.3.1 delivers one thing: a double-clickable, self-configuring daemon installer for both macOS and Windows. This is the prerequisite for beta users who are not engineers.

Engineering does **not** begin v0.4.0 Wave 0 until v0.3.1 closes and PM issues a GO.

> **Scope note (2026-05-09, updated)**: The Component Library Foundation (Storybook + Chromatic) — tickets #1621, #1622, #1625 — is **v0.3.1 scope** per Ray's direction. Previously removed in error; restored as Wave 5. Waves renumbered accordingly.

---

## Problem Statement

VaultMTG's daemon ships as raw unsigned binaries installed via shell scripts. Beta users cannot be expected to use a terminal. The product cannot move past alpha without:

1. A double-clickable installer on macOS (.dmg → .pkg) and Windows (.exe via NSIS)
2. A PKCE browser-based auth flow so the daemon self-configures without manual API key copy-paste
3. A BFF registration endpoint to mint API keys from Clerk JWTs
4. A SPA `/setup` page that guides users through install and first-run pairing
5. GA-prep documentation so notarization and Azure signing can be activated at GA without scrambling

---

## Target Users

- **Beta invitees**: MTG Arena players who are not engineers — they need a standard installer UX.
- **Future GA team**: Apple Developer Program enrollment and Azure Trusted Signing must be documented before GA, not scrambled at launch.

---

## Success Metrics

- **Primary**: ≥80% of users who download the installer complete daemon pairing within 10 minutes, measured via PostHog `daemon_paired` event.
- **Secondary**: Zero support tickets attributable to "could not install" in the first two weeks of beta.
- **Secondary**: `daemon_paired` PostHog event fires from at least one real test session before tag is cut.

---

## Wave Structure

### Wave 0 — Architect Gate (prerequisite — no tickets move to In Progress until cleared)

**Theme**: Validate architectural implications before engineering starts.

**Gate**: Ray (architect) reviews and posts architectural implications note. PM confirms receipt before any Wave 1 ticket moves to In Progress.

No tickets — ceremony only.

---

### Wave 1 — Binary Build + Installer Foundation

**Theme**: Produce signed-ready binaries and platform installers via GoReleaser.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1639 | feat(daemon): add GoReleaser config — produce darwin universal binary + windows amd64 binary | backend-engineer | M |
| #1640 | feat(daemon): macOS .pkg installer — port LaunchAgent logic from install.sh into pkgbuild/productbuild, wrap in .dmg | backend-engineer | M |
| #1641 | feat(daemon): Windows NSIS .exe installer — port Scheduled Task logic from install.ps1, no UAC | backend-engineer | M |
| #1642 | feat(ci): replace daemon-release.yml matrix with GoReleaser-driven workflow | infrastructure | S |

**Definition of done (Wave 1):**
- GoReleaser config produces darwin universal binary and windows amd64 binary from a single `goreleaser release --snapshot` run
- macOS `.pkg` installs per-user LaunchAgent; daemon starts after install without a terminal
- Windows `.exe` installs Scheduled Task; daemon starts after install without UAC prompt
- GoReleaser-driven CI workflow replaces the old matrix; tag `v*` triggers release build

---

### Wave 2 — First-Run Auth (PKCE + Keychain + BFF Endpoint)

**Theme**: Wire zero-terminal authentication: daemon detects missing config, opens browser, completes PKCE, stores key in OS keychain, registers with BFF.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1643 | feat(daemon): first-run config detection — missing daemon.json opens vaultmtg.app/setup or prints URL | backend-engineer | S |
| #1650 | feat(daemon): implement PKCE OAuth browser-redirect login flow | backend-engineer | M |
| #1651 | feat(daemon): store Clerk API key in OS keychain (go-keyring) | backend-engineer | S |
| #1652 | feat(bff): add POST /v1/daemon/register endpoint — accept Clerk JWT, mint first API key | backend-engineer | S |

**Definition of done (Wave 2):**
- Daemon with no `daemon.json` opens `vaultmtg.app/setup` in system browser (or prints URL if headless)
- PKCE flow: daemon binds localhost callback → opens Clerk login → captures auth code → exchanges for session token → calls `POST /v1/daemon/register`
- BFF verifies Clerk JWT, mints API key, returns it; endpoint covered by integration tests
- Daemon writes API key to OS keychain (macOS Keychain / Windows Credential Manager) — NOT plaintext in `daemon.json`
- On subsequent starts, daemon reads key from keychain and does not re-open browser
- Port conflict on localhost callback is handled gracefully (retry with ephemeral port, no crash)
- PostHog `daemon_paired` event fires after successful keychain write
- `go-keyring` added to `services/daemon/go.mod`

---

### Wave 3 — SPA Setup Page + Download UX

**Theme**: Give first-time users a guided install path from the web app, replacing the broken shell-script-based install flow.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1644 | feat(spa): /setup page — first-time install warnings with Gatekeeper + SmartScreen screenshots and bypass instructions | front-engineer | S |
| #1645 | feat(spa): /setup page — PKCE auth flow replaces SPA-mint-key pairing (ADR-020) | front-engineer | S |
| #1646 | feat(spa): DaemonDownload.tsx — replace broken install script links with .dmg and .exe download buttons | front-engineer | XS |
| #1647 | docs(daemon): update install README — describe .pkg/.dmg and NSIS .exe paths, mark shell scripts as power-user fallback | documentation | XS |

**Definition of done (Wave 3):**
- `/setup` page renders Gatekeeper (macOS) and SmartScreen (Windows) bypass instructions with screenshots
- PKCE flow on SPA replaces the old SPA-mint-key flow per ADR-020
- `DaemonDownload.tsx` shows .dmg and .exe buttons linked to GitHub Releases; no broken shell script links
- Install README updated and accurate

---

### Wave 4 — GA Readiness Documentation

**Theme**: Document notarization and Azure signing workflows so GA activation is a checklist, not a scramble.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1648 | chore(ga-prep): enroll in Apple Developer Program — document notarization workflow and notarytool credentials in SSM | infrastructure | S |
| #1649 | chore(ga-prep): onboard Azure Trusted Signing — document signing workflow in GoReleaser config, budget approval | infrastructure | S |

**Definition of done (Wave 4):**
- Apple Developer Program enrollment documented; `notarytool` credential path in SSM documented
- Azure Trusted Signing workflow documented in GoReleaser config comments; budget approval recorded
- Neither ticket requires actual notarization/signing to be active — documentation is the deliverable

---

### Wave 5 — Component Library Foundation

**Theme**: Establish Storybook + Chromatic baseline so component visual regression is tracked before beta ships.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1621 | spike(storybook): Storybook + Chromatic discovery spike — evaluate Vite builder compatibility and Chromatic pricing | front-engineer | M |
| #1622 | feat(storybook): install and configure Storybook 8 with Vite builder | front-engineer | M |
| #1625 | feat(chromatic): capture Chromatic baseline snapshots for existing components | front-engineer | M |

**Definition of done (Wave 5):**
- Chromatic baseline approved by Ray (no unresolved snapshot diffs)
- Storybook deployed to Chromatic; Chromatic project URL documented in repo
- CI passes with Chromatic check as a required status
- All three tickets in Done state on Project #33

---

### Wave 6 — CI Hardening

**Theme**: Tighten supply-chain security and vulnerability scanning before shipping installers to beta users.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| TBD | Upgrade CI to Node.js 22 LTS across all jobs | infrastructure | S |
| TBD | Pin all GitHub Actions to SHA-pinned versions | infrastructure | XS |
| TBD | Add `govulncheck` to Go CI jobs | infrastructure | XS |
| TBD | Add `npm audit --audit-level=high` to frontend CI | infrastructure | XS |

**Definition of done (Wave 6):**
- All CI jobs green on Node.js 22 LTS
- All GitHub Actions pinned to SHA (not floating tag)
- `govulncheck` passes with zero high/critical findings
- `npm audit` passes with zero high-severity findings

> Note: Wave 6 tickets are not yet created on GitHub. PM to file before Wave 5 closes.

---

### Wave 7 — Staging Validation

**Theme**: Prove the staging environment is clean and smoke-tested before the release tag is cut.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| TBD | Staging deploy pipeline smoke test | infrastructure | S |
| TBD | BFF `/healthz` verified on staging | backend-engineer | XS |
| TBD | Playwright staging smoke suite runs clean | ui-tester | S |

**Definition of done (Wave 7):**
- Staging deploy completes clean from scratch
- BFF `/healthz` returns 200 on staging
- Playwright staging smoke suite passes with zero failures

> Note: Wave 7 tickets are not yet created on GitHub. PM to file before Wave 6 closes.

---

### Wave 8 — Release Gate (Smoke Test + Tag + Changelog)

**Theme**: Validate the full end-to-end install-to-event flow on both platforms, cut the v0.3.1 tag, and publish the changelog.

**Tickets**: No new code tickets — ceremony wave only.

**Definition of done (Wave 8 — all must be true before tag is cut):**
1. CI is green on main (hard gate — no exceptions per BROADCAST Active Directive 2)
2. Staging deploy pipeline runs from scratch; BFF `/healthz` returns 200
3. Playwright staging smoke suite passes
4. Manual install smoke test on macOS 14+ and Windows 11: download `.dmg`/`.exe` → install → PKCE login → daemon starts → first event appears in BFF (checked via DB or PostHog)
5. All Wave 1–7 tickets are in Done state on Project #33 board
6. PostHog `daemon_paired` event confirmed firing from at least one real test session
7. `CHANGELOG.md` entry written for v0.3.1
8. v0.3.1 git tag cut; GitHub Release created with `.dmg` and `.exe` artifacts attached

---

## Out of Scope (v0.3.1)

| Item | Reason deferred |
|------|----------------|
| Apple notarization (active) | Requires paid Apple Developer Program enrollment — GA milestone; Wave 4 documents the workflow only |
| Azure Trusted Signing (active) | $9.99/mo — budget approval needed; GA milestone; Wave 4 documents the workflow only |
| GoReleaser Pro features | Open-source tier sufficient for beta |
| System tray / menubar icon | Not on critical path for beta |
| MSI installer (Windows enterprise) | Post-GA only if requested |
| Linux packaging | Not a supported platform |
| Homebrew cask | Secondary distribution channel, post-GA |
| Automatic daemon updater | Post-GA |
| Device authorization flow (headless) | Fallback for CI/server — not a beta user scenario |
| Full component story library (beyond Wave 5 baseline) | v0.4.0 follow-on after Chromatic baseline is established |
| API key issuance/revoke UI (#1314) | Superseded by PKCE flow — daemon handles key acquisition automatically; SPA UI for key management deferred post-beta |

---

## Open Questions

| # | Question | Owner | Gate |
|---|----------|-------|------|
| OQ-1 | Apple Developer Program: has Ray confirmed account creation and payment method? Wave 4 (#1648) requires knowing credential storage path. | Ray | Wave 4 |
| OQ-2 | Azure Trusted Signing budget: approved? Wave 4 (#1649) includes budget approval as an AC — who signs off? | Ray | Wave 4 |
| OQ-3 | Storybook tickets (#1621, #1622, #1625) scope. | PM | ✅ Resolved — confirmed v0.3.1 per Ray's direction (2026-05-09); tickets relabeled from v0.4.0 to v0.3.1 |
| OQ-4 | Gatekeeper bypass: on macOS 14+ with no notarization, does right-click → Open produce a one-click bypass, or a hard block? Must be confirmed on a clean macOS 14 VM before Wave 7 closes. | Ray (eng) | Wave 7 |
| OQ-5 | Ephemeral port range for PKCE callback — fixed port (e.g., 51423) for UX consistency, or fully random? Fixed port simplifies firewall instructions. Decision needed before #1650 starts. | backend-engineer + Ray | Before Wave 2 |
| OQ-6 | API key scoping — per-machine or per-user-session? Per-machine is simpler but means reinstall requires re-pairing. | backend-engineer | Before Wave 2 |
| OQ-7 | Key revocation on reinstall — should the old key be revoked automatically on reinstall? | backend-engineer + Ray | Before Wave 2 |

---

## RICE Score

| Initiative | Reach | Impact | Confidence | Effort (pw) | Score |
|---|---|---|---|---|---|
| Daemon packaging (installer + PKCE) | 500 (all beta invitees) | 3 (enabling — no beta without this) | 95% | 6 | 237.5 |

---

## Dependencies

- ADR-011 — daemon distribution strategy
- ADR-020 — PKCE auth acquisition (supersedes ADR-011 first-run section)
- ADR-009 — Clerk auth (JWT on BFF, `clerk-sdk-go v2`)
- `go-keyring` — OS keychain library (add to `go.mod`)
- GoReleaser open-source — build orchestrator
- `pkgbuild` / `productbuild` — macOS system tools (available on macOS GitHub Actions runner)
- NSIS — Windows installer compiler (via `chocolatey install nsis` on Windows runner)
- v0.3.0 tag must be cut before engineering starts

---

## Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|------------|
| R-1 | Gatekeeper hard-blocks unsigned .dmg on macOS 14+ — right-click → Open does not produce a bypass | Medium | High | Confirm on clean macOS 14 VM in Wave 3; add explicit Gatekeeper bypass instructions to `/setup` page in Wave 3 regardless |
| R-2 | SmartScreen hard-blocks unsigned .exe on Windows 11 — no bypass path | Medium | High | Document SmartScreen bypass in Wave 3 SPA; confirm on clean Windows 11 VM; escalate to Ray if block is unbypassable without EV signing |
| R-3 | PKCE localhost callback port conflict | Low | Medium | Handle gracefully in #1650: retry with ephemeral port, surface error message, never crash |
| R-4 | `go-keyring` OS keychain integration fails on a specific macOS/Windows version | Low | High | Spike keychain write/read in #1651 before committing to keychain-only storage; keep plaintext fallback path documented |
| R-5 | Azure identity validation not approved before Wave 4 closes | Medium | Low | Wave 4 documents the workflow only — approval is not required to close the wave; escalate if approval is not received before GA prep begins |
| R-6 | Wave 5–6 tickets not created before those waves start | High | Medium | PM action item: file Wave 5 tickets before Wave 4 closes, Wave 6 tickets before Wave 5 closes |

---

## Sequencing Note

v0.3.1 is a blocking prerequisite for v0.4.0. Engineering does not begin v0.4.0 Wave 0 until PM issues a formal GO after all v0.3.1 Wave 7 release gate items are green.

Waves 1–4 can partially overlap where tickets have no inter-wave dependencies. Waves 5–6 must run sequentially after Wave 4. Wave 7 cannot start until all prior waves are closed.
