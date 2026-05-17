//go:build darwin

package main

import (
	"log"
	"os/exec"
)

// plistLabel is the LaunchAgent label registered by the installer
// (services/daemon/install/macos/pkg/postinstall and install/macos/uninstall.sh).
// It must stay in sync with those scripts.
const plistLabel = "com.mtga-companion.daemon"

// stopLaunchAgent tells launchd to stop the service so it does not restart
// after the process exits. This must be called before systray.Quit() / cancel().
//
// `launchctl stop` sends SIGTERM to the process and marks the job as stopped
// intentionally, preventing launchd from immediately respawning it per the
// KeepAlive=true directive in the plist. The agent is still registered and will
// restart on the next user login — this is the correct "Quit" semantic (stop
// now, not never-start-again).
//
// Failure is non-fatal: if launchctl is unavailable or the job is not loaded,
// the error is logged and the quit sequence continues.
func stopLaunchAgent() {
	cmd := exec.Command("launchctl", "stop", plistLabel)
	if err := cmd.Run(); err != nil {
		log.Printf("[mtga-daemon] launchctl stop %s: %v (non-fatal)", plistLabel, err)
	}
}
