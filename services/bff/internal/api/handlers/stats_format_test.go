package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetRankProgression_FormatValidation exercises the format query-param
// validation on GET /api/v1/stats/rank-progression.
// Before this ticket the lookup used knownFormats[format] without lowercasing,
// so "Standard" was incorrectly rejected. IsKnownFormat fixes that.
func TestGetRankProgression_FormatValidation(t *testing.T) {
	cases := []struct {
		name       string
		format     string
		wantStatus int
	}{
		// Known formats — lowercase.
		{"standard", "standard", http.StatusOK},
		{"historic", "historic", http.StatusOK},
		{"alchemy", "alchemy", http.StatusOK},
		{"timeless", "timeless", http.StatusOK},
		// Known formats — title-case (case-insensitive acceptance, bug fix).
		{"Standard title-case", "Standard", http.StatusOK},
		{"Historic title-case", "Historic", http.StatusOK},
		{"ALCHEMY all-caps", "ALCHEMY", http.StatusOK},
		// No format param — should be accepted (no filter applied).
		{"no format param", "", http.StatusOK},
		// Unknown formats — must be rejected.
		{"unknown format", "vintage", http.StatusBadRequest},
		{"garbage", "xyz", http.StatusBadRequest},
	}

	h := newFullStatsHandler(
		&stubAccountLookup{found: true, accountID: 10},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
		&stubDraftAnalyticsReader{},
		&stubRankProgressionReader{},
		&stubResultBreakdownReader{},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/stats/rank-progression"
			if tc.format != "" {
				url += "?format=" + tc.format
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()
			authedStatsHandler(h.GetRankProgression, 1).ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("format=%q: want %d, got %d (body: %s)", tc.format, tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

// TestGetResultBreakdown_FormatValidation exercises the format query-param
// validation on GET /api/v1/stats/result-breakdown.
// Before this ticket the lookup used knownFormats[format] without lowercasing,
// so "Standard" was incorrectly rejected. IsKnownFormat fixes that.
func TestGetResultBreakdown_FormatValidation(t *testing.T) {
	cases := []struct {
		name       string
		format     string
		wantStatus int
	}{
		// Known formats — lowercase.
		{"standard", "standard", http.StatusOK},
		{"explorer", "explorer", http.StatusOK},
		{"gladiator", "gladiator", http.StatusOK},
		// Known formats — title-case (case-insensitive acceptance, bug fix).
		{"Standard title-case", "Standard", http.StatusOK},
		{"Explorer title-case", "Explorer", http.StatusOK},
		{"GLADIATOR all-caps", "GLADIATOR", http.StatusOK},
		// No format param — should be accepted (no filter applied).
		{"no format param", "", http.StatusOK},
		// Unknown formats — must be rejected.
		{"unknown format", "pioneer", http.StatusBadRequest},
		{"garbage", "bad_format", http.StatusBadRequest},
	}

	h := newFullStatsHandler(
		&stubAccountLookup{found: true, accountID: 10},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
		&stubDraftAnalyticsReader{},
		&stubRankProgressionReader{},
		&stubResultBreakdownReader{},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/stats/result-breakdown"
			if tc.format != "" {
				url += "?format=" + tc.format
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()
			authedStatsHandler(h.GetResultBreakdown, 1).ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("format=%q: want %d, got %d (body: %s)", tc.format, tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}
