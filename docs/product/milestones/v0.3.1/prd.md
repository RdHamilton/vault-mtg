# PRD: v0.3.1 — Packaging

**Status**: Draft  
**Written**: 2026-05-09  
**Author**: Najah (PM)  
**Project Board**: #33 — v0.3.1 Packaging (`PVT_kwHOABsZ684BXMn-`)  
**Milestone**: Ships before v0.4.0 Beta Launch

---

## 1. Overview

v0.3.1 is a focused packaging and onboarding release. Its sole purpose is to make the daemon installable by a non-technical user on macOS and Windows — and to wire up the first-run authentication flow so that a user who installs the daemon can pair it to their VaultMTG account without touching a terminal or editing a config file.

Without v0.3.1, v0.4.0's beta invite flow has nowhere to send users. A beta invite email that links to a shell script is a support ticket, not a product. v0.3.1 closes that gap.

The full desired flow (approved by Ray):

1. User receives beta invite → downloads `.dmg` or `.exe` from `vaultmtg.app/download`
2. Installer runs silently — no terminal, no admin prompt
3. Daemon starts, detects no API key, opens system browser to Clerk PKCE login
4. User signs in → browser redirects to localhost callback → daemon exchanges code
5. Daemon calls `POST /v1/daemon/register` → BFF mints API key → keychain write
6. Daemon begins tailing `Player.log` and posting events
7. User opens SPA → `/setup` page shows "Daemon connected"

Target: download → first event < 5 minutes for any user with an existing VaultMTG account.

---

## 2. Goals

- **G1** — Ship a `.dmg` installer (macOS) and a `.exe` installer (Windows) built and signed by CI from a single GoReleaser config
- **G2** — Deliver a zero-terminal PKCE auth flow: daemon opens browser → user logs in → API key written to OS keychain
- **G3** — Update the SPA download page and `/setup` page to match the new artifacts and flow
- **G4** — Harden the existing CI sign-macos job (two known bugs) so the pipeline doesn't hang or waste runner minutes
- **G5** — Document the GA-prep path for notarization and Azure Trusted Signing so those steps are ready to execute when budget is approved

**Success metric (primary)**: ≥80% of users who download the installer complete daemon pairing within 10 minutes, measured via PostHog `daemon_paired` event.

**Success metrics (secondary)**:
- Download-to-pair conversion ≥80% in first 30 days of beta
- Zero CS tickets attributable to "could not install" or "could not log in" in the first two weeks of beta
- PKCE flow completes in <60 seconds on both platforms in happy-path testing

---

## 3. Non-Goals (save for v0.4.0 or later)

| Item | Reason deferred |
|------|----------------|
| Apple notarization (active) | Requires paid Apple Developer Program enrollment — GA milestone |
| Azure Trusted Signing (active) | $9.99/mo — budget approval needed; GA milestone |
| GoReleaser Pro features | Open-source tier sufficient for beta |
| System tray / menubar icon | Not on critical path for beta |
| MSI installer (Windows enterprise) | Post-GA only if requested |
| Linux packaging | Not a supported platform |
| Homebrew cask | Secondary distribution channel, post-GA |
| Automatic daemon updater | Post-GA |
| Device authorization flow (headless) | Fallback for CI/server — not a beta user scenario |
| Storybook / Chromatic component library (#1621, #1622, #1625) | Labeled v0.4.0; on Project #33 board by mistake — execute in v0.4.0 Wave 1 |

---

## 4. Waves

### Wave 1 — CI Hardening (Fix Before Anything Ships)

**Theme**: The sign-macos CI job has two live bugs. Fix them first so the pipeline is stable for all subsequent release work.

**Tickets**:

| # | Title | Owner |
|---|-------|-------|
| #1658 | `fix(ci): add tag guard to sign-macos job on workflow_dispatch` | infrastructure |
| #1659 | `fix(ci): add timeout-minutes to sign-macos job to prevent notarization hang` | infrastructure |

**Definition of done**:
- `sign-macos` job skips cleanly when triggered via `workflow_dispatch` without a release tag
- `sign-macos` job has `timeout-minutes: 30` and fails with a clear error (not an indefinite hang)
- CI is green on main after both fixes merge

**Why first**: These bugs waste runner minutes and can silently hang the pipeline. Every subsequent wave that touches CI depends on a stable foundation.

---

### Wave 2 — GoReleaser Foundation (Build Pipeline)

**Theme**: Replace the ad-hoc matrix release workflow with a single GoReleaser config that produces both platform binaries from one CI run.

**Tickets**:

| # | Title | Owner |
|---|-------|-------|
| #1639 | `feat(daemon): add GoReleaser config — produce darwin universal binary + windows amd64 binary` | backend |
| #1642 | `feat(ci): replace daemon-release.yml matrix with GoReleaser-driven workflow` | infrastructure / backend |

**Definition of done**:
- `.goreleaser.yml` exists and produces a darwin universal binary and a windows amd64 binary on a tag push
- `daemon-release.yml` is replaced by a GoReleaser-driven workflow
- Artifacts (checksums, binaries) are uploaded to a GitHub Release on tag push
- CI green on main

**Why before Wave 3**: The macOS and Windows installers in Wave 3 wrap these binaries. GoReleaser must produce valid artifacts before packaging work can be validated end-to-end.

---

### Wave 3 — Native Installers (macOS + Windows)

**Theme**: Wrap the GoReleaser binaries in native, double-click installers — `.dmg` (macOS) and `.exe` (Windows) — that install the daemon without a terminal or admin prompt.

**Tickets**:

| # | Title | Owner |
|---|-------|-------|
| #1640 | `feat(daemon): macOS .pkg installer — port LaunchAgent logic from install.sh into pkgbuild/productbuild, wrap in .dmg` | backend |
| #1641 | `feat(daemon): Windows NSIS .exe installer — port Scheduled Task logic from install.ps1, no UAC` | backend |

**Definition of done**:
- `mtga-companion-daemon.dmg` mounts and double-click installs a per-user LaunchAgent without requesting admin password
- Daemon auto-starts after logout/login on macOS 13+
- `mtga-companion-daemon.exe` installs a Scheduled Task on Windows without UAC elevation
- Both installers produced automatically by the GoReleaser pipeline (Wave 2)
- Manual smoke test: install on macOS 13+ and Windows 11, verify daemon process starts

---

### Wave 4 — First-Run Auth (PKCE + Keychain + BFF Endpoint)

**Theme**: Wire up the zero-terminal authentication flow: daemon detects missing API key, opens browser, completes PKCE OAuth, stores key in OS keychain, and registers with the BFF.

**Tickets**:

| # | Title | Owner |
|---|-------|-------|
| #1643 | `feat(daemon): first-run config detection — missing daemon.json opens vaultmtg.app/setup or prints URL` | backend |
| #1650 | `feat(daemon): implement PKCE OAuth browser-redirect login flow` | backend |
| #1651 | `feat(daemon): store Clerk API key in OS keychain (go-keyring)` | backend |
| #1652 | `feat(bff): add POST /v1/daemon/register endpoint — accept Clerk JWT, mint first API key` | backend |

**Definition of done**:
- Daemon with no `daemon.json` opens `vaultmtg.app/setup` in system browser (or prints URL if headless)
- PKCE flow: daemon binds localhost callback → opens Clerk login → captures auth code → exchanges for session token → calls `POST /v1/daemon/register`
- BFF verifies Clerk JWT, mints API key, returns it; endpoint covered by integration tests
- Daemon writes API key to OS keychain (macOS Keychain / Windows Credential Manager) — NOT plaintext in `daemon.json`
- On subsequent starts, daemon reads key from keychain and does not re-open browser
- Port conflict on localhost callback is handled gracefully (retry with ephemeral port, no crash)
- `go-keyring` added to `services/daemon/go.mod`
- PostHog `daemon_paired` event fires after successful keychain write
- All tickets ship together (dependency-coupled per Active Directive 5)

**Dependencies**: ADR-020 (PKCE auth), ADR-009 (Clerk JWT verification), `clerk-sdk-go v2`

---

### Wave 5 — SPA Install UX

**Theme**: Update the SPA to match the new installer artifacts and PKCE flow — replace broken script links, surface platform-aware download buttons, and add first-time install guidance for Gatekeeper and SmartScreen.

**Tickets**:

| # | Title | Owner |
|---|-------|-------|
| #1644 | `feat(spa): /setup page — first-time install warnings with Gatekeeper + SmartScreen screenshots and bypass instructions` | frontend |
| #1645 | `feat(spa): /setup page — PKCE auth flow replaces SPA-mint-key pairing (ADR-020)` | frontend |
| #1646 | `feat(spa): DaemonDownload.tsx — replace broken install script links with .dmg and .exe download buttons` | frontend |

**Definition of done**:
- `DaemonDownload.tsx` detects user platform and shows the correct primary download button (`.dmg` for macOS, `.exe` for Windows)
- `/setup` route is publicly accessible (no auth required)
- `/setup` contains a "First-time install warnings" section with Gatekeeper bypass screenshots (macOS) and SmartScreen bypass instructions (Windows)
- `/setup` reflects ADR-020 flow: no longer shows SPA-minted API key copy step; shows "open your browser" state while daemon is pairing
- `/setup` shows "Daemon connected — last seen N seconds ago" after successful pairing
- Component tests written for `DaemonDownload.tsx` and `/setup` page states

---

### Wave 6 — GA-Prep Documentation (Signing Runway)

**Theme**: Document the notarization and Azure Trusted Signing paths so GA signing is ready to activate on budget approval — no engineering discovery work needed at GA time.

**Tickets**:

| # | Title | Owner |
|---|-------|-------|
| #1647 | `docs(daemon): update install README — describe .pkg/.dmg and NSIS .exe paths, mark shell scripts as power-user fallback` | documentation |
| #1648 | `chore(ga-prep): enroll in Apple Developer Program — document notarization workflow and notarytool credentials in SSM` | infrastructure |
| #1649 | `chore(ga-prep): onboard Azure Trusted Signing — document signing workflow in GoReleaser config, budget approval` | infrastructure |

**Definition of done**:
- Install README updated: `.pkg/.dmg` and `.exe` paths are primary; shell scripts documented as power-user fallback only
- Apple Developer Program enrollment steps documented; `notarytool` credential storage path in SSM documented (enrollment itself may be async — budget/account decision outside engineering scope)
- Azure Trusted Signing onboarding documented in GoReleaser config comments; budget approval item tracked
- No new runtime code in this wave — docs and config only

**Note**: Actual notarization and signing activation are deferred to GA. This wave produces the runbook so GA signing is a one-day task, not a sprint.

---

### Wave 7 — Release Gate (Smoke Test + Tag + Changelog)

**Theme**: Validate the full end-to-end install-to-event flow on both platforms, cut the v0.3.1 tag, and publish the changelog.

**Tickets**: No new code tickets — this is a ceremony wave.

**Definition of done** (all must be true before the tag is cut):
1. CI is green on main (hard gate — no exceptions per BROADCAST Active Directive 2)
2. Staging deploy pipeline runs from scratch; BFF `/healthz` returns 200
3. Playwright staging smoke suite passes
4. Manual install smoke test on macOS 13+ and Windows 11: download `.dmg`/`.exe` → install → PKCE login → daemon starts → first event appears in BFF (checked via DB or PostHog)
5. All Wave 1–6 tickets are in Done state on Project #33 board
6. PostHog `daemon_paired` event confirmed firing from at least one real test session
7. `CHANGELOG.md` entry written for v0.3.1
8. v0.3.1 git tag cut; GitHub Release created with `.dmg` and `.exe` artifacts attached

---

## 5. Release Criteria

The following must ALL be true before the v0.3.1 tag is cut:

| Gate | Verified by |
|------|-------------|
| CI green on main | PM (BROADCAST PC-2) |
| Wave 1–6 tickets all Done on board #33 | PM |
| Manual smoke test passes on macOS 13+ | Ray |
| Manual smoke test passes on Windows 11 | Ray |
| PostHog `daemon_paired` event fires in real test session | Ray |
| BFF `/healthz` 200 on staging | infrastructure |
| Playwright staging smoke suite green | infrastructure |
| GitHub Release created with `.dmg` + `.exe` artifacts | PM |
| `CHANGELOG.md` entry published | PM |

**Hard no-go conditions** (any one blocks the tag):
- CI red on main
- PKCE flow fails on either platform
- Keychain write fails (API key lands in plaintext config)
- BFF `/v1/daemon/register` endpoint returns 5xx under normal conditions

---

## 6. Dependencies

| Dependency | Status | Needed for |
|-----------|--------|-----------|
| ADR-020 (PKCE auth design) | Approved | Wave 4 — daemon auth flow |
| ADR-009 (Clerk auth / `clerk-sdk-go v2`) | Deployed | Wave 4 — BFF endpoint JWT verification |
| ADR-011 (daemon distribution strategy) | Approved | Wave 2–3 — installer requirements |
| GoReleaser open-source tier | Available | Wave 2 — build pipeline |
| `go-keyring` library | New dep, add to `go.mod` | Wave 4 — keychain storage |
| NSIS (Windows installer compiler) | Available via `chocolatey` on GH Actions | Wave 3 |
| `pkgbuild` / `productbuild` | Available on macOS GH Actions runner | Wave 3 |
| Apple Developer Program enrollment | Pending budget/account decision | Wave 6 (docs only); active signing = GA |
| Azure Trusted Signing subscription | Pending budget approval ($9.99/mo) | Wave 6 (docs only); active signing = GA |
| PostHog (free tier) | Active | Wave 4 — `daemon_paired` instrumentation |

---

## 7. Open Questions

| # | Question | Owner | Gate? |
|---|----------|-------|-------|
| OQ-1 | Apple Developer Program: has Ray created the account and confirmed payment method? Docs in Wave 6 require knowing the credential storage path. | Ray | Wave 6 |
| OQ-2 | Azure Trusted Signing budget: approved? Wave 6 ticket (#1649) includes budget approval as an AC — who signs off? | Ray | Wave 6 |
| OQ-3 | Storybook tickets (#1621, #1622, #1625) appear on Project #33 board but are labeled v0.4.0. Confirm: these stay on v0.4.0 board only and are NOT part of v0.3.1 scope. | PM | Before Wave 1 starts |
| OQ-4 | Gatekeeper bypass: on macOS 13+ with no notarization, does right-click → Open produce a one-click bypass, or a hard block? Must be confirmed on a clean macOS 13 VM before Wave 3 closes. | Ray (eng) | Wave 3 close |

---

## 8. Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|-----------|
| R1 | Gatekeeper hard-blocks un-notarized `.pkg` on macOS 14+ (changes since macOS 13 docs) | Medium | High | Test on a clean macOS 14 VM before Wave 3 closes; add right-click bypass screenshots to SPA if needed |
| R2 | NSIS Windows installer triggers SmartScreen reputation warning (new binary, no signing) | High | Medium | SPA `/setup` page bypass instructions (Wave 5) cover this; warn users proactively |
| R3 | PKCE localhost callback blocked by firewall / security software on Windows | Low | High | Add fallback: print setup URL to stdout if browser open fails; document in Wave 6 README |
| R4 | GoReleaser cross-compilation fails for darwin universal binary (CGO dependencies) | Medium | Medium | Spike: verify GoReleaser builds clean before Wave 3 starts; use pure-Go `go-keyring` to avoid CGO |
| R5 | CI macOS runner unavailable / slow during release | Low | Low | Wave 1 timeout fix (#1659) mitigates hang risk; tag guard (#1658) prevents wasted minutes |
