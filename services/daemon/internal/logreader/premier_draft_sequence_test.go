package logreader_test

// premier_draft_sequence_test.go — full Premier-draft sequence test built from
// the real MTGA 2026.59.20 Premier capture
// (~/.vaultmtg/archives/Player_capture_premier_20260531T072112Z.log, #338).
//
// The grpIds below are verbatim from the real corpus (non-PII per ADR-041).
// The draftId is a sanitized stable fake — the real corpus draftId is a
// server-assigned UUID scoping the draft pod; we replace it per ADR-041 G3.
//
// This test proves the end-to-end Premier path: read raw log lines → parse →
// feed draftstate, and confirms the within-pack SelfPick reset against the
// real corpus (Ray's required assertion: Pack 2 / Pick 1 → cumulative pick 16).
//
// It lives in the external logreader_test package to feed draftstate without an
// import cycle (draftstate imports logreader).

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/draftstate"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sanitized stable-fake draftId (ADR-041); real corpus value replaced.
const seqDraftID = "00000000-0000-4000-8000-0000000003a8"

// premierStep models one pack+pick pair drawn from the real corpus.
type premierStep struct {
	selfPack  int // 1-based pack number on the wire
	selfPick  int // 1-based WITHIN-PACK pick number on the wire
	packCards []int
	picked    int
}

// corpusSteps are the first three picks of pack 1 plus pack 2 / pick 1 from the
// real Premier capture. Pack 2 / Pick 1 anchors the within-pack-reset assertion.
var corpusSteps = []premierStep{
	{
		selfPack:  1,
		selfPick:  1,
		packCards: []int{102614, 102609, 102691, 102506, 102648, 102470, 102496, 102649, 102532, 102720, 102543, 102789, 102647, 102714},
		picked:    102647,
	},
	{
		selfPack:  1,
		selfPick:  2,
		packCards: []int{102675, 102471, 102577, 102524, 102554, 102614, 102691, 102605, 102512, 102640, 102797, 102809, 102739},
		picked:    102554,
	},
	{
		selfPack:  1,
		selfPick:  3,
		packCards: []int{102709, 102613, 102573, 102535, 102621, 102577, 102571, 102473, 102601, 102540, 102774, 102721},
		picked:    102540,
	},
	{
		selfPack:  2,
		selfPick:  1, // resets to 1 on the new pack — within-pack, not cumulative
		packCards: []int{102565, 102467, 102607, 102548, 102651, 102525, 102679, 102676, 102569, 102693, 102758, 102781, 102638, 102739},
		picked:    102565,
	},
}

func notifyLine(s premierStep) string {
	cards := ""
	for i, c := range s.packCards {
		if i > 0 {
			cards += ","
		}
		cards += fmt.Sprintf("%d", c)
	}
	return fmt.Sprintf(
		`[UnityCrossThreadLogger]Draft.Notify {"draftId":%q,"SelfPick":%d,"SelfPack":%d,"PackCards":%q}`,
		seqDraftID, s.selfPick, s.selfPack, cards,
	)
}

func makePickLine(s premierStep) string {
	return fmt.Sprintf(
		`[UnityCrossThreadLogger]==> EventPlayerDraftMakePick {"id":"corr-%d-%d","request":"{\"DraftId\":\"%s\",\"GrpIds\":[%d],\"Pack\":%d,\"Pick\":%d}"}`,
		s.selfPack, s.selfPick, seqDraftID, s.picked, s.selfPack, s.selfPick,
	)
}

// readOneEntry writes a single raw log line to a temp file and reads it back
// through logreader.NewReader, mirroring the real daemon parse path.
func readOneEntry(t *testing.T, raw string) *logreader.LogEntry {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "line.log")
	require.NoError(t, os.WriteFile(path, []byte(raw+"\n"), 0o600))
	r, err := logreader.NewReader(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	entry, err := r.ReadEntry()
	require.NoError(t, err)
	require.True(t, entry.IsJSON, "line must parse as JSON: %s", raw)
	return entry
}

// TestPremierDraftSequence_FromRealCorpus drives the real-corpus sequence end
// to end and asserts the within-pack reset (Pack 2 / Pick 1 → cumulative 16).
func TestPremierDraftSequence_FromRealCorpus(t *testing.T) {
	store := draftstate.New()

	for _, s := range corpusSteps {
		// --- Pack (Draft.Notify) ---
		packEntry := readOneEntry(t, notifyLine(s))

		pack, err := logreader.ParsePremierDraftNotify(packEntry)
		require.NoErrorf(t, err, "ParsePremierDraftNotify (pack %d pick %d)", s.selfPack, s.selfPick)
		assert.Equal(t, seqDraftID, pack.DraftID)
		assert.Equal(t, "", pack.CourseName, "Premier pack carries no CourseName")
		assert.Equal(t, s.packCards, pack.DraftPack.PackCards)

		// Cumulative 1-based SelfPick = (SelfPack-1)*15 + SelfPick.
		wantCumulative := (s.selfPack-1)*15 + s.selfPick
		assert.Equalf(t, wantCumulative, pack.DraftPack.SelfPick,
			"cumulative SelfPick for pack %d pick %d", s.selfPack, s.selfPick)

		store.HandlePack(pack)

		// --- Pick (EventPlayerDraftMakePick) ---
		pickEntry := readOneEntry(t, makePickLine(s))

		pick, err := logreader.ParsePremierDraftMakePick(pickEntry)
		require.NoErrorf(t, err, "ParsePremierDraftMakePick (pack %d pick %d)", s.selfPack, s.selfPick)
		assert.Equal(t, seqDraftID, pick.DraftID)
		assert.Equal(t, []int{s.picked}, pick.PickedCards)
		assert.Equal(t, s.selfPack-1, pick.PackNumber, "0-based pack number")
		assert.Equal(t, s.selfPick-1, pick.PickNumber, "0-based within-pack pick number")

		store.HandlePick(pick)
	}

	// Ray's required assertion: Pack 2 / Pick 1 maps to cumulative pick 16,
	// proving the within-pack reset is confirmed against the real corpus.
	p2p1 := corpusSteps[3]
	require.Equal(t, 2, p2p1.selfPack)
	require.Equal(t, 1, p2p1.selfPick)
	assert.Equal(t, 16, (p2p1.selfPack-1)*15+p2p1.selfPick,
		"Pack 2 / Pick 1 must map to cumulative pick 16 (within-pack reset confirmed)")

	// All four picks landed in a single draftId-keyed session.
	require.Lenf(t, store.Sessions(), 1, "expected exactly 1 session keyed by draftId")
	sess, ok := store.Get("current")
	require.True(t, ok, "expected a current session")
	assert.Equal(t, seqDraftID, sess.CourseName, "session keyed by draftId when CourseName is empty")
	require.Len(t, sess.Picks, 4, "all four picks recorded")
	assert.Equal(t, 102647, sess.Picks[0].Picked)
	assert.Equal(t, 102565, sess.Picks[3].Picked)

	// The most recent pack the player saw is pack 2 (0-based pack index 1),
	// within-pack pick 0 — pulled from cumulative index 15 → 15/15=1, 15%15=0.
	assert.Equal(t, 1, sess.CurrentPack, "current pack is pack 2 (0-based)")
	assert.Equal(t, 0, sess.CurrentPick, "current within-pack pick is 0")
}
