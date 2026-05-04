// Package handlers provides HTTP request handlers for the BFF service.
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// EventBroadcaster is implemented by any type that can broadcast a daemon event
// to connected clients (e.g. an SSE broker).  userID scopes delivery to the
// authenticated user's SSE subscribers only — preventing cross-tenant leakage.
type EventBroadcaster interface {
	BroadcastDaemonEvent(userID int64, event contract.DaemonEvent)
}

// IngestHandler accepts daemon events posted by the daemon service and
// broadcasts them to connected frontend clients via the broadcaster.
type IngestHandler struct {
	broadcaster EventBroadcaster
}

// NewIngestHandler creates an IngestHandler that broadcasts received events
// through the provided broadcaster.
func NewIngestHandler(broadcaster EventBroadcaster) *IngestHandler {
	return &IngestHandler{broadcaster: broadcaster}
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

	if h.broadcaster != nil {
		h.broadcaster.BroadcastDaemonEvent(userID, event)
	}

	log.Printf("[IngestHandler] Received event %q from account %q (userID=%d)", event.Type, event.AccountID, userID)

	w.WriteHeader(http.StatusAccepted)
}
