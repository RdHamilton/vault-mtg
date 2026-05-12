package localapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/draftstate"
	"github.com/ramonehamilton/mtga-daemon/internal/localapi"
	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
)

// stubCards / stubRatings satisfy draftalgo.CardLookup / RatingsLookup.
// Test-injected lookups make the assertions deterministic.
type stubCards map[string]string

func (s stubCards) CardName(id string) string { return s[id] }

type stubRatings map[string]float64

func (s stubRatings) GIHWR(id, _ string) (float64, bool) {
	v, ok := s[id]
	return v, ok
}

// newDraftTestServer wires a localapi server + draftstate Store + the
// test-supplied lookups. Caller is responsible for Stop().
func newDraftTestServer(t *testing.T, prep func(*draftstate.Store)) *localapi.Server {
	t.Helper()
	srv := localapi.New(0, localapi.State{Version: "test", StartedAt: time.Now()})
	store := draftstate.New()
	if prep != nil {
		prep(store)
	}
	srv.SetDraftStore(store)
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return srv
}

func mustDecode[T any](t *testing.T, body io.Reader) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// ─── current-pack ──────────────────────────────────────────────────────────

func TestCurrentPack_NoSessionReturns404(t *testing.T) {
	srv := newDraftTestServer(t, nil)

	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/drafts/anything/current-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCurrentPack_ReturnsLiveSession(t *testing.T) {
	srv := newDraftTestServer(t, func(store *draftstate.Store) {
		store.HandlePack(&logreader.DraftPackPayload{
			CourseName: "PremierDraft_BLB",
			DraftPack:  logreader.DraftPackDetail{PackCards: []int{100, 200, 300}, SelfPick: 1},
		})
	})
	srv.SetDraftLookups(
		stubCards{"100": "Card A", "200": "Card B", "300": "Card C"},
		stubRatings{"100": 60.0, "200": 50.0, "300": 45.0},
	)

	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/drafts/current/current-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := mustDecode[struct {
		SessionID  string `json:"sessionId"`
		PackNumber int    `json:"packNumber"`
		PickNumber int    `json:"pickNumber"`
		Cards      []struct {
			ArenaID  int     `json:"arenaId"`
			CardName string  `json:"cardName"`
			GIHWR    float64 `json:"gihwr"`
		} `json:"cards"`
		SetCode string `json:"setCode"`
		Format  string `json:"format"`
	}](t, resp.Body)

	if body.PackNumber != 1 || body.PickNumber != 1 {
		t.Errorf("PackNumber/PickNumber = %d/%d, want 1/1 (1-based)", body.PackNumber, body.PickNumber)
	}
	if body.SetCode != "BLB" || body.Format != "PremierDraft" {
		t.Errorf("SetCode/Format = %q/%q", body.SetCode, body.Format)
	}
	if len(body.Cards) != 3 || body.Cards[0].CardName != "Card A" || body.Cards[0].GIHWR != 60.0 {
		t.Errorf("Cards mismatch: %+v", body.Cards)
	}
}

func TestCurrentPack_404OnMalformedPath(t *testing.T) {
	srv := newDraftTestServer(t, nil)
	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/drafts/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// ─── grade-pick ────────────────────────────────────────────────────────────

func TestGradePick_ExplicitAvailableCards(t *testing.T) {
	srv := newDraftTestServer(t, nil)
	srv.SetDraftLookups(
		stubCards{"100": "A", "200": "B", "300": "C"},
		stubRatings{"100": 60.0, "200": 55.0, "300": 50.0},
	)

	body := map[string]any{
		"session_id":         "s",
		"picked_card_id":     100,
		"available_card_ids": []int{100, 200, 300},
		"pick_number":        1,
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post(
		"http://"+srv.Addr()+"/api/v1/drafts/grade-pick",
		"application/json", bytes.NewReader(b),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := mustDecode[struct {
		Grade         string  `json:"grade"`
		Rank          int     `json:"rank"`
		PackBestGIHWR float64 `json:"pack_best_gihwr"`
	}](t, resp.Body)
	if got.Grade != "A+" || got.Rank != 1 {
		t.Errorf("expected A+ rank 1 (picked the best card), got %+v", got)
	}
	if got.PackBestGIHWR != 60.0 {
		t.Errorf("PackBestGIHWR = %v, want 60.0", got.PackBestGIHWR)
	}
}

func TestGradePick_BadBodyReturns400(t *testing.T) {
	srv := newDraftTestServer(t, nil)
	resp, err := http.Post(
		"http://"+srv.Addr()+"/api/v1/drafts/grade-pick",
		"application/json", bytes.NewReader([]byte("not json")),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestGradePick_RejectsNonPOST(t *testing.T) {
	srv := newDraftTestServer(t, nil)
	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/drafts/grade-pick")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

// ─── win-probability ───────────────────────────────────────────────────────

func TestWinProbability_NoSessionDefaultsToBaseline(t *testing.T) {
	srv := newDraftTestServer(t, nil)

	body, _ := json.Marshal(map[string]string{"session_id": "anything"})
	resp, err := http.Post(
		"http://"+srv.Addr()+"/api/v1/drafts/win-probability",
		"application/json", bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got := mustDecode[struct {
		Probability float64 `json:"probability"`
	}](t, resp.Body)
	if got.Probability != 0.50 {
		t.Errorf("Probability = %v, want 0.50 (baseline)", got.Probability)
	}
}

func TestWinProbability_ComputesFromSession(t *testing.T) {
	srv := newDraftTestServer(t, func(store *draftstate.Store) {
		// Seed a session with one recorded pick so the predictor has
		// something to chew on (even with no ratings; uses defaults).
		store.HandlePack(&logreader.DraftPackPayload{
			CourseName: "PremierDraft_BLB",
			DraftPack:  logreader.DraftPackDetail{PackCards: []int{1, 2, 3}, SelfPick: 1},
		})
		store.HandlePick(&logreader.DraftPickPayload{
			CourseName: "PremierDraft_BLB", PickedCards: []int{1},
			PackNumber: 0, PickNumber: 0,
		})
	})
	srv.SetDraftLookups(
		stubCards{"1": "Card"},
		stubRatings{"1": 50.0},
	)

	body, _ := json.Marshal(map[string]string{"session_id": "current"})
	resp, err := http.Post(
		"http://"+srv.Addr()+"/api/v1/drafts/win-probability",
		"application/json", bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := mustDecode[struct {
		Probability float64 `json:"probability"`
	}](t, resp.Body)
	// Predictor clamps to [0.30, 0.70]. We don't pin a specific value
	// because the heuristic touches several knobs — assert range only.
	if got.Probability < 0.30 || got.Probability > 0.70 {
		t.Errorf("Probability = %v, want in [0.30, 0.70]", got.Probability)
	}
}

func TestWinProbability_RejectsNonPOST(t *testing.T) {
	srv := newDraftTestServer(t, nil)
	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/drafts/win-probability")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
