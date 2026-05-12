//go:build !darwin && !windows

package localapi

// runPlatformUninstall on unsupported platforms — Linux has no installer
// today, so there's nothing to undo. Returns ErrUnsupportedPlatform so the
// handler maps it to 400, signalling the SPA to fall back to the manual-
// removal docs.
func runPlatformUninstall(_ bool) (string, error) {
	return "", ErrUnsupportedPlatform
}
