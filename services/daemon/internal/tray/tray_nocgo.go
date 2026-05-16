//go:build !cgo

// Headless (no-CGO) stub — used when the binary is cross-compiled without CGO
// (e.g. darwin targets built from the Linux GoReleaser runner). The daemon runs
// as a launchd service in that context and has no tray icon.
package tray

import "time"

// Status represents the daemon's connection state.
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

// App is a no-op tray stub for headless builds.
type App struct {
	appURL   string
	onQuit   func()
	status   Status
	lastSync time.Time

	quit        chan struct{}
	SyncNow     chan struct{}
	GrantAccess chan struct{}
}

// New creates a no-op App.
func New(appURL string, openURL func(string) error, onQuit func()) *App {
	return &App{
		appURL:      appURL,
		onQuit:      onQuit,
		status:      StatusStarting,
		quit:        make(chan struct{}),
		SyncNow:     make(chan struct{}, 1),
		GrantAccess: make(chan struct{}, 1),
	}
}

// Run calls onReady immediately then blocks until Quit is called.
func (a *App) Run(onReady func()) {
	if onReady != nil {
		onReady()
	}
	<-a.quit
	if a.onQuit != nil {
		a.onQuit()
	}
}

// Quit unblocks Run. Safe to call from any goroutine.
func (a *App) Quit() {
	select {
	case <-a.quit:
	default:
		close(a.quit)
	}
}

func (a *App) SetStatus(s Status)               { a.status = s }
func (a *App) SetHelperInstalled(_ bool)         {}
func (a *App) SetLastSync(t time.Time)           { a.lastSync = t }
