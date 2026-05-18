# VaultMTG Daemon — Installation Guide

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

## Linux — Manual Installation

There is no packaged Linux installer. Linux users install the daemon manually
and manage it via their preferred init system (systemd is documented below).

### Step 1 — Download

Download the `vaultmtg-daemon-linux-amd64` binary from the
[latest release](https://github.com/RdHamilton/MTGA-Companion/releases/latest)
and make it executable:

```bash
chmod +x vaultmtg-daemon-linux-amd64
sudo mv vaultmtg-daemon-linux-amd64 /usr/local/bin/vaultmtg-daemon
```

### Step 2 — Create the config directory

```bash
mkdir -p ~/.vaultmtg
```

### Step 3 — Run the daemon (first launch)

```bash
vaultmtg-daemon
```

On first launch the daemon runs the PKCE browser-redirect flow: it opens your
browser, you log in to your VaultMTG account, and the daemon receives the auth
code and registers with the cloud API. After pairing, `daemon.json` is written
to `~/.vaultmtg/daemon.json`.

### Step 4 — Run as a systemd user service (auto-start)

Create a systemd user unit file:

```bash
mkdir -p ~/.config/systemd/user
```

Create `~/.config/systemd/user/vaultmtg-daemon.service`:

```ini
[Unit]
Description=VaultMTG Daemon
After=network.target

[Service]
ExecStart=/usr/local/bin/vaultmtg-daemon
Restart=on-failure
RestartSec=5s
Environment=VAULTMTG_DAEMON_CLOUD_API_URL=https://api.vaultmtg.app/api/v1
# Legacy name also works: MTGA_DAEMON_CLOUD_API_URL (dual-read shim — see Upgrading section)

[Install]
WantedBy=default.target
```

Enable and start the unit:

```bash
systemctl --user enable vaultmtg-daemon
systemctl --user start vaultmtg-daemon
systemctl --user status vaultmtg-daemon
```

The daemon logs to the systemd journal:

```bash
journalctl --user -u vaultmtg-daemon -f
```

### Config path

| Item | Path |
|------|------|
| Config | `~/.vaultmtg/daemon.json` |
| Log archives | `~/.vaultmtg/archives/` |

---

## Linux — Upgrading from a Previous Release (Config Migration)

Older daemon releases stored configuration in `~/.mtga-companion/` (or
`~/.mtga-daemon/` on some early builds). Starting with the v0.3.2 release
series (ADR-022 Phase 2), the daemon automatically migrates those directories
to `~/.vaultmtg/` on first launch.

**What happens automatically when you upgrade:**

1. The new daemon binary starts.
2. It detects `~/.mtga-companion/` or `~/.mtga-daemon/` (whichever exists).
3. It copies all files to `~/.vaultmtg/` (copy-not-move — your old directory
   is retained for downgrade safety).
4. Subsequent launches are a no-op once `~/.vaultmtg/` is non-empty.

You do not need to copy files manually.

**If you manage the daemon via a custom systemd unit:**

Check whether your unit or environment references any old paths or env var
names. Update them to the new forms:

| Old value | New value |
|---|---|
| `ExecStart=.../mtga-companion-daemon` | `ExecStart=.../vaultmtg-daemon` |
| `Environment=MTGA_DAEMON_*=...` (legacy) | Preferred: `Environment=VAULTMTG_DAEMON_*=...` (see note below) |
| Any path referencing `~/.mtga-companion` | `~/.vaultmtg` |

> **Note on env vars:** The daemon now reads both the legacy `MTGA_DAEMON_*`
> names and the new preferred `VAULTMTG_DAEMON_*` names. Legacy names continue
> to work via a compatibility shim — if your existing unit sets
> `MTGA_DAEMON_CLOUD_API_URL`, `MTGA_DAEMON_API_KEY`, or any other
> `MTGA_DAEMON_*` variable, those values are still picked up by the new binary
> without modification. The new `VAULTMTG_DAEMON_*` names are preferred; when
> both are set, the `VAULTMTG_DAEMON_*` value takes precedence. New installs
> should use `VAULTMTG_DAEMON_*` as shown in the systemd template above.

After updating your unit file, reload and restart the service:

```bash
systemctl --user daemon-reload
systemctl --user restart vaultmtg-daemon
```

### Uninstall (Linux)

```bash
systemctl --user stop vaultmtg-daemon
systemctl --user disable vaultmtg-daemon
rm ~/.config/systemd/user/vaultmtg-daemon.service
systemctl --user daemon-reload
sudo rm /usr/local/bin/vaultmtg-daemon
rm -rf ~/.vaultmtg
```

To also remove the old config directory if you want a clean slate:

```bash
rm -rf ~/.mtga-companion ~/.mtga-daemon
```

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
