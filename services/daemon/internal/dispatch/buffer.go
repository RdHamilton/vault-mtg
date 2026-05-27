package dispatch

import (
	"sync"
	"sync/atomic"
)

// RingBuffer is a bounded, mutex-protected ring buffer that stores
// pre-marshaled event payloads ([]byte) in emission order.
//
// When the buffer is full, Enqueue evicts the oldest entry (drop-oldest)
// so the newest event is always retained. This preserves ADR-013 sequence
// ordering — sequence numbers are stamped into the bytes at Send time and
// stored verbatim; drain replays bytes without re-marshaling.
//
// All methods are safe for concurrent use.
type RingBuffer struct {
	mu      sync.Mutex
	items   [][]byte
	cap     int
	dropped atomic.Int64
}

// NewRingBuffer creates a RingBuffer with the given capacity. Panics if cap
// is less than 1.
func NewRingBuffer(cap int) *RingBuffer {
	if cap < 1 {
		panic("dispatch.NewRingBuffer: capacity must be >= 1")
	}
	return &RingBuffer{
		items: make([][]byte, 0, cap),
		cap:   cap,
	}
}

// Enqueue appends payload to the buffer. If the buffer is at capacity, the
// oldest entry is evicted first (drop-oldest) and Dropped is incremented.
// A copy of payload is stored so callers may safely reuse the slice after
// calling Enqueue.
func (b *RingBuffer) Enqueue(payload []byte) {
	stored := make([]byte, len(payload))
	copy(stored, payload)

	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) >= b.cap {
		// Drop oldest — shift the slice left by one.
		b.items = b.items[1:]
		b.dropped.Add(1)
	}
	b.items = append(b.items, stored)
}

// Drain removes and returns all buffered payloads in FIFO order. Returns nil
// when the buffer is empty.
func (b *RingBuffer) Drain() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return nil
	}
	out := b.items
	b.items = make([][]byte, 0, b.cap)
	return out
}

// Dropped returns the cumulative number of entries evicted due to overflow
// since the buffer was created. The counter is never reset (not even by
// Drain) so callers can accumulate a monotonic dropped count across drain
// cycles.
func (b *RingBuffer) Dropped() int64 {
	return b.dropped.Load()
}
