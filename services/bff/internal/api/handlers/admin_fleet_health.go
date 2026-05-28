package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// fleetHealthSnapshotter is the minimal interface required by
// AdminFleetHealthHandler. It returns aggregate daemon key counts with no
// per-user data — satisfied by *repository.DaemonAPIKeyRepository.
type fleetHealthSnapshotter interface {
	FleetHealthSnapshot(ctx context.Context) (repository.FleetHealthSnapshot, error)
}

// AdminFleetHealthHandler serves GET /api/v1/admin/daemons/fleet-health.
//
// Returns a JSON object with aggregate daemon registration counts. All fields
// are aggregate-only — no PII, no per-account data. Protected by
// AdminTokenAuth middleware (static Bearer token from SSM
// /vaultmtg/app/production/bff-admin-token).
type AdminFleetHealthHandler struct {
	repo fleetHealthSnapshotter
}

// NewAdminFleetHealthHandler returns an AdminFleetHealthHandler backed by repo.
func NewAdminFleetHealthHandler(repo fleetHealthSnapshotter) *AdminFleetHealthHandler {
	return &AdminFleetHealthHandler{repo: repo}
}

// fleetHealthResponse is the JSON body returned by GET
// /api/v1/admin/daemons/fleet-health. All fields are aggregate counts — zero
// PII.
type fleetHealthResponse struct {
	TotalPaired  int       `json:"total_paired"`
	ActiveLast5m int       `json:"active_last_5m"`
	ActiveLast1h int       `json:"active_last_1h"`
	Revoked      int       `json:"revoked"`
	AsOf         time.Time `json:"as_of"`
}

// ServeHTTP handles GET /api/v1/admin/daemons/fleet-health.
func (h *AdminFleetHealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snap, err := h.repo.FleetHealthSnapshot(r.Context())
	if err != nil {
		log.Printf("[admin_fleet_health] FleetHealthSnapshot: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := fleetHealthResponse{
		TotalPaired:  snap.TotalPaired,
		ActiveLast5m: snap.ActiveLast5m,
		ActiveLast1h: snap.ActiveLast1h,
		Revoked:      snap.Revoked,
		AsOf:         snap.AsOf,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[admin_fleet_health] encode: %v", err)
	}
}
