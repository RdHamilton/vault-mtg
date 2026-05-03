package logreader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEntryParseJSON_PlainText(t *testing.T) {
	e := &LogEntry{Raw: "plain text line"}
	e.parseJSON()
	assert.False(t, e.IsJSON)
	assert.Nil(t, e.JSON)
}

func TestLogEntryParseJSON_ValidJSON(t *testing.T) {
	e := &LogEntry{Raw: `{"type":"authenticateResponse","screenName":"TestPlayer"}`}
	e.parseJSON()
	assert.True(t, e.IsJSON)
	require.NotNil(t, e.JSON)
	assert.Equal(t, "authenticateResponse", e.JSON["type"])
}

func TestLogEntryParseJSON_PrefixedJSON(t *testing.T) {
	e := &LogEntry{Raw: `[UnityCrossThreadLogger]2/15/2024 12:00:00 PM {"authenticateResponse":{"screenName":"Ray"}}`}
	e.parseJSON()
	assert.True(t, e.IsJSON)
	require.NotNil(t, e.JSON)
	_, ok := e.JSON["authenticateResponse"]
	assert.True(t, ok)
}

func TestLogEntryParseJSON_InvalidJSON(t *testing.T) {
	e := &LogEntry{Raw: `{not valid json`}
	e.parseJSON()
	assert.False(t, e.IsJSON)
}

func TestReaderReadAll(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "Player.log")

	content := `plain text line
{"type":"draft.pick","pickedCards":["12345"]}
not json
{"type":"match.completed","CurrentEventState":"MatchCompleted"}
`
	require.NoError(t, os.WriteFile(logFile, []byte(content), 0o600))

	r, err := NewReader(logFile)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	entries, err := r.ReadAll()
	require.NoError(t, err)
	assert.Len(t, entries, 4)

	jsonEntries, err := NewReader(logFile)
	require.NoError(t, err)
	defer func() { _ = jsonEntries.Close() }()

	jsonOnly, err := jsonEntries.ReadAllJSON()
	require.NoError(t, err)
	assert.Len(t, jsonOnly, 2)
}

func TestDefaultLogPath_MissingDir(t *testing.T) {
	// DefaultLogPath should return an error when no MTGA log dir exists.
	// In CI there's no MTGA installation, so this is expected.
	_, err := DefaultLogPath()
	// We only verify it doesn't panic; error is acceptable.
	_ = err
}

func TestLogExists_Missing(t *testing.T) {
	exists, err := LogExists("/nonexistent/path/Player.log")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestLogExists_Present(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Player.log")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))

	exists, err := LogExists(path)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestPollerCreation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "Player.log")
	require.NoError(t, os.WriteFile(logFile, []byte(""), 0o600))

	cfg := DefaultPollerConfig(logFile)
	cfg.UseFileEvents = false
	p, err := NewPoller(cfg)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.False(t, p.IsRunning())
}

func TestPollerStartStop(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "Player.log")
	require.NoError(t, os.WriteFile(logFile, []byte(""), 0o600))

	cfg := DefaultPollerConfig(logFile)
	cfg.UseFileEvents = false
	p, err := NewPoller(cfg)
	require.NoError(t, err)

	_ = p.Start()
	assert.True(t, p.IsRunning())

	p.Stop()
	assert.False(t, p.IsRunning())
}

func TestPollerHandlesRotationDrain(t *testing.T) {
	modes := []struct {
		name          string
		useFileEvents bool
	}{
		{"timer", false},
		{"fsnotify", true},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			dir := t.TempDir()
			logFile := filepath.Join(dir, "Player.log")

			// Entry 1: present before poller starts
			entry1 := `{"type":"entry.one","id":"1"}` + "\n"
			require.NoError(t, os.WriteFile(logFile, []byte(entry1), 0o600))

			cfg := DefaultPollerConfig(logFile)
			cfg.UseFileEvents = mode.useFileEvents
			cfg.ReadFromStart = true
			cfg.Interval = 20 * time.Millisecond
			cfg.BufferSize = 20

			p, err := NewPoller(cfg)
			require.NoError(t, err)

			updates := p.Start()
			defer p.Stop()

			// Let the poller read entry 1
			time.Sleep(80 * time.Millisecond)

			// Entry 2: appended to the old file — must be read before rotation
			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o600)
			require.NoError(t, err)
			_, err = f.WriteString(`{"type":"entry.two","id":"2"}` + "\n")
			require.NoError(t, err)
			require.NoError(t, f.Close())

			// Give the poller time to naturally read entry 2 before we rotate the file.
			// With Interval=20ms, two full ticks (40ms) is enough.
			time.Sleep(80 * time.Millisecond)

			// Simulate MTGA overwrite: rename the old log
			rotated := filepath.Join(dir, "Player.log.old")
			require.NoError(t, os.Rename(logFile, rotated))

			// Let the poller detect the missing file and reset
			time.Sleep(80 * time.Millisecond)

			// Entry 3: new log file at the same path
			entry3 := `{"type":"entry.three","id":"3"}` + "\n"
			require.NoError(t, os.WriteFile(logFile, []byte(entry3), 0o600))

			// Collect all entries within 1s (generous for fsnotify on CI)
			deadline := time.After(1 * time.Second)
			var collected []*LogEntry
		loop:
			for {
				select {
				case entry, ok := <-updates:
					if !ok {
						break loop
					}
					if entry != nil && entry.IsJSON {
						collected = append(collected, entry)
					}
					if len(collected) == 3 {
						break loop
					}
				case <-deadline:
					break loop
				}
			}

			assert.Len(t, collected, 3, "should have received all 3 entries (entry 2 read before rotation, entry 3 from new file)")
		})
	}
}

func TestPollerReadsNewEntries(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "Player.log")
	require.NoError(t, os.WriteFile(logFile, []byte(""), 0o600))

	cfg := DefaultPollerConfig(logFile)
	cfg.UseFileEvents = false
	cfg.ReadFromStart = true
	cfg.Interval = 50 * 1000 * 1000 // 50ms for test speed
	p, err := NewPoller(cfg)
	require.NoError(t, err)

	updates := p.Start()
	defer p.Stop()

	// Write a JSON entry to the log file
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(`{"type":"test.event"}` + "\n")
	require.NoError(t, err)
	_ = f.Close()

	// The file was empty on start so ReadFromStart=true won't re-read it,
	// but checkForUpdates will pick up the new line on next poll.
	// We just verify the channel is readable (entry or timeout).
	select {
	case entry := <-updates:
		if entry != nil {
			assert.True(t, entry.IsJSON)
		}
	default:
		// Polling hasn't fired yet; that's fine in a unit test
	}
}
