package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// daemonHealthWindow is the look-back duration used to decide whether the
// daemon is connected.  A daemon_events row with received_at within this
// window means the daemon is actively polling.
//
// 5 minutes: the daemon sends a startup burst (collection, inventory, etc.)
// then goes quiet until the next MTGA activity.  60 s was too short —
// the icon turned red during normal idle gaps between game events.
// Once the daemon gains a periodic heartbeat event the window can shrink.
const daemonHealthWindow = 5 * time.Minute

// DaemonHealthChecker is the minimal interface DaemonHealthHandler needs.
type DaemonHealthChecker interface {
	HasRecentEventByUserID(ctx context.Context, userID int64, window time.Duration) (bool, error)
}

// DaemonHealthHandler handles GET /api/v1/health/daemon.
type DaemonHealthHandler struct {
	checker DaemonHealthChecker
}

// NewDaemonHealthHandler returns a DaemonHealthHandler backed by checker.
func NewDaemonHealthHandler(checker DaemonHealthChecker) *DaemonHealthHandler {
	return &DaemonHealthHandler{checker: checker}
}

// daemonHealthResponse is the JSON body for GET /api/v1/health/daemon.
type daemonHealthResponse struct {
	Status string `json:"status"` // "connected" or "disconnected"
}

// GetDaemonHealth handles GET /api/v1/health/daemon.
//
// Always returns 200.  The body status field is:
//   - "connected"    — a daemon_events row exists with received_at within 60 s
//   - "disconnected" — no recent row (daemon not running or not polling)
func (h *DaemonHealthHandler) GetDaemonHealth(w http.ResponseWriter, r *http.Request) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	connected, err := h.checker.HasRecentEventByUserID(r.Context(), userID, daemonHealthWindow)
	if err != nil {
		log.Printf("[DaemonHealthHandler] HasRecentEventByUserID userID=%d: %v", userID, err)
		// Internal error — treat as disconnected rather than surfacing 500.
		connected = false
	}

	status := "disconnected"
	if connected {
		status = "connected"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(daemonHealthResponse{Status: status}); err != nil {
		log.Printf("[DaemonHealthHandler] encode: %v", err)
	}
}
