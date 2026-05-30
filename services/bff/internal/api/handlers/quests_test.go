package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	bffmiddleware "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type questsAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (q *questsAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return q.accountID, q.found, q.err
}

type stubQuestsReader struct {
	active    []repository.QuestRow
	activeErr error

	history    []repository.QuestRow
	historyErr error

	winsCount int
	winsSince time.Time
	winsErr   error

	stats    repository.QuestStatsAggregate
	statsErr error

	lastSeen    time.Time
	lastSeenOK  bool
	lastSeenErr error
}

func (s *stubQuestsReader) ListActiveByAccountID(_ context.Context, _ int64) ([]repository.QuestRow, error) {
	return s.active, s.activeErr
}

func (s *stubQuestsReader) ListHistoryByAccountID(_ context.Context, _ int64, _, _ *time.Time, _ int) ([]repository.QuestRow, error) {
	return s.history, s.historyErr
}

func (s *stubQuestsReader) CountWinsSince(_ context.Context, _ int64, since time.Time) (int, error) {
	s.winsSince = since
	return s.winsCount, s.winsErr
}

func (s *stubQuestsReader) QuestStats(_ context.Context, _ int64, _, _ time.Time) (repository.QuestStatsAggregate, error) {
	return s.stats, s.statsErr
}

func (s *stubQuestsReader) LastQuestSeenAt(_ context.Context, _ int64) (time.Time, bool, error) {
	return s.lastSeen, s.lastSeenOK, s.lastSeenErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedQuestsRequest(t *testing.T, method, target string, userID int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeQuestsEnvelope(t *testing.T, body []byte, into any) {
	t.Helper()
	wrapper := struct {
		Data json.RawMessage `json:"data"`
	}{}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatalf("envelope decode: %v body=%s", err, string(body))
	}
	if err := json.Unmarshal(wrapper.Data, into); err != nil {
		t.Fatalf("payload decode: %v data=%s", err, string(wrapper.Data))
	}
}

// ─── Active ─────────────────────────────────────────────────────────────────

func TestQuestsActive_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubQuestsReader{
		active: []repository.QuestRow{
			{ID: 1, QuestID: "q1", Goal: 5, EndingProgress: 2, FirstSeenAt: now},
		},
		lastSeen: now, lastSeenOK: true,
	}
	h := handlers.NewQuestsHandler(reader, &questsAccountLookup{accountID: 7, found: true})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/active", 168)
	rr := httptest.NewRecorder()
	h.Active(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeQuestsEnvelope(t, rr.Body.Bytes(), &resp)
	if !resp["has_quest_data"].(bool) {
		t.Errorf("has_quest_data should be true: %v", resp)
	}
	quests, _ := resp["quests"].([]any)
	if len(quests) != 1 {
		t.Errorf("quests: %v", resp["quests"])
	}
	if resp["last_updated"] == "" {
		t.Errorf("last_updated should be set when has_quest_data=true: %v", resp["last_updated"])
	}
}

func TestQuestsActive_NoAccount(t *testing.T) {
	h := handlers.NewQuestsHandler(&stubQuestsReader{}, &questsAccountLookup{found: false})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/active", 168)
	rr := httptest.NewRecorder()
	h.Active(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeQuestsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["has_quest_data"] != false {
		t.Errorf("has_quest_data should be false: %v", resp)
	}
}

func TestQuestsActive_Unauthorized(t *testing.T) {
	h := handlers.NewQuestsHandler(&stubQuestsReader{}, &questsAccountLookup{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/quests/active", bytes.NewReader(nil))
	rr := httptest.NewRecorder()
	h.Active(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── History ────────────────────────────────────────────────────────────────

func TestQuestsHistory_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubQuestsReader{
		history: []repository.QuestRow{
			{ID: 1, QuestID: "q1", Goal: 5, EndingProgress: 5, Completed: true, FirstSeenAt: now.Add(-time.Hour), CompletedAt: &now},
		},
	}
	h := handlers.NewQuestsHandler(reader, &questsAccountLookup{accountID: 7, found: true})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/history?limit=10", 168)
	rr := httptest.NewRecorder()
	h.History(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeQuestsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["completed"] != true {
		t.Errorf("history: %v", arr)
	}
}

// ─── Wins ───────────────────────────────────────────────────────────────────

func TestQuestsDailyWins_HappyPath(t *testing.T) {
	reader := &stubQuestsReader{winsCount: 3}
	h := handlers.NewQuestsHandler(reader, &questsAccountLookup{accountID: 7, found: true})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/wins/daily", 168)
	rr := httptest.NewRecorder()
	h.DailyWins(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeQuestsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["dailyWins"].(float64) != 3 || resp["goal"].(float64) != 15 {
		t.Errorf("daily wins: %v", resp)
	}
	if reader.winsSince.IsZero() {
		t.Errorf("expected since timestamp to be passed to repo")
	}
}

func TestQuestsWeeklyWins_HappyPath(t *testing.T) {
	reader := &stubQuestsReader{winsCount: 12}
	h := handlers.NewQuestsHandler(reader, &questsAccountLookup{accountID: 7, found: true})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/wins/weekly", 168)
	rr := httptest.NewRecorder()
	h.WeeklyWins(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeQuestsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["weeklyWins"].(float64) != 12 || resp["goal"].(float64) != 15 {
		t.Errorf("weekly wins: %v", resp)
	}
}

// ─── Stats ──────────────────────────────────────────────────────────────────

func TestQuestsStats_HappyPath(t *testing.T) {
	reader := &stubQuestsReader{
		stats: repository.QuestStatsAggregate{
			TotalQuests: 10, CompletedQuests: 7, ActiveQuests: 3, RerollCount: 2, AverageCompletionMS: 50_000,
		},
	}
	h := handlers.NewQuestsHandler(reader, &questsAccountLookup{accountID: 7, found: true})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/stats?startDate=2026-01-01&endDate=2026-01-31", 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeQuestsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["total_quests"].(float64) != 10 || resp["completion_rate"].(float64) != 0.7 {
		t.Errorf("stats: %v", resp)
	}
}

func TestQuestsStats_RejectsBadDates(t *testing.T) {
	h := handlers.NewQuestsHandler(&stubQuestsReader{}, &questsAccountLookup{accountID: 7, found: true})
	req := authedQuestsRequest(t, http.MethodGet, "/api/v1/quests/stats?startDate=2026-02-01&endDate=2026-01-01", 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d (expected 400)", rr.Code)
	}
}
