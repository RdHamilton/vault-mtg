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

// daemonBearerToken extracts the Bearer token from the Authorization header.
func daemonBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "

	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimPrefix(header, prefix)

	return token, token != ""
}

// IngestEvent handles POST /v1/ingest/events.
// It validates the request against the DAEMON_SECRET environment variable
// before processing the event.
func (h *IngestHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("DAEMON_SECRET")
	if secret == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token, ok := daemonBearerToken(r)
	if !ok || token != secret {
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
