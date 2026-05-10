# v0.3.1 Kickoff: "Packaging"

**Milestone**: v0.3.1 "Packaging"
**Date**: 2026-05-09
**Board**: Project #33 (`PVT_kwHOABsZ684BXMn-`)
**Author**: Najah (Product Manager)
**Status**: ACTIVE

---

## 1. Milestone Overview

### What v0.3.1 Is

v0.3.1 delivers a double-clickable, self-configuring daemon installer for macOS and Windows. Beta users are not engineers. They cannot run shell scripts in a terminal. They cannot copy-paste API keys. This milestone closes that gap before v0.4.0 begins.

The milestone ships one primary track:

**Daemon Packaging** — GoReleaser produces a darwin universal binary and windows amd64 binary. Platform-specific installers (.dmg → .pkg on macOS, .exe via NSIS on Windows) handle LaunchAgent and Scheduled Task setup automatically. A PKCE browser-redirect auth flow (ADR-020) eliminates manual key management. The SPA `/setup` page guides users through download, install, and first-run pairing. The macOS `.dmg` is signed + notarized + stapled via the active `sign-macos` CI pipeline (PR #1655); Wave 5 verifies this end-to-end and documents Azure signing for GA.

### Why It Exists

v0.3.0 closed with telemetry parity proven but the daemon still requiring terminal installation. That is a beta blocker. No non-engineer beta user will successfully install the daemon without a standard installer. v0.3.1 removes that blocker before v0.4.0 begins beta-readiness work.

The waitlist opens **August 1, 2026**. The public closed beta launches **August 18, 2026**. v0.3.1 must close well before v0.4.0 completes its own work.

### Success Metrics

- A macOS user downloads the `.dmg`, double-clicks, and the daemon is installed and running within 5 minutes with zero terminal interaction.
- A Windows user downloads the `.exe`, double-clicks, and the daemon is installed and running within 5 minutes with zero terminal interaction.
- Both users complete Clerk login via a browser window that opens automatically on first run.
- PostHog `daemon_paired` event fires within 10 minutes of download for ≥80% of beta invitees.
- Zero support tickets attributable to "could not install" in the first two weeks of closed beta.

---

## 2. Wave 0 Gate — CLOSED

**Status**: CLOSED — Wave 1 is unblocked.
**Review completed**: 2026-05-09
**Reviewer**: Ray (Architect)

The architect reviewed the full v0.3.1 scope and issued **APPROVED WITH CONDITIONS**. Engineering may begin Wave 1 immediately.

### Confirmed Decisions

The following decisions were made during Wave 0 and are binding for all implementation tickets. Engineers must not re-open these questions:

| Decision | Resolution |
|---|---|
| **Port 51423** — PKCE callback port | Fixed port `51423`; one retry on `51424`. Clerk redirect URIs `http://localhost:51423/callback` and `http://localhost:51424/callback` must be registered before #1650 starts. |
| **Keychain service name** | `com.mtga-companion.daemon`, account key `api-key`. ADR-020 updated to reflect this. |
| **API key scoping** | Per-user, one-key-per-account for beta (`UNIQUE on account_id`). Per-machine expansion is post-GA. |
| **Key revocation on reinstall** | Silently re-use existing API key on reinstall (no revoke, no re-pair). BFF returns 200 (not 201) if key exists. `--reset` flag deferred to post-beta. |
| **Quarantine fix in postinstall** | macOS `.pkg` postinstall script (#1640) must call `xattr -dr com.apple.quarantine "$INSTALL_DIR/mtga-companion-daemon"` — added to #1640 ACs. |
| **Wire format** | `POST /v1/daemon/register` request/response JSON contract documented in ADR-020 §Contracts. |

### Wave Sequencing Confirmed by Architect

The architect confirmed the following order is correct and internally consistent:

- Wave 1 (CI Hardening) must precede all other waves — `sign-macos` bugs can silently hang the release pipeline.
- Wave 2 (GoReleaser Foundation) must precede Wave 3 (installers).
- Wave 3 (installers) must precede Wave 4 (PKCE auth) — working installer is the delivery vehicle.
- Wave 4 (PKCE auth + BFF) must precede Wave 5 (SPA setup page).
- Wave 5 (SPA) can partially overlap with Wave 6 (Storybook) — Storybook spike #1621 has no Wave 5 dependency.
- Waves 7 (staging validation) and 8 (release gate) are terminal.

---

## 3. Wave Plan

> Wave numbering in this document follows the PRD (Waves 1–8). The PRD is the source of truth.

---

### Wave 1 — CI Hardening — CLOSED ✓

| | |
|---|---|
| **Theme** | Fix CI bugs before they corrupt the release pipeline |
| **Goal** | `sign-macos` job is guarded, non-hanging, and cannot silently block tag releases; MTGA_ENV is explicit across all CI jobs |
| **Status** | CLOSED — all tickets merged; DoD verified by architect via workflow_dispatch run 25607888642 |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1658 | fix(ci): add tag guard to sign-macos job on workflow_dispatch | infrastructure | XS |
| #1659 | fix(ci): add timeout-minutes to sign-macos job to prevent notarization hang | infrastructure | XS |
| #1667 | fix(ci): add RULE-INFRA-01 lint gate rollout process to infrastructure agent | infrastructure | XS |
| #1668 | fix(ci): ensure MTGA_ENV is explicitly set in all CI jobs that start the BFF | infrastructure | XS |

> **Merge notes**: #1658, #1659, #1668 merged via PR #1679. #1667 applied directly (RULE-INFRA-01 doc — harness-blocked; Ray authorized manual edit).

**Definition of done:**
- [x] `sign-macos` job only runs on tag pushes — `workflow_dispatch` cannot trigger it
- [x] `sign-macos` job has `timeout-minutes` set; CI marks the job failed (not hung) if notarization exceeds the limit
- [x] RULE-INFRA-01 lint gate rollout process documented in infrastructure agent
- [x] `MTGA_ENV` explicitly set in all CI jobs that start the BFF; no jobs rely on implicit defaults
- [x] CI green on main after all four merges

**Assigned agents**: infrastructure
**Estimated effort**: S (4× XS)

---

### Wave 2 — Binary Build + Installer Foundation — CLOSED ✓

| | |
|---|---|
| **Theme** | Produce platform installers via GoReleaser |
| **Goal** | A tag push produces a `.dmg` (macOS) and `.exe` (Windows) installer with zero manual build steps |
| **Status** | CLOSED — all tickets merged; all 6 DoD conditions verified by LE via execution (goreleaser exits 0, both binaries produced, CI green on main) |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1639 | feat(daemon): add GoReleaser config — produce darwin universal binary + windows amd64 binary | backend-engineer | M |
| #1642 | feat(ci): replace daemon-release.yml matrix with GoReleaser-driven workflow | infrastructure | S |
| #1640 | feat(daemon): macOS .pkg installer — port LaunchAgent logic from install.sh into pkgbuild/productbuild, wrap in .dmg | backend-engineer | M |
| #1641 | feat(daemon): Windows NSIS .exe installer — port Scheduled Task logic from install.ps1, no UAC | backend-engineer | M |

> **Merge notes**: #1639, #1640, #1641 merged via PR #1678. #1642 merged via PR #1682. GoReleaser snapshot fix (after: key + GORELEASER_IS_SNAPSHOT bug) merged via PR #1687.

**Definition of done:**
- [x] `goreleaser release --snapshot` produces darwin universal binary and windows amd64 binary without errors
- [x] macOS `.pkg` postinstall script calls `xattr -dr com.apple.quarantine` after binary copy (per Wave 0 Decision 4)
- [x] macOS `.pkg` installs per-user LaunchAgent; daemon starts after install without a terminal
- [x] Windows `.exe` installs Scheduled Task; daemon starts after install without UAC prompt
- [x] GoReleaser-driven CI workflow active; `v*` tag triggers release build
- [x] CI green on main after merge

**Assigned agents**: backend-engineer (primary), infrastructure
**Estimated effort**: L (3× M + 1× S)

---

### Wave 3 — PKCE Auth (dependency-coupled — all 5 tickets ship together) — UNBLOCKED ✓

| | |
|---|---|
| **Theme** | Zero-terminal daemon authentication |
| **Goal** | Daemon detects missing config, opens browser, completes PKCE, stores key in OS keychain, registers with BFF — no manual key copy-paste ever |

> **Coupling note**: These 5 tickets have hard sub-dependencies. #1643 (config detection) and #1651 (keychain storage) must be implemented before #1650 (PKCE flow) can be tested end-to-end. #1652 (BFF endpoint) must exist before #1650 can complete registration. #1674 (migration) must merge before or with #1652. All 5 must ship together — no partial merges. Wave 0 conditions C-1 through C-8 must be satisfied before this wave starts (see Section 4).
>
> **Status (2026-05-09)**: UNBLOCKED. C-1 through C-5 satisfied; C-3 confirmed done (Clerk Native API enabled, redirect URIs registered). C-6, C-7, C-8 are resolved during Wave 3 itself per the coupling design — they are not pre-blockers. Wave 3 may start.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1643 | feat(daemon): first-run config detection — missing daemon.json opens vaultmtg.app/setup or prints URL | backend-engineer | S |
| #1651 | feat(daemon): store Clerk API key in OS keychain (go-keyring) | backend-engineer | S |
| #1674 | feat(dba): create daemon_api_keys table and migration for daemon registration | backend-engineer | S |
| #1652 | feat(bff): add POST /v1/daemon/register endpoint — accept Clerk JWT, mint first API key | backend-engineer | S |
| #1650 | feat(daemon): implement PKCE OAuth browser-redirect login flow | backend-engineer | M |

**Definition of done:**
- [x] Daemon with no `daemon.json` opens `vaultmtg.app/setup` in system browser (or prints URL if headless)
- [x] PKCE flow completes end-to-end: localhost callback on port 51423 (retry 51424) → Clerk login → auth code → `POST /v1/daemon/register`
- [x] BFF verifies Clerk JWT, mints API key using `daemon_api_keys` table; rate-limited at 5 req/hour per `account_id` (in-memory, no Redis)
- [x] `POST /v1/daemon/register` request body requires three fields: `device_id` (UUID, unique per daemon installation), `platform` (string — e.g. `darwin`, `windows`), `daemon_ver` (semver string — e.g. `0.3.1`); missing any field returns 400
- [x] `daemon_api_keys` schema includes `device_id UUID NOT NULL`, `platform TEXT NOT NULL`, `daemon_ver TEXT NOT NULL`, and `UNIQUE(device_id)` in addition to the existing `UNIQUE(account_id)`
- [x] BFF returns 200 + existing key if account already has one (not 201)
- [x] Daemon writes API key to OS keychain using service `com.mtga-companion.daemon`, account `api-key`
- [x] On subsequent starts, daemon reads key from keychain without re-opening browser
- [x] Port conflict handled gracefully — retry 51424, surface error message, never crash
- [x] `GOOS=windows GOARCH=amd64 go build ./...` passes with zero CGO in dependency graph (per Wave 0 Decision 5)
- [x] PostHog `daemon_paired` event fires after successful keychain write
- [x] `go-keyring` added to `services/daemon/go.mod`
- [x] Integration tests cover BFF `/v1/daemon/register` — JWT verification, key minting, idempotent re-use, rate limit

**Assigned agents**: backend-engineer (primary)
**Estimated effort**: L (1× M + 4× S)

---

### Wave 4 — SPA Routing Fix + Infrastructure Cleanup

| | |
|---|---|
| **Theme** | Fix production-blocking routing bugs before any SPA feature work |
| **Goal** | SPA routes all API calls to the correct target (cloud BFF vs local daemon); nginx/CloudFront returns clean 404+CORS on unhandled routes |
| **Status** | CLOSED — all tickets merged; DoD verified by LE. Wave 5 is unblocked. |

> **Rationale**: Without #1695, every API call that should hit the local daemon is 404ing against the cloud BFF in production — the SPA is functionally broken for daemon-dependent features. #1696 is an infra fix that can run in parallel (#1695 and #1696 have no shared files). Both must close before any SPA feature wave (#1697–#1700, Wave 5) ships to production.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1695 | feat(frontend): split apiClient.ts into dual base URLs — `VITE_BFF_URL` (cloud) and `VITE_DAEMON_URL` (local daemon); update 10 API modules to route to correct target | front-engineer | M |
| #1696 | fix(infra): nginx/CloudFront returns 503+CORS on unhandled cloud BFF routes — return clean 404 with CORS headers | infrastructure | S |

**Definition of done:**
- [x] `apiClient.ts` split: all cloud-BFF calls use `VITE_BFF_URL`; all local daemon calls use `VITE_DAEMON_URL`; no daemon calls route to cloud BFF
- [x] All 10 affected API modules updated and type-checked (`npx tsc --noEmit` clean)
- [x] nginx/CloudFront config updated: unhandled routes return 404 (not 503) with correct CORS headers
- [x] Component tests updated for API module changes; Playwright E2E smoke passes
- [x] CI green on main after merge

**Assigned agents**: front-engineer (#1695), infrastructure (#1696) — parallel
**Estimated effort**: M (1× M + 1× S, parallel)

---

### Wave 5 — First-Run Empty States + Onboarding Analytics

| | |
|---|---|
| **Theme** | First-run user experience — show empty states when daemon is not connected; instrument the onboarding funnel |
| **Goal** | New users who land on Match History, Collection, or Decks without a connected daemon see a clear, actionable empty state rather than a broken or blank UI; funnel analytics fire so we can measure drop-off |

> **Rationale**: Wave 4 (#1695) must close first — these pages rely on correct daemon vs BFF routing to detect the no-daemon state. Once routing is correct, empty states are the highest-impact UX fix for first-run users before beta launch. Funnel events (#1700) ship in the same wave so PostHog data is available from day one of beta.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1697 | feat(frontend): implement empty state for Match History page (first-run, no daemon) | front-engineer | S |
| #1698 | feat(frontend): implement empty state for Collection page (first-run, no daemon) | front-engineer | S |
| #1699 | feat(frontend): implement empty state for Decks page (first-run, no daemon) | front-engineer | S |
| #1700 | feat(frontend): implement first-run onboarding funnel analytics events (funnel_daemon_installed, funnel_first_game_played) | front-engineer | S |

**Definition of done:**
- [x] Match History, Collection, and Decks pages each render a design-spec-compliant empty state when daemon is not connected (not a blank screen or error)
- [x] Empty states include a clear CTA pointing users to `/setup` or daemon download
- [x] `funnel_daemon_installed` and `funnel_first_game_played` PostHog events fire at the correct points in the first-run flow
- [x] Component tests added for all three empty state components
- [x] Playwright E2E test covers the no-daemon empty state flow end-to-end
- [x] CI green on main after merge

**Assigned agents**: front-engineer
**Estimated effort**: L (4× S)

---

### Wave 6 — SPA Setup Page + Download UX

| | |
|---|---|
| **Theme** | Web-based guided install experience |
| **Goal** | First-time users land on `/setup`, understand platform-specific warnings, download the right installer, and complete PKCE pairing from the browser |
| **Status** | CLOSED — all tickets merged; DoD verified by LE. PRs: #1775, #1776, #1777, #1778, #1779, #1780. |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1644 | feat(spa): /setup page — first-time install warnings with Gatekeeper + SmartScreen screenshots and bypass instructions | front-engineer | S |
| #1645 | feat(spa): /setup page — PKCE auth flow replaces SPA-mint-key pairing (ADR-020) | front-engineer | S |
| #1646 | feat(spa): DaemonDownload.tsx — replace broken install script links with .dmg and .exe download buttons | front-engineer | XS |
| #1647 | docs(daemon): update install README — describe .pkg/.dmg and NSIS .exe paths, mark shell scripts as power-user fallback | front-engineer | XS |

**Definition of done:**
- [x] `/setup` page renders platform-appropriate Gatekeeper (macOS) and SmartScreen (Windows) bypass instructions with screenshots
- [x] PKCE flow on SPA replaces old SPA-mint-key flow per ADR-020
- [x] `DaemonDownload.tsx` shows `.dmg` and `.exe` download buttons linked to GitHub Releases; no broken shell-script links
- [x] Install README updated and accurate; shell scripts marked as power-user fallback
- [x] Playwright E2E test covers the `/setup` page download button and PKCE redirect
- [x] CI green on main after merge

**Assigned agents**: front-engineer (primary)
**Estimated effort**: M (2× S + 2× XS)

---

### Wave 7 — GA Readiness Documentation

| | |
|---|---|
| **Theme** | Verify active Apple signing + notarization end-to-end; document Azure signing workflow for GA |
| **Goal** | Confirm the `sign-macos` pipeline (codesign + notarytool + stapler) produces a Gatekeeper-passing `.dmg`; document Azure Trusted Signing so GA activation is a checklist, not a scramble |

> **Context**: The Apple signing pipeline was implemented and merged in PR #1655. It runs on every `daemon/v*` tag. These tickets are verification + documentation — not activation.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1648 | chore(ga-prep): verify Apple notarization end-to-end on a release tag — confirm `notarytool` credentials in SSM, stapled .dmg passes Gatekeeper on clean macOS VM | infrastructure | S |
| #1649 | chore(ga-prep): onboard Azure Trusted Signing — document signing workflow in GoReleaser config, budget approval | infrastructure | S |

**Definition of done:**
- [ ] Apple signing verified end-to-end: release tag triggers `sign-macos`; `.dmg` is notarized + stapled; Gatekeeper clears automatically on a clean macOS 14+ VM
- [ ] `notarytool` credential path in SSM confirmed and documented
- [ ] Azure Trusted Signing workflow documented in GoReleaser config comments; budget approval recorded
- [ ] Azure identity validation status confirmed with Ray before this wave closes
- [ ] Azure active signing is NOT required to close this wave — documentation is the Azure deliverable

**Assigned agents**: infrastructure
**Estimated effort**: M (2× S)

---

### Wave 8 — Component Library Foundation

| | |
|---|---|
| **Theme** | Storybook + Chromatic baseline — pre-beta quality gate |
| **Goal** | Storybook 8 installed and deployed to Chromatic; baseline snapshots approved; Chromatic check required in CI |

> Wave 8 can partially overlap with Wave 7 — the Storybook spike (#1621) has no dependency on Wave 7 tickets and may start as soon as Wave 3 closes.

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1621 | Discovery spike: Storybook + Chromatic setup on React 19 + Vite | front-engineer | M |
| #1622 | feat(frontend): install and configure Storybook 8 with Vite builder | front-engineer | M |
| #1625 | feat(frontend): capture Chromatic baseline snapshots and approve initial set | front-engineer | M |

**Definition of done:**
- [ ] Storybook 8 running locally with Vite builder against the existing component library
- [ ] Storybook deployed to Chromatic; Chromatic project URL documented in repo
- [ ] Chromatic baseline snapshots captured and approved by Ray (zero unresolved diffs)
- [ ] Chromatic check added as a required CI status; CI green on main
- [ ] All three tickets in Done state on Project #33

**Assigned agents**: front-engineer
**Estimated effort**: L (3× M)

---

### Wave 9 — Staging Validation

| | |
|---|---|
| **Theme** | Prove staging is clean before the release tag is cut |
| **Goal** | Staging deploy clean from scratch; BFF healthy; full platform smoke tests pass |

| Ticket | Title | Owner | Effort |
|--------|-------|-------|--------|
| #1669 | test(staging): run full daemon install smoke test on macOS 14+ VM | infrastructure | S |
| #1670 | test(staging): run full daemon install smoke test on Windows with SmartScreen | infrastructure | S |
| #1671 | test(staging): end-to-end PKCE daemon pairing flow on both platforms | infrastructure | S |

**Definition of done:**
- [ ] Staging deploy completes clean from scratch (no prior state assumed)
- [ ] BFF `/healthz` returns 200 on staging
- [ ] Full daemon install smoke test passes on macOS 14+ VM (download → install → daemon running)
- [ ] Full daemon install smoke test passes on Windows 11 with SmartScreen bypass documented
- [ ] PKCE pairing flow completes end-to-end on both platforms in staging environment
- [ ] Playwright staging smoke suite passes with zero failures

**Assigned agents**: infrastructure (primary), backend-engineer, ui-tester
**Estimated effort**: M (3× S)

---

### Wave 10 — Release Gate (ceremony)

| | |
|---|---|
| **Theme** | Final sign-off, tag, and publish |
| **Goal** | All exit gates verified; v0.3.1 tag cut; GitHub Release created |

**Tickets**: None — ceremony wave. PM files Wave 10 tracking ticket before Wave 8 closes.

**Definition of done (all must be true before PM issues GO):**
- [ ] All Waves 1–9 are closed; all tickets in Done state on Project #33
- [ ] CI green on main (hard gate — no exceptions)
- [ ] Staging `/healthz` returns 200 after deploy
- [ ] macOS DMG is signed + notarized + stapled; installs and clears Gatekeeper automatically on macOS 14+ (no bypass required)
- [ ] Windows installer runs without SmartScreen hard-block on Windows 11
- [ ] PKCE daemon pairing flow completes end-to-end on both platforms
- [ ] PostHog `daemon_paired` event confirmed firing from at least one real test session
- [ ] Chromatic baseline approved
- [ ] `CHANGELOG.md` entry written for v0.3.1
- [ ] v0.3.1 git tag cut; GitHub Release created with `.dmg` and `.exe` artifacts attached and checksums verified

**Assigned**: Ray (arch) + Najah (PM) + lead-engineer (co-sign required)
**Estimated effort**: S (ceremony)

---

## 4. Architecture Conditions (Wave 0 → Wave 3+ Gates)

These conditions were identified in the arch review. Engineering **MAY NOT begin Wave 3** until all C-1 through C-8 are satisfied. Wave 7 also has a specific condition (Azure identity validation).

| # | Condition | Owner | Deadline | Status |
|---|---|---|---|---|
| C-1 | Keychain naming convention resolved — service: `com.mtga-companion.daemon`, account: `api-key`. ADR-020 updated. | Architect (signed off) | Before #1651 In Progress | ✓ Resolved (Wave 0) |
| C-2 | PKCE callback port confirmed as `51423` (fallback `51424`). ADR-020 step 3 updated. | Resolved (Wave 0) | Before #1650 In Progress | ✓ Resolved (Wave 0) |
| C-3 | Clerk OAuth application configured with `http://localhost:51423/callback` and `http://localhost:51424/callback` | PM action (register URIs) + backend-engineer | Before #1650 In Progress | ✓ DONE 2026-05-09 — Clerk Native API enabled; both redirect URIs registered in Native Applications → Allowlist |
| C-4 | API key scoping confirmed: per-user, one-key-per-account, `UNIQUE on account_id`. | Resolved (Wave 0) | Before #1652 In Progress | ✓ Resolved (Wave 0) |
| C-5 | Key revocation behavior confirmed: re-use existing key on reinstall; BFF returns 200 (not 201). | Resolved (Wave 0) | Before #1652 In Progress | ✓ Resolved (Wave 0) |
| C-6 | `daemon_api_keys` migration ticket #1674 merged to main before or with #1652. | DBA / backend-engineer | Before #1652 In Progress | OPEN — resolved during Wave 3 |
| C-7 | `POST /v1/daemon/register` request/response JSON contract documented in ADR-020. | Architect + backend-engineer | Before Wave 3 starts | OPEN — resolved during Wave 3 |
| C-8 | `daemon.json` canonical schema documented in ADR-020 (with migration path from legacy plaintext `api_key`). | Backend-engineer | Before #1643 In Progress | OPEN — resolved during Wave 3 |

**Wave 5 condition** (non-blocking for Wave 3 start, must resolve before Wave 5 closes):
- Azure identity validation approval confirmed. If not received, escalate to Ray.

---

## 5. Exit Gates

All of the following must be true before the v0.3.1 tag is cut:

- **EG-1** — All Waves 1–7 closed; all tickets in Done state on Project #33
- **EG-2** — CI green on main (hard gate — no exceptions; BROADCAST Active Directive 2 applies)
- **EG-3** — macOS DMG is signed + notarized + stapled; installs and clears Gatekeeper automatically on macOS 14+ (no bypass required)
- **EG-4** — Windows installer runs without SmartScreen hard-block on Windows 11
- **EG-5** — PKCE daemon pairing flow works end-to-end on both platforms
- **EG-6** — Chromatic baseline approved (zero unresolved snapshot diffs)
- **EG-7** — Release checklist completed (`CHANGELOG.md` updated; GitHub Release created with `.dmg` and `.exe` artifacts; checksums verified)
- **EG-8** — Staging `/healthz` returns 200 after clean deploy

---

## 6. PM Action Items

Open items that must be resolved — updated 2026-05-09:

- [x] **Register Clerk OAuth redirect URIs** — DONE 2026-05-09. `http://localhost:51423/callback` and `http://localhost:51424/callback` registered in Clerk Native Applications → Allowlist. C-3 satisfied.
- [x] **Confirm Azure identity validation approved** — DONE 2026-05-10. Identity validation approved by Microsoft. Wave 7 #1649 unblocked.
- [ ] **File Wave 8 release gate tickets** before Wave 6 closes — Wave 8 is a ceremony wave; PM files a tracking ticket so the release gate has a board artifact.

**Pending cleanup**: PR #1686 (changelogs + gitignore + go.work.sum) — open, pending merge.

---

## 7. Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|------------|
| R-1 | Notarization credential misconfiguration silently fails the `sign-macos` job — notarized .dmg not produced | Low | High | Wave 5 (#1648) does an end-to-end verification on a real tag; `notarytool` credentials confirmed in SSM before Wave 5 closes |
| R-2 | SmartScreen hard-blocks unsigned .exe on Windows 11 — no bypass path | Medium | High | Document SmartScreen bypass in Wave 4 SPA; confirm on clean Windows 11 VM in Wave 7; escalate to Ray if block is unbypassable without EV signing (Azure Trusted Signing deferred to GA) |
| R-3 | PKCE localhost callback port conflict (51423 in use) | Low | Medium | #1650 must retry on 51424, then surface a clear error message — never crash; both ports registered in Clerk |
| R-4 | `go-keyring` OS keychain integration fails on a specific macOS/Windows version | Low | High | Spike keychain write/read in #1651 before committing; CGO-free cross-compile validated in CI (C-5 condition) |
| R-5 | Azure identity validation not approved before Wave 5 closes | Medium | Low | Wave 5 documents the workflow only — active signing not required to close the wave; escalate if unresolved before GA prep |
| R-6 | Clerk does not permit exact `http://localhost:51423/callback` as redirect URI on current plan tier | Low | High | Fallback: custom URL scheme (`mtgacompanion://callback`) — requires new ticket; scope to separate spike if Clerk rejects exact-match localhost URI |
| R-7 | `daemon_api_keys` migration (#1674) not merged before #1652 starts — Wave 3 integration failure | Medium | High | C-6 condition blocks #1652 from starting; PM must confirm migration is merged before unblocking backend-engineer on #1652 |
| R-8 | v0.4.0 engineers start work before v0.3.1 closes | Low | High | BROADCAST Active Directive 1 blocks this; PM must not issue GO until Wave 7 is fully green and all exit gates pass |
| R-A (arch) | Clerk wildcard redirect URI not supported — fixed port is the mitigation | Low | High | Fixed port 51423 chosen; PM must register URIs before Wave 3 starts |
| R-B (arch) | `go-keyring` requires macOS Keychain entitlement for notarized builds | Medium | Medium | Entitlement must be confirmed in #1648 (Wave 5) — signing is active, so this is a Wave 5 blocker, not GA scope |
| R-C (arch) | `go-keyring` CGO dependency fails on Windows cross-compile | Low | High | CGO-free validation added to #1651 ACs as a CI gate |
| R-D (arch) | Missing `daemon_api_keys` migration causes Wave 3 integration failure | Medium | High | Tracked as #1674; C-6 condition blocks #1652 until migration is merged |
| R-E (arch) | Gatekeeper quarantine on binary (not just installer) | Medium | High | `xattr -dr com.apple.quarantine` added to #1640 postinstall ACs (Wave 0 Decision 4) |

---

## 8. Out of Scope

Explicit list of what is NOT in v0.3.1:

| Item | Where it goes |
|------|--------------|
| Azure Trusted Signing (active signing) | GA milestone — Wave 5 documents the workflow only; identity validation + budget approval pending |
| Full Storybook component story library (beyond Wave 6 baseline) | v0.4.0 follow-on after Chromatic baseline is established |
| Security agent / supply-chain scanning beyond npm audit + govulncheck | Post-GA |
| Windows MSI installer (enterprise) | Post-GA only if requested — beta uses NSIS .exe |
| System tray / menubar icon | Post-GA |
| Linux packaging | Unsupported platform |
| Homebrew cask | Post-GA secondary distribution channel |
| Automatic daemon auto-updater | Post-GA |
| Device authorization flow (headless) | Fallback for CI/server — not a beta user scenario |
| API key issuance/revoke UI (#1314) | Superseded by PKCE — daemon handles key acquisition automatically; SPA key management UI deferred post-beta |
| GoReleaser Pro features | Open-source tier sufficient for beta |
| Any v0.4.0 feature work | Engineering does not start v0.4.0 until PM issues GO after Wave 7 |

---

## 9. Dependency Summary

```
v0.3.0 closed (2026-05-09) ✓
  │
  └─▶ Wave 1 — CI Hardening (#1658, #1659, #1667, #1668)
        │
        └─▶ Wave 2 — Binary Build + Installer Foundation (#1639, #1642, #1640, #1641)
              │
              ├─▶ Wave 3 — PKCE Auth [all 5 ship together] (#1643, #1651, #1674, #1652, #1650)
              │     │  [requires C-1 through C-8 satisfied]
              │     │
              │     └─▶ Wave 4 — SPA Setup Page + Download UX (#1644, #1645, #1646, #1647)
              │           │
              │           ├─▶ Wave 5 — GA Readiness Docs (#1648, #1649) [partially parallel]
              │           │
              │           └─▶ Wave 6 — Component Library (#1621, #1622, #1625)
              │                 [#1621 spike can start when Wave 3 closes, not Wave 4]
              │
              └─▶ Wave 7 — Staging Validation (#1669, #1670, #1671)
                    │
                    └─▶ Wave 8 — Release Gate (ceremony)
                          │
                          └─▶ v0.3.1 tag ✓
```

No v0.4.0 work begins until PM issues GO after Wave 8.
