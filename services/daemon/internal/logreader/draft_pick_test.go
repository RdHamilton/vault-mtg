package logreader

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDraftPackPayload_RoundTrip verifies that DraftPackPayload marshals and
// unmarshals with the correct JSON keys.
func TestDraftPackPayload_RoundTrip(t *testing.T) {
	original := DraftPackPayload{
		CourseName: "PremierDraft_BLB",
		DraftPack: DraftPackDetail{
			PackCards: []int{1, 2, 3},
			SelfPick:  5,
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded DraftPackPayload
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original, decoded)

	// Verify actual key names in the wire JSON.
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "CourseName")
	assert.Contains(t, raw, "draftPack")

	pack, ok := raw["draftPack"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, pack, "PackCards")
	assert.Contains(t, pack, "SelfPick")
}

// TestDraftPickPayload_RoundTrip verifies that DraftPickPayload marshals and
// unmarshals with the correct JSON keys.
func TestDraftPickPayload_RoundTrip(t *testing.T) {
	original := DraftPickPayload{
		CourseName:  "PremierDraft_BLB",
		PickedCards: []int{42},
		PackNumber:  0,
		PickNumber:  7,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded DraftPickPayload
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original, decoded)

	// Verify actual key names in the wire JSON.
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "CourseName")
	assert.Contains(t, raw, "pickedCards")
	assert.Contains(t, raw, "PackNumber")
	assert.Contains(t, raw, "PickNumber")
}

// ---------------------------------------------------------------------------
// Premier draft (#338) — Draft.Notify (pack) + EventPlayerDraftMakePick (pick)
// ---------------------------------------------------------------------------
//
// Real wire shapes (MTGA 2026.59.20, ~/.vaultmtg/archives Premier capture):
//
//	[UnityCrossThreadLogger]Draft.Notify {"draftId":"<uuid>","SelfPick":1,"SelfPack":1,"PackCards":"102614,102609,..."}
//	[UnityCrossThreadLogger]==> EventPlayerDraftMakePick {"id":"<uuid>","request":"{\"DraftId\":\"<uuid>\",\"GrpIds\":[102647],\"Pack\":1,\"Pick\":1}"}
//
// PackCards is a comma-separated STRING (not a JSON array). The pick request
// is a JSON STRING nested under "request" (double-unmarshal). SelfPick/SelfPack
// and Pick/Pack are 1-based; SelfPick resets to 1 on each new pack (within-pack).

// TestParsePremierDraftNotify_ValidEntry verifies a Draft.Notify line for
// pack 1, pick 3, 12 cards maps to the existing DraftPackPayload with the
// comma-string PackCards split into a slice and the cumulative SelfPick decode.
func TestParsePremierDraftNotify_ValidEntry(t *testing.T) {
	raw := `[UnityCrossThreadLogger]Draft.Notify {"draftId":"62a14a91-bb89-470a-a7c0-6ad8d7ddf227","SelfPick":3,"SelfPack":1,"PackCards":"102709,102613,102573,102535,102621,102577,102571,102473,102601,102540,102774,102721"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParsePremierDraftNotify(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "62a14a91-bb89-470a-a7c0-6ad8d7ddf227", p.DraftID)
	// CourseName is empty for Premier — Draft.Notify carries no CourseName.
	assert.Equal(t, "", p.CourseName)
	// PackCards comma-string is split into 12 ints.
	assert.Equal(t, []int{102709, 102613, 102573, 102535, 102621, 102577, 102571, 102473, 102601, 102540, 102774, 102721}, p.DraftPack.PackCards)
	// Cumulative 1-based SelfPick: (SelfPack-1)*15 + SelfPick = (1-1)*15 + 3 = 3.
	assert.Equal(t, 3, p.DraftPack.SelfPick)
}

// TestParsePremierDraftNotify_Pack2 verifies the within-pack reset: SelfPack=2,
// SelfPick=1 decodes to the cumulative 1-based index 16, which the draftstate
// normalises to PackNumber=1.
func TestParsePremierDraftNotify_Pack2(t *testing.T) {
	raw := `[UnityCrossThreadLogger]Draft.Notify {"draftId":"abc","SelfPick":1,"SelfPack":2,"PackCards":"1,2,3"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParsePremierDraftNotify(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	// (SelfPack-1)*15 + SelfPick = (2-1)*15 + 1 = 16.
	assert.Equal(t, 16, p.DraftPack.SelfPick)
	assert.Equal(t, []int{1, 2, 3}, p.DraftPack.PackCards)
}

// TestParsePremierDraftNotify_MissingDraftId errors when draftId is absent.
func TestParsePremierDraftNotify_MissingDraftId(t *testing.T) {
	raw := `{"SelfPick":1,"SelfPack":1,"PackCards":"1,2,3"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	_, err := ParsePremierDraftNotify(entry)
	assert.Error(t, err)
}

// TestParsePremierDraftNotify_EmptyPackCards verifies the empty-string guard:
// an empty PackCards must yield an empty slice, NOT []int{0} or a parse error.
func TestParsePremierDraftNotify_EmptyPackCards(t *testing.T) {
	raw := `{"draftId":"abc","SelfPick":14,"SelfPack":3,"PackCards":""}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParsePremierDraftNotify(entry)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Empty(t, p.DraftPack.PackCards, "empty PackCards must yield an empty slice, not [\"\"]→[0]")
}

// TestParsePremierDraftMakePick_ValidEntry verifies the EventPlayerDraftMakePick
// request line (pack 1, pick 1, grpId 102647) maps to DraftPickPayload with
// 0-based pack/pick numbers and the DraftId propagated.
func TestParsePremierDraftMakePick_ValidEntry(t *testing.T) {
	raw := `[UnityCrossThreadLogger]==> EventPlayerDraftMakePick {"id":"e1acfb90-a0c3-4230-9527-e64d7a0abc5e","request":"{\"DraftId\":\"62a14a91-bb89-470a-a7c0-6ad8d7ddf227\",\"GrpIds\":[102647],\"Pack\":1,\"Pick\":1}"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParsePremierDraftMakePick(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "62a14a91-bb89-470a-a7c0-6ad8d7ddf227", p.DraftID)
	assert.Equal(t, "", p.CourseName)
	assert.Equal(t, []int{102647}, p.PickedCards)
	// Pack:1 Pick:1 → 0-based.
	assert.Equal(t, 0, p.PackNumber)
	assert.Equal(t, 0, p.PickNumber)
}

// TestParsePremierDraftMakePick_Pack3Pick13 verifies 0-based conversion for a
// later pick (Pack:3, Pick:13 → PackNumber=2, PickNumber=12).
func TestParsePremierDraftMakePick_Pack3Pick13(t *testing.T) {
	raw := `[UnityCrossThreadLogger]==> EventPlayerDraftMakePick {"id":"x","request":"{\"DraftId\":\"d\",\"GrpIds\":[999],\"Pack\":3,\"Pick\":13}"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParsePremierDraftMakePick(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, 2, p.PackNumber)
	assert.Equal(t, 12, p.PickNumber)
	assert.Equal(t, []int{999}, p.PickedCards)
}

// TestParsePremierDraftMakePick_MissingRequest errors when the request key is absent.
func TestParsePremierDraftMakePick_MissingRequest(t *testing.T) {
	raw := `{"id":"abc"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	_, err := ParsePremierDraftMakePick(entry)
	assert.Error(t, err)
}

// TestParsePremierDraftMakePick_NonPickRequest errors when the nested request
// JSON does not carry a DraftId (strict unmarshal + DraftId != "" check, per
// Ray's note — the parser is strict; the classifier may keep a Contains shortcut).
func TestParsePremierDraftMakePick_NonPickRequest(t *testing.T) {
	raw := `{"id":"abc","request":"{\"SomethingElse\":1}"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	_, err := ParsePremierDraftMakePick(entry)
	assert.Error(t, err)
}
