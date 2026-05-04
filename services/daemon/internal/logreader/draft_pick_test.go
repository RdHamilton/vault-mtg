package logreader

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDraftPack_ValidEntry verifies that a well-formed BotDraft_DraftPack log line
// is parsed into a DraftPackPayload with the correct MTGA JSON keys.
func TestParseDraftPack_ValidEntry(t *testing.T) {
	raw := `{"draftPack":{"PackCards":[12345,67890,11111],"SelfPick":1},"CourseName":"PremierDraft_BLB"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()

	require.True(t, entry.IsJSON, "entry should be parsed as JSON")

	p, err := ParseDraftPack(entry)
	require.NoError(t, err)

	assert.Equal(t, "PremierDraft_BLB", p.CourseName)
	assert.Equal(t, []int{12345, 67890, 11111}, p.DraftPack.PackCards)
	assert.Equal(t, 1, p.DraftPack.SelfPick)
}

// TestParseDraftPack_HumanDraft verifies parsing of a human draft pack event.
func TestParseDraftPack_HumanDraft(t *testing.T) {
	raw := `{"draftPack":{"PackCards":[99999,88888],"SelfPick":2},"CourseName":"TradDraft_BLB"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParseDraftPack(entry)
	require.NoError(t, err)
	assert.Equal(t, "TradDraft_BLB", p.CourseName)
	assert.Equal(t, []int{99999, 88888}, p.DraftPack.PackCards)
	assert.Equal(t, 2, p.DraftPack.SelfPick)
}

// TestParseDraftPack_MissingKey returns an error when "draftPack" key is absent.
func TestParseDraftPack_MissingKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"pickedCards": []interface{}{float64(12345)}},
	}
	_, err := ParseDraftPack(entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftPack")
}

// TestParseDraftPack_NilEntry returns an error for a nil entry.
func TestParseDraftPack_NilEntry(t *testing.T) {
	_, err := ParseDraftPack(nil)
	assert.Error(t, err)
}

// TestParseDraftPack_NonJSONEntry returns an error for a non-JSON entry.
func TestParseDraftPack_NonJSONEntry(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	_, err := ParseDraftPack(entry)
	assert.Error(t, err)
}

// TestParseDraftPick_ValidEntry verifies that a well-formed BotDraft_DraftPickResp log line
// is parsed into a DraftPickPayload with the correct MTGA JSON keys.
func TestParseDraftPick_ValidEntry(t *testing.T) {
	raw := `{"pickedCards":[12345],"PackNumber":0,"PickNumber":3,"CourseName":"PremierDraft_BLB"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParseDraftPick(entry)
	require.NoError(t, err)

	assert.Equal(t, "PremierDraft_BLB", p.CourseName)
	assert.Equal(t, []int{12345}, p.PickedCards)
	assert.Equal(t, 0, p.PackNumber)
	assert.Equal(t, 3, p.PickNumber)
}

// TestParseDraftPick_Pack2 verifies pack/pick numbers for pack 2.
func TestParseDraftPick_Pack2(t *testing.T) {
	raw := `{"pickedCards":[67890],"PackNumber":1,"PickNumber":0,"CourseName":"PremierDraft_BLB"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParseDraftPick(entry)
	require.NoError(t, err)
	assert.Equal(t, 1, p.PackNumber)
	assert.Equal(t, 0, p.PickNumber)
	assert.Equal(t, []int{67890}, p.PickedCards)
}

// TestParseDraftPick_MissingKey returns an error when "pickedCards" key is absent.
func TestParseDraftPick_MissingKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"draftPack": map[string]interface{}{}},
	}
	_, err := ParseDraftPick(entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pickedCards")
}

// TestParseDraftPick_NilEntry returns an error for a nil entry.
func TestParseDraftPick_NilEntry(t *testing.T) {
	_, err := ParseDraftPick(nil)
	assert.Error(t, err)
}

// TestParseDraftPick_NonJSONEntry returns an error for a non-JSON entry.
func TestParseDraftPick_NonJSONEntry(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	_, err := ParseDraftPick(entry)
	assert.Error(t, err)
}

// TestParseDraftPick_EmptyPickedCards verifies that an empty pickedCards slice is valid.
func TestParseDraftPick_EmptyPickedCards(t *testing.T) {
	raw := `{"pickedCards":[],"PackNumber":0,"PickNumber":0,"CourseName":"PremierDraft_BLB"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParseDraftPick(entry)
	require.NoError(t, err)
	assert.Empty(t, p.PickedCards)
}

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

// TestParseDraftPack_PrefixedLogLine verifies parsing from a full MTGA log line
// including the UnityCrossThreadLogger prefix.
func TestParseDraftPack_PrefixedLogLine(t *testing.T) {
	raw := `[UnityCrossThreadLogger]5/1/2024 10:00:00 AM {"draftPack":{"PackCards":[55555,66666],"SelfPick":1},"CourseName":"PremierDraft_MKM"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParseDraftPack(entry)
	require.NoError(t, err)
	assert.Equal(t, "PremierDraft_MKM", p.CourseName)
	assert.Equal(t, []int{55555, 66666}, p.DraftPack.PackCards)
}

// TestParseDraftPick_PrefixedLogLine verifies parsing from a full MTGA log line
// including the UnityCrossThreadLogger prefix.
func TestParseDraftPick_PrefixedLogLine(t *testing.T) {
	raw := `[UnityCrossThreadLogger]5/1/2024 10:01:00 AM {"pickedCards":[55555],"PackNumber":0,"PickNumber":0,"CourseName":"PremierDraft_MKM"}`
	entry := &LogEntry{Raw: raw}
	entry.parseJSON()
	require.True(t, entry.IsJSON)

	p, err := ParseDraftPick(entry)
	require.NoError(t, err)
	assert.Equal(t, "PremierDraft_MKM", p.CourseName)
	assert.Equal(t, []int{55555}, p.PickedCards)
}
