package dispatch_test

import (
	"sync"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRingBuffer_EnqueueAndDrain verifies that a buffer with spare capacity
// returns enqueued bytes verbatim on drain without reordering or modification.
func TestRingBuffer_EnqueueAndDrain(t *testing.T) {
	buf := dispatch.NewRingBuffer(4)

	msgs := [][]byte{
		[]byte(`{"sequence":1,"type":"draft.pack"}`),
		[]byte(`{"sequence":2,"type":"draft.pick"}`),
		[]byte(`{"sequence":3,"type":"match.completed"}`),
	}

	for _, m := range msgs {
		buf.Enqueue(m)
	}

	drained := buf.Drain()
	require.Len(t, drained, 3)
	for i, got := range drained {
		assert.Equal(t, msgs[i], got, "item %d: bytes must be verbatim", i)
	}
}

// TestRingBuffer_DrainEmpty verifies that draining an empty buffer returns nil
// without panicking.
func TestRingBuffer_DrainEmpty(t *testing.T) {
	buf := dispatch.NewRingBuffer(4)
	result := buf.Drain()
	assert.Nil(t, result, "drain of empty buffer must return nil")
}

// TestRingBuffer_DrainClearsBuffer verifies that a second Drain after the
// first returns nil (items are consumed).
func TestRingBuffer_DrainClearsBuffer(t *testing.T) {
	buf := dispatch.NewRingBuffer(4)
	buf.Enqueue([]byte(`{"sequence":1}`))
	first := buf.Drain()
	require.Len(t, first, 1)

	second := buf.Drain()
	assert.Nil(t, second, "second drain must return nil")
}

// TestRingBuffer_DropOldestOnOverflow verifies that when the buffer is at
// capacity, the oldest entry is evicted and the newest is retained (FIFO
// overflow — drop-oldest semantics).
func TestRingBuffer_DropOldestOnOverflow(t *testing.T) {
	buf := dispatch.NewRingBuffer(3)

	// Fill to capacity.
	buf.Enqueue([]byte(`seq1`))
	buf.Enqueue([]byte(`seq2`))
	buf.Enqueue([]byte(`seq3`))

	// Overflow: seq1 must be dropped, seq4 retained.
	buf.Enqueue([]byte(`seq4`))

	drained := buf.Drain()
	require.Len(t, drained, 3, "buffer must still hold exactly 3 items after overflow")
	assert.Equal(t, []byte(`seq2`), drained[0], "oldest item (seq1) must have been evicted")
	assert.Equal(t, []byte(`seq3`), drained[1])
	assert.Equal(t, []byte(`seq4`), drained[2])
}

// TestRingBuffer_DropOldestMultipleOverflows verifies drop-oldest is applied
// consistently across repeated overflows.
func TestRingBuffer_DropOldestMultipleOverflows(t *testing.T) {
	buf := dispatch.NewRingBuffer(2)

	for i := 1; i <= 5; i++ {
		buf.Enqueue([]byte{byte(i)})
	}

	drained := buf.Drain()
	require.Len(t, drained, 2)
	// Only the two most-recently enqueued items survive.
	assert.Equal(t, []byte{4}, drained[0])
	assert.Equal(t, []byte{5}, drained[1])
}

// TestRingBuffer_DroppedCount verifies that Dropped() returns the cumulative
// number of evictions since the buffer was created.
func TestRingBuffer_DroppedCount(t *testing.T) {
	buf := dispatch.NewRingBuffer(2)
	assert.Equal(t, int64(0), buf.Dropped(), "no drops on fresh buffer")

	buf.Enqueue([]byte(`a`))
	buf.Enqueue([]byte(`b`))
	assert.Equal(t, int64(0), buf.Dropped(), "no drops while under capacity")

	buf.Enqueue([]byte(`c`)) // evicts a
	assert.Equal(t, int64(1), buf.Dropped())

	buf.Enqueue([]byte(`d`)) // evicts b
	assert.Equal(t, int64(2), buf.Dropped())

	// Drain does not reset the drop counter.
	buf.Drain()
	assert.Equal(t, int64(2), buf.Dropped(), "Drain must not reset the dropped counter")
}

// TestRingBuffer_Contention verifies no deadlock and no data race when
// multiple goroutines call Enqueue and Drain concurrently.
// Run with: go test -race ./services/daemon/...
func TestRingBuffer_Contention(t *testing.T) {
	buf := dispatch.NewRingBuffer(10)
	const goroutines = 8
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range perGoroutine {
				buf.Enqueue([]byte{byte(id), byte(i)})
				if i%5 == 0 {
					buf.Drain()
				}
			}
		}(g)
	}
	wg.Wait()
	// Verify the buffer is in a consistent state post-concurrency.
	drained := buf.Drain()
	assert.LessOrEqual(t, len(drained), 10, "buffer must not exceed capacity")
}

// TestRingBuffer_PreservesBytesVerbatim verifies that the bytes stored are
// the exact slice contents — no re-serialization or mutation occurs. This is
// the Option C contract: pre-marshaled bytes are stored as-is.
func TestRingBuffer_PreservesBytesVerbatim(t *testing.T) {
	buf := dispatch.NewRingBuffer(4)

	original := []byte(`{"sequence":42,"type":"draft.pack","accountId":"acc-123"}`)
	// Store a copy so we can mutate original after enqueue (the buffer must
	// hold its own reference, not be affected by external mutation).
	payload := make([]byte, len(original))
	copy(payload, original)
	buf.Enqueue(payload)

	// Mutate the source slice after enqueue.
	payload[0] = 'X'

	drained := buf.Drain()
	require.Len(t, drained, 1)
	// Buffer must have stored the original bytes, not a reference to the
	// caller's slice that could be mutated externally.
	assert.Equal(t, original, drained[0])
}
