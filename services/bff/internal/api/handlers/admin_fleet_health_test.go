package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── stub fleet health snapshot provider ──────────────────────────────────────

type stubFleetHealthRepo struct {
	snap repository.FleetHealthSnapshot
	err  error
}

func (s *stubFleetHealthRepo) FleetHealthSnapshot(_ context.Context) (repository.FleetHealthSnapshot, error) {
	return s.snap, s.err
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestAdminFleetHealthHandler_Returns200WithCorrectShape(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snap := repository.FleetHealthSnapshot{
		TotalPaired:  10,
		ActiveLast5m: 3,
		ActiveLast1h: 7,
		Revoked:      2,
		AsOf:         now,
	}

	h := handlers.NewAdminFleetHealthHandler(&stubFleetHealthRepo{snap: snap})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/daemons/fleet-health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))

	assert.EqualValues(t, 10, body["total_paired"])
	assert.EqualValues(t, 3, body["active_last_5m"])
	assert.EqualValues(t, 7, body["active_last_1h"])
	assert.EqualValues(t, 2, body["revoked"])
	assert.NotEmpty(t, body["as_of"])
}

func TestAdminFleetHealthHandler_RepoError_Returns500(t *testing.T) {
	h := handlers.NewAdminFleetHealthHandler(&stubFleetHealthRepo{
		err: errors.New("db exploded"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/daemons/fleet-health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestAdminFleetHealthHandler_ResponseContainsNoAccountIDs(t *testing.T) {
	// PII guard: the response body must not contain any key called "account_id"
	// or "accounts" — the endpoint returns only aggregate counts.
	now := time.Now().UTC()
	snap := repository.FleetHealthSnapshot{
		TotalPaired:  5,
		ActiveLast5m: 1,
		ActiveLast1h: 3,
		Revoked:      1,
		AsOf:         now,
	}

	h := handlers.NewAdminFleetHealthHandler(&stubFleetHealthRepo{snap: snap})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/daemons/fleet-health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))

	_, hasAccountID := body["account_id"]
	_, hasAccounts := body["accounts"]
	assert.False(t, hasAccountID, "response must not contain account_id")
	assert.False(t, hasAccounts, "response must not contain accounts array")
}
