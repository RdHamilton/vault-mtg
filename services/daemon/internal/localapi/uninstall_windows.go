//go:build windows

package localapi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// runPlatformUninstall performs the Windows uninstall steps. Mirrors
// services/daemon/install/windows/uninstall.ps1 for the user-scoped
// pieces (stopping + unregistering the scheduled task) but reimplemented
// in-process so we don't need PowerShell to be reachable through cmd
// (it usually is, but we don't want to depend on PATH).
func runPlatformUninstall(purge bool) (string, error) {
	const taskName = "MTGA-Companion-Daemon"

	// Stop the scheduled task. SilentlyContinue equivalent — we ignore
	// the exit code; if the task isn't running this is a no-op.
	_ = exec.Command("schtasks", "/End", "/TN", taskName).Run()

	// Unregister the task. /F = no prompt. Idempotent: returns non-zero
	// if the task didn't exist, which we tolerate.
	_ = exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

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
