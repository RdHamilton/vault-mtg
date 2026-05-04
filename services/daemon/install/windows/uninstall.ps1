# uninstall.ps1 — removes the MTGA Companion daemon from Windows.
#
# Usage (run in PowerShell as the current user):
#   .\uninstall.ps1
#
# Steps:
#   1. Stops the running daemon process (if any).
#   2. Removes the Task Scheduler logon task.
#   3. Removes the binary from the install directory.
#
# The config file and log files are NOT removed — delete them manually if
# desired.

#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Configuration — must match install.ps1.
# ---------------------------------------------------------------------------
$TaskName   = 'MTGA-Companion-Daemon'
$BinaryName = 'mtga-companion-daemon.exe'

# Determine install directory: prefer %ProgramFiles%, fall back to
# %LOCALAPPDATA% (mirrors install.ps1 fallback logic).
$InstallDir = Join-Path $Env:ProgramFiles 'MTGA-Companion'
$BinaryPath = Join-Path $InstallDir $BinaryName

if (-not (Test-Path $BinaryPath)) {
    $InstallDir = Join-Path $Env:LOCALAPPDATA 'MTGA-Companion'
    $BinaryPath = Join-Path $InstallDir $BinaryName
}

# ---------------------------------------------------------------------------
# Stop the scheduled task (stops the running daemon process).
# SilentlyContinue so this is idempotent — safe to run if the task was
# never started or was already stopped.
# ---------------------------------------------------------------------------
Write-Host "Stopping task '$TaskName' (if running)..."
Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue

# ---------------------------------------------------------------------------
# Remove the scheduled task registration.
# Confirm:$false suppresses the interactive confirmation prompt.
# ---------------------------------------------------------------------------
Write-Host "Removing task '$TaskName' from Task Scheduler..."
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

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

Write-Host ''
Write-Host 'MTGA Companion daemon uninstalled.'
Write-Host "Config file ($InstallDir\daemon.yaml) was NOT removed."
Write-Host 'Remove it manually if desired.'
