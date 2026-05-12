package tray

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Status.label
// ---------------------------------------------------------------------------

func TestStatusLabel(t *testing.T) {
	cases := []struct {
		s    Status
		want string
	}{
		{StatusStarting, "◌ Starting..."},
		{StatusConnected, "● Connected"},
		{StatusWaitingForArena, "◌ Waiting for Arena..."},
		{StatusError, "✕ Error — check logs"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.s.label(), "Status(%d)", tc.s)
	}
}

// ---------------------------------------------------------------------------
// App state transitions (no real systray)
// ---------------------------------------------------------------------------

func newTestApp() *App {
	return New("https://app.vaultmtg.app", func(string) error { return nil }, func() {})
}

func TestAppInitialStatus(t *testing.T) {
	a := newTestApp()
	assert.Equal(t, StatusStarting, a.status)
}

func TestAppSetStatus(t *testing.T) {
	a := newTestApp()
	a.SetStatus(StatusConnected)
	assert.Equal(t, StatusConnected, a.status)

	a.SetStatus(StatusError)
	assert.Equal(t, StatusError, a.status)
}

func TestAppSetLastSync_Zero(t *testing.T) {
	a := newTestApp()
	a.SetLastSync(time.Time{})
	assert.True(t, a.lastSync.IsZero())
}

func TestAppSetLastSync_NonZero(t *testing.T) {
	a := newTestApp()
	ts := time.Date(2026, 5, 12, 14, 30, 0, 0, time.UTC)
	a.SetLastSync(ts)
	assert.Equal(t, ts, a.lastSync)
}

func TestAppSyncNowChannel_NonBlocking(t *testing.T) {
	a := newTestApp()
	// Sending twice without draining should not block (buffered, cap=1).
	a.SyncNow <- struct{}{}
	select {
	case a.SyncNow <- struct{}{}:
		// dropped — channel full, not a panic
	default:
	}
	assert.Len(t, a.SyncNow, 1)
}

func TestAppNew_SetsAppURL(t *testing.T) {
	url := "https://app.vaultmtg.app"
	a := New(url, nil, nil)
	assert.Equal(t, url, a.appURL)
}

func TestAppQuitCallback(t *testing.T) {
	called := false
	a := New("https://app.vaultmtg.app", nil, func() { called = true })
	// Simulate what onExit does inside Run.
	if a.onQuit != nil {
		a.onQuit()
	}
	assert.True(t, called)
}
