// Package localapi serves a small HTTP API on localhost so the VaultMTG SPA
// can detect that the daemon is running and (eventually) read live state.
//
// The server binds to 127.0.0.1 only — never an external interface — and
// listens on port 9001 by default. The SPA polls /health to drive the
// "daemon connected" indicator on Setup.tsx; future phases will add system
// status and proxy endpoints (see docs/product/milestones/v0.3.1/daemon-local-api-plan.md).
package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RdHamilton/MTGA-Companion/pkg/draftalgo"
)

// DefaultPort is the loopback TCP port the daemon's local HTTP API listens on.
// Hardcoded in both daemon and SPA (frontend/src/pages/Setup.tsx) to avoid a
// discovery handshake; users do not configure this.
const DefaultPort = 9001

// shutdownTimeout caps how long the local API server takes to drain on stop.
const shutdownTimeout = 5 * time.Second

// State is the subset of daemon state exposed by the local API. Some fields
// (Version, SessionID, StartedAt, AccountID, CloudAPIURL) are stable for the
// life of the daemon process; others (LastDispatchAt, BFFReachable) change as
// the daemon dispatches events. The daemon refreshes them via SetState.
type State struct {
	Version        string
	SessionID      string
	StartedAt      time.Time
	AccountID      string
	CloudAPIURL    string
	LastDispatchAt *time.Time
	BFFReachable   bool
}

// Server is the loopback HTTP server. Construct with New, then call Start
// before the daemon enters its main run loop and Stop on shutdown.
//
// State is held behind an atomic pointer so concurrent reads from HTTP
// handlers and writes from the daemon's dispatch goroutine are safe without
// a mutex on the hot path.
type Server struct {
	port          int
	state         atomic.Pointer[State]
	srv           *http.Server
	ln            net.Listener
	ctx           context.Context         // lifecycle context; cancelled when the daemon stops
	uninstaller   Uninstaller             // nil → defaultUninstaller; tests override via SetUninstaller
	draftStore    DraftStore              // nil → draft endpoints respond with empty/no-session
	cardsLookup   draftalgo.CardLookup    // nil → noopCards; defaults applied lazily in drafts.go
	ratingsLookup draftalgo.RatingsLookup // nil → noopRatings; defaults applied lazily in drafts.go
	replayTrigger ReplayFunc              // nil → /api/v1/replay returns 503
}

// New returns a Server bound to 127.0.0.1:port. Use DefaultPort unless tests
// need an ephemeral port (pass 0 to let the OS pick).
func New(port int, state State) *Server {
	s := &Server{port: port, ctx: context.Background()}
	s.state.Store(&state)
	return s
}

// SetState atomically replaces the published state snapshot. Callers should
// always pass a complete State (Snapshot pattern); the server does not merge
// partial updates.
func (s *Server) SetState(state State) {
	s.state.Store(&state)
}

// WithContext attaches the given context as the server lifecycle context.
// Call this before Start so that long-running background work (e.g. replay
// goroutines) can be cancelled when the daemon shuts down rather than relying
// on the short-lived HTTP request context.
func (s *Server) WithContext(ctx context.Context) *Server {
	s.ctx = ctx
	return s
}

// SetUninstaller installs a custom Uninstaller. Used by tests to swap in a
// fake; production never calls this (the default implementation is wired in
// handleSystemUninstall when uninstaller is nil).
func (s *Server) SetUninstaller(u Uninstaller) {
	s.uninstaller = u
}

// SetDraftStore wires the in-memory draft session store the daemon
// maintains (see services/daemon/internal/draftstate). When nil, the
// /api/v1/drafts/* live-state endpoints return 404 / empty payloads.
func (s *Server) SetDraftStore(store DraftStore) {
	s.draftStore = store
}

// SetDraftLookups installs the card-name + 17Lands ratings lookups the
// draft handlers use. Production wires this after fetching cached BFF
// data; tests inject deterministic stubs. nil values fall back to the
// noopCards / noopRatings defaults in drafts.go.
func (s *Server) SetDraftLookups(cards draftalgo.CardLookup, ratings draftalgo.RatingsLookup) {
	s.cardsLookup = cards
	s.ratingsLookup = ratings
}

// snapshot returns a copy of the current state. Always non-nil for a Server
// that was constructed via New.
func (s *Server) snapshot() State {
	if p := s.state.Load(); p != nil {
		return *p
	}
	return State{}
}

// Start binds the listener and serves in a background goroutine. Returns once
// the listener is accepting connections, so callers can rely on /health being
// reachable as soon as Start returns nil.
//
// CORS: every response includes Access-Control-Allow-Origin: * because the
// SPA is served from a different origin (e.g. https://stg-app.vaultmtg.app)
// and browser fetch() to localhost requires CORS even though the daemon
// binary is local. The data exposed here is non-sensitive liveness info.
func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("localapi: listen %s: %w", addr, err)
	}
	s.ln = ln

	mux := http.NewServeMux()
	// Liveness — kept at the root for the SPA's hardcoded Setup.tsx probe.
	mux.HandleFunc("/health", s.handleHealth)

	// Phase 1 — system endpoints under /api/v1/system/* mirroring the contract
	// the SPA's daemonClient expects (frontend/src/services/api/system.ts).
	mux.HandleFunc("/api/v1/system/status", s.handleSystemStatus)
	mux.HandleFunc("/api/v1/system/health", s.handleSystemHealth)
	mux.HandleFunc("/api/v1/system/version", s.handleSystemVersion)
	mux.HandleFunc("/api/v1/system/account", s.handleSystemAccount)
	mux.HandleFunc("/api/v1/system/database/path", s.handleSystemDatabasePath)
	mux.HandleFunc("/api/v1/system/daemon/status", s.handleSystemDaemonStatus)
	mux.HandleFunc("/api/v1/system/daemon/connect", s.handleSystemDaemonConnect)
	mux.HandleFunc("/api/v1/system/daemon/disconnect", s.handleSystemDaemonDisconnect)

	// Phase 2 PR #18 — uninstall surface. Lets the SPA's Settings UI
	// trigger a clean uninstall without forcing the user into a Terminal.
	mux.HandleFunc("/api/v1/system/uninstall", s.handleSystemUninstall)

	// Phase 2 PR #17b — live draft state. Endpoints answer from the
	// daemon's in-memory draftstate.Store (populated by the log entry
	// consumer in services/daemon/internal/daemon). Retire the BFF
	// stubs (PR #14) when this lands.
	mux.HandleFunc("/api/v1/drafts/grade-pick", s.handleDraftGradePick)
	mux.HandleFunc("/api/v1/drafts/win-probability", s.handleDraftWinProbability)
	mux.HandleFunc("/api/v1/drafts/", s.handleDraftsPathPrefix)

	// Data Recovery — triggers a historical log replay.  Progress is
	// reported via the BFF SSE stream as replay:* events.
	mux.HandleFunc("/api/v1/replay", s.handleReplay)

	s.srv = &http.Server{
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[localapi] serve error: %v", err)
		}
	}()

	log.Printf("[localapi] listening on http://%s", ln.Addr().String())
	return nil
}

// Addr returns the bound TCP address (host:port). Useful for tests that pass
// port=0 and need to know the OS-assigned port.
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Stop drains in-flight requests and closes the listener. Safe to call before
// Start has been called (no-op) or multiple times.
func (s *Server) Stop() error {
	if s.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

// healthResponse is the JSON body returned by GET /health.
type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	SessionID string `json:"session_id"`
	StartedAt string `json:"started_at"`
	AccountID string `json:"account_id,omitempty"`
}

// handleHealth returns the daemon's liveness snapshot. The "status" field is
// always "ok" while the server is running — if the server is down the SPA's
// fetch fails outright, which is the actual offline signal.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}

	st := s.snapshot()
	resp := healthResponse{
		Status:    "ok",
		Version:   st.Version,
		SessionID: st.SessionID,
		StartedAt: st.StartedAt.UTC().Format(time.RFC3339),
		AccountID: st.AccountID,
	}
	writeJSON(w, r, http.StatusOK, resp)
}

// requireGet returns true when the request is a GET/HEAD. Otherwise it writes
// 405 and returns false so the caller can early-return.
func (s *Server) requireGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	w.WriteHeader(http.StatusMethodNotAllowed)
	return false
}

// writeJSON serializes payload as JSON with the given status. HEAD requests
// get just the headers + status.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// withCORS wraps a handler with permissive CORS headers. The daemon serves
// only loopback traffic so the value of Access-Control-Allow-Origin is not a
// security boundary — the firewall (binding 127.0.0.1) is.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the origin should be echoed back in
// Access-Control-Allow-Origin. We allow the production + staging SPAs and any
// http(s)://localhost:* origin (local dev). Other origins still get "*" which
// is also acceptable for non-credentialed loopback traffic.
func isAllowedOrigin(origin string) bool {
	allow := []string{
		"https://app.vaultmtg.app",
		"https://stg-app.vaultmtg.app",
		"https://vaultmtg.app",
		"https://www.vaultmtg.app",
	}
	for _, o := range allow {
		if origin == o {
			return true
		}
	}
	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "https://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:")
}
