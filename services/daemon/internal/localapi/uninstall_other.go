//go:build !darwin && !windows

package localapi

import "fmt"

// runPlatformUninstall on unsupported platforms — Linux has no installer
// today, so there's nothing to undo. Surface a clear 4xx-shaped error to
// the SPA rather than pretending to succeed.
func runPlatformUninstall(_ bool) (string, error) {
	return "", fmt.Errorf("automatic uninstall not supported on this platform; remove the daemon binary manually")
}
