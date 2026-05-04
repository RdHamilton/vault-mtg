// Package sse provides a Server-Sent Events broker that fans out daemon events
// to connected browser clients over long-lived HTTP connections.
package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
)

// event is the internal wire format sent over each subscriber channel.
type event struct {
	Name string
	Data string
}

// subscriber represents a single connected SSE client.
type subscriber struct {
	ch     chan event
	userID string
}

// Broker manages SSE subscriptions and fans out published events to all
// connected clients.  It is safe for concurrent use.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
}

// New returns an initialised, ready-to-use Broker.
func New() *Broker {
	return &Broker{
		subscribers: make(map[*subscriber]struct{}),
	}
}

// subscribe registers a new client and returns its subscriber handle.
func (b *Broker) subscribe(userID string) *subscriber {
	sub := &subscriber{
		ch:     make(chan event, 64),
		userID: userID,
	}

	b.mu.Lock()
	b.subscribers[sub] = struct{}{}
	b.mu.Unlock()

	return sub
}

// unsubscribe removes a client and closes its channel.
func (b *Broker) unsubscribe(sub *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[sub]; ok {
		delete(b.subscribers, sub)
		close(sub.ch)
	}
}

// Publish fans out a daemon event to all connected subscribers.
func (b *Broker) Publish(e contract.DaemonEvent) {
	data, err := json.Marshal(e)
	if err != nil {
		log.Printf("[sse] marshal error: %v", err)
		return
	}

	ev := event{
		Name: e.Type,
		Data: string(data),
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for sub := range b.subscribers {
		select {
		case sub.ch <- ev:
		default:
			// Slow client: drop rather than block.
			log.Printf("[sse] dropping event for slow client (user=%s)", sub.userID)
		}
	}
}

// SubscriberCount returns the number of currently connected SSE clients.
// Intended for metrics and tests.
func (b *Broker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// ServeHTTP handles GET /api/v1/events.
// It keeps the connection open and writes daemon events to the client as SSE
// frames.  The connection is closed when the client disconnects (ctx.Done()).
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// SSE requires the response writer to support flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Allow cross-origin EventSource from the Electron/localhost frontend.
	w.Header().Set("X-Accel-Buffering", "no")

	userID := r.Header.Get("X-User-ID")

	sub := b.subscribe(userID)
	defer b.unsubscribe(sub)

	log.Printf("[sse] client connected (user=%s)", userID)

	// Send a comment heartbeat immediately so the client knows it is connected.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[sse] client disconnected (user=%s)", userID)
			return
		case ev, ok := <-sub.ch:
			if !ok {
				// Channel closed (broker shutting down or forcibly unsubscribed).
				return
			}

			if ev.Name != "" {
				fmt.Fprintf(w, "event: %s\n", ev.Name)
			}

			fmt.Fprintf(w, "data: %s\n\n", ev.Data)
			flusher.Flush()
		}
	}
}
