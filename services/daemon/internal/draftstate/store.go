// Package draftstate keeps the daemon's view of in-progress MTGA draft
// sessions in memory. It is fed by the existing logreader event stream
// from services/daemon/internal/daemon.Service — every parsed draft.pack
// and draft.pick payload flows through Store.HandlePack / HandlePick.
//
// The localapi handlers in services/daemon/internal/localapi/drafts.go
// read from this Store to answer:
//
//	GET  /api/v1/drafts/{id}/current-pack
//	POST /api/v1/drafts/grade-pick
//	POST /api/v1/drafts/win-probability
//
// Why does this live in the daemon and not the BFF?
// Because only the daemon sees the live Player.log stream. The BFF has
// every completed draft in its database but cannot see the current pack
// the player is staring at right now. That's what the user wants the
// "draft live" panel to show.
//
// Session identity: MTGA's logs don't carry a stable per-draft session
// ID at the start of a draft — we synthesize one from CourseName plus
// the first-pack timestamp. The localapi accepts the literal string
// "current" for callers who just want "whatever draft is happening
// right now."
package draftstate

import (
	"strings"
	"sync"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
)

// Pick captures one pack-and-pick combination — the offered cards and
// (optionally) the card the player chose. Until the pick lands, Picked
// is 0.
type Pick struct {
	PackNumber int   // 0-based
	PickNumber int   // 0-based
	PackCards  []int // arena IDs offered in this pack
	Picked     int   // arena ID picked; 0 before the pick lands
}

// Session is the daemon's view of one in-progress (or recently finished)
// draft. Two timestamps disambiguate sessions sharing a CourseName.
type Session struct {
	ID         string // synthetic: "<CourseName>:<RFC3339 timestamp>"
	CourseName string // raw from MTGA, e.g. "PremierDraft_BLB"
	SetCode    string // derived suffix, e.g. "BLB"
	Format     string // derived prefix, e.g. "PremierDraft"
	Picks      []Pick
	StartedAt  time.Time
	UpdatedAt  time.Time

	// CurrentPack / CurrentPick are 0-based positions of the most recent
	// pack the player saw. CurrentCards is the offered pack — empty
	// once the pick lands.
	CurrentPack  int
	CurrentPick  int
	CurrentCards []int
}

// Store holds every active session keyed by its synthetic ID. Concurrent
// reads (from localapi handlers) and writes (from the daemon's log
// goroutine) share a RWMutex.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	// currentID tracks the session most recently touched by a draft
	// event. The localapi's "current" lookup returns this one.
	currentID string
	// now lets tests override time.Now() for deterministic IDs.
	now func() time.Time
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		sessions: map[string]*Session{},
		now:      time.Now,
	}
}

// SetClock overrides the time source. Tests only.
func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// HandlePack records a fresh pack the player is looking at. If this is
// the first pack of a new draft (pack 0, pick 0), a new Session is
// minted; otherwise the existing session is updated.
func (s *Store) HandlePack(p *logreader.DraftPackPayload) {
	if p == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// SelfPick in MTGA logs is 1-based. We normalise to 0-based here
	// so the math downstream is consistent with what the SPA + algos
	// expect.
	pickIdx := p.DraftPack.SelfPick - 1
	if pickIdx < 0 {
		pickIdx = 0
	}
	packIdx := pickIdx / 15 // 15 picks per pack in standard MTGA draft

	session := s.findOrCreate(p.CourseName, pickIdx)
	session.CurrentPack = packIdx
	session.CurrentPick = pickIdx % 15
	session.CurrentCards = append(session.CurrentCards[:0], p.DraftPack.PackCards...)
	session.UpdatedAt = s.now()
	s.currentID = session.ID
}

// HandlePick records a pick the player just made. If we have an open
// pack with no recorded pick yet, finalise it.
func (s *Store) HandlePick(p *logreader.DraftPickPayload) {
	if p == nil || len(p.PickedCards) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.findByCourse(p.CourseName)
	if !ok {
		// Pick arrived before we saw any pack — synthesize a session
		// keyed by the pick timestamp so the data isn't lost.
		session = s.create(p.CourseName, p.PackNumber*15+p.PickNumber)
	}

	pick := Pick{
		PackNumber: p.PackNumber,
		PickNumber: p.PickNumber,
		Picked:     p.PickedCards[0],
	}
	// If the in-flight pack matches this pick's coordinates, attach
	// the offered cards too so the historical record is complete.
	if session.CurrentPack == p.PackNumber && session.CurrentPick == p.PickNumber {
		pick.PackCards = append([]int(nil), session.CurrentCards...)
		// Clear current pack — the user has moved on.
		session.CurrentCards = nil
	}
	session.Picks = append(session.Picks, pick)
	session.UpdatedAt = s.now()
	s.currentID = session.ID
}

// Get returns the session with the given ID, or the most recently
// touched session when id == "current".
func (s *Store) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id == "current" {
		if s.currentID == "" {
			return nil, false
		}
		sess, ok := s.sessions[s.currentID]
		if !ok {
			return nil, false
		}
		return cloneSession(sess), true
	}
	sess, ok := s.sessions[id]
	if !ok {
		// Fallback — if the caller passed an opaque session ID we don't
		// know (e.g. the BFF-issued ID), surface the current session
		// rather than 404'ing. This is the live-draft path the SPA
		// almost always wants.
		if s.currentID != "" {
			sess, ok = s.sessions[s.currentID]
		}
	}
	if !ok || sess == nil {
		return nil, false
	}
	return cloneSession(sess), true
}

// Sessions returns a snapshot of every tracked session. Tests use this;
// no production caller currently.
func (s *Store) Sessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, cloneSession(sess))
	}
	return out
}

// ─── internals ─────────────────────────────────────────────────────────────

// findOrCreate locates the active session for course or mints a new one.
// pickIdx is the cumulative 0-based pick number; pickIdx == 0 always
// means "new draft."
func (s *Store) findOrCreate(course string, pickIdx int) *Session {
	if pickIdx > 0 {
		if sess, ok := s.findByCourse(course); ok {
			return sess
		}
	}
	return s.create(course, pickIdx)
}

func (s *Store) findByCourse(course string) (*Session, bool) {
	// Most recently updated session matching course. Reverse iteration
	// over the map is fine — we expect ≤1 active session per course at
	// any time.
	var best *Session
	for _, sess := range s.sessions {
		if sess.CourseName != course {
			continue
		}
		if best == nil || sess.UpdatedAt.After(best.UpdatedAt) {
			best = sess
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

func (s *Store) create(course string, _ int) *Session {
	now := s.now()
	id := course + ":" + now.UTC().Format(time.RFC3339Nano)
	format, setCode := splitCourse(course)
	sess := &Session{
		ID:         id,
		CourseName: course,
		SetCode:    setCode,
		Format:     format,
		StartedAt:  now,
		UpdatedAt:  now,
	}
	s.sessions[id] = sess
	return sess
}

// splitCourse pulls the format prefix and set suffix out of an MTGA
// CourseName like "PremierDraft_BLB" → ("PremierDraft", "BLB"). Falls
// back gracefully when the format doesn't match.
func splitCourse(course string) (string, string) {
	idx := strings.LastIndex(course, "_")
	if idx <= 0 || idx == len(course)-1 {
		return course, ""
	}
	return course[:idx], course[idx+1:]
}

// cloneSession returns a deep copy safe for handlers to read without
// holding the Store's RWMutex.
func cloneSession(sess *Session) *Session {
	if sess == nil {
		return nil
	}
	picks := make([]Pick, len(sess.Picks))
	for i, p := range sess.Picks {
		picks[i] = Pick{
			PackNumber: p.PackNumber,
			PickNumber: p.PickNumber,
			Picked:     p.Picked,
		}
		if len(p.PackCards) > 0 {
			picks[i].PackCards = append([]int(nil), p.PackCards...)
		}
	}
	var cards []int
	if len(sess.CurrentCards) > 0 {
		cards = append([]int(nil), sess.CurrentCards...)
	}
	return &Session{
		ID:           sess.ID,
		CourseName:   sess.CourseName,
		SetCode:      sess.SetCode,
		Format:       sess.Format,
		Picks:        picks,
		StartedAt:    sess.StartedAt,
		UpdatedAt:    sess.UpdatedAt,
		CurrentPack:  sess.CurrentPack,
		CurrentPick:  sess.CurrentPick,
		CurrentCards: cards,
	}
}
