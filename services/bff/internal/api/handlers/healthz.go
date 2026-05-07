package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// MigrationChecker is the function signature used by HealthzHandler to check
// whether the database schema is up-to-date.  Injected at construction time so
// tests can supply a stub without a real database.
type MigrationChecker func(databaseURL string) string

// HealthzHandler handles GET /healthz.
//
// This endpoint is intentionally public (no auth required) so that staging
// deploy health checks and uptime monitors can reach it without a Clerk token.
type HealthzHandler struct {
	env            string
	databaseURL    string
	checkMigration MigrationChecker
}

// NewHealthzHandler returns a HealthzHandler.
//
//   - env         — value of cfg.Env (e.g. "staging", "production")
//   - databaseURL — value of cfg.DatabaseURL; may be empty in development
//   - checker     — called to obtain the migration status string
func NewHealthzHandler(env, databaseURL string, checker MigrationChecker) *HealthzHandler {
	return &HealthzHandler{
		env:            env,
		databaseURL:    databaseURL,
		checkMigration: checker,
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
// Always returns 200.  The migration_version field is "up-to-date" when the
// DB is reachable and at the latest schema version; "unknown" otherwise.
func (h *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	migrations := h.checkMigration(h.databaseURL)

	resp := healthzResponse{
		Status:           "ok",
		Env:              h.env,
		MigrationVersion: migrations,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[HealthzHandler] encode: %v", err)
	}
}
