package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
)

// daemonAPIKeyRevokeRepo is the subset of DaemonAPIKeyRepository used by
// DaemonsRevokeHandler.
type daemonAPIKeyRevokeRepo interface {
	RevokeByAccountIDAndDeviceID(ctx context.Context, accountID, deviceID string) (bool, error)
}

// DaemonsRevokeHandler handles DELETE /api/v1/daemons/{device_id}.
//
// Authenticated via Clerk session (RequireClerkAuth middleware must run
// first). Per ADR-031 §3:
//   - 204 No Content — exactly one row updated (soft-delete via revoked_at).
//   - 404 Not Found — zero rows match (device_id doesn't exist OR belongs to
//     another account_id OR already revoked). All three collapse to 404 to
//     prevent cross-tenant device_id enumeration. 403 is NOT used.
//   - 401 Unauthorized — Clerk middleware rejected the request before the
//     handler ran (or, defensively, ClerkUserIDFromContext returned empty).
//   - 400 Bad Request — malformed device_id path parameter (not a UUID).
type DaemonsRevokeHandler struct {
	repo daemonAPIKeyRevokeRepo
}

// NewDaemonsRevokeHandler returns a handler backed by the given repository.
func NewDaemonsRevokeHandler(repo daemonAPIKeyRevokeRepo) *DaemonsRevokeHandler {
	return &DaemonsRevokeHandler{repo: repo}
}

// Revoke handles DELETE /api/v1/daemons/{device_id}.
func (h *DaemonsRevokeHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.ClerkUserIDFromContext(r)
	if !ok || accountID == "" {
		log.Printf("[daemons_revoke] missing Clerk user ID — RequireClerkAuth not applied")
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	deviceID := strings.TrimSpace(chi.URLParam(r, "device_id"))
	if deviceID == "" {
		writeJSONError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(deviceID); err != nil {
		// Malformed device_id — reject before the repo is touched. The DB
		// column is typed UUID and would error on a non-UUID anyway; we
		// fail fast with 400 to keep the error shape clean.
		writeJSONError(w, "invalid device_id", http.StatusBadRequest)
		return
	}

	revoked, err := h.repo.RevokeByAccountIDAndDeviceID(r.Context(), accountID, deviceID)
	if err != nil {
		log.Printf("[daemons_revoke] RevokeByAccountIDAndDeviceID account=%s device=%s: %v", accountID, deviceID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !revoked {
		// 404 — collapses {non-existent, cross-tenant, already-revoked}
		// per ADR-031 §3 to prevent cross-tenant device_id enumeration.
		writeJSONError(w, "device not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
