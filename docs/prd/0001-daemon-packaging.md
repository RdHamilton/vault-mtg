# PRD: Daemon Packaging — v0.3.1

## Problem Statement

VaultMTG's desktop daemon currently ships as raw unsigned binaries on GitHub
Releases, installed via shell scripts. This is acceptable for alpha-internal
use but is a hard barrier for the closed-beta audience: MTG Arena players are
not engineers, they cannot run `curl | bash`, and they do not know what a
terminal is. The product cannot move past closed beta without a
double-clickable installer on macOS and Windows, and the daemon cannot pair
with a user account without a browser-native login flow that non-technical
users can complete in under two minutes.

This PRD covers all packaging work for the v0.3.1 milestone: installer
production (GoReleaser + pkg/dmg + NSIS), CI pipeline, daemon first-run auth
(PKCE browser redirect per ADR-020), and OS keychain key storage.

---

## Target Users

**Primary**: MTG Arena players enrolling in the VaultMTG closed beta. These
users are comfortable installing apps from the internet (they already run
MTG Arena), but they are not comfortable with terminals, shell scripts, or
manually editing config files.

**Secondary**: Internal QA testers and developers validating the installer
pipeline before external rollout.

---

## Success Metrics

- **Primary**: ≥80% of users who download the installer complete daemon
  pairing (browser login → keychain write → first event posted to BFF)
  within 10 minutes, measured via PostHog `daemon_paired` event.
- **Secondary**:
  - Installer download-to-pair conversion ≥ 80% in first 30 days of beta
  - Zero support tickets attributable to "could not install" or "could not
    log in" in first two weeks of beta
  - PKCE flow completes in <60 seconds from browser open to keychain write
    in happy-path testing on macOS and Windows

---

## User Stories

### Story 1 — macOS Install

**As a** macOS MTG Arena player invited to the VaultMTG beta,
**I want to** download a single .dmg file and double-click an installer,
**so that** the daemon is running and paired to my account without touching a terminal.

**ACs:**
- [ ] Given I download `mtga-companion-daemon.dmg`, when I mount it and
  double-click the `.pkg`, then the installer runs without requesting admin
  password (per-user LaunchAgent install)
- [ ] Given the installer completes, when I log out and back in, then the
  daemon starts automatically via LaunchAgent
- [ ] Given I am on macOS 13+ without a notarized binary, when I right-click
  the `.pkg` and select Open, then Gatekeeper shows a "developer cannot be
  verified" dialog with a one-click bypass — not a hard block
- [ ] The SPA `/setup` page contains a "First-time install warnings" section
  with screenshots of the Gatekeeper dialog and step-by-step bypass instructions

### Story 2 — Windows Install

**As a** Windows MTG Arena player invited to the VaultMTG beta,
**I want to** download a single .exe installer and click Next/Next/Finish,
**so that** the daemon is running and paired to my account without opening
PowerShell or editing the registry.

**ACs:**
- [ ] Given I download `mtga-companion-daemon-setup.exe`, when I double-click
  it, then the NSIS installer runs without requesting UAC elevation
- [ ] Given the installer completes, when I log off and back in to Windows,
  then the daemon starts automatically via a Scheduled Task (RunLevel: LeastPrivilege)
- [ ] Given Windows SmartScreen shows a warning, when I click "More info →
  Run anyway", then the installer proceeds normally
- [ ] The SPA `/setup` page contains screenshots of the SmartScreen dialog
  and step-by-step "Run anyway" instructions

### Story 3 — PKCE Login Flow

**As a** user who has just installed the daemon for the first time,
**I want** the daemon to open my browser to a Clerk login page automatically,
**so that** I can log in with my existing VaultMTG account without copying
tokens or editing config files.

**ACs:**
- [ ] Given no `daemon.json` exists (or api_key is missing), when the daemon
  starts, then it binds a localhost callback server and opens the system
  browser to the Clerk PKCE authorization URL
- [ ] Given I complete login in the browser, when Clerk redirects to the
  localhost callback, then the daemon captures the auth code and the browser
  tab shows "Login complete — you can close this tab"
- [ ] Given the auth code is captured, when the daemon exchanges it with
  Clerk, then the daemon calls `POST /v1/daemon/register` and receives an
  API key in the response
- [ ] Given the API key is received, when the daemon writes it, then the key
  is stored in the OS keychain (macOS Keychain / Windows Credential Manager)
  and NOT written in plaintext to `daemon.json`
- [ ] Given the API key is stored, when the daemon starts subsequent times,
  then it reads the key from the keychain and does not re-open the browser
- [ ] If the chosen localhost callback port is in use, the daemon retries
  with a different ephemeral port (no crash)

### Story 4 — BFF Daemon Registration Endpoint

**As a** backend system,
**I want** `POST /v1/daemon/register` to accept a Clerk JWT and return a
per-machine API key,
**so that** the daemon can complete pairing without human intervention.

**ACs:**
- [ ] Given a valid Clerk JWT in `Authorization: Bearer`, when the endpoint
  is called, then it returns `{"api_key": "sk_..."}` with HTTP 201
- [ ] Given a missing or invalid JWT, when the endpoint is called, then it
  returns HTTP 401
- [ ] Given the same user calls the endpoint more than 5 times per minute,
  when the 6th request arrives, then it returns HTTP 429
- [ ] The endpoint is mounted inside the `ClerkAuthMiddleware`-wrapped router
  group and never accessible without a valid JWT

### Story 5 — GoReleaser Pipeline

**As a** developer releasing a new daemon version,
**I want** a single GoReleaser invocation (triggered by GitHub Action on tag
push) to produce the macOS `.dmg` and Windows `.exe` and publish them to
GitHub Releases,
**so that** the release process is one `git tag` command, not a manual
multi-step matrix.

**ACs:**
- [ ] `.goreleaser.yml` at `services/daemon/` produces: `darwin/arm64` +
  `darwin/amd64` universal binary wrapped in `.pkg` in `.dmg`; `windows/amd64`
  wrapped in NSIS `.exe`
- [ ] GoReleaser workflow replaces the hand-written matrix in
  `.github/workflows/daemon-release.yml`
- [ ] `GONOSUMDB` and `GOPRIVATE` env vars are preserved on every Go step
- [ ] Checksums file is published alongside the artifacts on the GitHub Release
- [ ] CI job completes in <15 minutes on the standard GitHub Actions runner

---

## Out of Scope (v0.3.1)

- Apple Developer Program enrollment and notarization (GA milestone)
- Azure Trusted Signing (GA milestone)
- GoReleaser Pro (open-source free tier for beta)
- System tray / menubar icon
- MSI installer (Windows enterprise — post-GA only if requested)
- Linux packaging (not a supported platform)
- Homebrew cask (secondary channel, post-GA)
- Device authorization flow fallback for headless environments
- Automatic daemon update / auto-updater

---

## Open Questions

1. **Ephemeral port range** — should the daemon try a fixed port (e.g., 51423)
   first for UX consistency, or go fully random? Fixed port simplifies
   "allow VaultMTG in firewall" instructions. Decision needed before ADR020-1.
2. **API key scoping** — is the key per-machine (tied to machine ID) or
   per-user-session? Per-machine is simpler but means reinstall requires
   re-pairing. Needs input from backend-engineer before ADR020-3.
3. **Revocation** — if a user reinstalls, should the old key be revoked
   automatically? The BFF needs to know whether to create a new key or
   return the existing one. Needs a call before ADR020-3.

---

## RICE Score

- **Reach**: All ~500 closed-beta invitees; every user needs to install the daemon
- **Impact**: 3 — without a working installer the product cannot ship beta
- **Confidence**: 95% — installer tooling is mature; PKCE is a well-understood pattern
- **Effort**: 6 person-weeks (GoReleaser + pkg + NSIS + PKCE flow + BFF endpoint + SPA updates)
- **Score**: 500 × 3 × 0.95 / 6 = **237.5** (highest-priority item in v0.3.1)

---

## Dependencies

- ADR-011 — daemon distribution strategy (source of installer requirements)
- ADR-020 — PKCE auth acquisition (supersedes ADR-011 first-run section)
- ADR-009 — Clerk auth (JWT verification on BFF, `clerk-sdk-go v2`)
- `go-keyring` — OS keychain library (new dependency, add to `go.mod`)
- GoReleaser open-source — build orchestrator (new CI dependency)
- `pkgbuild` / `productbuild` — macOS system tools (available on macOS GitHub Actions runner)
- NSIS — Windows installer compiler (available via `chocolatey install nsis` on Windows runner)

---

## Full Desired Flow (Ray's Description)

The end-to-end experience Ray has approved:

1. User receives beta invite email with download link to `vaultmtg.app/download`
2. SPA detects user platform and offers the correct artifact: `.dmg` (macOS)
   or `.exe` (Windows)
3. User downloads and double-clicks. Installer runs silently (no terminal,
   no admin prompt). Daemon is registered as LaunchAgent / Scheduled Task.
4. Daemon starts for the first time. It detects no API key. It binds a
   localhost callback on an ephemeral port.
5. Daemon opens the system browser to a Clerk-hosted login page.
6. User signs in with their VaultMTG account (they created it on the website
   when they signed up for the waitlist).
7. Clerk redirects to `localhost:PORT/callback?code=AUTH_CODE`. Browser shows
   "Login complete — you can close this tab."
8. Daemon exchanges the code for a Clerk session token via PKCE token endpoint.
9. Daemon calls `POST /v1/daemon/register` with the Clerk JWT. BFF mints an
   API key and returns it.
10. Daemon stores the API key in the OS keychain. Writes `daemon.json` with
    `cloud_api_url` and `keychain: true`.
11. Daemon begins tailing `Player.log` and POSTing events to the BFF.
12. User opens the SPA. The `/setup` page shows "Daemon connected — last seen
    N seconds ago."

Total time from download to first event: target <5 minutes for a user who
already has a VaultMTG account.
