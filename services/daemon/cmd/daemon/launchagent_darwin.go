//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// plistLabel is the LaunchAgent label registered by the installer
// (services/daemon/install/macos/pkg/postinstall and install/macos/uninstall.sh).
// It must stay in sync with those scripts.
// ADR-022 Phase 2: renamed from "com.mtga-companion.daemon" to "com.vaultmtg.daemon".
const plistLabel = "com.vaultmtg.daemon"

// plistLabelLegacy is the pre-rename LaunchAgent label.  The installer detects
// and unloads this label before registering plistLabel so two daemon instances
// never run simultaneously (ADR-022 Constraint 1).
const plistLabelLegacy = "com.mtga-companion.daemon"

// appBundlePath is the canonical path of the VaultMTG launcher app bundle
// placed by the .pkg installer. ADR-036 I-4 / I-8: single source of truth
// for this path — referenced here and in build-pkg.sh / uninstall.sh.
const appBundlePath = "/Applications/VaultMTG.app"

// launchdTarget returns the launchctl service target for the current user.
// Format: gui/<uid>/<label>
func launchdTarget(label string) string {
	return fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
}

// stopLaunchAgent fully unregisters the VaultMTG daemon from launchd so the
// process does not restart on the next user login. This implements the
// "Quit means quit" semantic: the daemon is removed from launchd's list
// entirely, not just stopped. The user can reopen the daemon via
// /Applications/VaultMTG.app (ADR-036 I-8, ticket #278).
//
// Uses `launchctl bootout` instead of the former `launchctl stop` because
// `stop` only sends SIGTERM and suppresses KeepAlive for the current session
// but leaves the agent registered — it would restart on next login, which
// contradicts the user's explicit Quit intent.
//
// Failure is non-fatal: if launchctl is unavailable or the job is not loaded,
// the error is logged and the quit sequence continues.
//
// ADR-022 Phase 2: also attempts to boot out the legacy label (plistLabelLegacy)
// in case an upgrade scenario left the old registration active. This is a
// best-effort no-op on machines that have already been migrated.
func stopLaunchAgent() {
	target := launchdTarget(plistLabel)
	cmd := exec.Command("launchctl", "bootout", target)
	if err := cmd.Run(); err != nil {
		log.Printf("[vaultmtg-daemon] launchctl bootout %s: %v (non-fatal)", target, err)
	}

	// Best-effort: boot out any running instance under the legacy label.
	// Silently ignore errors — a fully migrated machine has no legacy label.
	_ = exec.Command("launchctl", "bootout", launchdTarget(plistLabelLegacy)).Run()
}

// startLaunchAgent re-registers and starts the VaultMTG daemon LaunchAgent.
// This is the symmetric counterpart to stopLaunchAgent, called when the daemon
// is invoked directly (e.g., from a terminal) rather than via VaultMTG.app.
// The normal relaunch path is VaultMTG.app → launchctl enable + bootstrap;
// this function provides a programmatic alternative for the same effect.
//
// Steps:
//  1. launchctl enable — clears the disabled flag that bootout may have set.
//  2. launchctl bootstrap — re-registers the plist and starts the daemon.
//
// Both steps are best-effort. The daemon may already be bootstrapped (e.g.,
// first launch before any Quit); errors are logged non-fatally.
func startLaunchAgent() {
	plistPath := fmt.Sprintf("%s/Library/LaunchAgents/%s.plist",
		os.Getenv("HOME"), plistLabel)
	target := launchdTarget(plistLabel)
	userDomain := fmt.Sprintf("gui/%d", os.Getuid())

	// Step 1: clear any disabled flag from a prior bootout.
	if out, err := exec.Command("launchctl", "enable", target).CombinedOutput(); err != nil {
		log.Printf("[vaultmtg-daemon] launchctl enable %s: %v %s (non-fatal)", target, err, out)
	}

	// Step 2: bootstrap the plist so launchd manages the agent going forward.
	if out, err := exec.Command("launchctl", "bootstrap", userDomain, plistPath).CombinedOutput(); err != nil {
		log.Printf("[vaultmtg-daemon] launchctl bootstrap %s %s: %v %s (non-fatal)",
			userDomain, plistPath, err, out)
	}
}
