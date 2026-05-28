# uninstall.ps1 — removes the VaultMTG daemon from Windows.
#
# Usage (run in PowerShell as the current user):
#   .\uninstall.ps1 [-Purge]
#
# Options:
#   -Purge    Also delete the daemon's API key from Windows Credential Manager.
#             By default the credential (target: com.vaultmtg.daemon:api-key)
#             is retained for downgrade safety so that a reinstall does not
#             require re-authenticating.
#
# Steps:
#   1. Stops and removes the VaultMTG-Daemon scheduled task.
#   2. Stops and removes the legacy MTGA-Companion-Daemon task if still present.
#   3. Removes the binary from the install directory.
#   4. (-Purge only) Deletes the API key from Windows Credential Manager.
#
# The config file and log files are NOT removed — delete them manually if
# desired.

#Requires -Version 5.1
[CmdletBinding()]
param(
    [switch]$Purge
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Configuration — must match install.ps1.
# ---------------------------------------------------------------------------
$TaskName       = 'VaultMTG-Daemon'
$LegacyTaskName = 'MTGA-Companion-Daemon'
$BinaryName     = 'vaultmtg-daemon.exe'

# Determine install directory: prefer %ProgramFiles%\VaultMTG, fall back to
# %LOCALAPPDATA%\VaultMTG (mirrors install.ps1 fallback logic).
$InstallDir = Join-Path $Env:ProgramFiles 'VaultMTG'
$BinaryPath = Join-Path $InstallDir $BinaryName

if (-not (Test-Path $BinaryPath)) {
    $InstallDir = Join-Path $Env:LOCALAPPDATA 'VaultMTG'
    $BinaryPath = Join-Path $InstallDir $BinaryName
}

# ---------------------------------------------------------------------------
# Stop and remove the VaultMTG-Daemon scheduled task.
# SilentlyContinue so this is idempotent — safe to run if the task was
# never started or was already stopped.
# ---------------------------------------------------------------------------
Write-Host "Stopping task '$TaskName' (if running)..."
Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue

Write-Host "Removing task '$TaskName' from Task Scheduler..."
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

# ---------------------------------------------------------------------------
# Also stop and remove the legacy MTGA-Companion-Daemon task if it is still
# present (e.g. the user never ran install.ps1 after upgrading, or a partial
# upgrade left the old task registered alongside the new binary).
# ---------------------------------------------------------------------------
$legacyTask = Get-ScheduledTask -TaskName $LegacyTaskName -ErrorAction SilentlyContinue
if ($null -ne $legacyTask) {
    Write-Host "Removing legacy task '$LegacyTaskName'..."
    Stop-ScheduledTask -TaskName $LegacyTaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $LegacyTaskName -Confirm:$false -ErrorAction SilentlyContinue
}

# ---------------------------------------------------------------------------
# Remove the binary.
# We wait up to 5 seconds for the process to exit before deleting.
# ---------------------------------------------------------------------------
if (Test-Path $BinaryPath) {
    Write-Host "Removing binary: $BinaryPath"
    $deadline = (Get-Date).AddSeconds(5)
    while ((Get-Date) -lt $deadline) {
        try {
            Remove-Item -Path $BinaryPath -Force
            break
        } catch {
            # Process may still hold a handle — wait briefly and retry.
            Start-Sleep -Milliseconds 500
        }
    }
    if (Test-Path $BinaryPath) {
        Write-Warning "Could not remove $BinaryPath — it may still be in use. Remove it manually."
    }
} else {
    Write-Host "Binary not found ($BinaryPath), skipping."
}

# ---------------------------------------------------------------------------
# Credential Manager entry (target: com.vaultmtg.daemon:api-key).
# Default behaviour: RETAIN the credential for downgrade safety.
# -Purge: delete it so no credential remains in Credential Manager.
# go-keyring on Windows uses the target name "<service>:<account>".
# ---------------------------------------------------------------------------
$CredTarget = 'com.vaultmtg.daemon:api-key'

if ($Purge) {
    Write-Host "Removing Credential Manager entry '$CredTarget'..."
    try {
        $cred = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR(
            (New-Object System.Security.SecureString))
        # Use cmdkey to delete — available on all Windows versions supported by the daemon.
        $result = cmdkey /delete:$CredTarget 2>&1
        Write-Host "Credential Manager entry removed (or was already absent)."
    } catch {
        # Entry absent or access error — non-fatal.
        Write-Host "Credential Manager entry not found or already removed."
    }
}

Write-Host ''
Write-Host 'VaultMTG daemon uninstalled.'
Write-Host "Config dir ($Env:APPDATA\vaultmtg) was NOT removed."
Write-Host 'Remove it manually if desired.'
if (-not $Purge) {
    Write-Host 'API key retained in Credential Manager for downgrade safety. Run with -Purge to remove all data.'
}
