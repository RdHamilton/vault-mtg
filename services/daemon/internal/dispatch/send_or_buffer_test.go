package dispatch_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendOrBuffer_SuccessDoesNotBuffer verifies that when the BFF is
// reachable, SendOrBuffer sends the event and the buffer stays empty (no
// drain needed).
func TestSendOrBuffer_SuccessDoesNotBuffer(t *testing.T) {
	var received contract.DaemonEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	buf := dispatch.NewRingBuffer(10)
	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok").WithBuffer(buf)

	evt, err := dispatch.BuildEvent("draft.pack", "acc", "sess", map[string]string{})
	require.NoError(t, err)
	require.NoError(t, d.SendOrBuffer(context.Background(), evt))

	assert.Equal(t, "draft.pack", received.Type, "event must have been dispatched")
	assert.Nil(t, buf.Drain(), "buffer must be empty after successful send")
}

// TestSendOrBuffer_BuffersOnFailure verifies that when all retries are
// exhausted the pre-marshaled bytes are enqueued in the ring buffer and no
// error is returned to the caller (silent buffering).
func TestSendOrBuffer_BuffersOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	buf := dispatch.NewRingBuffer(10)
	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok").WithBuffer(buf)

	evt, err := dispatch.BuildEvent("draft.pack", "acc", "sess", map[string]string{"key": "val"})
	require.NoError(t, err)

	// SendOrBuffer must NOT return an error — it silently buffers.
	require.NoError(t, d.SendOrBuffer(context.Background(), evt))

	drained := buf.Drain()
	require.Len(t, drained, 1, "failed event must be buffered")

	// The buffered bytes must contain a valid DaemonEvent with sequence
	// already stamped (sequence >= 1).
	var decoded contract.DaemonEvent
	require.NoError(t, json.Unmarshal(drained[0], &decoded))
	assert.Equal(t, "draft.pack", decoded.Type)
	assert.GreaterOrEqual(t, decoded.Sequence, uint64(1), "sequence must be pre-stamped in buffered bytes")
}

// TestSendOrBuffer_SequencePreservedAcrossDrain verifies the Option C
// contract: sequence numbers stamped at Send time are preserved verbatim in
// the buffered bytes — drain does not re-number.
func TestSendOrBuffer_SequencePreservedAcrossDrain(t *testing.T) {
	var requestCount atomic.Int32
	// Fail first 2 sends, succeed from #3 onward.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		// Each SendOrBuffer attempt results in up to maxAttempts (3) requests.
		// The first two SendOrBuffer calls will fail (both exhaust 3 retries
		// each = 6 server hits) before succeeding.
		if n <= 6 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()

	buf := dispatch.NewRingBuffer(10)
	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok").WithBuffer(buf)

	// Events 1 and 2 fail → get buffered.
	for i := range 2 {
		evt, err := dispatch.BuildEvent("draft.pack", "acc", "sess", map[string]int{"i": i})
		require.NoError(t, err)
		require.NoError(t, d.SendOrBuffer(context.Background(), evt))
	}

	drained := buf.Drain()
	require.Len(t, drained, 2)

	// Decode both and verify sequences are 1 and 2 (stamped at emission order).
	var e1, e2 contract.DaemonEvent
	require.NoError(t, json.Unmarshal(drained[0], &e1))
	require.NoError(t, json.Unmarshal(drained[1], &e2))
	assert.Equal(t, uint64(1), e1.Sequence, "buffered event 1 must carry sequence=1")
	assert.Equal(t, uint64(2), e2.Sequence, "buffered event 2 must carry sequence=2")
}

// TestSendOrBuffer_DrainSuccessResumes verifies that after a successful send,
// Drain returns nothing — confirming the buffer is used only on failure paths.
func TestSendOrBuffer_DrainSuccessResumes(t *testing.T) {
	var received []contract.DaemonEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var evt contract.DaemonEvent
		_ = json.Unmarshal(body, &evt)
		received = append(received, evt)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	buf := dispatch.NewRingBuffer(10)
	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok").WithBuffer(buf)

	for i := range 3 {
		evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]int{"i": i})
		require.NoError(t, err)
		require.NoError(t, d.SendOrBuffer(context.Background(), evt))
	}

	assert.Len(t, received, 3, "all events must reach the BFF")
	assert.Nil(t, buf.Drain(), "buffer must be empty when all sends succeed")
}

// TestSendOrBuffer_WithoutBuffer_ReturnsSendError verifies backward
// compatibility: a Dispatcher without a buffer attached behaves identically
// to the old Send — it returns the error on failure.
func TestSendOrBuffer_WithoutBuffer_ReturnsSendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// No .WithBuffer() call — buffer is nil.
	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok")

	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)

	err = d.SendOrBuffer(context.Background(), evt)
	assert.Error(t, err, "without a buffer wired, SendOrBuffer must return the error")
}

// TestSendOrBuffer_BufferDropsOldest verifies the end-to-end overflow path:
// when the buffer is full and a new event arrives, the oldest buffered event
// is evicted (drop-oldest) and Dropped() increments.
func TestSendOrBuffer_BufferDropsOldest(t *testing.T) {
	// BFF always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	buf := dispatch.NewRingBuffer(2)
	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok").WithBuffer(buf)

	for i := range 3 {
		evt, err := dispatch.BuildEvent("draft.pack", "acc", "sess", map[string]int{"seq": i + 1})
		require.NoError(t, err)
		require.NoError(t, d.SendOrBuffer(context.Background(), evt))
	}

	// Buffer capacity 2: event 1 was evicted.
	assert.Equal(t, int64(1), buf.Dropped())
	drained := buf.Drain()
	require.Len(t, drained, 2)

	var e1, e2 contract.DaemonEvent
	require.NoError(t, json.Unmarshal(drained[0], &e1))
	require.NoError(t, json.Unmarshal(drained[1], &e2))
	assert.Equal(t, uint64(2), e1.Sequence, "oldest (seq=1) evicted; seq=2 must be first")
	assert.Equal(t, uint64(3), e2.Sequence)
}

// TestSendOrBuffer_NoBuffer_NoPanic_ContextCancelled verifies context
// cancellation does not cause a panic when no buffer is wired.
func TestSendOrBuffer_NoBuffer_NoPanic_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok")
	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// May return context.DeadlineExceeded or nil — just must not panic.
	_ = d.SendOrBuffer(ctx, evt)
}
