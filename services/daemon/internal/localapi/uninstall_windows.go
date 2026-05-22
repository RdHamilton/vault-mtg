//go:build windows

// NOTE: This file intentionally references the legacy "MTGA-Companion-Daemon"
// string. That is the actual Windows Task Scheduler task name created by
// pre-rename installs; the uninstall path must delete it verbatim to clean up
// orphaned legacy tasks during an upgrade. It is NOT a stale repo reference and
// must not be renamed. This file is excluded from the stale-repo-grep gate in
// .github/workflows/process-gates.yml.

package localapi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// schtasksRun is the package-level hook used to run schtasks.exe commands.
// It is a function variable so tests can inject a stub without touching the
// host machine's Task Scheduler.  Production code never reassigns this.
var schtasksRun = runSchtasksExec

// runPlatformUninstall performs the Windows uninstall steps. Mirrors
// services/daemon/install/windows/uninstall.ps1 for the user-scoped
// pieces (stopping + unregistering the scheduled task) but reimplemented
// in-process so we don't need PowerShell to be reachable through cmd
// (it usually is, but we don't want to depend on PATH).
//
// It removes both VaultMTG-Daemon (the current name) AND the legacy
// MTGA-Companion-Daemon task so that an upgrade scenario never leaves an
// orphaned old task running alongside the new one.
func runPlatformUninstall(purge bool) (string, error) {
	const (
		taskName       = "VaultMTG-Daemon"
		legacyTaskName = "MTGA-Companion-Daemon"
	)

	// Stop and remove the current VaultMTG-Daemon task.
	if err := schtasksRun([]string{"/End", "/TN", taskName}); err != nil {
		return "", fmt.Errorf("schtasks /End %s: %w", taskName, err)
	}
	if err := schtasksRun([]string{"/Delete", "/TN", taskName, "/F"}); err != nil {
		return "", fmt.Errorf("schtasks /Delete %s: %w", taskName, err)
	}

	// Also stop and remove the legacy MTGA-Companion-Daemon task if it is
	// still registered (upgrade scenario: the old task was never cleaned up).
	// Both operations are idempotent — runSchtasks swallows "task not found".
	if err := schtasksRun([]string{"/End", "/TN", legacyTaskName}); err != nil {
		return "", fmt.Errorf("schtasks /End %s: %w", legacyTaskName, err)
	}
	if err := schtasksRun([]string{"/Delete", "/TN", legacyTaskName, "/F"}); err != nil {
		return "", fmt.Errorf("schtasks /Delete %s: %w", legacyTaskName, err)
	}

	if purge {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA env var is empty; cannot purge config")
		}
		configDir := filepath.Join(appData, "vaultmtg")
		if err := os.RemoveAll(configDir); err != nil {
			return "", fmt.Errorf("remove config dir %s: %w", configDir, err)
		}
	}

	msg := "Daemon stopped and removed from Task Scheduler. Use Add/Remove Programs (or delete the install directory) to remove the binary."
	if purge {
		msg = "Daemon stopped, removed from Task Scheduler, and config wiped. Use Add/Remove Programs (or delete the install directory) to remove the binary."
	}
	return msg, nil
}

// runSchtasksExec runs schtasks.exe with the given args.  It is the
// production implementation of schtasksRun.
func runSchtasksExec(args []string) error {
	return runSchtasks(args)
}

// runSchtasks runs schtasks with the given args. Idempotent failure
// modes — task missing or already stopped — are swallowed; real
// failures (permission denied, schtasks unavailable) propagate.
//
// Uses CombinedOutput so we can sniff schtasks' messages: it returns
// non-zero in both the "task not found" case and the "permission
// denied" case but with different stderr, and we want only the latter
// to surface as an error.
func runSchtasks(args []string) error {
	out, err := exec.Command("schtasks", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	lower := strings.ToLower(string(out))
	switch {
	case strings.Contains(lower, "the system cannot find the file"),
		strings.Contains(lower, "task does not exist"),
		strings.Contains(lower, "specified task name"),
		strings.Contains(lower, "not running"),
		strings.Contains(lower, "is not currently running"):
		// Idempotent — task missing or already stopped. No-op.
		return nil
	}
	return fmt.Errorf("schtasks %s failed: %w (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
}
