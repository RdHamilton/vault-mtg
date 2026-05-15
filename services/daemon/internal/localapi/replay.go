// Replay endpoint — POST /api/v1/replay.
//
// Triggers a one-shot historical log replay on the daemon side.  The handler
// accepts an optional { "clearDataFirst": true } JSON body, responds
// immediately with 202 Accepted, and fires the replay in a background
// goroutine. Progress is reported via the BFF SSE stream as
// replay:started / replay:progress / replay:completed / replay:error events.
//
// The actual replay work is performed by a ReplayFunc injected via
// SetReplayTrigger.  When no trigger is set (e.g. the daemon is not fully
// initialised) the endpoint returns 503 Service Unavailable.

package localapi

import (
	"context"
	"encoding/json"
	"net/http"
)

// ReplayFunc is a callback the daemon service registers so the localapi server
// can trigger a log replay without importing the daemon package (import cycle
// avoidance).  The function must not block — implementations should start a
// goroutine internally.  The ctx is derived from the server's lifecycle and is
// cancelled when the daemon stops.
type ReplayFunc func(ctx context.Context, clearDataFirst bool)

// SetReplayTrigger wires the callback that POST /api/v1/replay invokes.
// Call this from daemon.Service.Run before the localapi server has a chance to
// receive replay requests.  Passing nil clears the trigger (makes the endpoint
// return 503).
func (s *Server) SetReplayTrigger(fn ReplayFunc) {
	s.replayTrigger = fn
}

// replayRequest mirrors the JSON body accepted by POST /api/v1/replay.
type replayRequest struct {
	ClearDataFirst bool `json:"clearDataFirst"`
}

// replayAcceptedResponse is the 202 body the endpoint sends to the caller
// before firing the replay goroutine.
type replayAcceptedResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// handleReplay handles POST /api/v1/replay.
//
// Behaviour:
//   - 405 when the method is not POST or OPTIONS.
//   - 503 when no ReplayFunc has been registered.
//   - 400 when the request body is present but not valid JSON.
//   - 202 otherwise: the replay goroutine is started and the response is sent
//     immediately so the caller is not blocked waiting for replay to finish.
func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.replayTrigger == nil {
		writeJSON(w, r, http.StatusServiceUnavailable, struct {
			Error string `json:"error"`
		}{"replay not available — daemon not fully initialised"})
		return
	}

	var req replayRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, r, http.StatusBadRequest, struct {
				Error string `json:"error"`
			}{"invalid request body: " + err.Error()})
			return
		}
	}

	// Fire the replay in a separate goroutine so the HTTP response is sent
	// immediately (202 Accepted).  The caller drives progress via the BFF SSE
	// stream (replay:started, replay:progress, replay:completed, replay:error).
	go s.replayTrigger(s.ctx, req.ClearDataFirst)

	writeJSON(w, r, http.StatusAccepted, replayAcceptedResponse{
		Status:  "accepted",
		Message: "log replay started; progress available via SSE replay:* events",
	})
}
