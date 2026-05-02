// Package handlers provides HTTP request handlers for the BFF service.
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

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
// It expects a valid "Authorization: Bearer <token>" header and a JSON body
// containing a contract.DaemonEvent.
func (h *IngestHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	if !h.authenticated(r) {
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

	if h.broadcaster != nil {
		h.broadcaster.BroadcastDaemonEvent(event)
	}

	log.Printf("[IngestHandler] Received event %q from account %q", event.Type, event.AccountID)

	w.WriteHeader(http.StatusAccepted)
}

// authenticated checks the Authorization header against the shared daemon secret.
// The expected token is read from the DAEMON_SECRET environment variable.
// If DAEMON_SECRET is not set, all requests are rejected.
func (h *IngestHandler) authenticated(r *http.Request) bool {
	secret := os.Getenv("DAEMON_SECRET")
	if secret == "" {
		log.Println("[IngestHandler] Warning: DAEMON_SECRET is not set — rejecting all ingest requests")
		return false
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return false
	}

	token := strings.TrimPrefix(authHeader, prefix)

	return token == secret
}
