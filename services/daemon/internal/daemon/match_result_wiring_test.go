package daemon

// match_result_wiring_test.go — regression guard for #336.
//
// Verifies the full authenticate→match-completed dispatch chain at the
// service/handleEntry layer. Before the fix, authenticateResponse["accountId"]
// was read (key absent in 2026.59.20) so s.mtgaUserID stayed "", causing every
// match to be dispatched with player_team_id=0 and result="" (→ "unknown" in
// the BFF). After the fix, authenticateResponse["clientId"] is read, which
// equals reservedPlayers[].userId, so player_team_id and result are populated.
//
// Fixtures: testdata/real/authenticate_2026.59.20.log (corrected — no userId
// key, clientId == reservedPlayers[].userId in match fixtures),
// testdata/real/match_completed_win_2026.59.20.log (Standard play WIN),
// testdata/real/match_completed_loss_2026.59.20.log (Standard ranked LOSS).

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realFixtureDir returns the absolute path to
// services/daemon/internal/logreader/testdata/real so the test is stable
// regardless of which directory 'go test' is invoked from.
func realFixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	// thisFile: .../services/daemon/internal/daemon/match_result_wiring_test.go
	// target:   .../services/daemon/internal/logreader/testdata/real
	return filepath.Join(filepath.Dir(thisFile), "..", "logreader", "testdata", "real")
}

// loadRealEntry reads a single-line JSON fixture from testdata/real and returns
// the parsed LogEntry. The fixture file must contain exactly one JSON line.
func loadRealEntry(t *testing.T, name string) *logreader.LogEntry {
	t.Helper()
	path := filepath.Join(realFixtureDir(t), name)
	r, err := logreader.NewReader(path)
	require.NoErrorf(t, err, "open fixture %s", name)
	t.Cleanup(func() { _ = r.Close() })
	entry, err := r.ReadEntry()
	require.NotEqualf(t, io.EOF, err, "fixture %s must have at least one entry", name)
	require.NoErrorf(t, err, "read fixture %s", name)
	require.NotNil(t, entry, "fixture %s must yield a non-nil entry", name)
	require.Truef(t, entry.IsJSON, "fixture %s must parse as JSON", name)
	return entry
}

// captureMatchCompleted builds a Service backed by a test HTTP server, feeds
// the given authenticate entry followed by the given match entry via handleEntry,
// and returns the dispatched contract.MatchCompletedPayload.
func captureMatchCompleted(
	t *testing.T,
	authEntry *logreader.LogEntry,
	matchEntry *logreader.LogEntry,
) contract.MatchCompletedPayload {
	t.Helper()

	var (
		mu       sync.Mutex
		captured *contract.MatchCompletedPayload
		gotEvent = make(chan struct{}, 1)
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var evt contract.DaemonEvent
		if json.Unmarshal(body, &evt) == nil && evt.Type == "match.completed" {
			var p contract.MatchCompletedPayload
			if json.Unmarshal(evt.Payload, &p) == nil {
				mu.Lock()
				if captured == nil {
					captured = &p
					select {
					case gotEvent <- struct{}{}:
					default:
					}
				}
				mu.Unlock()
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		APIKey:      "test-key-336",
		AccountID:   "acc-336-test",
	}
	svc := New(cfg)

	// Feed authenticate first — this should set s.mtgaUserID.
	require.NoError(t, svc.handleEntry(context.Background(), authEntry))

	// Feed match.completed — this should use s.mtgaUserID to derive result.
	require.NoError(t, svc.handleEntry(context.Background(), matchEntry))

	select {
	case <-gotEvent:
	default:
		// match.completed dispatch may be synchronous; check captured directly.
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, captured, "match.completed event must be dispatched")
	return *captured
}

// TestHandleEntry_AuthClientId_WinMatch_DispatchesCorrectResult is the
// regression test for #336. Before the fix this test fails because
// authenticateResponse has no "accountId" key (only "clientId"), so
// s.mtgaUserID stays "" and the match is dispatched with player_team_id=0
// and result="". After the fix, clientId is read and the WIN match is
// dispatched with player_team_id=1 and result="win".
func TestHandleEntry_AuthClientId_WinMatch_DispatchesCorrectResult(t *testing.T) {
	authEntry := loadRealEntry(t, "authenticate_2026.59.20.log")
	matchEntry := loadRealEntry(t, "match_completed_win_2026.59.20.log")

	payload := captureMatchCompleted(t, authEntry, matchEntry)

	assert.Equal(t, 1, payload.PlayerTeamID,
		"player_team_id must be 1 for WIN match (local player is teamId=1); got 0 means mtgaUserID was not set (accountId key absent — fix reads clientId)")
	assert.Equal(t, "win", payload.Result,
		"result must be 'win' when local player's team wins; got empty/unknown means player_team_id was 0")
}

// TestHandleEntry_AuthClientId_LossMatch_DispatchesCorrectResult is the
// regression test for #336 (LOSS side). Local player is teamId=2,
// winningTeamId=1, so result must be "loss".
func TestHandleEntry_AuthClientId_LossMatch_DispatchesCorrectResult(t *testing.T) {
	authEntry := loadRealEntry(t, "authenticate_2026.59.20.log")
	matchEntry := loadRealEntry(t, "match_completed_loss_2026.59.20.log")

	payload := captureMatchCompleted(t, authEntry, matchEntry)

	assert.Equal(t, 2, payload.PlayerTeamID,
		"player_team_id must be 2 for LOSS match (local player is teamId=2); got 0 means mtgaUserID was not set")
	assert.Equal(t, "loss", payload.Result,
		"result must be 'loss' when opponent wins; got empty/unknown means player_team_id was 0")
}
