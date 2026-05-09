# v0.3.1 Kickoff: "Packaging"

**Date**: 2026-05-09
**Milestone**: v0.3.1 "Packaging"
**Board**: #33 (`PVT_kwHOABsZ684BXMn-`)
**Author**: Najah (Product Manager)
**Status**: ACTIVE

---

## 1. Purpose

This document is the official kickoff for v0.3.1 "Packaging." It establishes wave structure, ownership, dependencies, exit gates, and open PM action items. All engineering agents must read this document before picking up any v0.3.1 ticket.

v0.3.1 ships before v0.4.0. Engineering does not begin v0.4.0 Wave 0 until PM issues a formal GO after the v0.3.1 Wave 7 release gate is green.

---

## 2. Milestone Overview

### What v0.3.1 Is

v0.3.1 delivers a double-clickable, self-configuring daemon installer for macOS and Windows. Beta users are not engineers. They cannot run shell scripts in a terminal. They cannot copy-paste API keys. This milestone closes that gap.

The milestone has one primary track:

1. **Daemon Packaging** — GoReleaser produces a darwin universal binary and windows amd64 binary. Platform-specific installers (.dmg → .pkg on macOS, .exe via NSIS on Windows) handle LaunchAgent and Scheduled Task setup. A PKCE browser-redirect auth flow eliminates manual key management. The SPA `/setup` page guides users through download, install, and first-run pairing. GA-prep documentation ensures notarization and Azure signing can be activated at GA without a scramble.

### Why It Exists

v0.3.0 closed with telemetry parity proven but the daemon still requiring terminal installation. That is a beta blocker. No beta user who is not an engineer will successfully install the daemon without a standard installer. v0.3.1 exists to remove that blocker before v0.4.0 begins beta-readiness work.

The waitlist opens **August 1, 2026**. The public closed beta launches **August 18, 2026**. v0.3.1 must close well before v0.4.0 can complete its own work. Time is not abundant.

### What Success Looks Like

- A macOS user downloads the `.dmg`, double-clicks, and the daemon is installed and running within 5 minutes with zero terminal interaction.
- A Windows user downloads the `.exe`, double-clicks, and the daemon is installed and running within 5 minutes with zero terminal interaction.
- Both users complete Clerk login via a browser window that opens automatically on first run.
- PostHog `daemon_paired` event fires within 10 minutes of download for ≥80% of beta invitees.
- Zero support tickets attributable to "could not install" in the first two weeks of beta.

---

## 3. Wave Plan

### Wave 0 — Architect Gate (prerequisite, no code)

| | |
|--|--|
| **Theme** | Validate architectural implications before engineering starts |
| **Goal** | Ray (architect) reviews v0.3.1 scope and posts an architectural implications note. PM confirms receipt before any Wave 1 ticket moves to In Progress. |
| **Tickets** | None — ceremony gate |
| **Definition of done** | Ray's architectural implications note received and acknowledged by PM |
| **Assigned** | Ray (architect) + Najah (PM) |
| **Effort** | XS |

---

### Wave 1 — Binary Build + Installer Foundation

| | |
|--|--|
| **Theme** | Produce platform installers via GoReleaser |
| **Goal** | A tag push produces a .dmg (macOS) and .exe (Windows) installer with zero manual build steps |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1639 | feat(daemon): add GoReleaser config — produce darwin universal binary + windows amd64 binary | backend-engineer | M |
| #1640 | feat(daemon): macOS .pkg installer — port LaunchAgent logic from install.sh into pkgbuild/productbuild, wrap in .dmg | backend-engineer | M |
| #1641 | feat(daemon): Windows NSIS .exe installer — port Scheduled Task logic from install.ps1, no UAC | backend-engineer | M |
| #1642 | feat(ci): replace daemon-release.yml matrix with GoReleaser-driven workflow | infrastructure | S |

**Definition of done:**
- [ ] `goreleaser release --snapshot` produces darwin universal binary and windows amd64 binary without errors
- [ ] macOS `.pkg` installs per-user LaunchAgent; daemon starts after install without a terminal
- [ ] Windows `.exe` installs Scheduled Task; daemon starts after install without UAC prompt
- [ ] GoReleaser-driven CI workflow active; tag `v*` triggers release build
- [ ] CI green on main after merge

**Assigned agents**: backend-engineer (primary), infrastructure
**Estimated effort**: L (3× M + 1× S)

---

### Wave 2 — First-Run Auth (PKCE + Keychain + BFF Endpoint)

| | |
|--|--|
| **Theme** | Zero-terminal daemon authentication |
| **Goal** | Daemon detects missing config, opens browser, completes PKCE, stores key in OS keychain, registers with BFF — no manual key copy-paste ever |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1643 | feat(daemon): first-run config detection — missing daemon.json opens vaultmtg.app/setup or prints URL | backend-engineer | S |
| #1650 | feat(daemon): implement PKCE OAuth browser-redirect login flow | backend-engineer | M |
| #1651 | feat(daemon): store Clerk API key in OS keychain (go-keyring) | backend-engineer | S |
| #1652 | feat(bff): add POST /v1/daemon/register endpoint — accept Clerk JWT, mint first API key | backend-engineer | S |

**Definition of done:**
- [ ] Daemon with no `daemon.json` opens `vaultmtg.app/setup` in system browser (or prints URL if headless)
- [ ] PKCE flow completes end-to-end: localhost callback → Clerk login → auth code → `POST /v1/daemon/register`
- [ ] BFF verifies Clerk JWT, mints API key; endpoint covered by integration tests
- [ ] Daemon writes API key to OS keychain (macOS Keychain / Windows Credential Manager) — NOT plaintext in `daemon.json`
- [ ] On subsequent starts, daemon reads key from keychain without re-opening browser
- [ ] Port conflict on localhost callback handled gracefully (retry, no crash)
- [ ] PostHog `daemon_paired` event fires after successful keychain write
- [ ] `go-keyring` added to `services/daemon/go.mod`
- [ ] OQ-5, OQ-6, OQ-7 resolved before this wave starts (port choice, key scoping, revocation on reinstall)

**Assigned agents**: backend-engineer (primary)
**Estimated effort**: L (1× M + 3× S)

---

### Wave 3 — SPA Setup Page + Download UX

| | |
|--|--|
| **Theme** | Web-based guided install experience |
| **Goal** | First-time users land on `/setup`, understand platform-specific warnings, download the right installer, and complete PKCE pairing from the browser |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1644 | feat(spa): /setup page — first-time install warnings with Gatekeeper + SmartScreen screenshots and bypass instructions | front-engineer | S |
| #1645 | feat(spa): /setup page — PKCE auth flow replaces SPA-mint-key pairing (ADR-020) | front-engineer | S |
| #1646 | feat(spa): DaemonDownload.tsx — replace broken install script links with .dmg and .exe download buttons | front-engineer | XS |
| #1647 | docs(daemon): update install README — describe .pkg/.dmg and NSIS .exe paths, mark shell scripts as power-user fallback | documentation | XS |

**Definition of done:**
- [ ] `/setup` page renders platform-appropriate Gatekeeper (macOS) and SmartScreen (Windows) bypass instructions with screenshots
- [ ] PKCE flow on SPA replaces old SPA-mint-key flow per ADR-020
- [ ] `DaemonDownload.tsx` shows `.dmg` and `.exe` download buttons linked to GitHub Releases; no broken shell script links
- [ ] Install README updated and accurate; shell scripts marked as power-user fallback
- [ ] Playwright E2E test covers the `/setup` page download button and PKCE redirect
- [ ] CI green on main after merge

**Assigned agents**: front-engineer (primary)
**Estimated effort**: M (2× S + 2× XS)

---

### Wave 4 — GA Readiness Documentation

| | |
|--|--|
| **Theme** | Document signing workflows so GA activation is a checklist, not a scramble |
| **Goal** | Apple Developer Program and Azure Trusted Signing are documented and ready to activate at GA — neither requires actual active signing in v0.3.1 |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1648 | chore(ga-prep): enroll in Apple Developer Program — document notarization workflow and notarytool credentials in SSM | infrastructure | S |
| #1649 | chore(ga-prep): onboard Azure Trusted Signing — document signing workflow in GoReleaser config, budget approval | infrastructure | S |

**Definition of done:**
- [ ] Apple Developer Program enrollment documented; `notarytool` credential path in SSM confirmed
- [ ] Azure Trusted Signing workflow documented in GoReleaser config comments; budget approval recorded
- [ ] Neither ticket requires notarization or signing to be active — documentation is the deliverable
- [ ] OQ-1 (Apple account confirmed) and OQ-2 (Azure budget approved) resolved before this wave closes

**Assigned agents**: infrastructure
**Estimated effort**: M (2× S)

---

### Wave 5 — Component Library Foundation

| | |
|--|--|
| **Theme** | Component Library Foundation — Storybook + Chromatic baseline |
| **Goal** | Storybook 8 installed and deployed to Chromatic; baseline snapshots approved; CI Chromatic check required |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1621 | spike(storybook): Storybook + Chromatic discovery spike — evaluate Vite builder compatibility and Chromatic pricing | front-engineer | M |
| #1622 | feat(storybook): install and configure Storybook 8 with Vite builder | front-engineer | M |
| #1625 | feat(chromatic): capture Chromatic baseline snapshots for existing components | front-engineer | M |

**Definition of done:**
- [ ] Chromatic baseline approved by Ray (no unresolved snapshot diffs)
- [ ] Storybook deployed to Chromatic; Chromatic project URL documented in repo
- [ ] CI passes with Chromatic check as a required status
- [ ] All three tickets in Done state on Project #33

**Assigned agents**: front-engineer
**Estimated effort**: L (3× M)

---

### Wave 6 — CI Hardening

| | |
|--|--|
| **Theme** | Supply-chain security before shipping installers to beta users |
| **Goal** | Pinned Actions, Node 22 LTS, govulncheck, npm audit — zero high/critical findings |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| TBD | Upgrade CI to Node.js 22 LTS across all jobs | infrastructure | S |
| TBD | Pin all GitHub Actions to SHA-pinned versions | infrastructure | XS |
| TBD | Add `govulncheck` to Go CI jobs | infrastructure | XS |
| TBD | Add `npm audit --audit-level=high` to frontend CI | infrastructure | XS |

**Definition of done:**
- [ ] All CI jobs green on Node.js 22 LTS
- [ ] All GitHub Actions pinned to SHA (no floating tags)
- [ ] `govulncheck` passes with zero high/critical findings
- [ ] `npm audit` passes with zero high-severity findings

**Assigned agents**: infrastructure
**Estimated effort**: M (1× S + 3× XS)

> PM action item: file Wave 6 tickets before Wave 5 closes.

---

### Wave 7 — Staging Validation

| | |
|--|--|
| **Theme** | Prove staging is clean before release tag |
| **Goal** | Staging deploy clean from scratch; smoke suite passes; BFF healthy |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| TBD | Staging deploy pipeline smoke test | infrastructure | S |
| TBD | BFF `/healthz` verified on staging | backend-engineer | XS |
| TBD | Playwright staging smoke suite runs clean | ui-tester | S |

**Definition of done:**
- [ ] Staging deploy completes clean from scratch (no prior state assumed)
- [ ] BFF `/healthz` returns 200 on staging
- [ ] Playwright staging smoke suite passes with zero failures

**Assigned agents**: infrastructure (primary), backend-engineer, ui-tester
**Estimated effort**: M (2× S + 1× XS)

> PM action item: file Wave 7 tickets before Wave 6 closes.

---

### Wave 8 — Release Gate (Smoke Test + Tag + Changelog)

| | |
|--|--|
| **Theme** | End-to-end validation, tag, and publish |
| **Goal** | Manual install-to-event flow verified on both platforms; tag cut; changelog published |

**Tickets**: None — ceremony wave.

**Definition of done (all must be true before PM issues GO):**
- [ ] CI is green on main (hard gate — no exceptions per BROADCAST Active Directive 2)
- [ ] Staging deploy pipeline runs from scratch; BFF `/healthz` returns 200
- [ ] Playwright staging smoke suite passes
- [ ] Manual install smoke test passed on macOS 14+ and Windows 11:
  - Download `.dmg`/`.exe` → install → PKCE login → daemon starts → first event appears in BFF (verified via DB query or PostHog)
- [ ] All Wave 1–7 tickets are in Done state on Project #33 board
- [ ] PostHog `daemon_paired` event confirmed firing from at least one real test session
- [ ] OQ-4 (Gatekeeper hard-block behavior) confirmed before this wave closes
- [ ] `CHANGELOG.md` entry written for v0.3.1
- [ ] v0.3.1 git tag cut; GitHub Release created with `.dmg` and `.exe` artifacts attached

**Assigned**: Ray (arch) + Najah (PM) + lead-engineer (LE co-sign required)
**Estimated effort**: S (ceremony)

---

## 4. Dependencies

Before engineering starts:

- [ ] **v0.3.0 tag is cut** ✓ — v0.3.0 closed 2026-05-09
- [ ] **Apple Developer Program accepted** ✓ — per BROADCAST; documented in #1648
- [ ] **Azure Artifact Signing account deployed** — identity validation pending Microsoft review (Wave 4 documents the workflow; active signing is GA scope)
- [ ] **Wave 0 gate** — Ray (architect) architectural implications review must be received and acknowledged by PM before any Wave 1 ticket moves to In Progress

No v0.4.0 work begins until PM issues GO after Wave 7.

---

## 5. Exit Gates

All of the following must be true before the v0.3.1 tag is cut:

- [ ] All 7 waves closed (all tickets in Done state on Board #33)
- [ ] CI green on main
- [ ] macOS DMG installs and runs without Gatekeeper hard-block (tested on macOS 14+)
- [ ] Windows installer (.exe) installs and runs without SmartScreen hard-block (tested on Windows 11)
- [ ] Daemon pairs successfully via PKCE flow on both platforms (at least one successful pair each)
- [ ] PostHog `daemon_paired` event confirmed firing from a real test session
- [ ] Chromatic baseline approved (Wave 5 — required for v0.3.1)
- [ ] Release checklist completed (RELEASE_CHECKLIST.md)
- [ ] `CHANGELOG.md` updated
- [ ] GitHub Release created with `.dmg` and `.exe` artifacts attached and checksums verified

---

## 6. PM Action Items

Open items that must be resolved — not stale, verified as of 2026-05-09:

- [ ] **File Wave 5 tickets** (CI Hardening) before Wave 4 closes — 4 tickets, infrastructure owner
- [ ] **File Wave 6 tickets** (Staging Validation) before Wave 5 closes — 3 tickets, infrastructure/backend/ui-tester owners
- [ ] **Confirm Azure identity validation approved** — Microsoft review in progress; escalate to Ray if no approval by Wave 4 start
- [ ] **Confirm Ray's architectural implications note received** before any Wave 1 ticket moves to In Progress (Wave 0 gate)
- [ ] **Resolve OQ-5, OQ-6, OQ-7** (port choice, key scoping, revocation on reinstall) with backend-engineer and Ray before Wave 2 starts
- [ ] **Confirm OQ-1 and OQ-2** (Apple account + Azure budget) with Ray before Wave 4 starts
- [ ] **Confirm OQ-4** (Gatekeeper bypass behavior on macOS 14) with Ray before Wave 7 closes — requires a clean macOS 14 VM test

---

## 7. Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|------------|
| R-1 | Gatekeeper hard-blocks unsigned .dmg on macOS 14+ | Medium | High | Confirm on clean macOS 14 VM in Wave 3; add Gatekeeper bypass instructions to `/setup` page regardless |
| R-2 | SmartScreen hard-blocks unsigned .exe on Windows 11 | Medium | High | Document SmartScreen bypass in Wave 3 SPA; confirm on clean Windows 11 VM; escalate to Ray if bypass is not available without EV signing |
| R-3 | PKCE localhost callback port conflict | Low | Medium | Handle gracefully in #1650: retry with ephemeral port, surface error, never crash |
| R-4 | `go-keyring` fails on a specific macOS/Windows version | Low | High | Spike keychain write/read in #1651 before committing; keep fallback path documented |
| R-5 | Azure identity validation not approved before Wave 4 closes | Medium | Low | Wave 4 documents workflow only — approval not required to close wave; escalate if not received before GA prep |
| R-6 | Wave 5–6 tickets not created before those waves start | High | Medium | PM action item: file Wave 5 before Wave 4 closes, Wave 6 before Wave 5 closes |
| R-7 | v0.4.0 engineers start work before v0.3.1 closes | Low | High | BROADCAST Active Directive 1 blocks this; PM must not issue GO until Wave 7 is fully green |

---

## 8. Out of Scope

Explicit list of what is NOT in v0.3.1 — save for v0.4.0 or later:

| Item | Where it goes |
|------|--------------|
| Apple notarization (active signing) | GA milestone |
| Azure Trusted Signing (active signing) | GA milestone |
| System tray / menubar icon | Post-GA |
| MSI installer (Windows enterprise) | Post-GA if requested |
| Linux packaging | Unsupported platform |
| Homebrew cask | Post-GA secondary channel |
| Automatic daemon auto-updater | Post-GA |
| Full component story library (beyond Wave 5 baseline) | v0.4.0 follow-on after Chromatic baseline is established |
| API key issuance/revoke UI (#1314) | Superseded by PKCE; full key management UI post-beta |
| Full component story library | v0.4.0 Wave 1 follow-on |
| Stripe integration / paid tiers | Post-beta GA |
| Any v0.4.0 features | Engineering does not start v0.4.0 until PM issues GO |
