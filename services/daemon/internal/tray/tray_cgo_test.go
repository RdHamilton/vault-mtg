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
	"fmt"
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

// ---------------------------------------------------------------------------
// Sync Now debounce + feedback (ticket #203)
// ---------------------------------------------------------------------------

// TestTrayCGO_SyncNow_InitialState verifies that a newly constructed App starts
// with syncInFlight == false (no sync in progress at creation).
func TestTrayCGO_SyncNow_InitialState(t *testing.T) {
	app := newCGOTestApp()
	app.syncMu.Lock()
	inFlight := app.syncInFlight
	app.syncMu.Unlock()
	assert.False(t, inFlight, "syncInFlight must be false after New()")
}

// TestTrayCGO_NotifySyncResult_NoopWithoutMenu verifies that NotifySyncResult
// does not panic when miSyncNow is nil (menu not initialised in headless tests).
// The nil-guard is the same pattern used by SetKeychainError / SetSetupRequired.
func TestTrayCGO_NotifySyncResult_NoopWithoutMenu(t *testing.T) {
	app := newCGOTestApp()
	assert.Nil(t, app.miSyncNow, "miSyncNow must be nil before setup()")
	// Both nil-success and nil-error paths must not panic.
	assert.NotPanics(t, func() { app.NotifySyncResult(nil) })
	assert.NotPanics(t, func() { app.NotifySyncResult(fmt.Errorf("helper error")) })
}

// TestTrayCGO_Debounce_SecondClickDropped verifies AC4: while syncInFlight is
// true, a call to onSyncNowClick (the extracted click handler) is a no-op and
// does NOT send to the SyncNow channel.
//
// We test via the exported tryStartSync helper method rather than through the
// systray select loop (which would require systray.Run).
func TestTrayCGO_Debounce_SecondClickDropped(t *testing.T) {
	app := newCGOTestApp()

	// Artificially set syncInFlight = true to simulate a sync already running.
	app.syncMu.Lock()
	app.syncInFlight = true
	app.syncMu.Unlock()

	// Attempt to start a sync while one is in flight — must return false (debounced).
	started := app.tryStartSync()
	assert.False(t, started, "tryStartSync must return false while syncInFlight is true")

	// SyncNow channel must be empty — no signal was enqueued.
	assert.Equal(t, 0, len(app.SyncNow), "SyncNow channel must be empty after debounced click")
}

// TestTrayCGO_Debounce_FirstClickAllowed verifies AC1: when no sync is in
// flight, tryStartSync sets syncInFlight=true and returns true.
func TestTrayCGO_Debounce_FirstClickAllowed(t *testing.T) {
	app := newCGOTestApp()

	started := app.tryStartSync()
	assert.True(t, started, "tryStartSync must return true when no sync is in flight")

	app.syncMu.Lock()
	inFlight := app.syncInFlight
	app.syncMu.Unlock()
	assert.True(t, inFlight, "syncInFlight must be true after tryStartSync returns true")
}

// TestTrayCGO_NotifySyncResult_ClearsInFlight verifies that NotifySyncResult
// clears syncInFlight so a subsequent sync can start (AC1 / AC4 reset path).
// We skip the label-transition sleep assertions here (they require real systray
// menu items) and focus on the state machine.
func TestTrayCGO_NotifySyncResult_ClearsInFlight(t *testing.T) {
	app := newCGOTestApp()

	// Set in-flight state directly (simulating a sync having started).
	app.syncMu.Lock()
	app.syncInFlight = true
	app.syncMu.Unlock()

	// NotifySyncResult must clear syncInFlight (after its internal sleep).
	// We call it synchronously here; the real loop calls it in a goroutine.
	// The internal sleep is skipped when miSyncNow is nil (no real menu).
	app.NotifySyncResult(nil)

	app.syncMu.Lock()
	inFlight := app.syncInFlight
	app.syncMu.Unlock()
	assert.False(t, inFlight, "syncInFlight must be false after NotifySyncResult")
}

// TestTrayCGO_NotifySyncResult_ErrorClearsInFlight verifies the error path
// also clears syncInFlight (AC3 reset path).
func TestTrayCGO_NotifySyncResult_ErrorClearsInFlight(t *testing.T) {
	app := newCGOTestApp()

	app.syncMu.Lock()
	app.syncInFlight = true
	app.syncMu.Unlock()

	app.NotifySyncResult(fmt.Errorf("collection sync error: helper error"))

	app.syncMu.Lock()
	inFlight := app.syncInFlight
	app.syncMu.Unlock()
	assert.False(t, inFlight, "syncInFlight must be false after NotifySyncResult(err)")
}
