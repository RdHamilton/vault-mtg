package gre

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// --- helpers ---

func rawEntry(s string) json.RawMessage {
	return json.RawMessage([]byte(`"` + s + `"`))
}

type flushRecord struct {
	sessionID string
	entries   []json.RawMessage
	partial   bool
}

type fakeFlushSink struct {
	mu      sync.Mutex
	records []flushRecord
	err     error
}

func (f *fakeFlushSink) flush(ctx context.Context, sessionID string, entries []json.RawMessage, partial bool) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	f.records = append(f.records, flushRecord{sessionID: sessionID, entries: entries, partial: partial})
	f.mu.Unlock()
	return nil
}

func (f *fakeFlushSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

func (f *fakeFlushSink) last() flushRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.records[len(f.records)-1]
}

func newManager(threshold, staleMinutes int, sink *fakeFlushSink) *Manager {
	return NewManager(ManagerConfig{
		FlushThreshold: threshold,
		StaleMinutes:   staleMinutes,
		SweepInterval:  100 * time.Millisecond, // fast for tests
		Flush:          sink.flush,
	})
}

// --- threshold flush tests ---

func TestAppend_ThresholdFlush_EmitsPartialAndResetsBuffer(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(3, 15, sink) // threshold = 3

	ctx := context.Background()

	// Append 3 entries — 3rd triggers threshold flush.
	for i := 0; i < 3; i++ {
		if err := mgr.Append(ctx, "session-A", rawEntry("e")); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	if sink.count() != 1 {
		t.Fatalf("expected 1 flush after threshold, got %d", sink.count())
	}
	rec := sink.last()
	if rec.sessionID != "session-A" {
		t.Errorf("sessionID: want session-A, got %q", rec.sessionID)
	}
	if len(rec.entries) != 3 {
		t.Errorf("entries: want 3, got %d", len(rec.entries))
	}
	if !rec.partial {
		t.Errorf("partial: want true, got false")
	}
	// Buffer should be reset — 0 entries remain.
	if mgr.EntryCount("session-A") != 0 {
		t.Errorf("after threshold flush, expected 0 buffered entries, got %d", mgr.EntryCount("session-A"))
	}
}

func TestAppend_ThresholdFlush_ContinuesBufferingAfterReset(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(2, 15, sink) // threshold = 2

	ctx := context.Background()

	// First flush: entries 1 + 2.
	for i := 0; i < 2; i++ {
		if err := mgr.Append(ctx, "sess", rawEntry("e")); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	// Post-flush, add one more entry — should buffer without flush.
	if err := mgr.Append(ctx, "sess", rawEntry("e")); err != nil {
		t.Fatalf("Append after flush: %v", err)
	}

	if sink.count() != 1 {
		t.Errorf("expected 1 flush, got %d", sink.count())
	}
	if mgr.EntryCount("sess") != 1 {
		t.Errorf("expected 1 buffered entry after reset, got %d", mgr.EntryCount("sess"))
	}
}

func TestAppend_BelowThreshold_NoFlush(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(5, 15, sink)

	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := mgr.Append(ctx, "sess-B", rawEntry("e")); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	if sink.count() != 0 {
		t.Errorf("expected 0 flushes below threshold, got %d", sink.count())
	}
	if mgr.EntryCount("sess-B") != 4 {
		t.Errorf("expected 4 buffered entries, got %d", mgr.EntryCount("sess-B"))
	}
}

// --- stale eviction tests ---

func TestSweepStale_EvictsExpiredSession(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(100, 1 /* 1 min stale */, sink)

	ctx := context.Background()

	// Add an entry then backdate its last_updated to simulate staleness.
	if err := mgr.Append(ctx, "stale-sess", rawEntry("e")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := mgr.SetLastUpdated("stale-sess", time.Now().Add(-2*time.Minute)); err != nil {
		t.Fatalf("SetLastUpdated: %v", err)
	}

	// Manually trigger the stale sweep.
	mgr.sweepStale(ctx)

	if sink.count() != 1 {
		t.Fatalf("expected 1 flush from stale sweep, got %d", sink.count())
	}
	rec := sink.last()
	if rec.sessionID != "stale-sess" {
		t.Errorf("sessionID: want stale-sess, got %q", rec.sessionID)
	}
	if !rec.partial {
		t.Errorf("partial: want true for stale eviction")
	}
	// Session should be evicted.
	if mgr.BufferCount() != 0 {
		t.Errorf("expected 0 sessions after stale eviction, got %d", mgr.BufferCount())
	}
}

func TestSweepStale_IgnoresFreshSession(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(100, 15, sink)

	ctx := context.Background()

	if err := mgr.Append(ctx, "fresh-sess", rawEntry("e")); err != nil {
		t.Fatalf("Append: %v", err)
	}

	mgr.sweepStale(ctx)

	if sink.count() != 0 {
		t.Errorf("expected 0 flushes for fresh session, got %d", sink.count())
	}
	if mgr.BufferCount() != 1 {
		t.Errorf("expected 1 session to remain, got %d", mgr.BufferCount())
	}
}

func TestSweepStale_IgnoresEmptySession(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(100, 1, sink)

	ctx := context.Background()

	// Create a session with no entries (rare but possible if reset happened).
	// We backdoor: append then manually clear by setting threshold == 1.
	mgr2 := newManager(1, 1, &fakeFlushSink{})
	if err := mgr2.Append(ctx, "empty-sess", rawEntry("e")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// mgr2 flushed and buffer is now empty (0 entries).

	// Now test mgr: add a session with 0 entries (fresh map).
	mgr.mu.Lock()
	mgr.sessions["empty-sess"] = &SessionBuffer{
		entries:     nil,
		lastUpdated: time.Now().Add(-2 * time.Minute),
	}
	mgr.mu.Unlock()

	mgr.sweepStale(ctx)

	// empty session should NOT be flushed (nothing to send).
	if sink.count() != 0 {
		t.Errorf("expected 0 flushes for empty session, got %d", sink.count())
	}
}

// --- graceful shutdown flush tests ---

func TestFlushAll_FlushesFlusheableSessionsAsPartial(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(100, 15, sink)

	ctx := context.Background()

	if err := mgr.Append(ctx, "sess-1", rawEntry("a")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := mgr.Append(ctx, "sess-2", rawEntry("b")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := mgr.Append(ctx, "sess-2", rawEntry("c")); err != nil {
		t.Fatalf("Append: %v", err)
	}

	mgr.FlushAll(ctx)

	if sink.count() != 2 {
		t.Fatalf("expected 2 flushes (one per session), got %d", sink.count())
	}
	// All flushes must be partial.
	for _, r := range sink.records {
		if !r.partial {
			t.Errorf("session %q: expected partial=true on shutdown flush", r.sessionID)
		}
	}
	// All sessions cleared.
	if mgr.BufferCount() != 0 {
		t.Errorf("expected 0 sessions after FlushAll, got %d", mgr.BufferCount())
	}
}

func TestFlushAll_EmptySessions_NoFlushCalled(t *testing.T) {
	sink := &fakeFlushSink{}
	mgr := newManager(100, 15, sink)

	mgr.FlushAll(context.Background())

	if sink.count() != 0 {
		t.Errorf("expected 0 flushes when no sessions, got %d", sink.count())
	}
}

// --- sweep goroutine integration test ---

func TestRunSweep_EvictsStaleSessionViaTicker(t *testing.T) {
	sink := &fakeFlushSink{}
	// Very short sweep interval and very short stale threshold (1 ms via direct manipulation).
	mgr := NewManager(ManagerConfig{
		FlushThreshold: 100,
		StaleMinutes:   1, // 1 minute stale
		SweepInterval:  20 * time.Millisecond,
		Flush:          sink.flush,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := mgr.Append(ctx, "sweep-sess", rawEntry("e")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// Backdate so sweep sees it as stale.
	if err := mgr.SetLastUpdated("sweep-sess", time.Now().Add(-2*time.Minute)); err != nil {
		t.Fatalf("SetLastUpdated: %v", err)
	}

	go mgr.RunSweep(ctx)
	<-ctx.Done()

	if sink.count() < 1 {
		t.Errorf("expected at least 1 stale-sweep flush via goroutine, got %d", sink.count())
	}
}
