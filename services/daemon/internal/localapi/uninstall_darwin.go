//go:build darwin

package localapi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// runPlatformUninstall performs the macOS uninstall steps. Keep the
// same plist label the installer registers under (com.mtga-companion.daemon
// per services/daemon/install/macos/uninstall.sh).
func runPlatformUninstall(purge bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	plistLabel := "com.mtga-companion.daemon"
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")

	// Remove the plist FIRST so launchctl can't auto-relaunch the
	// daemon after we exit. `os.Remove` returns nil if the file is
	// already gone, which is the idempotent behaviour we want.
	if _, statErr := os.Stat(plistPath); statErr == nil {
		if err := os.Remove(plistPath); err != nil {
			return "", fmt.Errorf("remove plist %s: %w", plistPath, err)
		}
	}

	// Unload the launchctl job. `launchctl unload` returns non-zero
	// when the job was never loaded, which is fine; we treat it as a
	// no-op rather than an error.
	_ = exec.Command("launchctl", "unload", "-w", plistPath).Run()

	if purge {
		configDir := filepath.Join(home, ".config", "mtga-companion")
		if err := os.RemoveAll(configDir); err != nil {
			return "", fmt.Errorf("remove config dir %s: %w", configDir, err)
		}
	}

	msg := "Daemon stopped and removed from launchd. Drag VaultMTG to the Trash to remove the app bundle."
	if purge {
		msg = "Daemon stopped, removed from launchd, and config wiped. Drag VaultMTG to the Trash to remove the app bundle."
	}
	return msg, nil
}
