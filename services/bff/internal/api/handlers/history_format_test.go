package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
)

// TestGetMatches_FormatValidation exercises the format query-param validation
// on GET /api/v1/history/matches via IsKnownFormat (case-insensitive).
func TestGetMatches_FormatValidation(t *testing.T) {
	cases := []struct {
		name       string
		format     string
		wantStatus int
	}{
		// Known formats — exact case.
		{"standard", "standard", http.StatusOK},
		{"historic", "historic", http.StatusOK},
		{"brawl", "brawl", http.StatusOK},
		{"limited", "limited", http.StatusOK},
		{"draft", "draft", http.StatusOK},
		{"sealed", "sealed", http.StatusOK},
		{"alchemy", "alchemy", http.StatusOK},
		{"explorer", "explorer", http.StatusOK},
		{"timeless", "timeless", http.StatusOK},
		{"gladiator", "gladiator", http.StatusOK},
		{"pauper", "pauper", http.StatusOK},
		// Known formats — title-case (case-insensitive acceptance).
		{"Standard title-case", "Standard", http.StatusOK},
		{"Historic title-case", "Historic", http.StatusOK},
		{"Alchemy title-case", "Alchemy", http.StatusOK},
		{"TIMELESS all-caps", "TIMELESS", http.StatusOK},
		// Unknown formats — must be rejected.
		{"unknown vintage", "vintage", http.StatusBadRequest},
		{"unknown modern", "modern", http.StatusBadRequest},
		{"empty string (no filter)", "", http.StatusOK},
		{"garbage input", "notaformat", http.StatusBadRequest},
	}

	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{rows: nil}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/history/matches"
			if tc.format != "" {
				url += "?format=" + tc.format
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()
			authedMatchHandler(h, 1).ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("format=%q: want %d, got %d (body: %s)", tc.format, tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}
