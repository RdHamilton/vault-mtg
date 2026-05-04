// Package handlers provides HTTP request handlers for the BFF service.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// EventBroadcaster is implemented by any type that can broadcast a daemon event
// to connected clients (e.g. an SSE broker).  userID scopes delivery to the
// authenticated user's SSE subscribers only — preventing cross-tenant leakage.
type EventBroadcaster interface {
	BroadcastDaemonEvent(userID int64, event contract.DaemonEvent)
}

// DaemonEventInserter is implemented by any type that can persist a daemon event
// to durable storage.  It is satisfied by *repository.DaemonEventsRepository.
type DaemonEventInserter interface {
	Insert(ctx context.Context, userID int64, accountID string, eventType string, payload json.RawMessage, occurredAt time.Time) error
}

// IngestHandler accepts daemon events posted by the daemon service and
// broadcasts them to connected frontend clients via the broadcaster.
// When a DaemonEventInserter is wired, each event is also persisted to the
// database before broadcasting.
type IngestHandler struct {
	broadcaster EventBroadcaster
	repo        DaemonEventInserter
}

// NewIngestHandler creates an IngestHandler that broadcasts received events
// through the provided broadcaster.  Pass nil for repo to run in
// broadcast-only mode (no persistence).
func NewIngestHandler(broadcaster EventBroadcaster) *IngestHandler {
	return &IngestHandler{broadcaster: broadcaster}
}

// WithRepository returns a copy of h with repo wired for persistence.
// This enables optional dependency injection without changing the existing
// NewIngestHandler call-sites.
func (h *IngestHandler) WithRepository(repo DaemonEventInserter) *IngestHandler {
	return &IngestHandler{broadcaster: h.broadcaster, repo: repo}
}

// IngestEvent handles POST /v1/ingest/events.
// Authentication is enforced by either APIKeyAuth or DaemonJWTAuth middleware
// upstream. By the time this handler runs, at least one of UserIDFromContext or
// DaemonUserIDFromContext is set on the request context.
func (h *IngestHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// Resolve the effective user ID. DaemonJWTAuth sets DaemonUserIDFromContext;
	// APIKeyAuth sets UserIDFromContext. Accept either — reject only when neither
	// is present. The JWT-scoped value takes precedence when both are set.
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if jwtUserID, jwtOK := bffmiddleware.DaemonUserIDFromContext(r.Context()); jwtOK {
		userID = jwtUserID
		ok = true
	}

	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var event contract.DaemonEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if event.Type == "" {
		http.Error(w, "event type is required", http.StatusBadRequest)
		return
	}

	// When the request was authenticated via daemon JWT, override AccountID so
	// it is always scoped to the JWT-derived user — prevents a daemon from
	// injecting events for a different account.
	if _, jwtOK := bffmiddleware.DaemonUserIDFromContext(r.Context()); jwtOK {
		event.AccountID = fmt.Sprintf("user:%d", userID)
	}

	// Persist the event before broadcasting. A persistence failure is logged
	// but does not drop the live event — the broadcast still proceeds so the
	// frontend receives the event even when the database is degraded.
	if h.repo != nil {
		if err := h.repo.Insert(r.Context(), userID, event.AccountID, event.Type, event.Payload, event.OccurredAt); err != nil {
			log.Printf("[IngestHandler] ERROR persisting event %q for userID=%d account=%q: %v", event.Type, userID, event.AccountID, err)
		}
	}

	if h.broadcaster != nil {
		h.broadcaster.BroadcastDaemonEvent(userID, event)
	}

	log.Printf("[IngestHandler] Received event %q from account %q (userID=%d)", event.Type, event.AccountID, userID)

	w.WriteHeader(http.StatusAccepted)
}
