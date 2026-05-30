//go:build cgo

package tray

// What this catches: tray_test.go testing the !cgo stub instead of the real systray UI — see ADR-041 G2
//
// This file is compiled ONLY with CGO_ENABLED=1. It exercises the real tray.go
// (//go:build cgo) code path — New(), SetStatus(), SetWaitingForArena(),
// SetLastSync(), and the non-blocking channel sends — without invoking
// systray.Run() or any Cocoa/AppKit rendering. A headless macOS-latest CI
// runner can run this freely.
//
// Seam left for #221 (otool+nm binary-contract check): this file proves the
// cgo build compiles and links; the binary-contract step (assert Cocoa/AppKit
// symbols present in the release binary) slots in as a separate CI step that
// runs AFTER the binary is built in daemon.yml. No changes to this file are
// needed to extend it.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newCGOTestApp returns an App constructed via the real cgo New(). No systray
// menu is initialised; all menu-item fields remain nil. Public state-mutation
// methods must guard against nil menu pointers (they do — see tray.go).
func newCGOTestApp() *App {
	return New("https://app.vaultmtg.app", "v0.3.4-test", func(string) error { return nil }, func() {})
}

// TestTrayCGO_RealTrayAPIReachable is the primary AC1 test. It constructs
// the real tray.App and drives the public API methods that the daemon calls
// during normal operation. No systray.Run invocation, no menubar render.
func TestTrayCGO_RealTrayAPIReachable(t *testing.T) {
	app := newCGOTestApp()

	// AC1: New() produces a non-nil App with channels initialised.
	assert.NotNil(t, app)
	assert.NotNil(t, app.SyncNow)
	assert.NotNil(t, app.GrantAccess)
	assert.NotNil(t, app.TryAgain)
	assert.NotNil(t, app.RetrySetup)

	// SetStatus — real tray.go guards against nil miStatus pointer.
	app.SetStatus(StatusConnected)
	assert.Equal(t, StatusConnected, app.status)

	// SetWaitingForArena — delegates to SetStatus.
	app.SetWaitingForArena(true)
	assert.Equal(t, StatusWaitingForArena, app.status)

	app.SetWaitingForArena(false)
	assert.Equal(t, StatusConnected, app.status)

	// SetLastSync — real tray.go guards against nil miLastSync.
	ts := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	app.SetLastSync(ts)
	assert.Equal(t, ts, app.lastSync)

	// Non-blocking SyncNow channel exercise (AC1).
	// Send once — must succeed (buffered cap=1).
	select {
	case app.SyncNow <- struct{}{}:
	default:
		t.Fatal("SyncNow channel was full before first send — unexpected")
	}
	// Second send must drop without blocking.
	select {
	case app.SyncNow <- struct{}{}:
	default: // dropped — correct
	}
	assert.Equal(t, 1, len(app.SyncNow))

	// Drain.
	<-app.SyncNow

	// TryAgain channel — non-blocking.
	select {
	case app.TryAgain <- struct{}{}:
	default:
		t.Fatal("TryAgain channel was full before first send — unexpected")
	}
	select {
	case app.TryAgain <- struct{}{}:
	default:
	}
	assert.Equal(t, 1, len(app.TryAgain))
	<-app.TryAgain

	// RetrySetup channel — non-blocking.
	select {
	case app.RetrySetup <- struct{}{}:
	default:
		t.Fatal("RetrySetup channel was full before first send — unexpected")
	}
	select {
	case app.RetrySetup <- struct{}{}:
	default:
	}
	assert.Equal(t, 1, len(app.RetrySetup))
	<-app.RetrySetup
}

// TestTrayCGO_SetKeychainError_NoopWithoutMenu verifies the real tray.go
// nil-guard on miTryAgain (menu not initialised in headless tests).
func TestTrayCGO_SetKeychainError_NoopWithoutMenu(t *testing.T) {
	app := newCGOTestApp()
	assert.NotPanics(t, func() { app.SetKeychainError(true) })
	assert.Equal(t, StatusKeychainError, app.status)
	assert.NotPanics(t, func() { app.SetKeychainError(false) })
}

// TestTrayCGO_SetSetupRequired_NoopWithoutMenu verifies the real tray.go
// nil-guard on miRetrySetup (menu not initialised in headless tests).
func TestTrayCGO_SetSetupRequired_NoopWithoutMenu(t *testing.T) {
	app := newCGOTestApp()
	assert.NotPanics(t, func() { app.SetSetupRequired(true) })
	assert.Equal(t, StatusSetupRequired, app.status)
	assert.NotPanics(t, func() { app.SetSetupRequired(false) })
}

// TestTrayCGO_SetHelperInstalled_NoopWithoutMenu verifies the real tray.go
// nil-guard on miGrantAccess / miSyncNow (menu not initialised).
func TestTrayCGO_SetHelperInstalled_NoopWithoutMenu(t *testing.T) {
	app := newCGOTestApp()
	assert.NotPanics(t, func() { app.SetHelperInstalled(true) })
	assert.NotPanics(t, func() { app.SetHelperInstalled(false) })
}

// TestTrayCGO_QuitCallback exercises the onQuit path without invoking
// systray.Quit() (which requires systray.Run to have been called first).
func TestTrayCGO_QuitCallback(t *testing.T) {
	called := false
	app := New("https://app.vaultmtg.app", "v0.3.4-test", nil, func() { called = true })
	// Simulate the onExit callback path without calling systray.Quit.
	if app.onQuit != nil {
		app.onQuit()
	}
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// About / Check for Updates (ticket #2156)
// ---------------------------------------------------------------------------

// TestTrayCGO_New_StoresVersion verifies that New() stores the version string
// so it can be rendered in the "About" menu item label.
func TestTrayCGO_New_StoresVersion(t *testing.T) {
	app := New("https://app.vaultmtg.app", "v0.3.4", func(string) error { return nil }, func() {})
	assert.Equal(t, "v0.3.4", app.version)
}

// TestTrayCGO_New_DefaultVersionDev verifies that an empty version string is
// stored as-is (callers pass "dev" for local builds; no clamping here).
func TestTrayCGO_New_DefaultVersionDev(t *testing.T) {
	app := New("https://app.vaultmtg.app", "dev", func(string) error { return nil }, func() {})
	assert.Equal(t, "dev", app.version)
}

// TestTrayCGO_AboutItem_NoopWithoutMenu verifies that the miAbout field is nil
// before setup() has run (tests never call systray.Run) and that the App does
// not panic when version is set but no real menu exists.
func TestTrayCGO_AboutItem_NoopWithoutMenu(t *testing.T) {
	app := newCGOTestApp()
	// miAbout is nil pre-setup — must not panic when accessed indirectly.
	assert.Nil(t, app.miAbout)
	assert.Equal(t, "v0.3.4-test", app.version)
}

// TestTrayCGO_CheckForUpdatesItem_NoopWithoutMenu verifies that miCheckForUpdates
// is nil before setup() has run and that no panic occurs.
func TestTrayCGO_CheckForUpdatesItem_NoopWithoutMenu(t *testing.T) {
	app := newCGOTestApp()
	assert.Nil(t, app.miCheckForUpdates)
}

// TestTrayCGO_OpenURLCalled_OnCheckForUpdates verifies that openURL is invoked
// with the GitHub Releases URL when the check-for-updates action is triggered.
// We test this via openCheckForUpdates() which is the extracted helper the loop
// goroutine calls on click.
func TestTrayCGO_OpenURLCalled_OnCheckForUpdates(t *testing.T) {
	var gotURL string
	app := New("https://app.vaultmtg.app", "v0.3.4", func(u string) error {
		gotURL = u
		return nil
	}, func() {})

	app.openCheckForUpdates()

	assert.Equal(t, "https://github.com/RdHamilton/vault-mtg/releases?q=daemon", gotURL)
}

// TestTrayCGO_CheckForUpdates_URLIsVaultMTGRepo verifies the exact URL constant
// to prevent accidental references to the legacy repo slug (pre-rename).
func TestTrayCGO_CheckForUpdates_URLIsVaultMTGRepo(t *testing.T) {
	var gotURL string
	app := New("https://app.vaultmtg.app", "dev", func(u string) error {
		gotURL = u
		return nil
	}, func() {})

	app.openCheckForUpdates()

	assert.Contains(t, gotURL, "RdHamilton/vault-mtg", "URL must reference the vault-mtg repo")
	assert.NotContains(t, gotURL, "mtga-companion", "legacy repo slug must not appear in Check for Updates URL")
}
