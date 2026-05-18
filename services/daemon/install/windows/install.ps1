# install.ps1 — Windows installer for the MTGA Companion daemon
#
# Usage (run as the current user — no admin required for Task Scheduler):
#   irm https://raw.githubusercontent.com/RdHamilton/MTGA-Companion/main/services/daemon/install/windows/install.ps1 | iex
#
# Steps:
#   1. Resolves the latest daemon GitHub Release (or uses $Env:RELEASE_TAG).
#   2. Downloads the Windows amd64 binary to %ProgramFiles%\MTGA-Companion\.
#   3. Writes cloud_api_url and api_key as JSON to %APPDATA%\mtga-companion\daemon.json.
#   4. Registers a Task Scheduler startup task for the current user so the
#      daemon starts at logon without requiring UAC elevation.

#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
$GitHubRepo  = 'RdHamilton/MTGA-Companion'
$AssetName   = 'mtga-companion-daemon-windows-amd64.exe'
$BinaryName  = 'mtga-companion-daemon.exe'
# Install to a subfolder of %ProgramFiles%; fall back to %LOCALAPPDATA% if
# the user does not have write access to %ProgramFiles%.
$InstallDir  = Join-Path $Env:ProgramFiles 'MTGA-Companion'
$TaskName    = 'MTGA-Companion-Daemon'
# Config is written to %APPDATA%\mtga-companion\daemon.json — this matches the
# Windows default path used by defaultConfigPath() in cmd/daemon/main.go.
# Note: Task Scheduler passes -config explicitly (see action below), so the
# default path is only relevant when running the binary directly without that flag.
$ConfigDir   = Join-Path $Env:APPDATA 'mtga-companion'
$ConfigFile  = Join-Path $ConfigDir 'daemon.json'

# Optional overrides via environment variables.
$ReleaseTag  = $Env:RELEASE_TAG   # e.g. "daemon/v0.2.0"
$BffUrl      = $Env:BFF_URL       # e.g. "https://api.yourdomain.com"
$AuthToken   = $Env:DAEMON_AUTH_TOKEN

# ---------------------------------------------------------------------------
# Helper: resolve the latest daemon release tag from the GitHub API.
# ---------------------------------------------------------------------------
function Get-LatestDaemonTag {
    $apiUrl   = "https://api.github.com/repos/$GitHubRepo/releases"
    $headers  = @{ 'User-Agent' = 'mtga-companion-installer' }
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

Write-Host "Installing MTGA Companion daemon $ReleaseTag (windows-amd64)..."

# ---------------------------------------------------------------------------
# Ensure install directory exists.
# If %ProgramFiles% is not writable by the current user, fall back to
# %LOCALAPPDATA%\MTGA-Companion so no UAC prompt is needed.
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
    Write-Warning "%ProgramFiles% not writable — falling back to %LOCALAPPDATA%\MTGA-Companion"
    $InstallDir = Join-Path $Env:LOCALAPPDATA 'MTGA-Companion'
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
$settings = New-ScheduledTaskSettingsSet `
    -ExecutionTimeLimit ([TimeSpan]::Zero) `   # No timeout — the daemon is long-running.
    -StartWhenAvailable $true                  # Idempotency handled by Unregister-ScheduledTask above.

$principal = New-ScheduledTaskPrincipal `
    -UserId $Env:USERNAME `
    -LogonType Interactive `
    -RunLevel Highest   # Highest privilege within the user's token — still no UAC.

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
Write-Host 'MTGA Companion daemon installed and running.'
Write-Host "  Binary : $BinaryPath"
Write-Host "  Config : $ConfigFile"
Write-Host "  Task   : $TaskName (Task Scheduler)"
Write-Host ''
Write-Host 'The daemon will start automatically at each logon.'
