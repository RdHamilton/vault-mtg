// Phase 2 PR #5a — gameplays handlers.
//
// Replaces the SPA's daemonClient gameplays.* surface. Routes are mounted
// under /api/v1/matches/{matchId}/... and /api/v1/gameplays/... per the
// SPA's existing URL contract. Responses use snake_case JSON keys to
// match the SPA's local TypeScript interfaces (GamePlay, GameStateSnapshot,
// OpponentCard, PlayTimelineEntry, GamePlaySummary in gameplays.ts) and are
// wrapped in the {"data": ...} envelope apiClient expects.
//
// Auth: every route is guarded by DaemonAPIKeyAuth. Match-scoped queries
// validate that matchID belongs to the authenticated user's account
// (via the matches.account_id join in the repository).

package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// gamePlaysReader is the minimal repo surface the handler needs.
type gamePlaysReader interface {
	PlaysByMatch(ctx context.Context, accountID int64, matchID string) ([]repository.GamePlayActionRow, error)
	PlaysByGameID(ctx context.Context, accountID int64, gameID int64) ([]repository.GamePlayActionRow, error)
	SnapshotsByMatch(ctx context.Context, accountID int64, matchID string, gameID int64) ([]repository.GameSnapshotRow, error)
	OpponentCardsByMatch(ctx context.Context, accountID int64, matchID string) ([]repository.OpponentCardRow, error)
	MatchExistsForAccount(ctx context.Context, accountID int64, matchID string) (bool, error)
}

// GamePlaysHandler serves the cloud-data Phase 2 game-plays API.
type GamePlaysHandler struct {
	plays    gamePlaysReader
	accounts AccountLookup
}

// NewGamePlaysHandler returns a handler wired with the given reader + lookup.
func NewGamePlaysHandler(p gamePlaysReader, accounts AccountLookup) *GamePlaysHandler {
	return &GamePlaysHandler{plays: p, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// gamePlayResponse mirrors the SPA's GamePlay interface (snake_case keys).
// Pointer fields are nullable in the schema; omitempty so a clean
// serialisation drops them when absent.
type gamePlayResponse struct {
	ID             int64     `json:"id"`
	GameID         int64     `json:"game_id"`
	MatchID        string    `json:"match_id"`
	TurnNumber     int       `json:"turn_number"`
	Phase          string    `json:"phase"`
	Step           string    `json:"step,omitempty"`
	PlayerType     string    `json:"player_type"`
	ActionType     string    `json:"action_type"`
	CardID         *int      `json:"card_id,omitempty"`
	CardName       string    `json:"card_name,omitempty"`
	ZoneFrom       string    `json:"zone_from,omitempty"`
	ZoneTo         string    `json:"zone_to,omitempty"`
	LifeFrom       *int      `json:"life_from,omitempty"`
	LifeTo         *int      `json:"life_to,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	SequenceNumber int       `json:"sequence_number"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
}

// gameSnapshotResponse mirrors GameStateSnapshot.
type gameSnapshotResponse struct {
	ID                  int64     `json:"id"`
	GameID              int64     `json:"game_id"`
	MatchID             string    `json:"match_id"`
	TurnNumber          int       `json:"turn_number"`
	ActivePlayer        string    `json:"active_player"`
	PlayerLife          *int      `json:"player_life,omitempty"`
	OpponentLife        *int      `json:"opponent_life,omitempty"`
	PlayerCardsInHand   *int      `json:"player_cards_in_hand,omitempty"`
	OpponentCardsInHand *int      `json:"opponent_cards_in_hand,omitempty"`
	PlayerLandsInPlay   *int      `json:"player_lands_in_play,omitempty"`
	OpponentLandsInPlay *int      `json:"opponent_lands_in_play,omitempty"`
	BoardStateJSON      string    `json:"board_state_json,omitempty"`
	Timestamp           time.Time `json:"timestamp"`
}

// opponentCardResponse mirrors OpponentCard.
type opponentCardResponse struct {
	ID            int64  `json:"id"`
	GameID        int64  `json:"game_id"`
	MatchID       string `json:"match_id"`
	CardID        int    `json:"card_id"`
	CardName      string `json:"card_name,omitempty"`
	ZoneObserved  string `json:"zone_observed,omitempty"`
	TurnFirstSeen int    `json:"turn_first_seen,omitempty"`
	TimesSeen     int    `json:"times_seen"`
}

// timelineEntryResponse mirrors PlayTimelineEntry — one bucket per turn,
// with player/opponent plays split and an optional snapshot reference.
type timelineEntryResponse struct {
	Turn          int                   `json:"turn"`
	ActivePlayer  string                `json:"active_player"`
	PlayerPlays   []gamePlayResponse    `json:"player_plays"`
	OpponentPlays []gamePlayResponse    `json:"opponent_plays"`
	Snapshot      *gameSnapshotResponse `json:"snapshot,omitempty"`
}

// gamePlaySummaryResponse mirrors GamePlaySummary.
type gamePlaySummaryResponse struct {
	MatchID           string `json:"match_id"`
	GameID            *int64 `json:"game_id,omitempty"`
	TotalPlays        int    `json:"total_plays"`
	PlayerPlays       int    `json:"player_plays"`
	OpponentPlays     int    `json:"opponent_plays"`
	CardPlays         int    `json:"card_plays"`
	Attacks           int    `json:"attacks"`
	Blocks            int    `json:"blocks"`
	LandDrops         int    `json:"land_drops"`
	TotalTurns        int    `json:"total_turns"`
	OpponentCardsSeen int    `json:"opponent_cards_seen"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// MatchPlays handles GET /api/v1/matches/{matchId}/plays.
func (h *GamePlaysHandler) MatchPlays(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "MatchPlays")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []gamePlayResponse{})
		return
	}
	rows, err := h.plays.PlaysByMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchPlays] PlaysByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, playRowsToResponse(rows))
}

// MatchTimeline handles GET /api/v1/matches/{matchId}/plays/timeline.
// Buckets the play stream by turn and splits player/opponent.
func (h *GamePlaysHandler) MatchTimeline(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "MatchTimeline")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []timelineEntryResponse{})
		return
	}
	rows, err := h.plays.PlaysByMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchTimeline] PlaysByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	snaps, err := h.plays.SnapshotsByMatch(r.Context(), accountID, matchID, 0)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchTimeline] SnapshotsByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, buildTimeline(rows, snaps))
}

// MatchPlaySummary handles GET /api/v1/matches/{matchId}/plays/summary.
func (h *GamePlaysHandler) MatchPlaySummary(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "MatchPlaySummary")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, gamePlaySummaryResponse{MatchID: matchID})
		return
	}
	rows, err := h.plays.PlaysByMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchPlaySummary] PlaysByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	opp, err := h.plays.OpponentCardsByMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchPlaySummary] OpponentCardsByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, buildPlaySummary(matchID, rows, opp))
}

// MatchOpponentCards handles GET /api/v1/matches/{matchId}/opponent-cards.
func (h *GamePlaysHandler) MatchOpponentCards(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "MatchOpponentCards")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []opponentCardResponse{})
		return
	}
	rows, err := h.plays.OpponentCardsByMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchOpponentCards] OpponentCardsByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, opponentCardRowsToResponse(rows))
}

// MatchSnapshots handles GET /api/v1/matches/{matchId}/snapshots?gameID=N.
func (h *GamePlaysHandler) MatchSnapshots(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "MatchSnapshots")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	gameID := int64(0)
	if v := strings.TrimSpace(r.URL.Query().Get("gameID")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			writeJSONError(w, "gameID must be a positive integer", http.StatusBadRequest)
			return
		}
		gameID = n
	}
	if !found {
		writeMatchesJSON(w, []gameSnapshotResponse{})
		return
	}
	rows, err := h.plays.SnapshotsByMatch(r.Context(), accountID, matchID, gameID)
	if err != nil {
		log.Printf("[GamePlaysHandler.MatchSnapshots] SnapshotsByMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, snapshotRowsToResponse(rows))
}

// PlaysByGame handles GET /api/v1/gameplays/game/{gameId}.
func (h *GamePlaysHandler) PlaysByGame(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "PlaysByGame")
	if !ok {
		return
	}
	gameIDStr := strings.TrimSpace(chi.URLParam(r, "gameId"))
	if gameIDStr == "" {
		writeJSONError(w, "gameId is required", http.StatusBadRequest)
		return
	}
	gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
	if err != nil || gameID <= 0 {
		writeJSONError(w, "gameId must be a positive integer", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []gamePlayResponse{})
		return
	}
	rows, err := h.plays.PlaysByGameID(r.Context(), accountID, gameID)
	if err != nil {
		log.Printf("[GamePlaysHandler.PlaysByGame] PlaysByGameID accountID=%d gameID=%d: %v", accountID, gameID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, playRowsToResponse(rows))
}

// ─── helpers ────────────────────────────────────────────────────────────────

// resolveAccount mirrors the helper used by other Phase 2 handlers.
func (h *GamePlaysHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[GamePlaysHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func playRowsToResponse(rows []repository.GamePlayActionRow) []gamePlayResponse {
	out := make([]gamePlayResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, gamePlayResponse{
			ID: p.ID, GameID: p.GameID, MatchID: p.MatchID,
			TurnNumber: p.TurnNumber, Phase: derefOr(p.Phase, ""), Step: derefOr(p.Step, ""),
			PlayerType: p.PlayerType, ActionType: p.ActionType,
			CardID: p.CardID, CardName: derefOr(p.CardName, ""),
			ZoneFrom: derefOr(p.ZoneFrom, ""), ZoneTo: derefOr(p.ZoneTo, ""),
			LifeFrom: p.LifeFrom, LifeTo: p.LifeTo,
			Timestamp: p.Timestamp, SequenceNumber: p.SequenceNumber, CreatedAt: p.CreatedAt,
		})
	}
	return out
}

func snapshotRowsToResponse(rows []repository.GameSnapshotRow) []gameSnapshotResponse {
	out := make([]gameSnapshotResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, gameSnapshotResponse{
			ID: s.ID, GameID: s.GameID, MatchID: s.MatchID,
			TurnNumber: s.TurnNumber, ActivePlayer: s.ActivePlayer,
			PlayerLife: s.PlayerLife, OpponentLife: s.OpponentLife,
			PlayerCardsInHand: s.PlayerCardsInHand, OpponentCardsInHand: s.OpponentCardsInHand,
			PlayerLandsInPlay: s.PlayerLandsInPlay, OpponentLandsInPlay: s.OpponentLandsInPlay,
			BoardStateJSON: derefOr(s.BoardStateJSON, ""), Timestamp: s.Timestamp,
		})
	}
	return out
}

func opponentCardRowsToResponse(rows []repository.OpponentCardRow) []opponentCardResponse {
	out := make([]opponentCardResponse, 0, len(rows))
	for _, c := range rows {
		entry := opponentCardResponse{
			ID: c.ID, GameID: c.GameID, MatchID: c.MatchID,
			CardID: c.CardID, CardName: derefOr(c.CardName, ""),
			ZoneObserved: derefOr(c.ZoneObserved, ""), TimesSeen: c.TimesSeen,
		}
		if c.TurnFirstSeen != nil {
			entry.TurnFirstSeen = *c.TurnFirstSeen
		}
		out = append(out, entry)
	}
	return out
}

// buildTimeline groups plays by turn, splits player/opponent, and attaches
// the matching snapshot when one exists for the turn.
func buildTimeline(rows []repository.GamePlayActionRow, snaps []repository.GameSnapshotRow) []timelineEntryResponse {
	// Index snapshots by (gameID, turnNumber). Picking by turnNumber alone
	// is fine when the SPA renders one game at a time, but the (game, turn)
	// composite is the natural unique key in the schema.
	type snapKey struct {
		gameID int64
		turn   int
	}
	snapByTurn := map[snapKey]repository.GameSnapshotRow{}
	for _, s := range snaps {
		snapByTurn[snapKey{s.GameID, s.TurnNumber}] = s
	}

	// Bucket plays by turn while preserving the first-seen active-player.
	type bucket struct {
		turn      int
		gameID    int64
		active    string
		playerOps []repository.GamePlayActionRow
		oppOps    []repository.GamePlayActionRow
	}
	bucketsByTurn := map[int]*bucket{}
	turnOrder := make([]int, 0)
	for _, p := range rows {
		b, exists := bucketsByTurn[p.TurnNumber]
		if !exists {
			b = &bucket{turn: p.TurnNumber, gameID: p.GameID, active: p.PlayerType}
			bucketsByTurn[p.TurnNumber] = b
			turnOrder = append(turnOrder, p.TurnNumber)
		}
		switch strings.ToLower(p.PlayerType) {
		case "player":
			b.playerOps = append(b.playerOps, p)
		default:
			b.oppOps = append(b.oppOps, p)
		}
	}

	out := make([]timelineEntryResponse, 0, len(turnOrder))
	for _, turn := range turnOrder {
		b := bucketsByTurn[turn]
		entry := timelineEntryResponse{
			Turn: turn, ActivePlayer: b.active,
			PlayerPlays:   playRowsToResponse(b.playerOps),
			OpponentPlays: playRowsToResponse(b.oppOps),
		}
		if snap, hit := snapByTurn[snapKey{b.gameID, turn}]; hit {
			snapResp := snapshotRowsToResponse([]repository.GameSnapshotRow{snap})[0]
			entry.Snapshot = &snapResp
		}
		out = append(out, entry)
	}
	return out
}

// buildPlaySummary aggregates play counts + turn count + opponent-cards-seen
// for the SPA's GamePlaySummary shape.
func buildPlaySummary(matchID string, rows []repository.GamePlayActionRow, opp []repository.OpponentCardRow) gamePlaySummaryResponse {
	resp := gamePlaySummaryResponse{
		MatchID:           matchID,
		TotalPlays:        len(rows),
		OpponentCardsSeen: len(opp),
	}
	turnSet := map[int]struct{}{}
	for _, p := range rows {
		turnSet[p.TurnNumber] = struct{}{}
		switch strings.ToLower(p.PlayerType) {
		case "player":
			resp.PlayerPlays++
		default:
			resp.OpponentPlays++
		}
		switch p.ActionType {
		case "play_card", "cast_spell":
			resp.CardPlays++
		case "attack":
			resp.Attacks++
		case "block":
			resp.Blocks++
		case "land_drop":
			resp.LandDrops++
		}
	}
	resp.TotalTurns = len(turnSet)
	return resp
}
