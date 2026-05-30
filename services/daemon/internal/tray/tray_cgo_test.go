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
	return New("https://app.vaultmtg.app", func(string) error { return nil }, func() {})
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
	app := New("https://app.vaultmtg.app", nil, func() { called = true })
	// Simulate the onExit callback path without calling systray.Quit.
	if app.onQuit != nil {
		app.onQuit()
	}
	assert.True(t, called)
}
