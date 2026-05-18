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

  ; Write a minimal daemon.json if one does not already exist.
  ; The daemon's first-run flow (issue #1643) will populate cloud_api_url and
  ; api_key on first launch; we write an empty skeleton so the file exists and
  ; is valid JSON from day one.
  IfFileExists "$APPDATA\vaultmtg\daemon.json" SkipWriteConfig WriteConfig
  WriteConfig:
    FileOpen  $0 "$APPDATA\vaultmtg\daemon.json" w
    FileWrite $0 '{$\n  "cloud_api_url": "",$\n  "api_key": ""$\n}$\n'
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

  ; Remove binary and uninstaller.
  Delete "$INSTDIR\vaultmtg-daemon.exe"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir  "$INSTDIR"

  ; Leave %APPDATA%\vaultmtg\daemon.json intact — the user may want to
  ; keep their config for a re-install.  A future "full uninstall" option can
  ; add a checkbox to remove config too.

SectionEnd
