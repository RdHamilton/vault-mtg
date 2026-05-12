package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestJWT constructs a minimal unsigned JWT with the given exp Unix timestamp.
// The signature segment is a placeholder; it is never verified by the daemon.
func makeTestJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	return header + "." + claims + ".fakesig"
}

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

func TestClassifyEntry_InventoryUpdated(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"InventoryInfo": map[string]interface{}{
				"Gems": float64(1200),
				"Gold": float64(5000),
			},
		},
	}
	assert.Equal(t, "inventory.updated", classifyEntry(entry))
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

// TestHandleEntry_InventoryUpdatedDispatchesTypedPayload verifies that handleEntry
// parses an inventory.updated entry into a contract.InventoryUpdatedPayload and
// sends it to the BFF with the correct event type and JSON field names.
func TestHandleEntry_InventoryUpdatedDispatchesTypedPayload(t *testing.T) {
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
		AccountID:   "acc-inv",
	}
	svc := New(cfg)

	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"InventoryInfo": map[string]interface{}{
				"Gems":              float64(1200),
				"Gold":              float64(5000),
				"WildCardCommons":   float64(10),
				"WildCardUnCommons": float64(5),
				"WildCardRares":     float64(3),
				"WildCardMythics":   float64(1),
				"Boosters": []interface{}{
					map[string]interface{}{
						"CollationId": float64(100078),
						"SetCode":     "BLB",
						"Count":       float64(2),
					},
				},
			},
		},
	}

	require.NoError(t, svc.handleEntry(context.Background(), entry))
	assert.Equal(t, "inventory.updated", received.Type)
	assert.Equal(t, "acc-inv", received.AccountID)

	var payload contract.InventoryUpdatedPayload
	require.NoError(t, json.Unmarshal(received.Payload, &payload))
	assert.Equal(t, 1200, payload.Gems)
	assert.Equal(t, 5000, payload.Gold)
	assert.Equal(t, 10, payload.WildCardCommons)
	assert.Equal(t, 5, payload.WildCardUncommons)
	assert.Equal(t, 3, payload.WildCardRares)
	assert.Equal(t, 1, payload.WildCardMythics)
	require.Len(t, payload.Boosters, 1)
	assert.Equal(t, "BLB", payload.Boosters[0].SetCode)
	assert.Equal(t, 2, payload.Boosters[0].Count)
}

// ---------------------------------------------------------------------------
// Periodic JWT refresh tests
//
// Strategy: set jwtRefreshInterval to a very short duration so the ticker
// fires quickly, then cancel the context to stop the run loop. We inspect
// how many times the BFF registration endpoint was called.
// ---------------------------------------------------------------------------

// TestRunRefreshesJWTWhenNearExpiry verifies that the run loop re-registers
// when the stored JWT is within the refresh window (< 24 h remaining).
func TestRunRefreshesJWTWhenNearExpiry(t *testing.T) {
	var registerCalls atomic.Int32

	// Stub: registration endpoint returns a fresh long-lived JWT.
	freshJWT := makeTestJWT(time.Now().Add(30 * 24 * time.Hour).Unix())
	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/daemon/register" {
			registerCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"token":%q,"daemon_id":"test-daemon-id"}`, freshJWT)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer regSrv.Close()

	// JWT that expires in 23 h — within the 24 h refresh window.
	nearExpiryJWT := makeTestJWT(time.Now().Add(23 * time.Hour).Unix())

	cfg := &config.Config{
		CloudAPIURL: regSrv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-api-key",
		AccountID:   "acc-refresh-test",
		SyncEnabled: true,
		DaemonJWT:   nearExpiryJWT,
		LogPath:     "/dev/null",
	}

	svc := New(cfg)

	// Shorten the ticker to 10 ms so it fires quickly in the test.
	oldInterval := jwtRefreshInterval
	jwtRefreshInterval = 10 * time.Millisecond
	t.Cleanup(func() { jwtRefreshInterval = oldInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	// Run returns when ctx is cancelled; ignore the context-cancelled error.
	_ = svc.Run(ctx)

	// The ticker should have fired at least once and called register.
	assert.GreaterOrEqual(t, registerCalls.Load(), int32(1),
		"expected at least one periodic register call when JWT is near expiry")
}

// TestRunDoesNotRefreshJWTWhenFresh verifies that the run loop does NOT
// call the registration endpoint when the stored JWT has plenty of time left.
func TestRunDoesNotRefreshJWTWhenFresh(t *testing.T) {
	var registerCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/daemon/register" {
			registerCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError) // should never be reached
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	// JWT expiring 30 days from now — well outside the 24 h refresh window.
	freshJWT := makeTestJWT(time.Now().Add(30 * 24 * time.Hour).Unix())

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-api-key",
		AccountID:   "acc-fresh-test",
		SyncEnabled: true,
		DaemonJWT:   freshJWT,
		LogPath:     "/dev/null",
	}

	svc := New(cfg)

	oldInterval := jwtRefreshInterval
	jwtRefreshInterval = 10 * time.Millisecond
	t.Cleanup(func() { jwtRefreshInterval = oldInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_ = svc.Run(ctx)

	assert.Equal(t, int32(0), registerCalls.Load(),
		"register must not be called when JWT is fresh")
}

// TestRunSkipsRefreshWhenSyncDisabled verifies that the ticker does not
// attempt re-registration when SyncEnabled is false, even if the JWT is stale.
func TestRunSkipsRefreshWhenSyncDisabled(t *testing.T) {
	var registerCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/daemon/register" {
			registerCalls.Add(1)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	expiredJWT := makeTestJWT(time.Now().Add(-1 * time.Hour).Unix())

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-api-key",
		AccountID:   "acc-sync-disabled",
		SyncEnabled: false, // sync is off — no registration should occur
		DaemonJWT:   expiredJWT,
		LogPath:     "/dev/null",
	}

	svc := New(cfg)

	oldInterval := jwtRefreshInterval
	jwtRefreshInterval = 10 * time.Millisecond
	t.Cleanup(func() { jwtRefreshInterval = oldInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_ = svc.Run(ctx)

	assert.Equal(t, int32(0), registerCalls.Load(),
		"register must not be called when sync is disabled")
}

// ---------------------------------------------------------------------------
// Update check ticker tests
//
// Strategy: set updateCheckInterval to a very short duration so the ticker
// fires quickly, then cancel the context. Count how many times the BFF
// version endpoint was called.
// ---------------------------------------------------------------------------

// TestRunFiresUpdateCheckOnStartupAndTicker verifies that the update check is
// called once on startup (via goroutine) and again when the ticker fires.
func TestRunFiresUpdateCheckOnStartupAndTicker(t *testing.T) {
	var versionCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/daemon/version":
			versionCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"latest":"0.1.0","released_at":"2026-01-01T00:00:00Z","download_url":"https://example.com"}`)
		case "/api/daemon/register":
			w.Header().Set("Content-Type", "application/json")
			freshJWT := makeTestJWT(time.Now().Add(30 * 24 * time.Hour).Unix())
			fmt.Fprintf(w, `{"token":%q,"daemon_id":"test-id"}`, freshJWT)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()

	freshJWT := makeTestJWT(time.Now().Add(30 * 24 * time.Hour).Unix())
	cfg := &config.Config{
		CloudAPIURL:        srv.URL,
		IngestPath:         "/v1/ingest/events",
		APIKey:             "test-api-key",
		AccountID:          "acc-update-check",
		SyncEnabled:        true,
		DaemonJWT:          freshJWT,
		LogPath:            "/dev/null",
		DisableUpdateCheck: false,
	}

	svc := New(cfg)
	svc.WithVersion("0.1.0") // same as latest — no WARN, but check still fires

	oldUpdateInterval := updateCheckInterval
	updateCheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { updateCheckInterval = oldUpdateInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = svc.Run(ctx)

	// Startup goroutine + at least one ticker fire = at least 2 calls.
	assert.GreaterOrEqual(t, versionCalls.Load(), int32(2),
		"expected startup check plus at least one ticker-driven check")
}

// TestRunSkipsUpdateCheckWhenDisabled verifies that no call to the version
// endpoint is made when DisableUpdateCheck is true.
func TestRunSkipsUpdateCheckWhenDisabled(t *testing.T) {
	var versionCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/daemon/version" {
			versionCalls.Add(1)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	freshJWT := makeTestJWT(time.Now().Add(30 * 24 * time.Hour).Unix())
	cfg := &config.Config{
		CloudAPIURL:        srv.URL,
		IngestPath:         "/v1/ingest/events",
		APIKey:             "test-api-key",
		AccountID:          "acc-no-update-check",
		SyncEnabled:        true,
		DaemonJWT:          freshJWT,
		LogPath:            "/dev/null",
		DisableUpdateCheck: true,
	}

	svc := New(cfg)
	svc.WithVersion("0.1.0")

	oldUpdateInterval := updateCheckInterval
	updateCheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { updateCheckInterval = oldUpdateInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = svc.Run(ctx)

	assert.Equal(t, int32(0), versionCalls.Load(),
		"version endpoint must not be called when DisableUpdateCheck is true")
}

// TestClassifyEntry_CollectionUpdated verifies that a flat card-ID map entry
// is classified as "collection.updated".
func TestClassifyEntry_CollectionUpdated(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"12345": float64(4),
			"67890": float64(2),
		},
	}
	assert.Equal(t, "collection.updated", classifyEntry(entry))
}

// TestHandleEntry_CollectionUpdatedDispatchesTypedPayload verifies that
// handleEntry parses a collection.updated entry into a
// contract.CollectionUpdatedPayload and sends it to the BFF with the correct
// event type and JSON field names.
func TestHandleEntry_CollectionUpdatedDispatchesTypedPayload(t *testing.T) {
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
		AccountID:   "acc-coll",
	}
	svc := New(cfg)

	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"12345": float64(4),
			"67890": float64(2),
		},
	}

	require.NoError(t, svc.handleEntry(context.Background(), entry))
	assert.Equal(t, "collection.updated", received.Type)
	assert.Equal(t, "acc-coll", received.AccountID)

	var payload contract.CollectionUpdatedPayload
	require.NoError(t, json.Unmarshal(received.Payload, &payload))
	assert.False(t, payload.IsDelta)
	require.Len(t, payload.Cards, 2)
	// Verify both arena IDs appear in the result.
	ids := make(map[int]int, len(payload.Cards))
	for _, c := range payload.Cards {
		ids[c.ArenaID] = c.Count
	}
	assert.Equal(t, 4, ids[12345])
	assert.Equal(t, 2, ids[67890])
}

// TestClassifyEntry_DeckUpdated verifies that an entry containing a deck upsert
// request JSON string is classified as "deck.updated".
func TestClassifyEntry_DeckUpdated(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-123","Name":"Test Deck","Attributes":[{"name":"Format","value":"Standard"}]},"Deck":{"MainDeck":[{"cardId":11111,"quantity":4}],"Sideboard":[]}}`
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}
	assert.Equal(t, "deck.updated", classifyEntry(entry))
}

// TestHandleEntry_DeckUpdatedDispatchesTypedPayload verifies that handleEntry
// parses a deck.updated entry into a contract.DeckUpdatedPayload and sends it
// to the BFF with the correct event type and JSON field names.
func TestHandleEntry_DeckUpdatedDispatchesTypedPayload(t *testing.T) {
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
		AccountID:   "acc-deck",
	}
	svc := New(cfg)

	req := `{"Summary":{"DeckId":"deck-abc","Name":"Mono Red","Attributes":[{"name":"Format","value":"Standard"}]},"Deck":{"MainDeck":[{"cardId":55555,"quantity":4},{"cardId":66666,"quantity":2}],"Sideboard":[]}}`
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	require.NoError(t, svc.handleEntry(context.Background(), entry))
	assert.Equal(t, "deck.updated", received.Type)
	assert.Equal(t, "acc-deck", received.AccountID)

	var payload contract.DeckUpdatedPayload
	require.NoError(t, json.Unmarshal(received.Payload, &payload))
	assert.Equal(t, "deck-abc", payload.DeckID)
	assert.Equal(t, "Mono Red", payload.Name)
	assert.Equal(t, "Standard", payload.Format)
	require.Len(t, payload.Cards, 2)
	assert.Equal(t, 55555, payload.Cards[0].ArenaID)
	assert.Equal(t, 4, payload.Cards[0].Quantity)
	assert.Equal(t, 66666, payload.Cards[1].ArenaID)
	assert.Equal(t, 2, payload.Cards[1].Quantity)
}

// matchCompletedEntry builds a LogEntry that mirrors the real
// matchGameRoomStateChangedEvent structure observed in Player.log.
func matchCompletedEntry() *logreader.LogEntry {
	return &logreader.LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"matchGameRoomStateChangedEvent": map[string]interface{}{
				"gameRoomInfo": map[string]interface{}{
					"stateType": "MatchGameRoomStateType_MatchCompleted",
					"gameRoomConfig": map[string]interface{}{
						"eventId": "Ladder",
						"reservedPlayers": []interface{}{
							map[string]interface{}{
								"userId":     "USER_A",
								"playerName": "OpponentPlayer",
								"teamId":     float64(1),
							},
							map[string]interface{}{
								"userId":     "USER_B",
								"playerName": "LocalPlayer",
								"teamId":     float64(2),
							},
						},
					},
					"finalMatchResult": map[string]interface{}{
						"matchId":              "test-match-uuid",
						"matchCompletedReason": "MatchCompletedReasonType_Success",
						"resultList": []interface{}{
							map[string]interface{}{
								"scope":         "MatchScope_Game",
								"result":        "ResultType_WinLoss",
								"winningTeamId": float64(2),
								"reason":        "ResultReason_Game",
							},
							map[string]interface{}{
								"scope":         "MatchScope_Match",
								"result":        "ResultType_WinLoss",
								"winningTeamId": float64(2),
								"reason":        "ResultReason_Game",
							},
						},
					},
				},
			},
		},
	}
}

// TestClassifyEntry_MatchCompleted_GREEvent verifies that an entry containing
// matchGameRoomStateChangedEvent with MatchCompleted state type is classified
// as "match.completed".
func TestClassifyEntry_MatchCompleted_GREEvent(t *testing.T) {
	assert.Equal(t, "match.completed", classifyEntry(matchCompletedEntry()))
}

// TestClassifyEntry_MatchCompletedLegacy verifies the legacy CurrentEventState
// path still classifies as "match.completed".
func TestClassifyEntry_MatchCompletedLegacy(t *testing.T) {
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"CurrentEventState": "MatchCompleted"},
	}
	assert.Equal(t, "match.completed", classifyEntry(entry))
}

// TestHandleEntry_MatchCompletedDispatchesTypedPayload verifies that
// handleEntry parses a match.completed entry into a
// contract.MatchCompletedPayload and sends it to the BFF with the correct
// event type and JSON field names.
func TestHandleEntry_MatchCompletedDispatchesTypedPayload(t *testing.T) {
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
		AccountID:   "acc-match",
	}
	svc := New(cfg)

	require.NoError(t, svc.handleEntry(context.Background(), matchCompletedEntry()))
	assert.Equal(t, "match.completed", received.Type)
	assert.Equal(t, "acc-match", received.AccountID)

	var payload contract.MatchCompletedPayload
	require.NoError(t, json.Unmarshal(received.Payload, &payload))
	assert.Equal(t, "test-match-uuid", payload.MatchID)
	assert.Equal(t, "Ladder", payload.Format)
	assert.Equal(t, 2, payload.WinningTeamID)
	require.Len(t, payload.ResultList, 2)
	assert.Equal(t, "MatchScope_Match", payload.ResultList[1].Scope)
	assert.Equal(t, 2, payload.ResultList[1].WinningTeamID)
}

// TestWithVersion sets version and verifies it is stored correctly.
func TestWithVersion(t *testing.T) {
	cfg := &config.Config{
		CloudAPIURL: "http://localhost",
		IngestPath:  "/v1/ingest/events",
	}
	svc := New(cfg)
	assert.Equal(t, "dev", svc.version)

	svc.WithVersion("1.2.3")
	assert.Equal(t, "1.2.3", svc.version)

	// Empty string must be ignored.
	svc.WithVersion("")
	assert.Equal(t, "1.2.3", svc.version)
}

// ---------------------------------------------------------------------------
// Heartbeat ticker tests
// ---------------------------------------------------------------------------

// TestRunSendsHeartbeatWhenAccountIDSet verifies that the run loop dispatches a
// daemon.heartbeat event on each ticker fire when AccountID is non-empty.
func TestRunSendsHeartbeatWhenAccountIDSet(t *testing.T) {
	var heartbeatCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/ingest/events" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var evt contract.DaemonEvent
			require.NoError(t, json.Unmarshal(body, &evt))
			if evt.Type == "daemon.heartbeat" {
				heartbeatCalls.Add(1)
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-key",
		AccountID:   "acc-heartbeat",
		LogPath:     "/dev/null",
	}
	svc := New(cfg)

	oldInterval := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = oldInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_ = svc.Run(ctx)

	assert.GreaterOrEqual(t, heartbeatCalls.Load(), int32(1),
		"expected at least one daemon.heartbeat event when AccountID is set")
}

// TestRunSkipsHeartbeatWhenAccountIDEmpty verifies that no heartbeat is sent
// when the daemon has no AccountID (not yet authenticated).
func TestRunSkipsHeartbeatWhenAccountIDEmpty(t *testing.T) {
	var heartbeatCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/ingest/events" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var evt contract.DaemonEvent
			if json.Unmarshal(body, &evt) == nil && evt.Type == "daemon.heartbeat" {
				heartbeatCalls.Add(1)
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-key",
		AccountID:   "", // not authenticated
		LogPath:     "/dev/null",
	}
	svc := New(cfg)

	oldInterval := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = oldInterval })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_ = svc.Run(ctx)

	assert.Equal(t, int32(0), heartbeatCalls.Load(),
		"heartbeat must not be sent when AccountID is empty")
}
