package websocket

import (
	"encoding/json"
	"log"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
)

// DaemonEventForwarder forwards daemon events to the API server's WebSocket hub.
// This bridges the gap between the daemon (which runs on a separate port)
// and the frontend (which connects to the API server's WebSocket).
type DaemonEventForwarder struct {
	hub *Hub
}

// NewDaemonEventForwarder creates a new forwarder that sends events to the given hub.
func NewDaemonEventForwarder(hub *Hub) *DaemonEventForwarder {
	return &DaemonEventForwarder{hub: hub}
}

// ForwardEvent forwards a daemon event to all connected WebSocket clients.
// The event parameter must be a contract.DaemonEvent. The interface{} type is
// retained to satisfy the daemon.EventForwarder interface without an import
// cycle between the api and daemon packages. No reflection is used; the
// value is decoded via a JSON round-trip into the strongly-typed envelope.
func (f *DaemonEventForwarder) ForwardEvent(event interface{}) {
	var daemonEvent contract.DaemonEvent

	switch v := event.(type) {
	case contract.DaemonEvent:
		daemonEvent = v
	case *contract.DaemonEvent:
		if v == nil {
			log.Printf("[DaemonForwarder] Warning: received nil *contract.DaemonEvent")
			return
		}
		daemonEvent = *v
	default:
		// Fall back to a JSON round-trip for callers that still pass the old
		// daemon.Event struct. This path will be removed once the daemon is
		// migrated to emit contract.DaemonEvent directly.
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("[DaemonForwarder] Warning: cannot marshal event %T: %v", event, err)
			return
		}
		if err := json.Unmarshal(data, &daemonEvent); err != nil {
			log.Printf("[DaemonForwarder] Warning: cannot decode event %T into DaemonEvent: %v", event, err)
			return
		}
	}

	if daemonEvent.Type == "" {
		log.Printf("[DaemonForwarder] Warning: received event with empty Type")
		return
	}

	wsEvent := Event{
		Type: daemonEvent.Type,
		Data: daemonEvent,
	}
	f.hub.BroadcastEvent(wsEvent)
	log.Printf("[DaemonForwarder] Forwarded event %s to %d clients", wsEvent.Type, f.hub.ClientCount())
}
