// Phase 2 PR #12 — user_settings handler.
//
// Replaces the SPA's daemonClient surface for settings.ts. Mounts four
// routes under DaemonAPIKeyAuth, account-scoped through the standard
// AccountLookup:
//
//   GET  /api/v1/settings              entire settings map
//   PUT  /api/v1/settings              replace many (request body object)
//   GET  /api/v1/settings/{key}        single value
//   PUT  /api/v1/settings/{key}        single value (body: {"value": ...})
//
// Storage is JSONB key/value (see migration 000076), so any JSON-encodable
// type lands intact. The SPA's AppSettings constructor applies defaults
// for absent keys — the BFF never inlines that list.

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// settingsReader is the minimal repository surface SettingsHandler needs.
type settingsReader interface {
	ListByAccount(ctx context.Context, accountID int64) (map[string]json.RawMessage, error)
	Get(ctx context.Context, accountID int64, key string) (json.RawMessage, bool, error)
	Upsert(ctx context.Context, accountID int64, key string, value json.RawMessage) error
	UpsertMany(ctx context.Context, accountID int64, values map[string]json.RawMessage) error
}

// SettingsHandler serves /api/v1/settings[/{key}].
type SettingsHandler struct {
	settings settingsReader
	accounts AccountLookup
}

// NewSettingsHandler wires the handler with its dependencies.
func NewSettingsHandler(s settingsReader, accounts AccountLookup) *SettingsHandler {
	return &SettingsHandler{settings: s, accounts: accounts}
}

// updateSingleRequest mirrors the SPA's PUT /settings/{key} body.
type updateSingleRequest struct {
	Value json.RawMessage `json:"value"`
}

// settingsValueResponse wraps a single setting's value for GET /settings/{key}.
// Returning a structured envelope (rather than the raw scalar) keeps the
// shape JSON-friendly for primitives without quoting hacks.
type settingsValueResponse struct {
	Value json.RawMessage `json:"value"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// GetSettings handles GET /api/v1/settings. Returns an object of every
// key/value pair set for the authenticated account. Missing keys are
// absent — the SPA applies defaults.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetSettings")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, map[string]json.RawMessage{})
		return
	}
	values, err := h.settings.ListByAccount(r.Context(), accountID)
	if err != nil {
		log.Printf("[SettingsHandler.GetSettings] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, values)
}

// UpdateSettings handles PUT /api/v1/settings. The request body is the
// full AppSettings object — every field becomes a (key, value) row.
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "UpdateSettings")
	if !ok {
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSONError(w, "request body too large", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()
	if len(body) == 0 {
		writeJSONError(w, "request body is required", http.StatusBadRequest)
		return
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(body, &values); err != nil {
		writeJSONError(w, "request body must be a JSON object", http.StatusBadRequest)
		return
	}
	if err := h.settings.UpsertMany(r.Context(), accountID, values); err != nil {
		log.Printf("[SettingsHandler.UpdateSettings] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSetting handles GET /api/v1/settings/{key}. Returns `{"value": ...}`
// or 404 when the key is unset.
func (h *SettingsHandler) GetSetting(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetSetting")
	if !ok {
		return
	}
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if key == "" {
		writeJSONError(w, "key is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "setting not found", http.StatusNotFound)
		return
	}
	value, present, err := h.settings.Get(r.Context(), accountID, key)
	if err != nil {
		log.Printf("[SettingsHandler.GetSetting] accountID=%d key=%s: %v", accountID, key, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !present {
		writeJSONError(w, "setting not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, settingsValueResponse{Value: value})
}

// UpdateSetting handles PUT /api/v1/settings/{key}. Body shape:
// {"value": <any JSON>}.
func (h *SettingsHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "UpdateSetting")
	if !ok {
		return
	}
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if key == "" {
		writeJSONError(w, "key is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusNotFound)
		return
	}
	var req updateSingleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Value) == 0 {
		writeJSONError(w, "value is required", http.StatusBadRequest)
		return
	}
	if err := h.settings.Upsert(r.Context(), accountID, key, req.Value); err != nil {
		log.Printf("[SettingsHandler.UpdateSetting] accountID=%d key=%s: %v", accountID, key, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *SettingsHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[SettingsHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}
