// Package daemonstate manages runtime state that must survive a daemon restart.
//
// Configuration lives in daemon.json (ADR-020). This file holds volatile
// runtime signals that are not configuration — values written by the running
// daemon and consumed on the next startup.
//
// The canonical file is daemon-state.json in the same directory as daemon.json.
// Writes are atomic (tmp-file + rename), matching the pattern used by
// config.saveToFile.
package daemonstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// State is the on-disk representation of daemon-state.json.
//
// Fields must remain backward-compatible: old daemons will ignore unknown
// fields; new daemons must handle missing fields gracefully (zero values).
type State struct {
	// AuthPaused is set to true when the daemon has exhausted its maximum
	// allowed PKCE auth attempts without success. When true, the daemon must
	// not open a browser or start a new PKCE flow on startup — instead it
	// shows the paused auth state in the tray and local API until the user
	// explicitly clicks "Retry Setup".
	//
	// Reset to false (and the file rewritten) only on:
	//   - a successful PKCE completion, OR
	//   - an explicit user-initiated "Retry Setup" action.
	// A timer-based auto-reset is intentionally absent (RC3).
	AuthPaused bool `json:"auth_paused"`

	// AuthAttempts is the number of consecutive failed PKCE attempts since the
	// last successful auth or manual retry. The daemon increments this on each
	// failure and resets it to 0 on success or manual retry (RC3).
	AuthAttempts int `json:"auth_attempts"`
}

// StateFilePath derives the daemon-state.json path from the daemon.json path.
// Both files live in the same directory; only the basename differs.
func StateFilePath(daemonJSONPath string) string {
	return filepath.Join(filepath.Dir(daemonJSONPath), "daemon-state.json")
}

// Load reads daemon-state.json from path. If the file does not exist a
// zero-value State is returned with no error — first-install and pre-feature
// upgrades start with zero state. All other errors are returned.
func Load(path string) (State, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("daemonstate: open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var s State
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		// Treat a corrupt file as zero-state so the daemon is not permanently
		// bricked by a bad write. Log at the call site.
		return State{}, fmt.Errorf("daemonstate: decode %q: %w", path, err)
	}
	return s, nil
}

// Save writes s to path atomically using a tmp-file + rename pattern,
// matching the atomicity guarantee of config.saveToFile (RC4).
func Save(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("daemonstate: marshal: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("daemonstate: mkdir %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".daemon-state-*.tmp")
	if err != nil {
		return fmt.Errorf("daemonstate: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, werr := tmp.Write(data); werr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemonstate: write temp file: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemonstate: close temp file: %w", cerr)
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemonstate: rename to %q: %w", path, rerr)
	}
	return nil
}
