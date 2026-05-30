package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

// projectionErrorsCounter is the minimal interface required by
// AdminProjectionErrorsCountHandler. Satisfied by
// *repository.ProjectionErrorsRepository.
type projectionErrorsCounter interface {
	CountProjectionErrors(ctx context.Context) (int64, error)
}

// AdminProjectionErrorsCountHandler serves
// GET /api/v1/admin/projection-errors/count.
//
// Returns a JSON object {"count": N} where N is the total number of rows in
// the projection_errors dead-letter table. Protected by AdminTokenAuth
// middleware (static Bearer token from SSM
// /vaultmtg/app/production/bff-admin-token). Global count — no per-account
// scoping — because the endpoint is admin-only and the DLQ is an operational
// view across all tenants.
type AdminProjectionErrorsCountHandler struct {
	repo projectionErrorsCounter
}

// NewAdminProjectionErrorsCountHandler returns an
// AdminProjectionErrorsCountHandler backed by repo.
func NewAdminProjectionErrorsCountHandler(repo projectionErrorsCounter) *AdminProjectionErrorsCountHandler {
	return &AdminProjectionErrorsCountHandler{repo: repo}
}

// projectionErrorsCountResponse is the JSON body returned by
// GET /api/v1/admin/projection-errors/count.
type projectionErrorsCountResponse struct {
	Count int64 `json:"count"`
}

// ServeHTTP handles GET /api/v1/admin/projection-errors/count.
func (h *AdminProjectionErrorsCountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n, err := h.repo.CountProjectionErrors(r.Context())
	if err != nil {
		log.Printf("[admin_projection_errors_count] CountProjectionErrors: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(projectionErrorsCountResponse{Count: n}); err != nil {
		log.Printf("[admin_projection_errors_count] encode: %v", err)
	}
}
