//go:build windows

package localapi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runPlatformUninstall performs the Windows uninstall steps. Mirrors
// services/daemon/install/windows/uninstall.ps1 for the user-scoped
// pieces (stopping + unregistering the scheduled task) but reimplemented
// in-process so we don't need PowerShell to be reachable through cmd
// (it usually is, but we don't want to depend on PATH).
func runPlatformUninstall(purge bool) (string, error) {
	const taskName = "MTGA-Companion-Daemon"

	if err := runSchtasks([]string{"/End", "/TN", taskName}); err != nil {
		return "", fmt.Errorf("schtasks /End %s: %w", taskName, err)
	}
	if err := runSchtasks([]string{"/Delete", "/TN", taskName, "/F"}); err != nil {
		return "", fmt.Errorf("schtasks /Delete %s: %w", taskName, err)
	}

	if purge {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA env var is empty; cannot purge config")
		}
		configDir := filepath.Join(appData, "MTGA-Companion")
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
