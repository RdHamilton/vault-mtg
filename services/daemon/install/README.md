# MTGA Companion Daemon — Install Scripts

Platform-specific scripts to install, run at startup, and uninstall the MTGA
Companion daemon binary.

The daemon reads MTGA's `Player.log` on the local machine and ships events to
the cloud BFF.  It must run as the same user that runs MTGA Arena so it can
access `Player.log` in the user's home directory.

## Config file format

The daemon reads a **JSON** config file.  The canonical keys are:

```json
{
  "cloud_api_url": "https://api.yourdomain.com",
  "api_key": "<daemon-jwt>",
  "sync_enabled": true
}
```

See `services/daemon/internal/config/config.go` for the full list of supported fields.

---

## macOS

### Install (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/install.sh | bash
```

The script:

1. Detects your architecture (Apple Silicon or Intel).
2. Downloads the correct binary from the latest GitHub Release.
3. Installs the binary to `/usr/local/bin/mtga-companion-daemon` (requires
   `sudo` for that directory).
4. Prompts for the BFF URL and daemon auth token and writes them to
   `~/.mtga-companion/daemon.json`.
5. Writes a launchd plist to
   `~/Library/LaunchAgents/com.mtga-companion.daemon.plist` and loads it.

The daemon starts immediately and restarts automatically on login.

**Logs**: `~/Library/Logs/mtga-companion-daemon.log`

**Config**: `~/.mtga-companion/daemon.json`

#### Pin to a specific release

```bash
RELEASE_TAG=daemon/v0.1.0 curl -fsSL \
  https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/install.sh | bash
```

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/macos/uninstall.sh | bash
```

Or run the script directly if you have a local copy:

```bash
bash services/daemon/install/macos/uninstall.sh
```

---

## Windows

### Install (PowerShell one-liner)

Run in a PowerShell terminal (no admin / UAC required):

```powershell
irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/install.ps1 | iex
```

The script:

1. Downloads the Windows amd64 binary from the latest GitHub Release.
2. Installs it to `%ProgramFiles%\MTGA-Companion\` (falls back to
   `%LOCALAPPDATA%\MTGA-Companion\` if `%ProgramFiles%` is not writable without
   elevation).
3. Prompts for the BFF URL and daemon auth token and writes them to
   `%APPDATA%\mtga-companion\daemon.json`.
4. Registers a Task Scheduler **AtLogon** task for the current user so the
   daemon starts automatically without UAC elevation.
5. Starts the daemon immediately.

**Config**: `%APPDATA%\mtga-companion\daemon.json`

#### Provide credentials non-interactively

```powershell
$Env:BFF_URL = 'https://api.yourdomain.com'
$Env:DAEMON_AUTH_TOKEN = '<your-daemon-jwt>'
$Env:RELEASE_TAG = 'daemon/v0.1.0'   # optional — defaults to latest
irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/install.ps1 | iex
```

### Uninstall

```powershell
irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/uninstall.ps1 | iex
```

Or if you have a local copy:

```powershell
.\services\daemon\install\windows\uninstall.ps1
```

---

## Binary names (GitHub Releases)

| Platform          | Binary name                              |
|-------------------|------------------------------------------|
| macOS arm64       | `mtga-companion-daemon-darwin-arm64`     |
| macOS amd64       | `mtga-companion-daemon-darwin-amd64`     |
| Windows amd64     | `mtga-companion-daemon-windows-amd64.exe`|

Release tags follow the pattern `daemon/vX.Y.Z`.
