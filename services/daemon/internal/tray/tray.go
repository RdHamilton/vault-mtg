// Package tray manages the system tray (menu bar) icon and menu for the
// VaultMTG daemon. systray.Run must be called on the main OS thread; callers
// must invoke App.Run from main() and start the daemon event loop inside the
// onReady callback.
package tray

import (
	_ "embed"
	"fmt"
	"runtime"
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
)

func (s Status) label() string {
	switch s {
	case StatusConnected:
		return "● Connected"
	case StatusWaitingForArena:
		return "◌ Waiting for Arena..."
	case StatusError:
		return "✕ Error — check logs"
	default:
		return "◌ Starting..."
	}
}

// App manages the tray icon, menu items, and status state.
type App struct {
	appURL  string
	openURL func(string) error
	onQuit  func()

	// protected by single-goroutine access after setup()
	status   Status
	lastSync time.Time

	miStatus   *systray.MenuItem
	miLastSync *systray.MenuItem
	miSyncNow  *systray.MenuItem
	miOpenApp  *systray.MenuItem
	miQuit     *systray.MenuItem

	// SyncNow is signalled when the user clicks "Sync Now".
	SyncNow chan struct{}
}

// New creates an App. appURL is opened when "Open VaultMTG" is clicked.
// openURL is the platform open-browser function. onQuit is called when the
// tray exits (Quit clicked or process terminated).
func New(appURL string, openURL func(string) error, onQuit func()) *App {
	return &App{
		appURL:  appURL,
		openURL: openURL,
		onQuit:  onQuit,
		status:  StatusStarting,
		SyncNow: make(chan struct{}, 1),
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

	a.miStatus = systray.AddMenuItem(a.status.label(), "Daemon status")
	a.miStatus.Disable()

	systray.AddSeparator()

	a.miLastSync = systray.AddMenuItem("Collection: never synced", "")
	a.miLastSync.Disable()
	a.miSyncNow = systray.AddMenuItem("Sync Now", "Read collection from MTGA")

	systray.AddSeparator()

	a.miOpenApp = systray.AddMenuItem("Open VaultMTG", "Open the VaultMTG web app")

	systray.AddSeparator()

	a.miQuit = systray.AddMenuItem("Quit", "Stop the VaultMTG daemon")
}

func (a *App) loop() {
	for {
		select {
		case <-a.miSyncNow.ClickedCh:
			select {
			case a.SyncNow <- struct{}{}:
			default: // already queued
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
