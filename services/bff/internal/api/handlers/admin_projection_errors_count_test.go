package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── stub projection errors counter ──────────────────────────────────────────

type stubProjectionErrorsCounter struct {
	count int64
	err   error
}

func (s *stubProjectionErrorsCounter) CountProjectionErrors(_ context.Context) (int64, error) {
	return s.count, s.err
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestAdminProjectionErrorsCountHandler_Returns200WithCount(t *testing.T) {
	h := handlers.NewAdminProjectionErrorsCountHandler(&stubProjectionErrorsCounter{count: 7})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projection-errors/count", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))

	assert.EqualValues(t, 7, body["count"])
}

func TestAdminProjectionErrorsCountHandler_ZeroCount(t *testing.T) {
	h := handlers.NewAdminProjectionErrorsCountHandler(&stubProjectionErrorsCounter{count: 0})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projection-errors/count", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))

	assert.EqualValues(t, 0, body["count"])
}

func TestAdminProjectionErrorsCountHandler_RepoError_Returns500(t *testing.T) {
	h := handlers.NewAdminProjectionErrorsCountHandler(&stubProjectionErrorsCounter{
		err: errors.New("db exploded"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projection-errors/count", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestAdminProjectionErrorsCountHandler_ResponseContainsOnlyCount(t *testing.T) {
	// Shape guard: the response body must contain exactly the "count" key —
	// no PII, no per-account data, no stray fields.
	h := handlers.NewAdminProjectionErrorsCountHandler(&stubProjectionErrorsCounter{count: 3})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projection-errors/count", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))

	_, hasCount := body["count"]
	assert.True(t, hasCount, "response must contain count")

	_, hasAccountID := body["account_id"]
	assert.False(t, hasAccountID, "response must not contain account_id")

	assert.Len(t, body, 1, "response must contain exactly one field")
}
