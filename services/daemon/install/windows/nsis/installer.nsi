; installer.nsi — NSIS per-user installer for the VaultMTG daemon.
;
; Design constraints (ADR-011-C):
;   - Per-user install: binary to %LOCALAPPDATA%\VaultMTG\
;   - No UAC elevation — RequestExecutionLevel user
;   - No MSI, no WiX, no Windows Service
;   - Scheduled Task at logon using RunLevel LeastPrivilege (no UAC popup)
;   - Config file (daemon.json) written to %APPDATA%\vaultmtg\
;
; Build command (from the repo root):
;   makensis services/daemon/install/windows/nsis/installer.nsi
;
; GoReleaser calls makensis automatically via the `nfpms` / `before` hook
; in .goreleaser.yml (see goreleaser-nsis job in daemon-release.yml).
;
; The installer is self-contained — the daemon binary is embedded at compile
; time via the File directive.  VERSION and BINARY_PATH are passed in via
; /DVERSION=x.y.z and /DBINARY_PATH=path\to\binary on the makensis command
; line.

!ifndef VERSION
  !define VERSION "dev"
!endif

!ifndef BINARY_PATH
  !define BINARY_PATH "bin\vaultmtg-daemon-windows-amd64.exe"
!endif

; CLOUD_API_URL is the BFF endpoint baked in at build time via /DCLOUD_API_URL=
; on the makensis command line.  The installer uses this value to:
;   1. Detect a cross-env reinstall (stored URL != new URL) and clear the stale
;      Windows Credential Manager entry so the daemon doesn't auth against the
;      wrong environment (#194).
;   2. Write / update cloud_api_url in daemon.json during install.
; Default is empty so a developer build with no /D flag still compiles.
!ifndef CLOUD_API_URL
  !define CLOUD_API_URL ""
!endif

;----------------------------------------------------------------------
; General attributes
;----------------------------------------------------------------------
Name              "VaultMTG Daemon ${VERSION}"
; Output is written to the current directory (repo root when invoked via GoReleaser CI).
; GoReleaser extra_files glob uses vaultmtg-daemon-setup-*.exe at repo root.
OutFile           "vaultmtg-daemon-setup-${VERSION}.exe"

; Per-user install — no UAC prompt (RequestExecutionLevel user)
RequestExecutionLevel user

; Default install dir: %LOCALAPPDATA%\VaultMTG
InstallDir        "$LOCALAPPDATA\VaultMTG"

; Modern UI
!include MUI2.nsh
!define MUI_ABORTWARNING

;----------------------------------------------------------------------
; Pages
;----------------------------------------------------------------------
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

;----------------------------------------------------------------------
; Language
;----------------------------------------------------------------------
!insertmacro MUI_LANGUAGE "English"

;----------------------------------------------------------------------
; Installer section
;----------------------------------------------------------------------
Section "Install" SecInstall

  SetOutPath "$INSTDIR"

  ; Copy the binary.
  File /oname=vaultmtg-daemon.exe "${BINARY_PATH}"

  ; Config-dir migration (ADR-022 Phase 2, upgrade path).
  ; Copy %APPDATA%\mtga-companion\daemon.json → %APPDATA%\vaultmtg\daemon.json
  ; so existing users retain their configuration after upgrading.
  ; Copy-not-move: the legacy dir is retained for downgrade safety.
  ; The daemon binary also runs this migration at startup (idempotent).
  CreateDirectory "$APPDATA\vaultmtg"
  IfFileExists "$APPDATA\vaultmtg\daemon.json" ConfigMigrationDone CheckLegacyConfig
  CheckLegacyConfig:
    IfFileExists "$APPDATA\mtga-companion\daemon.json" DoConfigMigration ConfigMigrationDone
    DoConfigMigration:
      CopyFiles "$APPDATA\mtga-companion\daemon.json" "$APPDATA\vaultmtg\daemon.json"
  ConfigMigrationDone:

  ; --- Cross-env reinstall guard (#194) ----------------------------------------
  ; If daemon.json already exists AND CLOUD_API_URL was supplied at build time,
  ; compare the stored cloud_api_url to the baked-in value.  On a mismatch (e.g.
  ; staging → prod reinstall) clear the stale Windows Credential Manager entry
  ; and update cloud_api_url in-place, preserving all other JSON fields.
  ;
  ; Implementation uses the .ps1 temp-file pattern (Ray correction #1) to avoid
  ; NSIS/PowerShell quote-nesting issues — same approach as the health-check block.
  ; $$ = NSIS escape for a literal dollar sign in the written PowerShell script.
  IfFileExists "$APPDATA\vaultmtg\daemon.json" CheckEnvMismatch SkipEnvMismatch
  CheckEnvMismatch:
    FileOpen  $2 "$TEMP\vaultmtg-env-check.ps1" w
    FileWrite $2 '$$configFile = "$${Env:APPDATA}\vaultmtg\daemon.json"$\n'
    FileWrite $2 '$$newUrl     = "${CLOUD_API_URL}"$\n'
    FileWrite $2 'if ([string]::IsNullOrEmpty($$newUrl)) { exit 0 }$\n'
    FileWrite $2 'try {$\n'
    FileWrite $2 '    $$data = Get-Content $$configFile -Raw | ConvertFrom-Json$\n'
    FileWrite $2 '    $$oldUrl = $$data.cloud_api_url$\n'
    FileWrite $2 '    if ($$oldUrl -and $$oldUrl -ne $$newUrl) {$\n'
    FileWrite $2 '        Write-Host "cross-env reinstall: old=$${oldUrl} new=$${newUrl}"$\n'
    FileWrite $2 '        cmdkey /delete:vaultmtg-daemon-api-key 2>$$null$\n'
    FileWrite $2 '        $$data.cloud_api_url = $$newUrl$\n'
    FileWrite $2 '        $$data | ConvertTo-Json -Depth 10 | Set-Content $$configFile -Encoding UTF8$\n'
    FileWrite $2 '        Write-Host "cloud_api_url updated and stale credential cleared"$\n'
    FileWrite $2 '    } else {$\n'
    FileWrite $2 '        Write-Host "cloud_api_url unchanged — preserving existing config"$\n'
    FileWrite $2 '    }$\n'
    FileWrite $2 '} catch {$\n'
    FileWrite $2 '    Write-Host "env-check: could not read daemon.json ($${_}) — skipping"$\n'
    FileWrite $2 '}$\n'
    FileClose $2
    ExecWait 'powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$TEMP\vaultmtg-env-check.ps1"'
    Delete "$TEMP\vaultmtg-env-check.ps1"
  SkipEnvMismatch:

  ; Write a minimal daemon.json if one does not already exist.
  ; When CLOUD_API_URL is supplied at build time it is written into the skeleton
  ; so the daemon can reach the correct BFF from first launch.
  ; The daemon's first-run flow (issue #1643) will populate api_key on first
  ; launch; we write a skeleton so the file exists and is valid JSON from day one.
  IfFileExists "$APPDATA\vaultmtg\daemon.json" SkipWriteConfig WriteConfig
  WriteConfig:
    FileOpen  $0 "$APPDATA\vaultmtg\daemon.json" w
    FileWrite $0 '{$\n  "cloud_api_url": "${CLOUD_API_URL}",$\n  "api_key": ""$\n}$\n'
    FileClose $0
  SkipWriteConfig:

  ; Write uninstaller.
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  ; Remove legacy MTGA-Companion-Daemon scheduled task before registering the
  ; new VaultMTG-Daemon task. This is CRITICAL on upgrade — without it, two
  ; daemon processes run simultaneously after the first logon.
  ; /F silences "task not found" so this is a no-op on fresh installs.
  ExecWait 'schtasks /End /TN "MTGA-Companion-Daemon"'
  ExecWait 'schtasks /Delete /TN "MTGA-Companion-Daemon" /F'

  ; Register Scheduled Task at logon — no UAC (RunLevel LeastPrivilege).
  ; We use schtasks.exe because it is available on all Windows versions
  ; without requiring PowerShell or admin rights for per-user tasks.
  ; /RL LIMITEDACCESS maps to TaskPrincipalRunLevel LeastPrivilege — the task
  ; runs with the user's standard token, no elevation prompt, no UAC.
  ExecWait 'schtasks /Delete /TN "VaultMTG-Daemon" /F'
  ExecWait 'schtasks /Create /TN "VaultMTG-Daemon" /TR "\"$INSTDIR\vaultmtg-daemon.exe\" -config \"$APPDATA\vaultmtg\daemon.json\"" /SC ONLOGON /RL LIMITED /F'

  ; Start the daemon immediately without requiring a logoff/logon.
  ExecWait 'schtasks /Run /TN "VaultMTG-Daemon"'

  ; Create a Start-menu shortcut so the user can relaunch the daemon after
  ; exiting the tray without opening a terminal (AC5 / ticket #278).
  ; The shortcut launches the daemon binary directly; the Scheduled Task handles
  ; auto-start at logon — the shortcut is the manual-relaunch affordance.
  CreateDirectory "$SMPROGRAMS\VaultMTG"
  CreateShortCut "$SMPROGRAMS\VaultMTG\VaultMTG Daemon.lnk" "$INSTDIR\vaultmtg-daemon.exe"

  ; Post-install health check (issue #2131).
  ; Poll GET http://127.0.0.1:9001/health for up to 15 s (5 attempts x 3 s delay).
  ; A healthy response has HTTP 200 with a non-empty "account_id" field, confirming
  ; the daemon started and authenticated.  Exit code 1 from the PowerShell script
  ; causes the installer to report a failure so the user sees an error dialog rather
  ; than a false "Installation complete" screen.
  ;
  ; The health-check logic is written to a temporary .ps1 file rather than passed
  ; inline via -Command, because NSIS single-quoted strings terminate at the next
  ; literal single-quote character — so any PowerShell string literal containing
  ; a single-quote (e.g. Write-Error 'msg') would split the NSIS token and cause
  ; "ExecWait expects 1-2 parameters, got N" at compile time (issue #147 / PR #2131
  ; regression fix).  Writing to a file sidesteps NSIS/PowerShell quote-nesting
  ; entirely and keeps the script readable.
  ; Note: $$ is the NSIS escape for a literal dollar sign — necessary so NSIS does
  ; not attempt to interpolate the PowerShell variable names written into the .ps1.
  FileOpen  $1 "$TEMP\vaultmtg-health-check.ps1" w
  ; SKIP_HEALTH_CHECK=1 lets CI steps that use a stub binary (which never
  ; answers /health) bypass the health check without hanging.  The env var is
  ; only set in CI workflow steps — production installs never set it.
  FileWrite $1 'if ($$env:SKIP_HEALTH_CHECK -eq "1") { Write-Host "SKIP: health check bypassed (SKIP_HEALTH_CHECK=1)"; exit 0 }$\n'
  FileWrite $1 '$$maxAttempts = 5$\n'
  FileWrite $1 '$$delay = 3$\n'
  FileWrite $1 '$$healthy = $$false$\n'
  FileWrite $1 'for ($$i = 1; $$i -le $$maxAttempts; $$i++) {$\n'
  FileWrite $1 '    try {$\n'
  FileWrite $1 '        $$r = Invoke-WebRequest -Uri http://127.0.0.1:9001/health -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop$\n'
  FileWrite $1 '        if ($$r.StatusCode -eq 200) {$\n'
  FileWrite $1 '            $$j = $$r.Content | ConvertFrom-Json$\n'
  FileWrite $1 '            if ($$j.account_id) { $$healthy = $$true; break }$\n'
  FileWrite $1 '        }$\n'
  FileWrite $1 '    } catch {}$\n'
  FileWrite $1 '    if ($$i -lt $$maxAttempts) { Start-Sleep -Seconds $$delay }$\n'
  FileWrite $1 '}$\n'
  FileWrite $1 'if (-not $$healthy) {$\n'
  FileWrite $1 '    Write-Error "VaultMTG daemon did not start or authenticate within 15s. Check $$env:APPDATA\vaultmtg\ for logs."$\n'
  FileWrite $1 '    exit 1$\n'
  FileWrite $1 '}$\n'
  FileClose $1
  ExecWait 'powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$TEMP\vaultmtg-health-check.ps1"' $0
  Delete "$TEMP\vaultmtg-health-check.ps1"
  IntCmp $0 0 HealthOK HealthFail HealthFail
  HealthFail:
    ; IfSilent skips the modal dialog when the installer runs with /S (CI silent
    ; mode).  A modal MessageBox blocks indefinitely in non-interactive runners
    ; even when /S is active — use Abort-only in silent mode to avoid hangs.
    IfSilent +2
    MessageBox MB_OK|MB_ICONSTOP "VaultMTG daemon did not start correctly.$\n$\nThe daemon may have failed to start or has not yet authenticated.$\nCheck $APPDATA\vaultmtg\ for log files and try reinstalling."
    Abort "Daemon health check failed — installation incomplete."
  HealthOK:

SectionEnd

;----------------------------------------------------------------------
; Uninstaller section
;----------------------------------------------------------------------
Section "Uninstall"

  ; Stop and remove the VaultMTG-Daemon scheduled task.
  ExecWait 'schtasks /End /TN "VaultMTG-Daemon"'
  ExecWait 'schtasks /Delete /TN "VaultMTG-Daemon" /F'

  ; Also remove the legacy MTGA-Companion-Daemon task if still present.
  ExecWait 'schtasks /End /TN "MTGA-Companion-Daemon"'
  ExecWait 'schtasks /Delete /TN "MTGA-Companion-Daemon" /F'

  ; Remove Start-menu shortcut created during install (ticket #278).
  Delete "$SMPROGRAMS\VaultMTG\VaultMTG Daemon.lnk"
  RMDir  "$SMPROGRAMS\VaultMTG"

  ; Remove binary and uninstaller.
  Delete "$INSTDIR\vaultmtg-daemon.exe"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir  "$INSTDIR"

  ; Leave %APPDATA%\vaultmtg\daemon.json intact — the user may want to
  ; keep their config for a re-install.  A future "full uninstall" option can
  ; add a checkbox to remove config too.

SectionEnd
