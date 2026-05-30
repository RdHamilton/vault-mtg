//go:build cgo

// Package tray manages the system tray (menu bar) icon and menu for the
// VaultMTG daemon. systray.Run must be called on the main OS thread; callers
// must invoke App.Run from main() and start the daemon event loop inside the
// onReady callback.
package tray

import (
	_ "embed"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

//go:embed assets/icon.png
var iconData []byte

// Status represents the daemon's connection state shown in the menu bar.
type Status int

const (
	StatusStarting Status = iota
	StatusConnected
	StatusWaitingForArena
	StatusError
	StatusKeychainError
	StatusSetupRequired
)

func (s Status) label() string {
	switch s {
	case StatusConnected:
		return "● Connected"
	case StatusWaitingForArena:
		return "◌ Waiting for Arena..."
	case StatusError:
		return "✕ Error — check logs"
	case StatusKeychainError:
		return "Keychain unavailable — unlock to continue"
	case StatusSetupRequired:
		return "⚠ Setup required — auth failed"
	default:
		return "◌ Starting..."
	}
}

// App manages the tray icon, menu items, and status state.
type App struct {
	appURL  string
	version string
	openURL func(string) error
	onQuit  func()

	// protected by single-goroutine access after setup()
	status          Status
	lastSync        time.Time
	helperInstalled bool

	miStatus          *systray.MenuItem
	miAbout           *systray.MenuItem
	miCheckForUpdates *systray.MenuItem
	miLastSync        *systray.MenuItem
	miSyncNow         *systray.MenuItem
	miGrantAccess     *systray.MenuItem
	miTryAgain        *systray.MenuItem
	miRetrySetup      *systray.MenuItem
	miOpenApp         *systray.MenuItem
	miQuit            *systray.MenuItem

	// syncMu guards syncInFlight.
	syncMu sync.Mutex
	// syncInFlight is true while a Sync Now operation is in progress.
	// Concurrent clicks are dropped until the current sync completes (AC4).
	syncInFlight bool

	// SyncNow is signalled when the user clicks "Sync Now".
	SyncNow chan struct{}
	// GrantAccess is signalled when the user clicks "Grant Access".
	GrantAccess chan struct{}
	// TryAgain is signalled when the user clicks "Try Again" (keychain retry).
	TryAgain chan struct{}
	// RetrySetup is signalled when the user clicks "Retry Setup…". The handler
	// opens https://vaultmtg.app/setup in the browser and re-runs the PKCE flow.
	// Buffered cap=1 so a second click before the first is handled is dropped.
	RetrySetup chan struct{}
}

// New creates an App. appURL is opened when "Open VaultMTG" is clicked.
// version is the daemon build version (injected via -ldflags -X main.Version=<ver>;
// defaults to "dev" for local builds) and is displayed in the "About" menu item.
// openURL is the platform open-browser function. onQuit is called when the
// tray exits (Quit clicked or process terminated).
func New(appURL, version string, openURL func(string) error, onQuit func()) *App {
	return &App{
		appURL:      appURL,
		version:     version,
		openURL:     openURL,
		onQuit:      onQuit,
		status:      StatusStarting,
		SyncNow:     make(chan struct{}, 1),
		GrantAccess: make(chan struct{}, 1),
		TryAgain:    make(chan struct{}, 1),
		RetrySetup:  make(chan struct{}, 1),
	}
}

// Run blocks the calling goroutine (must be the main OS thread on macOS).
// onReady is called after the menu bar icon is ready; start the daemon event
// loop inside it (in a new goroutine).
func (a *App) Run(onReady func()) {
	systray.Run(func() {
		a.setup()
		if onReady != nil {
			onReady()
		}
		go a.loop()
	}, func() {
		if a.onQuit != nil {
			a.onQuit()
		}
	})
}

// Quit tears down the tray icon and unblocks Run. Safe to call from any goroutine.
func (a *App) Quit() {
	systray.Quit()
}

// SetStatus updates the status label in the menu. Safe to call from any goroutine.
func (a *App) SetStatus(s Status) {
	a.status = s
	if a.miStatus != nil {
		a.miStatus.SetTitle(s.label())
	}
}

// SetHelperInstalled shows or hides the "Grant Access" menu item.
// Call with true once the helper is confirmed running; false shows the install prompt.
func (a *App) SetHelperInstalled(installed bool) {
	a.helperInstalled = installed
	if a.miGrantAccess == nil || a.miSyncNow == nil {
		return
	}
	if installed {
		a.miGrantAccess.Hide()
		a.miSyncNow.Show()
	} else {
		a.miGrantAccess.Show()
		a.miSyncNow.Hide()
	}
}

// SetSetupRequired shows or hides the "Retry Setup…" menu item and updates the
// status label to StatusSetupRequired. Call with true when PKCE auth fails in
// onReady; false to hide the item once setup completes.
func (a *App) SetSetupRequired(show bool) {
	if show {
		a.SetStatus(StatusSetupRequired)
		if a.miRetrySetup != nil {
			a.miRetrySetup.Show()
		}
	} else {
		if a.miRetrySetup != nil {
			a.miRetrySetup.Hide()
		}
	}
}

// SetKeychainError shows or hides the "Try Again" item and updates the status label.
// Call with true when keychain is unavailable; false to restore normal state.
func (a *App) SetKeychainError(show bool) {
	if show {
		a.SetStatus(StatusKeychainError)
		if a.miTryAgain != nil {
			a.miTryAgain.Show()
		}
	} else {
		if a.miTryAgain != nil {
			a.miTryAgain.Hide()
		}
	}
}

// SetWaitingForArena switches the tray status to StatusWaitingForArena (waiting=true)
// or StatusConnected (waiting=false). Called by the daemon idle loop when MTGA is not
// installed and the daemon is polling for Player.log.
func (a *App) SetWaitingForArena(waiting bool) {
	if waiting {
		a.SetStatus(StatusWaitingForArena)
	} else {
		a.SetStatus(StatusConnected)
	}
}

// SetLastSync updates the "last synced" timestamp label. Safe to call from any goroutine.
func (a *App) SetLastSync(t time.Time) {
	a.lastSync = t
	if a.miLastSync != nil {
		if t.IsZero() {
			a.miLastSync.SetTitle("Collection: never synced")
		} else {
			a.miLastSync.SetTitle(fmt.Sprintf("Collection: synced %s", t.Format("3:04 PM")))
		}
	}
}

func (a *App) setup() {
	systray.SetIcon(iconData)
	systray.SetTooltip("VaultMTG Companion")

	// On macOS the menu bar title is shown next to the icon.
	if runtime.GOOS == "darwin" {
		systray.SetTitle("VaultMTG")
	}

	// About item — disabled (informational label showing the running version).
	// Positioned at the top so the version is immediately visible without scrolling.
	a.miAbout = systray.AddMenuItem("VaultMTG Daemon "+a.version, "Running version")
	a.miAbout.Disable()

	// Check for Updates — opens the GitHub Releases page for the daemon.
	a.miCheckForUpdates = systray.AddMenuItem("Check for Updates", "Opens GitHub Releases page for the VaultMTG daemon")

	systray.AddSeparator()

	a.miStatus = systray.AddMenuItem(a.status.label(), "Daemon status")
	a.miStatus.Disable()

	systray.AddSeparator()

	a.miLastSync = systray.AddMenuItem("Collection: never synced", "")
	a.miLastSync.Disable()
	a.miSyncNow = systray.AddMenuItem("Sync Now", "Read collection from MTGA")
	a.miGrantAccess = systray.AddMenuItem("Grant Access…", "Install the collection helper (requires admin password)")
	// Show whichever is appropriate; default to showing Grant Access until the
	// daemon confirms the helper is running.
	a.miSyncNow.Hide()

	a.miTryAgain = systray.AddMenuItem("Try Again", "Retry reading from macOS keychain")
	a.miTryAgain.Hide()

	a.miRetrySetup = systray.AddMenuItem("Retry Setup…", "Re-open setup page and retry authentication")
	a.miRetrySetup.Hide()

	systray.AddSeparator()

	a.miOpenApp = systray.AddMenuItem("Open VaultMTG", "Open the VaultMTG web app")

	systray.AddSeparator()

	a.miQuit = systray.AddMenuItem("Quit", "Stop the VaultMTG daemon")
}

// openCheckForUpdates opens the GitHub Releases page for the VaultMTG daemon
// in the default browser. Extracted so it can be tested without systray.
func (a *App) openCheckForUpdates() {
	if a.openURL != nil {
		_ = a.openURL("https://github.com/RdHamilton/vault-mtg/releases?q=daemon")
	}
}

// tryStartSync attempts to claim the sync lock. Returns true if the sync may
// proceed (syncInFlight was false and is now set to true), false if a sync is
// already in flight (debounce — AC4). Extracted for testability.
func (a *App) tryStartSync() bool {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncInFlight {
		return false
	}
	a.syncInFlight = true
	return true
}

// NotifySyncResult is called by the daemon after a Sync Now operation completes.
// It updates the tray item label to show success ("Synced") or failure
// ("Sync failed"), holds it briefly, then resets to "Sync Now" and clears the
// in-flight flag so subsequent clicks are accepted again.
//
// When miSyncNow is nil (headless / pre-setup), the label steps are skipped but
// the in-flight flag is still cleared — matching the nil-guard pattern used by
// SetKeychainError and SetSetupRequired.
//
// Safe to call from any goroutine.
func (a *App) NotifySyncResult(err error) {
	if a.miSyncNow != nil {
		if err != nil {
			a.miSyncNow.SetTitle("Sync failed")
			time.Sleep(3 * time.Second)
		} else {
			a.miSyncNow.SetTitle("Synced")
			time.Sleep(2 * time.Second)
		}
		a.miSyncNow.SetTitle("Sync Now")
	}
	a.syncMu.Lock()
	a.syncInFlight = false
	a.syncMu.Unlock()
}

func (a *App) loop() {
	for {
		select {
		case <-a.miCheckForUpdates.ClickedCh:
			a.openCheckForUpdates()
		case <-a.miSyncNow.ClickedCh:
			if a.tryStartSync() {
				a.miSyncNow.SetTitle("Syncing...")
				select {
				case a.SyncNow <- struct{}{}:
				default: // channel full — daemon is busy; clear in-flight so the next click works
					a.syncMu.Lock()
					a.syncInFlight = false
					a.syncMu.Unlock()
					a.miSyncNow.SetTitle("Sync Now")
				}
			}
		case <-a.miGrantAccess.ClickedCh:
			select {
			case a.GrantAccess <- struct{}{}:
			default:
			}
		case <-a.miTryAgain.ClickedCh:
			select {
			case a.TryAgain <- struct{}{}:
			default:
			}
		case <-a.miRetrySetup.ClickedCh:
			select {
			case a.RetrySetup <- struct{}{}:
			default:
			}
		case <-a.miOpenApp.ClickedCh:
			if a.openURL != nil {
				_ = a.openURL(a.appURL)
			}
		case <-a.miQuit.ClickedCh:
			systray.Quit()
		}
	}
}
