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
		return "⚠ Keychain unavailable"
	case StatusSetupRequired:
		return "⚠ Setup required — auth failed"
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
	TryAgain    chan struct{}
	// RetrySetup is signalled when the user requests setup retry. Always
	// buffered cap=1 so callers can send without blocking even in headless mode.
	RetrySetup chan struct{}
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
		TryAgain:    make(chan struct{}, 1),
		RetrySetup:  make(chan struct{}, 1),
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

func (a *App) SetStatus(s Status)        { a.status = s }
func (a *App) SetHelperInstalled(_ bool) {}
func (a *App) SetLastSync(t time.Time)   { a.lastSync = t }
func (a *App) SetKeychainError(_ bool)   {}
func (a *App) SetSetupRequired(_ bool)   {}
func (a *App) SetWaitingForArena(_ bool) {}
