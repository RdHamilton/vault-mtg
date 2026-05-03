// Package handlers provides HTTP request handlers for the BFF service.
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	contract "github.com/ramonehamilton/mtga-contract"
)

// EventBroadcaster is implemented by any type that can broadcast a daemon event
// to connected clients (e.g. a WebSocket hub).
type EventBroadcaster interface {
	BroadcastDaemonEvent(event contract.DaemonEvent)
}

// IngestHandler accepts daemon events posted by the daemon service and
// broadcasts them to connected frontend clients via the hub.
type IngestHandler struct {
	broadcaster EventBroadcaster
}

// NewIngestHandler creates an IngestHandler that broadcasts received events
// through the provided broadcaster.
func NewIngestHandler(broadcaster EventBroadcaster) *IngestHandler {
	return &IngestHandler{broadcaster: broadcaster}
}

// IngestEvent handles POST /v1/ingest/events.
// Authentication is enforced by the APIKeyAuth middleware upstream; by the time
// this handler runs the request is already verified.
func (h *IngestHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var event contract.DaemonEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if event.Type == "" {
		http.Error(w, "event type is required", http.StatusBadRequest)
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.BroadcastDaemonEvent(event)
	}

	log.Printf("[IngestHandler] Received event %q from account %q", event.Type, event.AccountID)

	w.WriteHeader(http.StatusAccepted)
}
