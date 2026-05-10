# MTGA Companion Daemon — Installation Guide

The VaultMTG daemon runs in the background while you play MTG Arena and syncs
your match history, draft picks, and collection to your account in real time.

The daemon must run as the same user that runs MTGA Arena so it can read
`Player.log` from the user's home directory.

---

## Quick Install (Recommended)

Download the installer for your platform from the
[GitHub Releases page](https://github.com/RdHamilton/MTGA-Companion/releases/latest)
and follow the instructions for your OS below.

---

## macOS — `.dmg` Installer

### Step 1 — Download

Download `mtga-companion-daemon-darwin-arm64.dmg` (Apple Silicon / M1/M2/M3)
or `mtga-companion-daemon-darwin-amd64.dmg` (Intel) from the
[latest release](https://github.com/RdHamilton/MTGA-Companion/releases/latest).

### Step 2 — Install

1. Double-click the `.dmg` file to mount it.
2. Drag the VaultMTG daemon app to your **Applications** folder.
3. Eject the disk image.

### Step 3 — Bypass Gatekeeper (first run only)

Because VaultMTG is unsigned indie beta software, macOS Gatekeeper will warn
you the first time you open it. This is expected — the app is safe.

**Right-click method (easiest):**
1. Right-click (or Control-click) the app icon.
2. Choose **Open**.
3. Click **Open** again in the dialog.

**System Settings method:**
1. Open **System Settings → Privacy & Security**.
2. Scroll down to the Security section.
3. Click **Open Anyway** next to the VaultMTG daemon entry.

You only need to approve once. macOS remembers your choice for all future launches.

> The `.dmg` installer's postinstall script also clears the quarantine attribute
> (`com.apple.quarantine`) from the daemon binary automatically, which prevents
> Gatekeeper from blocking the daemon process itself after first-run approval.

### Auto-start (LaunchAgent)

The installer registers a `launchd` LaunchAgent so the daemon starts
automatically when you log in.

| Item | Path |
|------|------|
| LaunchAgent plist | `~/Library/LaunchAgents/com.mtga-companion.daemon.plist` |
| Logs | `~/Library/Logs/mtga-companion-daemon.log` |
| Config | `~/.mtga-companion/daemon.json` |

---

## Windows — `.exe` Installer (NSIS)

### Step 1 — Download

Download `mtga-companion-daemon-windows-amd64.exe` from the
[latest release](https://github.com/RdHamilton/MTGA-Companion/releases/latest).

### Step 2 — Bypass SmartScreen (first run only)

Windows Defender SmartScreen may show a blue dialog that says
"Windows protected your PC". This is expected for unsigned beta software — the
installer is safe.

1. Click **More info** in the SmartScreen dialog.
2. Click **Run anyway**.

You only need to do this once.

### Step 3 — Install

Run through the NSIS installer:
**Next → Next → Finish**.

No admin elevation (UAC) is required. The installer places the daemon in
`%LOCALAPPDATA%\MTGA-Companion\` and registers an **AtLogon** Task Scheduler
task so the daemon starts automatically when you log in.

| Item | Path |
|------|------|
| Daemon binary | `%LOCALAPPDATA%\MTGA-Companion\vaultmtg-daemon.exe` |
| Config | `%APPDATA%\mtga-companion\daemon.json` |

---

## Daemon Config Format

On first launch the daemon completes the PKCE pairing flow (opens your browser,
you log in, the daemon receives the auth code and registers with the cloud API).
After pairing, the daemon writes a config file.

```json
{
  "cloud_api_url": "https://api.vaultmtg.app",
  "keychain": true,
  "sync_enabled": true
}
```

> **Note**: The API key is stored in the OS keychain (macOS Keychain /
> Windows Credential Manager), not in plaintext in the config file.
> `"keychain": true` indicates the key is present in the keychain.

See `services/daemon/internal/config/config.go` for the full list of
supported fields.

---

## Uninstall

### macOS

Delete the app from Applications, then remove the LaunchAgent:

```bash
launchctl unload ~/Library/LaunchAgents/com.mtga-companion.daemon.plist
rm ~/Library/LaunchAgents/com.mtga-companion.daemon.plist
rm -rf ~/.mtga-companion
```

To also remove the keychain entry:

```bash
security delete-generic-password -s com.mtga-companion.daemon
```

### Windows

Use **Add or Remove Programs** in Windows Settings to uninstall the VaultMTG
daemon. The uninstaller removes the binary, the Task Scheduler entry, and the
config directory.

To remove the Windows Credential Manager entry, open Credential Manager →
Windows Credentials and delete the `com.mtga-companion.daemon` entry.

---

## Binary Names (GitHub Releases)

| Platform | Installer file |
|----------|---------------|
| macOS arm64 (Apple Silicon) | `mtga-companion-daemon-darwin-arm64.dmg` |
| macOS amd64 (Intel) | `mtga-companion-daemon-darwin-amd64.dmg` |
| Windows amd64 | `mtga-companion-daemon-windows-amd64.exe` |

Release tags follow the pattern `daemon/vX.Y.Z`.

---

## Power-User / Developer Fallback: Shell Scripts

> **These scripts are for developers and advanced users only.**
> For standard installation, use the `.dmg` or `.exe` installers above.

Shell scripts are provided for users who prefer to install from the command
line or who are integrating the daemon into a custom workflow.

### macOS shell script

```bash
curl -fsSL https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/install.sh | bash
```

The script detects your architecture, downloads the correct binary from the
latest GitHub Release, writes a `daemon.json` stub, and registers a launchd
LaunchAgent.

#### Pin to a specific release

```bash
RELEASE_TAG=daemon/v0.1.0 curl -fsSL \
  https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/install.sh | bash
```

#### Uninstall (shell)

```bash
curl -fsSL https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/uninstall.sh | bash
```

### Windows PowerShell script

Run in a PowerShell terminal (no admin / UAC required):

```powershell
irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/install.ps1 | iex
```

#### Provide credentials non-interactively

```powershell
$Env:BFF_URL = 'https://api.vaultmtg.app'
$Env:RELEASE_TAG = 'daemon/v0.1.0'   # optional — defaults to latest
irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/install.ps1 | iex
```

#### Uninstall (PowerShell)

```powershell
irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/uninstall.ps1 | iex
```
