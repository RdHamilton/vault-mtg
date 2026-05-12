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
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type gpAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (g *gpAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return g.accountID, g.found, g.err
}

type stubGamePlaysReader struct {
	playsByMatch    []repository.GamePlayActionRow
	playsByMatchErr error

	playsByGame    []repository.GamePlayActionRow
	playsByGameErr error

	snapshots    []repository.GameSnapshotRow
	snapshotsErr error

	opponentCards    []repository.OpponentCardRow
	opponentCardsErr error
}

func (s *stubGamePlaysReader) PlaysByMatch(_ context.Context, _ int64, _ string) ([]repository.GamePlayActionRow, error) {
	return s.playsByMatch, s.playsByMatchErr
}

func (s *stubGamePlaysReader) PlaysByGameID(_ context.Context, _ int64, _ int64) ([]repository.GamePlayActionRow, error) {
	return s.playsByGame, s.playsByGameErr
}

func (s *stubGamePlaysReader) SnapshotsByMatch(_ context.Context, _ int64, _ string, _ int64) ([]repository.GameSnapshotRow, error) {
	return s.snapshots, s.snapshotsErr
}

func (s *stubGamePlaysReader) OpponentCardsByMatch(_ context.Context, _ int64, _ string) ([]repository.OpponentCardRow, error) {
	return s.opponentCards, s.opponentCardsErr
}

func (s *stubGamePlaysReader) MatchExistsForAccount(_ context.Context, _ int64, _ string) (bool, error) {
	return true, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedGPRequest(t *testing.T, method, target string, userID int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeGPEnvelope(t *testing.T, body []byte, into any) {
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

func chiGPContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ─── MatchPlays ────────────────────────────────────────────────────────────

func TestGamePlaysMatchPlays_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubGamePlaysReader{playsByMatch: []repository.GamePlayActionRow{
		{ID: 1, GameID: 10, MatchID: "m1", TurnNumber: 1, PlayerType: "player", ActionType: "play_card", Timestamp: now, SequenceNumber: 1},
		{ID: 2, GameID: 10, MatchID: "m1", TurnNumber: 1, PlayerType: "opponent", ActionType: "play_card", Timestamp: now, SequenceNumber: 2},
	}}
	h := handlers.NewGamePlaysHandler(reader, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/matches/m1/plays", 168)
	req = chiGPContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.MatchPlays(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeGPEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 2 || arr[0]["match_id"] != "m1" {
		t.Errorf("plays: %v", arr)
	}
}

func TestGamePlaysMatchPlays_Unauthorized(t *testing.T) {
	h := handlers.NewGamePlaysHandler(&stubGamePlaysReader{}, &gpAccountLookup{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/m1/plays", nil)
	rr := httptest.NewRecorder()
	h.MatchPlays(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── MatchTimeline ─────────────────────────────────────────────────────────

func TestGamePlaysMatchTimeline_BucketsByTurn(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubGamePlaysReader{
		playsByMatch: []repository.GamePlayActionRow{
			{ID: 1, GameID: 10, MatchID: "m1", TurnNumber: 1, PlayerType: "player", ActionType: "land_drop", Timestamp: now, SequenceNumber: 1},
			{ID: 2, GameID: 10, MatchID: "m1", TurnNumber: 2, PlayerType: "opponent", ActionType: "play_card", Timestamp: now, SequenceNumber: 2},
		},
		snapshots: []repository.GameSnapshotRow{
			{ID: 5, GameID: 10, MatchID: "m1", TurnNumber: 2, ActivePlayer: "opponent", Timestamp: now},
		},
	}
	h := handlers.NewGamePlaysHandler(reader, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/matches/m1/plays/timeline", 168)
	req = chiGPContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.MatchTimeline(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeGPEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 2 {
		t.Fatalf("turns: %v", arr)
	}
	if arr[1]["snapshot"] == nil {
		t.Errorf("expected snapshot attached to turn 2: %v", arr[1])
	}
}

// ─── MatchPlaySummary ──────────────────────────────────────────────────────

func TestGamePlaysMatchPlaySummary_Aggregates(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubGamePlaysReader{
		playsByMatch: []repository.GamePlayActionRow{
			{ID: 1, GameID: 10, MatchID: "m1", TurnNumber: 1, PlayerType: "player", ActionType: "land_drop", Timestamp: now, SequenceNumber: 1},
			{ID: 2, GameID: 10, MatchID: "m1", TurnNumber: 1, PlayerType: "player", ActionType: "play_card", Timestamp: now, SequenceNumber: 2},
			{ID: 3, GameID: 10, MatchID: "m1", TurnNumber: 2, PlayerType: "opponent", ActionType: "attack", Timestamp: now, SequenceNumber: 3},
			{ID: 4, GameID: 10, MatchID: "m1", TurnNumber: 2, PlayerType: "player", ActionType: "block", Timestamp: now, SequenceNumber: 4},
		},
		opponentCards: []repository.OpponentCardRow{
			{ID: 100, GameID: 10, MatchID: "m1", CardID: 99, TimesSeen: 1},
		},
	}
	h := handlers.NewGamePlaysHandler(reader, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/matches/m1/plays/summary", 168)
	req = chiGPContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.MatchPlaySummary(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeGPEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["total_plays"].(float64) != 4 || resp["total_turns"].(float64) != 2 {
		t.Errorf("summary totals: %v", resp)
	}
	if resp["land_drops"].(float64) != 1 || resp["attacks"].(float64) != 1 || resp["blocks"].(float64) != 1 {
		t.Errorf("action counts: %v", resp)
	}
	if resp["opponent_cards_seen"].(float64) != 1 {
		t.Errorf("opponent_cards_seen: %v", resp)
	}
}

// ─── MatchOpponentCards ────────────────────────────────────────────────────

func TestGamePlaysMatchOpponentCards_HappyPath(t *testing.T) {
	reader := &stubGamePlaysReader{opponentCards: []repository.OpponentCardRow{
		{ID: 1, GameID: 10, MatchID: "m1", CardID: 100, TimesSeen: 2},
	}}
	h := handlers.NewGamePlaysHandler(reader, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/matches/m1/opponent-cards", 168)
	req = chiGPContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.MatchOpponentCards(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeGPEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["card_id"].(float64) != 100 {
		t.Errorf("opp cards: %v", arr)
	}
}

// ─── MatchSnapshots ────────────────────────────────────────────────────────

func TestGamePlaysMatchSnapshots_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubGamePlaysReader{snapshots: []repository.GameSnapshotRow{
		{ID: 1, GameID: 10, MatchID: "m1", TurnNumber: 1, ActivePlayer: "player", Timestamp: now},
	}}
	h := handlers.NewGamePlaysHandler(reader, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/matches/m1/snapshots", 168)
	req = chiGPContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.MatchSnapshots(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeGPEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Errorf("snapshots: %v", arr)
	}
}

func TestGamePlaysMatchSnapshots_RejectsBadGameID(t *testing.T) {
	h := handlers.NewGamePlaysHandler(&stubGamePlaysReader{}, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/matches/m1/snapshots?gameID=notanumber", 168)
	req = chiGPContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.MatchSnapshots(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── PlaysByGame ───────────────────────────────────────────────────────────

func TestGamePlaysPlaysByGame_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubGamePlaysReader{playsByGame: []repository.GamePlayActionRow{
		{ID: 1, GameID: 10, MatchID: "m1", TurnNumber: 1, PlayerType: "player", ActionType: "play_card", Timestamp: now, SequenceNumber: 1},
	}}
	h := handlers.NewGamePlaysHandler(reader, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/gameplays/game/10", 168)
	req = chiGPContext(req, "gameId", "10")
	rr := httptest.NewRecorder()
	h.PlaysByGame(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeGPEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Errorf("plays: %v", arr)
	}
}

func TestGamePlaysPlaysByGame_BadGameID(t *testing.T) {
	h := handlers.NewGamePlaysHandler(&stubGamePlaysReader{}, &gpAccountLookup{accountID: 7, found: true})
	req := authedGPRequest(t, http.MethodGet, "/api/v1/gameplays/game/abc", 168)
	req = chiGPContext(req, "gameId", "abc")
	rr := httptest.NewRecorder()
	h.PlaysByGame(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}
