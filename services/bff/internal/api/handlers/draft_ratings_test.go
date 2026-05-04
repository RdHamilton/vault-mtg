package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/config"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// stubRatingsGetter is a test double for DraftRatingsGetter.
type stubRatingsGetter struct {
	result *repository.DraftRatingsResult
	err    error
}

func (s *stubRatingsGetter) GetRatings(_ context.Context, _, _ string) (*repository.DraftRatingsResult, error) {
	return s.result, s.err
}

// freshCfg returns a Config with default threshold and bypass disabled.
func freshCfg() *config.Config {
	return &config.Config{
		DraftRatingsStalenessThresholdHours: 48,
		DraftRatingsBypassFreshnessCheck:    false,
	}
}

// makeResult builds a DraftRatingsResult with the given cached_at timestamp.
func makeResult(cachedAt time.Time) *repository.DraftRatingsResult {
	gihwr := 55.5

	return &repository.DraftRatingsResult{
		SetCode:     "DSK",
		DraftFormat: "PremierDraft",
		CachedAt:    cachedAt,
		CardRatings: []repository.CardRating{
			{ArenaID: 1, Name: "Test Card", GIHWR: &gihwr},
		},
		ColorRatings: []repository.ColorRating{},
	}
}

// buildDraftRequest creates a chi-routed request for the given setCode/format.
func buildDraftRequest(t *testing.T, setCode, format string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/draft-ratings/"+setCode+"/"+format, nil)

	// Inject chi URL params.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("setCode", setCode)
	rctx.URLParams.Add("format", format)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	return req, httptest.NewRecorder()
}

func TestDraftRatingsHandler_FreshData_NoHeaders(t *testing.T) {
	// cached_at = 1 hour ago — well within 48h threshold.
	cachedAt := time.Now().UTC().Add(-1 * time.Hour)
	stub := &stubRatingsGetter{result: makeResult(cachedAt)}

	h := handlers.NewDraftRatingsHandler(stub, freshCfg())
	req, rr := buildDraftRequest(t, "DSK", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if v := rr.Header().Get("X-Cache-Degraded"); v != "" {
		t.Errorf("expected no X-Cache-Degraded header, got %q", v)
	}

	if v := rr.Header().Get("X-Cache-Age-Hours"); v != "" {
		t.Errorf("expected no X-Cache-Age-Hours header, got %q", v)
	}
}

func TestDraftRatingsHandler_StaleData_DegradedHeaders(t *testing.T) {
	// cached_at = 72 hours ago — exceeds 48h default threshold.
	cachedAt := time.Now().UTC().Add(-72 * time.Hour)
	stub := &stubRatingsGetter{result: makeResult(cachedAt)}

	h := handlers.NewDraftRatingsHandler(stub, freshCfg())
	req, rr := buildDraftRequest(t, "DSK", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (stale data must not cause non-200)", rr.Code)
	}

	if v := rr.Header().Get("X-Cache-Degraded"); v != "true" {
		t.Errorf("expected X-Cache-Degraded=true, got %q", v)
	}

	if v := rr.Header().Get("X-Cache-Age-Hours"); v == "" {
		t.Error("expected X-Cache-Age-Hours header to be present")
	}
}

func TestDraftRatingsHandler_BypassMode_NoHeaders(t *testing.T) {
	// Even with very stale data, bypass disables the freshness check.
	cachedAt := time.Now().UTC().Add(-720 * time.Hour) // 30 days old
	stub := &stubRatingsGetter{result: makeResult(cachedAt)}

	cfg := &config.Config{
		DraftRatingsStalenessThresholdHours: 48,
		DraftRatingsBypassFreshnessCheck:    true,
	}

	h := handlers.NewDraftRatingsHandler(stub, cfg)
	req, rr := buildDraftRequest(t, "DSK", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if v := rr.Header().Get("X-Cache-Degraded"); v != "" {
		t.Errorf("expected no X-Cache-Degraded in bypass mode, got %q", v)
	}
}

func TestDraftRatingsHandler_NoRows_Returns404(t *testing.T) {
	stub := &stubRatingsGetter{result: nil, err: nil}

	h := handlers.NewDraftRatingsHandler(stub, freshCfg())
	req, rr := buildDraftRequest(t, "ZZZNONE", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDraftRatingsHandler_DBError_Returns500(t *testing.T) {
	stub := &stubRatingsGetter{err: context.DeadlineExceeded}

	h := handlers.NewDraftRatingsHandler(stub, freshCfg())
	req, rr := buildDraftRequest(t, "DSK", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDraftRatingsHandler_ResponseBodyStructure(t *testing.T) {
	cachedAt := time.Now().UTC().Add(-1 * time.Hour)
	stub := &stubRatingsGetter{result: makeResult(cachedAt)}

	h := handlers.NewDraftRatingsHandler(stub, freshCfg())
	req, rr := buildDraftRequest(t, "DSK", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body struct {
		SetCode     string `json:"set_code"`
		DraftFormat string `json:"draft_format"`
		CardRatings []struct {
			ArenaID int    `json:"arena_id"`
			Name    string `json:"name"`
		} `json:"card_ratings"`
	}

	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.SetCode != "DSK" {
		t.Errorf("expected set_code=DSK, got %q", body.SetCode)
	}

	if body.DraftFormat != "PremierDraft" {
		t.Errorf("expected draft_format=PremierDraft, got %q", body.DraftFormat)
	}

	if len(body.CardRatings) != 1 {
		t.Errorf("expected 1 card rating, got %d", len(body.CardRatings))
	}
}

func TestDraftRatingsHandler_ExactThresholdBoundary_NotDegraded(t *testing.T) {
	// cached_at exactly at threshold should not trigger degraded mode.
	cfg := &config.Config{
		DraftRatingsStalenessThresholdHours: 48,
		DraftRatingsBypassFreshnessCheck:    false,
	}

	// 47h59m old — just under the threshold.
	cachedAt := time.Now().UTC().Add(-47*time.Hour - 59*time.Minute)
	stub := &stubRatingsGetter{result: makeResult(cachedAt)}

	h := handlers.NewDraftRatingsHandler(stub, cfg)
	req, rr := buildDraftRequest(t, "DSK", "PremierDraft")

	h.GetDraftRatings(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if v := rr.Header().Get("X-Cache-Degraded"); v != "" {
		t.Errorf("expected no X-Cache-Degraded below threshold, got %q", v)
	}
}
