# install.ps1 — Windows installer for the VaultMTG daemon
#
# Usage (run as the current user — no admin required for Task Scheduler):
#   irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/install.ps1 | iex
#
# Steps:
#   1. Resolves the latest daemon GitHub Release (or uses $Env:RELEASE_TAG).
#   2. Migrates config from %APPDATA%\mtga-companion\ → %APPDATA%\vaultmtg\ (upgrade path).
#   3. Removes any legacy MTGA-Companion-Daemon scheduled task so two daemons
#      cannot run simultaneously after an upgrade.
#   4. Downloads the Windows amd64 binary to %ProgramFiles%\VaultMTG\.
#   5. Writes cloud_api_url and api_key as JSON to %APPDATA%\vaultmtg\daemon.json.
#   6. Registers a Task Scheduler startup task for the current user so the
#      daemon starts at logon without requiring UAC elevation.

#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
$GitHubRepo     = 'RdHamilton/MTGA-Companion'
$AssetName      = 'vaultmtg-daemon-windows-amd64.exe'
$BinaryName     = 'vaultmtg-daemon.exe'
# Install to a subfolder of %ProgramFiles%; fall back to %LOCALAPPDATA% if
# the user does not have write access to %ProgramFiles%.
$InstallDir     = Join-Path $Env:ProgramFiles 'VaultMTG'
$TaskName       = 'VaultMTG-Daemon'
# Legacy task name — removed during upgrade so two daemons cannot run at once.
$LegacyTaskName = 'MTGA-Companion-Daemon'
# Config is written to %APPDATA%\vaultmtg\daemon.json — this matches the
# Windows default path used by defaultConfigPath() in cmd/daemon/main.go.
# Note: Task Scheduler passes -config explicitly (see action below), so the
# default path is only relevant when running the binary directly without that flag.
$ConfigDir      = Join-Path $Env:APPDATA 'vaultmtg'
$ConfigFile     = Join-Path $ConfigDir 'daemon.json'
# Legacy config dir — migrated on upgrade (copy-not-move for downgrade safety).
$LegacyConfigDir = Join-Path $Env:APPDATA 'mtga-companion'

# Optional overrides via environment variables.
$ReleaseTag  = $Env:RELEASE_TAG   # e.g. "daemon/v0.2.0"
$BffUrl      = $Env:BFF_URL       # e.g. "https://api.yourdomain.com"
$AuthToken   = $Env:DAEMON_AUTH_TOKEN

# ---------------------------------------------------------------------------
# Helper: resolve the latest daemon release tag from the GitHub API.
# ---------------------------------------------------------------------------
function Get-LatestDaemonTag {
    $apiUrl   = "https://api.github.com/repos/$GitHubRepo/releases"
    $headers  = @{ 'User-Agent' = 'vaultmtg-installer' }
    $releases = Invoke-RestMethod -Uri $apiUrl -Headers $headers -Method Get

    foreach ($r in $releases) {
        if ($r.tag_name -like 'daemon/*') {
            return $r.tag_name
        }
    }
    throw 'Could not determine latest daemon release. Set $Env:RELEASE_TAG and retry.'
}

# ---------------------------------------------------------------------------
# Resolve release tag.
# ---------------------------------------------------------------------------
if (-not $ReleaseTag) {
    Write-Host 'Fetching latest daemon release tag...'
    $ReleaseTag = Get-LatestDaemonTag
}

Write-Host "Installing VaultMTG daemon $ReleaseTag (windows-amd64)..."

# ---------------------------------------------------------------------------
# Config-dir migration (ADR-022 Phase 2, upgrade path).
#
# Copy %APPDATA%\mtga-companion\ → %APPDATA%\vaultmtg\ so existing users
# retain their configuration after upgrading.
#
# Rules (mirror Go migrate.MigrateConfigDir):
#   - No-op when %APPDATA%\mtga-companion does not exist (fresh install).
#   - No-op when %APPDATA%\vaultmtg already exists and is non-empty
#     (migration already ran, e.g. the daemon binary ran first).
#   - Copy-not-move: %APPDATA%\mtga-companion is retained for downgrade safety.
#
# Note: the daemon binary itself also runs this migration at startup
# (runConfigDirMigration in cmd/daemon/main.go). The installer-side copy
# here ensures config is in place before the new task ever runs for the
# first time. The two are consistent — the Go helper is idempotent.
# ---------------------------------------------------------------------------
if (Test-Path $LegacyConfigDir) {
    $newDirPopulated = (Test-Path $ConfigDir) -and `
        ((Get-ChildItem -Path $ConfigDir -Force -ErrorAction SilentlyContinue | Measure-Object).Count -gt 0)
    if (-not $newDirPopulated) {
        Write-Host "Migrating config: $LegacyConfigDir → $ConfigDir"
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
        Get-ChildItem -Path $LegacyConfigDir -Recurse -Force | ForEach-Object {
            $rel     = $_.FullName.Substring($LegacyConfigDir.Length).TrimStart('\')
            $dstPath = Join-Path $ConfigDir $rel
            if ($_.PSIsContainer) {
                New-Item -ItemType Directory -Path $dstPath -Force | Out-Null
            } elseif (-not (Test-Path $dstPath)) {
                $dstParent = Split-Path $dstPath -Parent
                New-Item -ItemType Directory -Path $dstParent -Force | Out-Null
                Copy-Item -Path $_.FullName -Destination $dstPath -Force
            }
        }
        Write-Host "Config migration complete (legacy dir retained for downgrade safety)."
    } else {
        Write-Host "Config dir $ConfigDir already populated — skipping migration."
    }
}

# ---------------------------------------------------------------------------
# Remove legacy scheduled task (ADR-022 Phase 2, upgrade safety).
#
# If MTGA-Companion-Daemon is still registered, unregister it NOW — before
# registering VaultMTG-Daemon — so two daemons cannot run simultaneously.
# This is a silent no-op on fresh installs where the old task never existed.
# ---------------------------------------------------------------------------
$legacyTask = Get-ScheduledTask -TaskName $LegacyTaskName -ErrorAction SilentlyContinue
if ($null -ne $legacyTask) {
    Write-Host "Removing legacy scheduled task '$LegacyTaskName'..."
    Stop-ScheduledTask -TaskName $LegacyTaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $LegacyTaskName -Confirm:$false -ErrorAction SilentlyContinue
    Write-Host "Legacy task removed."
}

# ---------------------------------------------------------------------------
# Ensure install directory exists.
# If %ProgramFiles% is not writable by the current user, fall back to
# %LOCALAPPDATA%\VaultMTG so no UAC prompt is needed.
# ---------------------------------------------------------------------------
try {
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    # Quick write test.
    $testFile = Join-Path $InstallDir '.write-test'
    [System.IO.File]::WriteAllText($testFile, '')
    Remove-Item $testFile -Force
} catch {
    Write-Warning "%ProgramFiles% not writable — falling back to %LOCALAPPDATA%\VaultMTG"
    $InstallDir = Join-Path $Env:LOCALAPPDATA 'VaultMTG'
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# ---------------------------------------------------------------------------
# Download the binary.
# ---------------------------------------------------------------------------
$DownloadUrl = "https://github.com/$GitHubRepo/releases/download/$ReleaseTag/$AssetName"
$BinaryPath  = Join-Path $InstallDir $BinaryName

Write-Host "Downloading $DownloadUrl..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $BinaryPath -UseBasicParsing

Write-Host "Binary installed: $BinaryPath"

# ---------------------------------------------------------------------------
# Write the config file.
# Key names must match the json struct tags in
# services/daemon/internal/config/config.go.
# ConvertTo-Json is used to safely serialise the values — special characters
# (quotes, backslashes, control chars) are escaped by the JSON encoder, so
# the file is always valid JSON regardless of what the user types.
# Prompt the user for values not provided via environment variables.
# ---------------------------------------------------------------------------
if (-not $BffUrl) {
    $BffUrl = Read-Host 'Enter BFF URL (e.g. https://api.yourdomain.com)'
}
if (-not $AuthToken) {
    $AuthToken = Read-Host 'Enter DAEMON_AUTH_TOKEN (daemon JWT from first registration)'
}

New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null

$config = [ordered]@{ cloud_api_url = $BffUrl; api_key = $AuthToken }
$config | ConvertTo-Json | Set-Content -Path $ConfigFile -Encoding UTF8

Write-Host "Config written: $ConfigFile"

# ---------------------------------------------------------------------------
# Register a Task Scheduler startup task for the current user.
#
# Why Task Scheduler instead of New-Service:
#   - New-Service requires admin elevation (UAC prompt).
#   - A Logon trigger with the current user principal runs without elevation
#     and with access to the user's home directory (where Player.log lives).
#
# The task is registered with:
#   Trigger  : AtLogon (current user)
#   Action   : run the binary
#   RunLevel : LeastPrivilege (no UAC)
# ---------------------------------------------------------------------------
Write-Host "Registering startup task '$TaskName'..."

# Remove any existing registration so this script is idempotent.
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

$action  = New-ScheduledTaskAction -Execute $BinaryPath -Argument "-config `"$ConfigFile`""
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $Env:USERNAME
# No timeout — the daemon is long-running. Don't start a second copy if already running.
$settings = New-ScheduledTaskSettingsSet `
    -ExecutionTimeLimit ([TimeSpan]::Zero) `
    -MultipleInstances IgnoreNew `
    -StartWhenAvailable

# Highest privilege within the user's token — still no UAC elevation.
$principal = New-ScheduledTaskPrincipal `
    -UserId $Env:USERNAME `
    -LogonType Interactive `
    -RunLevel Highest

Register-ScheduledTask `
    -TaskName $TaskName `
    -Action   $action `
    -Trigger  $trigger `
    -Settings $settings `
    -Principal $principal `
    -Force | Out-Null

# ---------------------------------------------------------------------------
# Start the daemon immediately so the user does not need to log out first.
# ---------------------------------------------------------------------------
Write-Host "Starting daemon..."
Start-ScheduledTask -TaskName $TaskName

Write-Host ''
Write-Host 'VaultMTG daemon installed and running.'
Write-Host "  Binary : $BinaryPath"
Write-Host "  Config : $ConfigFile"
Write-Host "  Task   : $TaskName (Task Scheduler)"
Write-Host ''
Write-Host 'The daemon will start automatically at each logon.'
