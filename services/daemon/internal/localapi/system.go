// Phase 1 system endpoints — handlers for /api/v1/system/* that mirror the
// contract the SPA's daemonClient expects (frontend/src/services/api/system.ts).
//
// These are loopback-only and do not require auth — the daemon trusts every
// request that reaches its 127.0.0.1 listener. Payload shapes follow the
// TypeScript types defined in frontend/src/services/api/system.ts and
// frontend/src/types/models.ts (gui.ConnectionStatus, gui.Account, etc.).

package localapi

import (
	"net/http"
	"time"
)

// connectionStatusResponse mirrors gui.ConnectionStatus on the frontend.
// `connected` is true while the daemon process is up (we are answering),
// `mode` distinguishes live-log from playback (placeholder "live" for v0.3.x),
// `url` and `port` echo the cloud API the daemon is dispatching to.
type connectionStatusResponse struct {
	Status    string `json:"status"`
	Connected bool   `json:"connected"`
	Mode      string `json:"mode"`
	URL       string `json:"url"`
	Port      int    `json:"port"`
}

// daemonStatusResponse mirrors the SPA's DaemonStatus type. Returned by both
// /system/status and /system/daemon/status (the SPA polls both during setup).
type daemonStatusResponse struct {
	Status    string `json:"status"`
	Connected bool   `json:"connected"`
}

// versionInfoResponse mirrors VersionInfo. `service` is a stable identifier
// so the SPA can distinguish the daemon from other backends in the future.
type versionInfoResponse struct {
	Version string `json:"version"`
	Service string `json:"service"`
}

// healthStatusResponse mirrors HealthStatus on the frontend. For v0.3.x the
// daemon has no local DB / WebSocket so those sub-blocks report "n/a"; the
// SPA tolerates unknown sub-statuses.
type healthStatusResponse struct {
	Status     string                  `json:"status"`
	Version    string                  `json:"version"`
	Uptime     int64                   `json:"uptime"` // seconds
	Database   subsystemHealthResponse `json:"database"`
	LogMonitor subsystemHealthResponse `json:"logMonitor"`
	Websocket  websocketHealthResponse `json:"websocket"`
	Metrics    healthMetricsResponse   `json:"metrics"`
}

type subsystemHealthResponse struct {
	Status   string `json:"status"`
	LastRead string `json:"lastRead,omitempty"`
}

type websocketHealthResponse struct {
	Status           string `json:"status"`
	ConnectedClients int    `json:"connectedClients"`
}

type healthMetricsResponse struct {
	TotalProcessed  int64 `json:"totalProcessed"`
	TotalErrors     int64 `json:"totalErrors"`
	DispatchDropped int64 `json:"dispatchDropped"`
}

// accountResponse mirrors models.Account. For v0.3.x the daemon doesn't store
// MTGA account profile data (mastery, daily wins, etc.) locally — those come
// from the BFF via separate endpoints. We return a minimal stub so the SPA
// can render "no MTGA account paired yet" without crashing on null.
type accountResponse struct {
	ID           int64     `json:"ID"`
	Name         string    `json:"Name"`
	ScreenName   string    `json:"ScreenName,omitempty"`
	ClientID     string    `json:"ClientID,omitempty"`
	DailyWins    int       `json:"DailyWins"`
	WeeklyWins   int       `json:"WeeklyWins"`
	MasteryLevel int       `json:"MasteryLevel"`
	MasteryPass  string    `json:"MasteryPass"`
	MasteryMax   int       `json:"MasteryMax"`
	IsDefault    bool      `json:"IsDefault"`
	CreatedAt    time.Time `json:"CreatedAt"`
	UpdatedAt    time.Time `json:"UpdatedAt"`
}

// databasePathResponse — the daemon has no local DB in v0.3.x. Returning an
// empty path is the truthful answer and the SPA renders it as "(none)".
type databasePathResponse struct {
	Path string `json:"path"`
}

// statusOKResponse is the trivial body used by no-op POST endpoints
// (/daemon/connect, /daemon/disconnect) that exist only because the SPA
// expects to call them. The daemon manages its own lifecycle.
type statusOKResponse struct {
	Status string `json:"status"`
}

// handleSystemStatus reports the connection-to-cloud state. The SPA polls
// this on a tight loop to drive the connection indicator.
func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	st := s.snapshot()
	status := "connected"
	if !st.BFFReachable {
		status = "degraded"
	}
	writeJSON(w, r, http.StatusOK, connectionStatusResponse{
		Status:    status,
		Connected: true,
		Mode:      "live",
		URL:       st.CloudAPIURL,
		Port:      DefaultPort,
	})
}

// handleSystemDaemonStatus is the daemon-specific status (vs system-wide).
// For v0.3.x they are equivalent; kept as separate routes so the SPA's
// distinct DaemonStatus type continues to deserialize cleanly.
func (s *Server) handleSystemDaemonStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	writeJSON(w, r, http.StatusOK, daemonStatusResponse{
		Status:    "connected",
		Connected: true,
	})
}

func (s *Server) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	st := s.snapshot()

	logStatus := "ok"
	lastRead := ""
	if st.LastDispatchAt != nil {
		lastRead = st.LastDispatchAt.UTC().Format(time.RFC3339)
	}

	writeJSON(w, r, http.StatusOK, healthStatusResponse{
		Status:  "ok",
		Version: st.Version,
		Uptime:  int64(time.Since(st.StartedAt).Seconds()),
		Database: subsystemHealthResponse{
			Status: "n/a", // daemon has no local DB in v0.3.x
		},
		LogMonitor: subsystemHealthResponse{
			Status:   logStatus,
			LastRead: lastRead,
		},
		Websocket: websocketHealthResponse{
			Status:           "n/a",
			ConnectedClients: 0,
		},
		Metrics: healthMetricsResponse{
			DispatchDropped: st.DispatchDropped,
		},
	})
}

func (s *Server) handleSystemVersion(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	st := s.snapshot()
	writeJSON(w, r, http.StatusOK, versionInfoResponse{
		Version: st.Version,
		Service: "vaultmtg-daemon",
	})
}

// handleSystemAccount returns the placeholder MTGA account. Until the daemon
// tracks per-MTGA-account profile data locally (post-v0.3.x), this is
// effectively "no Arena account paired yet" but shaped so the SPA's strict
// type-checker accepts it.
func (s *Server) handleSystemAccount(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	now := time.Now().UTC()
	writeJSON(w, r, http.StatusOK, accountResponse{
		ID:        0,
		Name:      "",
		IsDefault: true,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// handleSystemDatabasePath answers GET with an empty path. POST is accepted
// as a no-op for forward-compat — the SPA's "change DB location" UI is moot
// in v0.3.x where there is no local DB.
func (s *Server) handleSystemDatabasePath(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		writeJSON(w, r, http.StatusOK, databasePathResponse{Path: ""})
	case http.MethodPost:
		writeJSON(w, r, http.StatusOK, statusOKResponse{Status: "ok"})
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleSystemDaemonConnect / Disconnect are no-ops. The daemon manages its
// own lifecycle (started by LaunchAgent, stopped by SIGTERM); these endpoints
// exist only because the SPA expects to call them.
func (s *Server) handleSystemDaemonConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, r, http.StatusOK, statusOKResponse{Status: "ok"})
}

func (s *Server) handleSystemDaemonDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, r, http.StatusOK, statusOKResponse{Status: "ok"})
}
