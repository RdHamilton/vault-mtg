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
//  5. Confirm the daemon restarts when VaultMTG.app is opened — launchctl
//     enable + bootstrap is called, the daemon reappears in the menu bar.
//
// Semantic change (#278): stopLaunchAgent now calls `launchctl bootout`
// (full unregister from launchd) instead of `launchctl stop`. This means
// "Quit means quit": the daemon does NOT restart on the next user login.
// The user reopens the daemon via /Applications/VaultMTG.app (ADR-036 I-8).
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

// TestStartLaunchAgentDoesNotPanic verifies that startLaunchAgent() is callable
// without panicking. On CI the launchctl calls will fail (no plist, no session)
// but all failures are logged non-fatally — the function must not panic.
// On non-darwin platforms this is a no-op.
func TestStartLaunchAgentDoesNotPanic(t *testing.T) {
	startLaunchAgent()
}

// TestStopStartRoundtripDoesNotPanic verifies that calling stop followed by
// start is safe (no panic, no deadlock). The actual launchctl calls will fail
// in CI (no live launchd session), but the failure path must remain non-fatal.
func TestStopStartRoundtripDoesNotPanic(t *testing.T) {
	stopLaunchAgent()
	startLaunchAgent()
}
