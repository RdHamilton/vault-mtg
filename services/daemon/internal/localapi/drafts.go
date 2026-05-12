// Phase 2 PR #17b — daemon live-state draft endpoints.
//
// Three handlers under /api/v1/drafts/* that the SPA hits during a
// live draft. State comes from the daemon's in-memory draftstate.Store
// (populated by the log entry consumer in
// services/daemon/internal/daemon). The grading algorithms live in
// pkg/draftalgo and are injected via the same small CardLookup /
// RatingsLookup interfaces the algorithms accept.
//
// This file deliberately ships with no-op lookup stubs as the default:
// grade-pick returns "N/A" and win-probability falls back to the
// neutral baseline until a follow-up PR wires a real ratings client
// (fetching the BFF's /api/v1/draft-ratings/{set}/{format} with TTL
// cache). The handler shapes are correct end-to-end; only the data
// quality improves later.
//
// Routes:
//
//	GET  /api/v1/drafts/{id}/current-pack    handleDraftsPathPrefix
//	POST /api/v1/drafts/grade-pick           handleDraftGradePick
//	POST /api/v1/drafts/win-probability      handleDraftWinProbability

package localapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/RdHamilton/MTGA-Companion/pkg/draftalgo"
	"github.com/RdHamilton/MTGA-Companion/pkg/draftalgo/pickquality"
	"github.com/RdHamilton/MTGA-Companion/pkg/draftalgo/prediction"
	"github.com/ramonehamilton/mtga-daemon/internal/draftstate"
)

// DraftStore is the minimal read surface localapi needs from the
// daemon's draftstate package. Keeping it as an interface here avoids
// pinning localapi tests to the full draftstate.Store concrete type.
type DraftStore interface {
	Get(id string) (*draftstate.Session, bool)
}

// noopRatings is the default RatingsLookup the handlers use until a
// real BFF-backed client lands. Always reports "not found" so grades
// degrade to "N/A" and win-probability uses the neutral baseline. The
// shape of the response stays correct.
type noopRatings struct{}

func (noopRatings) GIHWR(_ string, _ string) (float64, bool) { return 0, false }

// noopCards returns empty card names for everything. The SPA renders
// "Unknown Card" placeholders until the lookup is wired.
type noopCards struct{}

func (noopCards) CardName(_ string) string { return "" }

// ─── /api/v1/drafts/{id}/current-pack ─────────────────────────────────────

// currentPackCard mirrors the SPA's gui.CurrentPackResponse.cards entry
// (frontend/src/types/models.ts).
type currentPackCard struct {
	ArenaID  int     `json:"arenaId"`
	CardName string  `json:"cardName"`
	GIHWR    float64 `json:"gihwr"`
}

// currentPackResponse mirrors the SPA's gui.CurrentPackResponse.
type currentPackResponse struct {
	SessionID      string            `json:"sessionId"`
	PackNumber     int               `json:"packNumber"` // 1-based for display
	PickNumber     int               `json:"pickNumber"` // 1-based for display
	Cards          []currentPackCard `json:"cards"`
	Recommendation string            `json:"recommendation,omitempty"`
	SetCode        string            `json:"setCode"`
	Format         string            `json:"format"`
}

// gradePickRequest mirrors the SPA's drafts.GradePickRequest body.
type gradePickRequest struct {
	SessionID        string `json:"session_id"`
	PickNumber       int    `json:"pick_number"`
	PickedCardID     int    `json:"picked_card_id"`
	AvailableCardIDs []int  `json:"available_card_ids"`
	// Legacy alt shape some SPA call sites use (PackNumber + PickNumber
	// indices into the daemon-known session). When picked_card_id is 0
	// and available_card_ids is empty, fall back to looking up the
	// pack from draftstate using these.
	PackNumberHint int `json:"pack_number,omitempty"`
	PickNumberHint int `json:"-"`
}

// gradePickResponse re-uses pickquality.PickQuality directly — same
// snake_case JSON tags the SPA already consumes.

// winProbabilityRequest mirrors the SPA's drafts.WinProbabilityRequest body.
type winProbabilityRequest struct {
	SessionID string `json:"session_id"`
}

// winProbabilityResponse is the trivial {probability: float} shape the
// SPA's predictWinProbability wrapper expects.
type winProbabilityResponse struct {
	Probability float64 `json:"probability"`
}

// handleDraftsPathPrefix routes GET /api/v1/drafts/{id}/current-pack.
// The shorter sibling paths (grade-pick, win-probability) get
// explicit registrations above us in the mux and win the
// longest-prefix-match — anything else here either resolves to a
// current-pack request or 404s.
func (s *Server) handleDraftsPathPrefix(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}

	// Strip the /api/v1/drafts/ prefix and split. We accept exactly
	// "<sessionId>/current-pack" — anything else gets 404'd.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/drafts/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "current-pack" {
		http.NotFound(w, r)
		return
	}
	sessionID := parts[0]
	if sessionID == "" {
		http.NotFound(w, r)
		return
	}

	if s.draftStore == nil {
		writeJSONError(w, "no active draft session", http.StatusNotFound)
		return
	}
	sess, ok := s.draftStore.Get(sessionID)
	if !ok {
		writeJSONError(w, "no active draft session", http.StatusNotFound)
		return
	}

	cards := make([]currentPackCard, 0, len(sess.CurrentCards))
	for _, id := range sess.CurrentCards {
		cards = append(cards, currentPackCard{
			ArenaID:  id,
			CardName: lookupName(s, strconv.Itoa(id)),
			GIHWR:    lookupGIHWR(s, strconv.Itoa(id), sess.Format),
		})
	}

	writeJSON(w, r, http.StatusOK, currentPackResponse{
		SessionID:      sess.ID,
		PackNumber:     sess.CurrentPack + 1, // SPA displays 1-based
		PickNumber:     sess.CurrentPick + 1,
		Cards:          cards,
		Recommendation: "", // Wails-era simple recommendation logic deferred
		SetCode:        sess.SetCode,
		Format:         sess.Format,
	})
}

// handleDraftGradePick — POST /api/v1/drafts/grade-pick
func (s *Server) handleDraftGradePick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req gradePickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	pack, pickedID, format, err := s.resolvePack(req)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	packIDs := make([]string, 0, len(pack))
	for _, id := range pack {
		packIDs = append(packIDs, strconv.Itoa(id))
	}
	quality, err := pickquality.Analyze(
		format,
		packIDs,
		strconv.Itoa(pickedID),
		s.ratings(),
		s.cards(),
	)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, r, http.StatusOK, quality)
}

// handleDraftWinProbability — POST /api/v1/drafts/win-probability
func (s *Server) handleDraftWinProbability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req winProbabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if s.draftStore == nil {
		writeJSON(w, r, http.StatusOK, winProbabilityResponse{Probability: 0.50})
		return
	}
	sess, ok := s.draftStore.Get(req.SessionID)
	if !ok || len(sess.Picks) == 0 {
		writeJSON(w, r, http.StatusOK, winProbabilityResponse{Probability: 0.50})
		return
	}

	deck := make([]prediction.Card, 0, len(sess.Picks))
	for _, p := range sess.Picks {
		if p.Picked == 0 {
			continue
		}
		idStr := strconv.Itoa(p.Picked)
		deck = append(deck, prediction.Card{
			Name:  lookupName(s, idStr),
			Color: "", // unknown until cards lookup grows color metadata
			GIHWR: lookupGIHWR(s, idStr, sess.Format),
		})
	}
	if len(deck) == 0 {
		writeJSON(w, r, http.StatusOK, winProbabilityResponse{Probability: 0.50})
		return
	}

	pred, err := prediction.PredictWinRate(deck)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, http.StatusOK, winProbabilityResponse{Probability: pred.PredictedWinRate})
}

// ─── helpers ───────────────────────────────────────────────────────────────

// ratings returns the lookup the handler should use. Production today
// always gets noopRatings; tests can override via SetDraftLookups.
func (s *Server) ratings() draftalgo.RatingsLookup {
	if s.ratingsLookup != nil {
		return s.ratingsLookup
	}
	return noopRatings{}
}

// cards returns the card-name lookup. Same override path as ratings.
func (s *Server) cards() draftalgo.CardLookup {
	if s.cardsLookup != nil {
		return s.cardsLookup
	}
	return noopCards{}
}

// resolvePack figures out the offered pack + the picked card ID for a
// grade-pick request. Accepts either the "explicit cards" shape
// (available_card_ids + picked_card_id) or the "session lookup" shape
// (pack_number / pick_number → look in draftstate).
func (s *Server) resolvePack(req gradePickRequest) ([]int, int, string, error) {
	if req.PickedCardID > 0 && len(req.AvailableCardIDs) > 0 {
		return req.AvailableCardIDs, req.PickedCardID, "", nil
	}
	if s.draftStore == nil {
		return nil, 0, "", fmt.Errorf("no draft session: pass picked_card_id + available_card_ids")
	}
	sess, ok := s.draftStore.Get(req.SessionID)
	if !ok {
		return nil, 0, "", fmt.Errorf("session %q not found", req.SessionID)
	}
	// Try the live pack first.
	if len(sess.CurrentCards) > 0 {
		return sess.CurrentCards, firstNonZero(sess.CurrentCards), sess.Format, nil
	}
	// Otherwise fall back to a recorded historical pick if the SPA
	// passed pack/pick coordinates.
	for _, p := range sess.Picks {
		if p.PackNumber == req.PackNumberHint && p.PickNumber == req.PickNumberHint && len(p.PackCards) > 0 {
			return p.PackCards, p.Picked, sess.Format, nil
		}
	}
	return nil, 0, "", fmt.Errorf("no pack data for session %q", req.SessionID)
}

func firstNonZero(ids []int) int {
	for _, id := range ids {
		if id != 0 {
			return id
		}
	}
	return 0
}

// lookupName / lookupGIHWR are thin shims around the Server's lookups
// so handlers stay terse.
func lookupName(s *Server, id string) string { return s.cards().CardName(id) }

func lookupGIHWR(s *Server, id, format string) float64 {
	v, _ := s.ratings().GIHWR(id, format)
	return v
}

// writeJSONError writes an envelope-shaped error response. Kept tiny
// — most handlers don't need the structured error type that the BFF
// uses; a JSON object with a message string is enough for the SPA.
func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
