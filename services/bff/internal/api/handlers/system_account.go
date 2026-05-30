package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	bffmiddleware "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// systemAccountReader is the minimal repo surface SystemAccountHandler needs.
type systemAccountReader interface {
	GetByUserID(ctx context.Context, userID int64) (*repository.AccountRow, bool, error)
}

// SystemAccountHandler handles GET /api/v1/system/account.
type SystemAccountHandler struct {
	repo systemAccountReader
}

// NewSystemAccountHandler returns a SystemAccountHandler backed by repo.
func NewSystemAccountHandler(repo systemAccountReader) *SystemAccountHandler {
	return &SystemAccountHandler{repo: repo}
}

// systemAccountResponse mirrors models.Account from frontend/src/types/models.ts.
// Field names are PascalCase so encoding/json emits them verbatim — the SPA's
// Account constructor reads source["ID"], source["Name"], etc. (PascalCase).
// Do NOT add snake_case json tags here; that would silently zero SPA fields.
type systemAccountResponse struct {
	ID           int64     `json:"ID"`
	Name         string    `json:"Name"`
	ScreenName   string    `json:"ScreenName"`
	ClientID     string    `json:"ClientID"`
	IsDefault    bool      `json:"IsDefault"`
	DailyWins    int       `json:"DailyWins"`
	WeeklyWins   int       `json:"WeeklyWins"`
	MasteryLevel int       `json:"MasteryLevel"`
	MasteryPass  string    `json:"MasteryPass"`
	MasteryMax   int       `json:"MasteryMax"`
	CreatedAt    time.Time `json:"CreatedAt"`
	UpdatedAt    time.Time `json:"UpdatedAt"`
}

// GetSystemAccount handles GET /api/v1/system/account.
//
// Returns 200 with the authenticated user's account data wrapped in the
// standard {"data": ...} envelope the SPA's apiClient unwraps.
// Returns 404 when the user has no account row yet (first-run state, before
// the daemon has paired — the SPA renders empty state on 404).
// Returns 401 when the user ID cannot be resolved from context.
func (h *SystemAccountHandler) GetSystemAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok || userID == 0 {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	acct, found, err := h.repo.GetByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[SystemAccountHandler] GetByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusNotFound)
		return
	}

	resp := systemAccountResponse{
		ID:           acct.ID,
		Name:         acct.Name,
		ScreenName:   acct.ScreenName.String,
		ClientID:     acct.ClientID.String,
		IsDefault:    acct.IsDefault != 0,
		DailyWins:    acct.DailyWins,
		WeeklyWins:   acct.WeeklyWins,
		MasteryLevel: acct.MasteryLevel,
		MasteryPass:  acct.MasteryPass.String,
		MasteryMax:   acct.MasteryMax,
		CreatedAt:    acct.CreatedAt,
		UpdatedAt:    acct.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]any{"data": resp}); err != nil {
		log.Printf("[SystemAccountHandler] encode: %v", err)
	}
}
