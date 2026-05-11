; installer.nsi — NSIS per-user installer for the MTGA Companion daemon.
;
; Design constraints (ADR-011-C):
;   - Per-user install: binary to %LOCALAPPDATA%\MTGA-Companion\
;   - No UAC elevation — RequestExecutionLevel user
;   - No MSI, no WiX, no Windows Service
;   - Scheduled Task at logon using RunLevel LeastPrivilege (no UAC popup)
;   - Config file (daemon.json) written to %APPDATA%\mtga-companion\
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
Name              "MTGA Companion Daemon ${VERSION}"
; Output path is relative to where makensis is invoked (repo root when run via GoReleaser CI).
; GoReleaser extra_files glob expects services/daemon/vaultmtg-daemon-setup-*.exe.
OutFile           "services/daemon/vaultmtg-daemon-setup-${VERSION}.exe"

; Per-user install — no UAC prompt (RequestExecutionLevel user)
RequestExecutionLevel user

; Default install dir: %LOCALAPPDATA%\MTGA-Companion
InstallDir        "$LOCALAPPDATA\MTGA-Companion"

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

  ; Create config directory.
  CreateDirectory "$APPDATA\mtga-companion"

  ; Write a minimal daemon.json if one does not already exist.
  ; The daemon's first-run flow (issue #1643) will populate cloud_api_url and
  ; api_key on first launch; we write an empty skeleton so the file exists and
  ; is valid JSON from day one.
  IfFileExists "$APPDATA\mtga-companion\daemon.json" ConfigExists WriteConfig
  WriteConfig:
    FileOpen  $0 "$APPDATA\mtga-companion\daemon.json" w
    FileWrite $0 '{$\n  "cloud_api_url": "",$\n  "api_key": ""$\n}$\n'
    FileClose $0
  ConfigExists:

  ; Write uninstaller.
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  ; Register Scheduled Task at logon — no UAC (RunLevel LeastPrivilege).
  ; We use schtasks.exe because it is available on all Windows versions
  ; without requiring PowerShell or admin rights for per-user tasks.
  ; /RL LIMITEDACCESS maps to TaskPrincipalRunLevel LeastPrivilege — the task
  ; runs with the user's standard token, no elevation prompt, no UAC.
  ExecWait 'schtasks /Delete /TN "MTGA-Companion-Daemon" /F'
  ExecWait 'schtasks /Create \
    /TN "MTGA-Companion-Daemon" \
    /TR "\"$INSTDIR\vaultmtg-daemon.exe\" -config \"$APPDATA\mtga-companion\daemon.json\"" \
    /SC ONLOGON \
    /RL LIMITED \
    /F'

  ; Start the daemon immediately without requiring a logoff/logon.
  ExecWait 'schtasks /Run /TN "MTGA-Companion-Daemon"'

SectionEnd

;----------------------------------------------------------------------
; Uninstaller section
;----------------------------------------------------------------------
Section "Uninstall"

  ; Stop and remove the scheduled task.
  ExecWait 'schtasks /End /TN "MTGA-Companion-Daemon"'
  ExecWait 'schtasks /Delete /TN "MTGA-Companion-Daemon" /F'

  ; Remove binary and uninstaller.
  Delete "$INSTDIR\vaultmtg-daemon.exe"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir  "$INSTDIR"

  ; Leave %APPDATA%\mtga-companion\daemon.json intact — the user may want to
  ; keep their config for a re-install.  A future "full uninstall" option can
  ; add a checkbox to remove config too.

SectionEnd
