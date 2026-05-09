// Package gre manages GRE (Game Rules Engine) session buffers.
// Each session accumulates log entries until a threshold is reached, a stale
// sweep evicts it, or the daemon shuts down.  On any of these conditions the
// buffer is flushed as a partial GamePlayEvent.
package gre

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// FlushFunc is called whenever a session buffer is flushed.  The caller is
// responsible for dispatching the resulting event to the BFF.
type FlushFunc func(ctx context.Context, sessionID string, entries []json.RawMessage, partial bool) error

// SessionBuffer holds accumulated GRE log entries for a single MTGA session.
type SessionBuffer struct {
	entries     []json.RawMessage
	lastUpdated time.Time
}

// Manager holds all active GRE session buffers and background sweep goroutine.
type Manager struct {
	mu             sync.Mutex
	sessions       map[string]*SessionBuffer
	flushThreshold int
	staleMinutes   int
	flush          FlushFunc
	sweepInterval  time.Duration // injectable for tests
}

// ManagerConfig configures a Manager.
type ManagerConfig struct {
	// FlushThreshold is the number of entries that triggers a partial flush.
	// Valid range enforced by config.Load; default 500.
	FlushThreshold int

	// StaleMinutes is how long a buffer can sit idle before the sweep evicts it.
	// Default 15.
	StaleMinutes int

	// SweepInterval overrides the goroutine sweep period (default 10 min).
	// Used by tests to shorten the interval.
	SweepInterval time.Duration

	// Flush is called each time a buffer is flushed (threshold hit, stale, or shutdown).
	Flush FlushFunc
}

const defaultSweepInterval = 10 * time.Minute

// NewManager creates a Manager with the given config.
func NewManager(cfg ManagerConfig) *Manager {
	si := cfg.SweepInterval
	if si == 0 {
		si = defaultSweepInterval
	}
	return &Manager{
		sessions:       make(map[string]*SessionBuffer),
		flushThreshold: cfg.FlushThreshold,
		staleMinutes:   cfg.StaleMinutes,
		flush:          cfg.Flush,
		sweepInterval:  si,
	}
}

// Append adds an entry to the named session buffer.
// If the buffer reaches the threshold it is flushed immediately as partial and
// then reset; the new entry begins the next buffer.
func (m *Manager) Append(ctx context.Context, sessionID string, entry json.RawMessage) error {
	m.mu.Lock()
	buf, ok := m.sessions[sessionID]
	if !ok {
		buf = &SessionBuffer{}
		m.sessions[sessionID] = buf
	}
	buf.entries = append(buf.entries, entry)
	buf.lastUpdated = time.Now()

	if len(buf.entries) >= m.flushThreshold {
		toFlush := buf.entries
		// Reset the buffer — new entry begins fresh slice.
		buf.entries = nil
		m.mu.Unlock()

		log.Printf("[gre] session=%s threshold=%d reached — flushing partial", sessionID, m.flushThreshold)
		return m.flush(ctx, sessionID, toFlush, true)
	}
	m.mu.Unlock()
	return nil
}

// FlushAll flushes every non-empty session buffer as partial.  Called on
// SIGTERM/SIGINT before daemon exit.
func (m *Manager) FlushAll(ctx context.Context) {
	m.mu.Lock()
	sessions := make(map[string][]json.RawMessage, len(m.sessions))
	for id, buf := range m.sessions {
		if len(buf.entries) > 0 {
			sessions[id] = buf.entries
		}
	}
	m.sessions = make(map[string]*SessionBuffer)
	m.mu.Unlock()

	for id, entries := range sessions {
		log.Printf("[gre] shutdown flush session=%s entries=%d", id, len(entries))
		if err := m.flush(ctx, id, entries, true); err != nil {
			log.Printf("[gre] warn: shutdown flush session=%s: %v", id, err)
		}
	}
}

// RunSweep starts the background stale-buffer sweep goroutine.
// It returns when ctx is cancelled.
func (m *Manager) RunSweep(ctx context.Context) {
	ticker := time.NewTicker(m.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sweepStale(ctx)
		}
	}
}

// sweepStale evicts and flushes any session buffer whose last-updated time
// is older than staleMinutes.
func (m *Manager) sweepStale(ctx context.Context) {
	threshold := time.Duration(m.staleMinutes) * time.Minute
	cutoff := time.Now().Add(-threshold)

	m.mu.Lock()
	stale := make(map[string][]json.RawMessage)
	for id, buf := range m.sessions {
		if len(buf.entries) > 0 && buf.lastUpdated.Before(cutoff) {
			stale[id] = buf.entries
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()

	for id, entries := range stale {
		log.Printf("[gre] stale sweep flushing session=%s entries=%d last_updated=%s",
			id, len(entries), time.Since(time.Now().Add(-threshold)))
		if err := m.flush(ctx, id, entries, true); err != nil {
			log.Printf("[gre] warn: stale sweep flush session=%s: %v", id, err)
		}
	}
}

// BufferCount returns the number of active session buffers.  Used in tests.
func (m *Manager) BufferCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// EntryCount returns the number of buffered entries for a session.  Used in tests.
func (m *Manager) EntryCount(sessionID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if buf, ok := m.sessions[sessionID]; ok {
		return len(buf.entries)
	}
	return 0
}

// SetLastUpdated overrides the last-updated timestamp for a session.  Used in tests.
func (m *Manager) SetLastUpdated(sessionID string, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	buf, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	buf.lastUpdated = t
	return nil
}
