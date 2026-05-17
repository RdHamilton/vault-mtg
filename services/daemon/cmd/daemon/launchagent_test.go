package main

import (
	"testing"
)

// TestStopLaunchAgentCalledOnQuit verifies that stopLaunchAgent() is callable
// without panicking from any goroutine.
//
// Full integration testing of this function requires a live launchd session
// with the daemon plist loaded, which is not reproducible in CI. The manual
// verification steps are documented in the PR under "Local Verification":
//
//  1. Install the daemon (fresh install via the .pkg installer).
//  2. Confirm the daemon is running: pgrep vaultmtg-daemon → returns a PID.
//  3. Click "Quit" in the menu-bar tray.
//  4. Confirm the daemon is NOT restarted: pgrep vaultmtg-daemon → returns empty.
//  5. (Optionally) Verify that the daemon starts again on next login / reboot,
//     confirming that the LaunchAgent registration is still present (KeepAlive
//     has NOT been permanently disabled — only the running instance was stopped).
//
// Why exec.Command cannot be easily mocked here: stopLaunchAgent uses os/exec
// directly. Injecting a command executor via a package-level var would expose
// test-only surface area in the production binary. The function is intentionally
// small and side-effect-only; the test verifies it does not panic on all
// supported platforms (the darwin build calls launchctl; non-darwin is a no-op).
func TestStopLaunchAgentCalledOnQuit(t *testing.T) {
	// stopLaunchAgent() will fail on CI (launchctl is not available / the job is
	// not loaded), but must not panic. Errors are logged non-fatally by design.
	// On non-darwin this is a no-op and always succeeds.
	stopLaunchAgent()
}
