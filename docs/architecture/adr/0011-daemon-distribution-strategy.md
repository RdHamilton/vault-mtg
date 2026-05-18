# ADR-011: Daemon Native Installer Distribution Strategy

**Date**: 2026-05-06
**Status**: Accepted
**Deciders**: Ray Hamilton, Architect Agent
**Related**: ADR-003 (sync service deployment), ADR-008 (frontend serving model), ADR-009 (Clerk auth), ADR-010 (draft overlay architecture)

---

## Context

The VaultMTG desktop daemon is the data-collection half of the product. It
tails MTG Arena's `Player.log` via `fsnotify` and POSTs draft and match
events to the BFF. As of v0.3.x the daemon ships as **raw, unpackaged
binaries** uploaded to GitHub Releases by `.github/workflows/daemon-release.yml`:

- `vaultmtg-daemon-windows-amd64.exe`
- `vaultmtg-daemon-darwin-arm64`
- `vaultmtg-daemon-darwin-amd64`

The runtime install logic — copy binary into place, write a launchd plist
on macOS or register a Task Scheduler task on Windows, prompt the user for
a BFF URL and an auth token — already exists as two scripts:

- `services/daemon/install/macos/install.sh`
- `services/daemon/install/windows/install.ps1`

These scripts work, but they are not how a non-technical user installs
software. The SPA's `DaemonDownload.tsx` component links to "install
script" artifacts that do not yet exist on the Releases page in any
end-user form. The product cannot move past closed beta without a
double-clickable installer on each supported platform.

Three concerns drove the decision:

1. **End-user friendliness.** The target audience is MTG Arena players,
   not engineers. A `curl | bash` instruction is acceptable for power users
   in alpha; it is a churn-driver for a paid beta.
2. **Solo-developer release overhead.** Every platform-specific
   installer, signing certificate, and notarization step adds operational
   cost. The strategy must be cheap to maintain at one engineer.
3. **Cost-staged rollout.** Code-signing certificates are not free. The
   beta must ship without paying for trust infrastructure that GA
   requires; signing dollars activate at the GA milestone.

This ADR commits to a single distribution strategy across both supported
platforms (macOS, Windows), the build orchestration that produces the
installers, and the cost timeline for trust infrastructure.

---

## Decision

**GoReleaser orchestrates the daemon build and packages a `.pkg`-in-`.dmg`
installer for macOS and an NSIS `.exe` installer for Windows. The beta
ships unsigned with documented OS warnings; GA adds Apple Developer ID
notarization and Azure Trusted Signing for Windows. The daemon stays
headless — status surfaces only through the SPA `/setup` page.**

### Specifics

1. **Build orchestrator: GoReleaser.**
   - GoReleaser replaces the hand-written matrix in
     `.github/workflows/daemon-release.yml`. The matrix's three explicit
     `goos`/`goarch` jobs collapse into a single `.goreleaser.yml` config
     consumed by the `goreleaser/goreleaser-action@v6` GitHub Action.
   - GoReleaser handles cross-compilation, archive packaging, checksum
     generation, and GitHub Release publication. The existing
     `GONOSUMDB` / `GOPRIVATE` env vars are preserved on every Go step
     per project policy.
   - Beta uses **GoReleaser open-source** (free). GA upgrades to
     **GoReleaser Pro ($79/yr)** to unlock `.pkg` and notarization
     hooks that are Pro-only features.

2. **macOS distribution: universal `.pkg` inside a `.dmg`.**
   - GoReleaser builds `darwin/arm64` and `darwin/amd64` binaries and
     `lipo`-merges them into a single **universal binary**. One download,
     one binary, both Apple Silicon and Intel.
   - The universal binary is wrapped in an Apple Installer Package
     (`.pkg`) generated via `pkgbuild` + `productbuild`. The `.pkg` is
     then enclosed in a `.dmg` so the user sees the conventional macOS
     download UX (mount the disk image, double-click the installer).
   - The `.pkg` **postinstall** script ports the existing logic from
     `services/daemon/install/macos/install.sh`: write the LaunchAgent
     plist to `~/Library/LaunchAgents/com.mtga-companion.daemon.plist`,
     `launchctl load -w` it. **Per-user, not system-wide** — no
     LaunchDaemon, no `sudo`-only install path.
   - The artifact is **NOT** a drag-to-Applications `.app` bundle. The
     daemon is headless background software; the conventional UX for
     headless tools on macOS is `.pkg` + LaunchAgent, not an `.app`.

3. **Windows distribution: NSIS `.exe` installer.**
   - GoReleaser builds `windows/amd64` and packages it into an **NSIS**
     installer (`.exe`). NSIS is preferred over MSI for v0.4.0 because
     it requires no WiX toolchain, runs without admin elevation when the
     installer is per-user, and is the standard for indie Windows
     installers.
   - The NSIS script ports the existing logic from
     `services/daemon/install/windows/install.ps1`: copy the binary into
     `%LOCALAPPDATA%\VaultMTG\`, write `daemon.json` to
     `%APPDATA%\vaultmtg\`, register a **Scheduled Task at logon**
     under the current user with `RunLevel: LeastPrivilege` so no UAC
     prompt fires.
   - The artifact is **NOT** a Windows Service. Services require admin
     elevation to install and run as `LocalSystem` or `NetworkService`,
     which has no access to the user's `Player.log`. A user-level
     Scheduled Task at logon matches the daemon's actual operating
     model.
   - The artifact is **NOT** an MSI for v0.4.0. MSI is reconsidered only
     if enterprise/managed-deployment customers ask for it post-GA.

4. **Beta ships unsigned. Code signing is a GA milestone.**
   - The beta `.dmg` and `.exe` are uploaded **unsigned**. macOS
     Gatekeeper will warn ("cannot verify the developer"); Windows
     SmartScreen will warn ("Windows protected your PC"). Both warnings
     can be bypassed by the user with a documented two-click flow.
   - The SPA `/setup` page **must** include a "First-time install
     warnings" section that screenshots the Gatekeeper and SmartScreen
     dialogs and walks the user through right-click-Open (macOS) and
     "More info → Run anyway" (Windows). This is a release-blocker for
     the v0.4.0 SPA work.
   - Defer signing to GA on the explicit reasoning that closed-beta
     users tolerate one-time OS warnings; a paid public-GA audience does
     not.

5. **GA code signing: Apple Developer ID + Azure Trusted Signing.**
   - **macOS**: enroll in the **Apple Developer Program** ($99/yr).
     Sign the `.pkg` with the resulting Developer ID Installer
     certificate, then submit to Apple's notary service via
     `notarytool`. GoReleaser Pro automates the sign + notarize + staple
     pipeline. End result: zero Gatekeeper warning on first launch.
   - **Windows**: use **Azure Trusted Signing** ($9.99/mo ≈ $120/yr).
     Microsoft eliminated the EV-certificate SmartScreen advantage in
     2024 — non-EV certificates now achieve equivalent SmartScreen
     reputation. Azure Trusted Signing is the cheapest compliant path
     and integrates cleanly with GitHub Actions via Microsoft's official
     signing action.
   - **Explicitly NOT** an EV certificate. EV certs cost $300–$600/yr
     plus hardware-token shipping cost and provide no remaining
     SmartScreen benefit for a solo publisher.

6. **Daemon stays headless in v0.4.0.**
   - The daemon does not ship a system tray icon (Windows) or menubar
     icon (macOS) in v0.4.0. No native UI surface, no Cocoa, no Win32.
   - All daemon status (running / not running, last event, last error,
     paired account) is surfaced **exclusively through the SPA `/setup`
     page**, which polls a local daemon health endpoint.
   - This keeps the daemon binary small, dependency-free, and trivial
     to cross-compile. A native tray UI is reconsidered only if user
     research after GA identifies "I didn't know the daemon was running"
     as a top churn signal.

7. **First-run config: zero installer prompts.**
   - The installers do **NOT** prompt the user for `cloud_api_url` or
     `api_key`. The existing `install.sh` and `install.ps1` shell
     prompts are removed during the port to `.pkg` / NSIS.
   - On first launch the daemon detects a missing `daemon.json`, writes
     a stub config, and immediately directs the user to
     `https://vaultmtg.app/setup`. The setup flow on the SPA mints a
     Clerk API key (per ADR-009) and writes the config to disk via the
     daemon's local health endpoint.
   - This decouples installer success from auth setup. The user can
     install the daemon and pair their account in two separate
     sessions, and the installer never has to handle a
     password/token/secret.

### What this changes

- **Adds** `.goreleaser.yml` at the daemon module root.
- **Adds** a macOS `.pkg` postinstall script (ported from
  `services/daemon/install/macos/install.sh`).
- **Adds** an NSIS `.nsi` script (ported from
  `services/daemon/install/windows/install.ps1`).
- **Adds** a `/setup` "First-time install warnings" content block to the
  SPA before v0.4.0 ships.
- **Adds** a daemon first-run check that detects missing config and
  directs to `vaultmtg.app/setup`.
- **Removes** the hand-written matrix from
  `.github/workflows/daemon-release.yml`; replaces it with a
  GoReleaser-driven workflow.
- **Removes** end-user prompts from the install scripts (the scripts
  themselves remain as a power-user fallback).
- **Defers** Apple Developer Program enrollment and Azure Trusted
  Signing onboarding until the GA milestone.
- **Does not** change the daemon's runtime code, the BFF ingest API, or
  the LaunchAgent / Scheduled Task semantics. Auto-start behavior is
  identical to the current scripts.

---

## Consequences

### Positive

- **End-user-friendly install.** Double-click `.dmg` → double-click
  `.pkg` → done on macOS. Double-click `.exe` → Next/Next/Finish on
  Windows. No Terminal, no PowerShell, no `curl | bash`.
- **Cheap beta.** Beta ships at $0/year in trust infrastructure. The
  Gatekeeper and SmartScreen warnings are documented and survivable for
  a closed-beta audience.
- **Cost-staged rollout.** $219/year activates at GA (Apple $99 + Azure
  $120). $298/year if GoReleaser Pro is added at GA for `.pkg` and
  notarization automation. Below the AWS Activate credit envelope and
  trivial relative to a paid-tier MRR target.
- **Universal macOS binary.** One artifact for both Apple Silicon and
  Intel. The current matrix produces two darwin binaries and a
  user-confusion vector ("which one is mine?"); the universal binary
  eliminates that.
- **Reuse of proven runtime logic.** The launchd and Task Scheduler
  configurations have already been validated on real user machines via
  the existing scripts. The port to `.pkg` postinstall and NSIS is a
  packaging change, not a behavior change.
- **No daemon UI work.** Skipping the tray/menubar UI in v0.4.0 keeps
  the daemon module small and avoids a per-platform native-UI
  dependency. The SPA absorbs the status surface.
- **Decoupled install vs pair flow.** The installer only puts files in
  the right place; pairing happens in the SPA where the user is already
  signed in to Clerk. Failure modes during pairing do not roll back the
  install.

### Negative

- **Beta install UX includes OS warnings.** Gatekeeper and SmartScreen
  will scare some users on first launch. Mitigated by documented
  walkthroughs on `/setup`. Acceptable for closed beta; would be a
  release-blocker for GA.
- **GoReleaser learning curve.** The team has not used GoReleaser
  before. Migration from the hand-written matrix is a one-time cost;
  long-term maintenance is materially lower than per-OS scripts.
- **NSIS is not a modern toolchain.** Scripts are written in NSIS's own
  DSL. Mitigated because the script is short (copy file, write JSON,
  register Scheduled Task) and a single engineer can hold the whole
  thing in their head.
- **No tray icon means some users will not realize the daemon is
  running.** Mitigated by the SPA `/setup` page's status indicator and
  by a "first event ingested" notification in the SPA. Re-evaluated
  post-GA.
- **GA signing is a hard cutover.** Once we sign, we cannot un-sign
  without breaking auto-update trust chains. Documented and accepted.

### Neutral

- **Existing `install.sh` / `install.ps1` scripts.** Kept in-tree as a
  power-user / CI install path. Not advertised in the SPA after v0.4.0.
- **Linux daemon.** Out of scope. If added later, GoReleaser already
  supports `.deb` / `.rpm` / Snap builds with no change to this ADR's
  approach.

---

## Cost Summary

| Item | When | Cost |
|---|---|---|
| GoReleaser open-source | Beta (now) | $0 |
| Apple Developer Program | GA | $99/yr |
| Azure Trusted Signing | GA | $9.99/mo (~$120/yr) |
| GoReleaser Pro (optional) | GA | $79/yr |
| **Beta total (annualized)** | | **$0/yr** |
| **GA total (annualized)** | | **$219/yr** (or $298/yr with GoReleaser Pro) |

EV certificate is **explicitly excluded** ($300–$600/yr) — Microsoft
removed the SmartScreen advantage in 2024.

---

## Implementation Tickets

These tickets land in milestone v0.4.0 ("Daemon installer + setup flow")
on project board #27. Project Manager owns final ticket creation,
milestone assignment, and sequencing; this ADR is the source of truth.

| Ticket | Scope | Owner |
|---|---|---|
| **TBD-A** | Add `.goreleaser.yml` at `services/daemon/.goreleaser.yml`; produce darwin universal binary + windows amd64 binary; verify checksums | backend-engineer |
| **TBD-B** | Port `services/daemon/install/macos/install.sh` LaunchAgent logic into a `.pkg` postinstall script; build `.pkg` via `pkgbuild`/`productbuild`; wrap in `.dmg` | backend-engineer |
| **TBD-C** | Port `services/daemon/install/windows/install.ps1` Scheduled Task logic into an NSIS `.nsi` script; produce per-user `.exe` installer (no UAC) | backend-engineer |
| **TBD-D** | Replace the hand-written matrix in `.github/workflows/daemon-release.yml` with a GoReleaser-driven workflow; preserve `GONOSUMDB` / `GOPRIVATE` env vars on every Go step | backend-engineer |
| **TBD-E** | Daemon first-run config detection: missing `daemon.json` → write stub + open `vaultmtg.app/setup` (or print URL on headless platforms) | backend-engineer |
| **TBD-F** | SPA `/setup` page: "First-time install warnings" section with Gatekeeper + SmartScreen screenshots and bypass instructions | front-engineer |
| **TBD-G** | SPA `/setup` page: daemon pairing flow that mints a Clerk API key and posts it to the daemon's local health endpoint to write `daemon.json` | front-engineer |
| **TBD-H** | SPA `DaemonDownload.tsx`: replace broken "install script" links with `.dmg` and `.exe` download buttons; surface platform detection | front-engineer |
| **TBD-I** | Docs: update `services/daemon/install/README.md` to describe the `.pkg`/`.dmg` and NSIS `.exe` install paths; mark the shell scripts as power-user fallback | architect |
| **TBD-J** | GA-prep: enroll in Apple Developer Program ($99/yr); document notarization workflow and `notarytool` credentials in SSM | architect + infrastructure |
| **TBD-K** | GA-prep: onboard Azure Trusted Signing ($9.99/mo); document signing workflow in GoReleaser Pro config; budget approval | architect + infrastructure |

Each ticket gets its own acceptance criteria when the Project Manager
files it.

---

## Alternatives Considered

### A. Keep raw binaries on GitHub Releases + shell-script install

**Rejected for beta and beyond.** This is the current state. Acceptable
for alpha and engineer-internal use; not acceptable for a paid beta
audience that does not know what a terminal is. The shell scripts are
preserved as a power-user fallback.

### B. macOS `.app` bundle (drag to Applications)

**Rejected.** The daemon is headless background software. A `.app` is
the conventional surface for a foreground GUI application; the
conventional surface for a headless background tool on macOS is `.pkg`
plus LaunchAgent. Shipping an `.app` would mean either (a) building a
GUI we have no plans to write, or (b) shipping a fake `.app` that
launches a daemon and immediately exits — which confuses users.

### C. macOS Homebrew cask

**Rejected for primary distribution; viable as a secondary channel.**
Homebrew reaches power users only and assumes the user already has
Homebrew installed. May be added post-GA as a convenience channel
alongside the canonical `.dmg`.

### D. Windows MSI (WiX)

**Rejected for v0.4.0.** MSI requires the WiX toolchain (heavy build
dependency) and is the right answer only when an enterprise customer
asks for Group Policy / SCCM deployment. Reconsidered post-GA if such a
customer materializes.

### E. Windows Service (instead of Scheduled Task)

**Rejected.** Windows Services require admin elevation to install and
run under `LocalSystem` or `NetworkService` accounts that have no
access to the user's `%LOCALAPPDATA%\Wizards Of The Coast\MTGA\Logs\`
directory. The daemon must run as the logged-in user; a Scheduled Task
at logon is the correct primitive.

### F. Microsoft Store / Mac App Store

**Rejected.** Sandboxing in both stores prevents the daemon from
reading another application's log file, which is the daemon's entire
purpose. Distributing through either store would require either store
exemptions (unlikely to be granted for a third-party companion tool) or
a fundamentally different data-collection architecture.

### G. EV code-signing certificate (Windows)

**Rejected.** EV certs cost $300–$600/yr plus hardware-token shipping
and used to grant immediate SmartScreen reputation. Microsoft removed
that advantage in 2024 — non-EV certificates from Azure Trusted Signing
now achieve the same SmartScreen behavior for a fraction of the cost.

### H. System tray / menubar UI in the daemon

**Rejected for v0.4.0.** Adds per-platform native-UI dependencies
(Cocoa on macOS, Win32/WinForms on Windows or a Go tray library), grows
the binary, and creates a second status surface that competes with the
SPA `/setup` page. Reconsidered only if user research after GA shows
"I didn't know the daemon was running" as a top churn driver.

### I. Installer prompts the user for BFF URL + auth token

**Rejected.** This is the current shell-script behavior, and it has
two failure modes the installer is the wrong place to handle:
(1) the user does not have a token yet because they have not signed up,
and (2) the installer must keep secrets out of process arguments and
env vars. Pairing belongs on the SPA where the user is signed in to
Clerk and can mint a per-machine API key on demand.

---

## References

- ADR-003 — sync service deployment strategy. Establishes the
  separate-binary, separate-release pattern that this ADR extends.
- ADR-008 — frontend serving model (S3+CloudFront). The `/setup` page
  with install instructions ships in the same SPA bundle.
- ADR-009 — Clerk user auth. The first-run pairing flow mints a Clerk
  API key and writes it to `daemon.json`.
- ADR-010 — draft overlay architecture. Confirms the daemon stays
  headless and the SPA owns all user-facing surfaces.
- `services/daemon/install/macos/install.sh` — current LaunchAgent
  install logic; ported into the `.pkg` postinstall.
- `services/daemon/install/windows/install.ps1` — current Scheduled
  Task install logic; ported into the NSIS script.
- `.github/workflows/daemon-release.yml` — current hand-written matrix;
  replaced by GoReleaser config.
- GoReleaser — <https://goreleaser.com/>.
- Apple Developer Program — <https://developer.apple.com/programs/>.
- Azure Trusted Signing — <https://learn.microsoft.com/azure/trusted-signing/>.
- NSIS — <https://nsis.sourceforge.io/>.
