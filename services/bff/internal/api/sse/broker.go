// Package sse provides a Server-Sent Events broker that fans out daemon events
// to connected browser clients over long-lived HTTP connections.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
)

// DefaultHeartbeatInterval is the period between SSE heartbeat comment frames
// when no events are published.  It must be shorter than the nginx
// proxy_read_timeout configured for /api/v1/events (recommended: 60s).
const DefaultHeartbeatInterval = 30 * time.Second

// event is the internal wire format sent over each subscriber channel.
type event struct {
	Name string
	Data string
}

// subscriber represents a single connected SSE client.
type subscriber struct {
	ch     chan event
	userID int64
}

// Broker manages SSE subscriptions and fans out published events to connected
// clients scoped by user ID.  It is safe for concurrent use.
type Broker struct {
	mu                sync.RWMutex
	subscribers       map[*subscriber]struct{}
	heartbeatInterval time.Duration
}

// UserIDExtractor is a function that extracts an authenticated user ID from a
// request context.  main.go injects middleware.UserIDFromContext so the broker
// package does not depend on the middleware package.
type UserIDExtractor func(ctx context.Context) (int64, bool)

// New returns an initialised, ready-to-use Broker with the default heartbeat
// interval (30 seconds).
func New() *Broker {
	return NewWithHeartbeat(DefaultHeartbeatInterval)
}

// NewWithHeartbeat returns a Broker that sends SSE keep-alive comment frames at
// the given interval.  Pass a non-positive duration to disable heartbeats
// (not recommended for production).  Intended for tests that need a fast
// ticker.
func NewWithHeartbeat(interval time.Duration) *Broker {
	return &Broker{
		subscribers:       make(map[*subscriber]struct{}),
		heartbeatInterval: interval,
	}
}

// subscribe registers a new client and returns its subscriber handle.
func (b *Broker) subscribe(userID int64) *subscriber {
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

// Publish fans out a daemon event only to subscribers whose user ID matches
// the provided userID.  This prevents cross-tenant event delivery.
func (b *Broker) Publish(userID int64, e contract.DaemonEvent) {
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
		if sub.userID != userID {
			continue
		}

		select {
		case sub.ch <- ev:
		default:
			// Slow client: channel buffer full; drop event to avoid blocking the
			// publisher.  Log with structured fields so the operator can alert on
			// this in production.
			log.Printf("[sse] slow_client_drop userID=%d channel_capacity=%d", sub.userID, cap(sub.ch))
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

// Handler returns an http.HandlerFunc for GET /api/v1/events.
//
// extractUserID must be middleware.UserIDFromContext (or a test stub).  It is
// injected rather than imported to avoid a package-cycle dependency between
// sse and middleware.
//
// The connection is kept open and daemon events are written as SSE frames.
// A periodic heartbeat comment (": heartbeat\n\n") is sent every
// HeartbeatInterval when no events arrive, preventing nginx and load-balancer
// idle-timeout disconnections.  The handler returns immediately when the
// client disconnects (ctx.Done()).
//
// If extractUserID cannot resolve an authenticated user the handler responds
// with 401 Unauthorized — this prevents unauthenticated SSE subscriptions.
func (b *Broker) Handler(extractUserID UserIDExtractor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// SSE requires the response writer to support flushing.
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		userID, ok := extractUserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Prevent nginx proxy from buffering the SSE stream.
		w.Header().Set("X-Accel-Buffering", "no")

		sub := b.subscribe(userID)
		defer b.unsubscribe(sub)

		log.Printf("[sse] client_connected userID=%d", userID)

		// Send a comment frame immediately so the client knows it is connected.
		_, _ = fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ctx := r.Context()

		// Start a heartbeat ticker to keep the connection alive through nginx and
		// load-balancer idle timeouts.  A non-positive interval disables the
		// ticker (disabled heartbeat is not recommended in production).
		var tickerC <-chan time.Time

		if b.heartbeatInterval > 0 {
			ticker := time.NewTicker(b.heartbeatInterval)
			defer ticker.Stop()

			tickerC = ticker.C
		}

		for {
			select {
			case <-ctx.Done():
				log.Printf("[sse] client_disconnected userID=%d", userID)
				return

			case <-tickerC:
				_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()

			case ev, ok := <-sub.ch:
				if !ok {
					// Channel closed (broker shutting down or forcibly unsubscribed).
					return
				}

				if ev.Name != "" {
					_, _ = fmt.Fprintf(w, "event: %s\n", ev.Name)
				}

				_, _ = fmt.Fprintf(w, "data: %s\n\n", ev.Data)
				flusher.Flush()
			}
		}
	}
}
