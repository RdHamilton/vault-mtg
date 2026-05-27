package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// daemonAPIKeyListRepo is the subset of DaemonAPIKeyRepository used by
// DaemonsListHandler. Defined here so the handler can be unit-tested with a
// stub without dragging in the full repo.
type daemonAPIKeyListRepo interface {
	ListByAccountID(ctx context.Context, accountID string) ([]repository.DaemonAPIKey, error)
}

// DaemonsListHandler handles GET /api/v1/daemons.
//
// Authenticated via Clerk session (RequireClerkAuth middleware must run
// first). Returns the caller's own active daemon registrations. Per
// ADR-031 §4: cross-tenancy is enforced by the SQL WHERE clause; sensitive
// columns (key_hash, key_prefix, internal id) are never projected.
type DaemonsListHandler struct {
	repo daemonAPIKeyListRepo
}

// NewDaemonsListHandler returns a handler backed by the given repository.
func NewDaemonsListHandler(repo daemonAPIKeyListRepo) *DaemonsListHandler {
	return &DaemonsListHandler{repo: repo}
}

// daemonsListResponseDevice is the per-device entry in the GET /v1/daemons
// response. Field set is explicit and minimal — only the columns ADR-031 §4
// names. Sensitive columns are not even present in this struct so they
// cannot leak by accident.
type daemonsListResponseDevice struct {
	DeviceID   string     `json:"device_id"`
	Platform   string     `json:"platform"`
	DaemonVer  string     `json:"daemon_ver"`
	PairedAt   time.Time  `json:"paired_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// daemonsListResponse is the JSON response shape for GET /api/v1/daemons.
type daemonsListResponse struct {
	Devices []daemonsListResponseDevice `json:"devices"`
}

// List handles GET /api/v1/daemons. RequireClerkAuth middleware must run
// first; the handler extracts the caller's Clerk user_id from context and
// uses it as the SQL WHERE clause scoping value.
func (h *DaemonsListHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.ClerkUserIDFromContext(r)
	if !ok || accountID == "" {
		log.Printf("[daemons_list] missing Clerk user ID — RequireClerkAuth not applied")
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	keys, err := h.repo.ListByAccountID(r.Context(), accountID)
	if err != nil {
		log.Printf("[daemons_list] ListByAccountID account=%s: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	devices := make([]daemonsListResponseDevice, 0, len(keys))
	for _, k := range keys {
		devices = append(devices, daemonsListResponseDevice{
			DeviceID:   k.DeviceID,
			Platform:   k.Platform,
			DaemonVer:  k.DaemonVer,
			PairedAt:   k.PairedAt,
			LastUsedAt: k.LastUsedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(daemonsListResponse{Devices: devices}); err != nil {
		log.Printf("[daemons_list] encode: %v", err)
	}
}
