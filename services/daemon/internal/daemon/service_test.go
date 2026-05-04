package daemon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyEntry_DraftPack(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"draftPack": []interface{}{"card1", "card2"}},
	}
	assert.Equal(t, "draft.pack", classifyEntry(entry))
}

func TestClassifyEntry_DraftPick(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"pickedCards": []interface{}{"card1"}},
	}
	assert.Equal(t, "draft.pick", classifyEntry(entry))
}

func TestClassifyEntry_MatchCompleted(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"CurrentEventState": "MatchCompleted"},
	}
	assert.Equal(t, "match.completed", classifyEntry(entry))
}

func TestClassifyEntry_DraftStarted(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"toSceneName": "Draft"},
	}
	assert.Equal(t, "draft.started", classifyEntry(entry))
}

func TestClassifyEntry_PlayerAuthenticated(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"authenticateResponse": map[string]interface{}{"screenName": "Ray"}},
	}
	assert.Equal(t, "player.authenticated", classifyEntry(entry))
}

func TestClassifyEntry_RankUpdated(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"rankClass": "Gold", "rankTier": float64(2)},
	}
	assert.Equal(t, "player.rank_updated", classifyEntry(entry))
}

func TestClassifyEntry_MatchStarted(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"CurrentEventState": "MatchInProgress"},
	}
	assert.Equal(t, "match.started", classifyEntry(entry))
}

func TestClassifyEntry_DraftEnded(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"toSceneName":   "Home",
			"fromSceneName": "Draft",
		},
	}
	assert.Equal(t, "draft.ended", classifyEntry(entry))
}

func TestClassifyEntry_Unknown(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"someOtherKey": "value"},
	}
	assert.Equal(t, "", classifyEntry(entry))
}

func TestClassifyEntry_NotJSON(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: false,
		Raw:    "plain text",
	}
	assert.Equal(t, "", classifyEntry(entry))
}

// TestHandleEntry_DraftPackDispatchesTypedPayload verifies that handleEntry
// parses a draft.pack entry into a DraftPackPayload and sends it to the BFF
// with the correct typed JSON keys (PackCards, SelfPick, CourseName).
func TestHandleEntry_DraftPackDispatchesTypedPayload(t *testing.T) {
	var received contract.DaemonEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-key",
		AccountID:   "acc-1",
	}
	svc := New(cfg)

	// Construct the entry as the reader would after parsing the log line.
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"CourseName": "PremierDraft_BLB",
			"draftPack": map[string]interface{}{
				"PackCards": []interface{}{float64(12345), float64(67890)},
				"SelfPick":  float64(1),
			},
		},
	}

	require.NoError(t, svc.handleEntry(context.Background(), entry))
	assert.Equal(t, "draft.pack", received.Type)
	assert.Equal(t, "acc-1", received.AccountID)

	var payload logreader.DraftPackPayload
	require.NoError(t, json.Unmarshal(received.Payload, &payload))
	assert.Equal(t, "PremierDraft_BLB", payload.CourseName)
	assert.Equal(t, []int{12345, 67890}, payload.DraftPack.PackCards)
	assert.Equal(t, 1, payload.DraftPack.SelfPick)
}

// TestHandleEntry_DraftPickDispatchesTypedPayload verifies that handleEntry
// parses a draft.pick entry into a DraftPickPayload and sends it to the BFF
// with the correct typed JSON keys (pickedCards, PackNumber, PickNumber, CourseName).
func TestHandleEntry_DraftPickDispatchesTypedPayload(t *testing.T) {
	var received contract.DaemonEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-key",
		AccountID:   "acc-2",
	}
	svc := New(cfg)

	// Construct the entry as the reader would after parsing the log line.
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"CourseName":  "PremierDraft_BLB",
			"pickedCards": []interface{}{float64(12345)},
			"PackNumber":  float64(0),
			"PickNumber":  float64(3),
		},
	}

	require.NoError(t, svc.handleEntry(context.Background(), entry))
	assert.Equal(t, "draft.pick", received.Type)
	assert.Equal(t, "acc-2", received.AccountID)

	var payload logreader.DraftPickPayload
	require.NoError(t, json.Unmarshal(received.Payload, &payload))
	assert.Equal(t, "PremierDraft_BLB", payload.CourseName)
	assert.Equal(t, []int{12345}, payload.PickedCards)
	assert.Equal(t, 0, payload.PackNumber)
	assert.Equal(t, 3, payload.PickNumber)
}
