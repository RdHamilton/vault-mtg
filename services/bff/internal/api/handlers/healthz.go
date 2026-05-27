package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Pinger is a minimal interface that wraps the single DB method used by
// HealthzHandler.  *sql.DB satisfies it; tests supply a lightweight fake.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthzHandler handles GET /healthz.
//
// This endpoint is intentionally public (no auth required) so that staging
// deploy health checks and uptime monitors can reach it without a Clerk token.
type HealthzHandler struct {
	env             string
	db              Pinger
	embeddedVersion string
}

// NewHealthzHandler returns a HealthzHandler.
//
//   - env             — value of cfg.Env (e.g. "staging", "production")
//   - db              — shared *sql.DB pool injected at startup; nil in development
//   - embeddedVersion — pre-computed value from storage.EmbeddedMaxVersion()
func NewHealthzHandler(env string, db Pinger, embeddedVersion string) *HealthzHandler {
	return &HealthzHandler{
		env:             env,
		db:              db,
		embeddedVersion: embeddedVersion,
	}
}

// healthzResponse is the JSON body returned by GET /healthz.
type healthzResponse struct {
	Status           string `json:"status"`
	Env              string `json:"env"`
	MigrationVersion string `json:"migration_version"`
}

// ServeHTTP handles GET /healthz.
//
// Always returns 200.  The migration_version field is the highest embedded
// migration version when the DB is reachable; "unknown" otherwise.
func (h *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	migrationVersion := h.embeddedVersion

	if h.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := h.db.PingContext(ctx); err != nil {
			log.Printf("[HealthzHandler] db ping: %v", err)
			migrationVersion = "unknown"
		}
	} else {
		migrationVersion = "unknown"
	}

	resp := healthzResponse{
		Status:           "ok",
		Env:              h.env,
		MigrationVersion: migrationVersion,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[HealthzHandler] encode: %v", err)
	}
}
