//go:build darwin

package localapi

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runPlatformUninstall performs the macOS uninstall steps. Keep the
// same plist label the installer registers under (com.mtga-companion.daemon
// per services/daemon/install/macos/uninstall.sh).
//
// Order is `launchctl unload` first, then plist removal. Unloading is
// what stops the running launchd-supervised process and deregisters the
// job; removing the plist after prevents it from coming back at next
// login. The reverse order leaves a window where the job is orphaned in
// launchd's in-memory state.
func runPlatformUninstall(purge bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	plistLabel := "com.mtga-companion.daemon"
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")

	// `launchctl unload` exits non-zero in two distinct cases:
	//   - job was never loaded / plist already gone  → treat as no-op
	//   - real failure (permission, malformed plist) → propagate
	// We disambiguate by inspecting stderr rather than the exit code,
	// because launchctl uses the same non-zero code for both.
	cmd := exec.Command("launchctl", "unload", "-w", plistPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		out := strings.ToLower(stderr.String())
		switch {
		case strings.Contains(out, "no such file"),
			strings.Contains(out, "not loaded"),
			strings.Contains(out, "could not find"):
			// Idempotent — the job wasn't registered. Continue.
		default:
			return "", fmt.Errorf("launchctl unload %s: %w (stderr: %s)", plistPath, err, strings.TrimSpace(stderr.String()))
		}
	}

	// Plist removal is idempotent — silence the missing-file case,
	// propagate everything else.
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove plist %s: %w", plistPath, err)
	}

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
