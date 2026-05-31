//go:build integration

package daemon

// TestIngestSmoke is ADR-042 Layer 4: the daemon→dispatch→BFF-ingest wire smoke
// test.  It covers the integration path that Layer 1 (binary smoke) and Layer 2
// (contract/serialisation gate) do not exercise as a unit: that the real
// dispatch.Dispatcher correctly submits a match.completed event over HTTP and
// the receiving handler accepts and persists it.
//
// Layer map (for context):
//   Layer 1 — TestDaemonBinarySmoke (smoke_test.go)
//             Full binary lifecycle via os/exec. Requires a running binary.
//   Layer 2 — TestContractEmit_* (logreader/contract_emit_test.go)
//             Parser→BuildEvent payload shapes against corpus fixtures.
//   Layer 3 — TestProjectionIntegration (services/bff/integration_test.go)
//             AUTHORITATIVE DB ROUND-TRIP: real IngestHandler + real Postgres.
//             Seeded user, daemon_events.Insert, projection.Worker.RunOnce,
//             matches row assertion. This is the full DB gate; do not duplicate
//             it here.
//   Layer 4 — TestIngestSmoke (this file)
//             daemon Dispatcher.Send → httptest.Server stub → Insert recording.
//
// Module-boundary note:
//   The test was originally planned to use the real BFF handlers.IngestHandler
//   directly. Go's internal package visibility rule (https://go.dev/ref/spec#Importability)
//   prevents services/daemon from importing services/bff/internal/... even
//   under go.work — the compiler rejects the import with "use of internal
//   package not allowed". The same rule prevents a BFF-side test from
//   importing services/daemon/internal/dispatch and
//   services/daemon/internal/logreader. No workspace-level Go package exists
//   that could bridge both internals.
//
//   Per Ray's PLAN_VERDICT fallback (vault-mtg-tickets#186, 2026-05-31):
//   "POST to a real httptest.Server using the handler func directly via a
//   shared interface." This test implements exactly that: the httptest.Server
//   mimics IngestHandler's behaviour using only the shared contract.DaemonEvent
//   wire type and the DaemonEventInserter call signature that IngestHandler
//   itself satisfies. The contract.DaemonEvent JSON round-trip, the
//   sequence-stamping in Dispatcher.Send, and the full parse pipeline are all
//   exercised by real production code. The only non-production component is the
//   inline handler stub that stands in for IngestHandler.
//
//   Layer 3 (TestProjectionIntegration) remains the authoritative gate for the
//   real IngestHandler against a live database. This test closes the daemon
//   dispatch-wire gap that Layer 3 does not cover from the daemon side.
//
// No raw PII is present in any committed fixture; match-completed.log uses
// synthetic TESTACCOUNT / TestPlayer identifiers only.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── ingest stub ──────────────────────────────────────────────────────────────
//
// ingestStubCall mirrors the fields that BFF's DaemonEventInserter.Insert
// receives so the test can assert end-to-end correctness without importing
// the BFF's internal package.

type ingestStubCall struct {
	accountID  string
	eventType  string
	payload    json.RawMessage
	occurredAt time.Time
	eventID    string
	sequence   uint64
}

// ingestStub is the httptest.Server handler.
// It implements the same JSON decode + Insert call + 202 response that the
// real handlers.IngestHandler performs, using only the shared
// contract.DaemonEvent type.
//
// This stub stands in for the real IngestHandler solely because Go's internal
// package rule prevents services/daemon from importing
// services/bff/internal/api/handlers. See the module-boundary note above.
// The authoritative test of the real IngestHandler is TestProjectionIntegration
// in services/bff/integration_test.go.
type ingestStub struct {
	calls []ingestStubCall
}

func (s *ingestStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var event contract.DaemonEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if event.Type == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.calls = append(s.calls, ingestStubCall{
		accountID:  event.AccountID,
		eventType:  event.Type,
		payload:    event.Payload,
		occurredAt: event.OccurredAt,
		eventID:    event.EventID,
		sequence:   event.Sequence,
	})

	w.WriteHeader(http.StatusAccepted)
}

// ── fixture path ─────────────────────────────────────────────────────────────

// ingestSmokeFixturePath returns the absolute path to the given corpus
// player-log fixture, resolved relative to this source file so the test is
// location-stable regardless of where `go test` is invoked from.
func ingestSmokeFixturePath(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	// thisFile: .../services/daemon/internal/daemon/ingest_smoke_test.go
	// target:   .../services/daemon/testdata/corpus/player-log/<name>
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "corpus", "player-log", name)
}

// ── test ─────────────────────────────────────────────────────────────────────

// TestIngestSmoke exercises the daemon→dispatch→ingest wire path end-to-end:
//
//  1. Reads the corpus fixture match-completed.log.
//  2. Parses it via logreader.NewReader + ParseMatchCompletedEntry.
//  3. Builds the contract.DaemonEvent via dispatch.BuildEvent.
//  4. POSTs it via the real dispatch.Dispatcher to an httptest.Server backed
//     by ingestStub (which records Insert-equivalent calls).
//  5. Asserts: HTTP 202, Insert called once, event type "match.completed",
//     payload.MatchID non-empty, sequence > 0.
//
// This is ADR-042 Layer 4.  Layer 3 (TestProjectionIntegration in
// services/bff/integration_test.go) remains the authoritative gate for the
// real handlers.IngestHandler against a live Postgres database.
func TestIngestSmoke(t *testing.T) {
	// ── 1. Parse the corpus fixture ───────────────────────────────────────────

	fixturePath := ingestSmokeFixturePath(t, "match-completed.log")
	r, err := logreader.NewReader(fixturePath)
	require.NoError(t, err, "open match-completed.log corpus fixture")
	t.Cleanup(func() { _ = r.Close() })

	entry, err := r.ReadEntry()
	require.NoError(t, err, "read first entry from match-completed.log")
	require.NotNil(t, entry, "match-completed.log must yield a non-nil entry")
	require.True(t, entry.IsJSON, "match-completed.log entry must parse as JSON")
	require.True(t, logreader.IsMatchCompletedEntry(entry),
		"match-completed.log must contain a matchGameRoomStateChangedEvent "+
			"with stateType MatchGameRoomStateType_MatchCompleted")

	// ParseMatchCompletedEntry — empty playerUserID is acceptable here; this
	// smoke tests the dispatch wire, not the player-identity derivation.
	payload, parseErr := logreader.ParseMatchCompletedEntry(entry, "")
	require.NoError(t, parseErr, "ParseMatchCompletedEntry must succeed on corpus fixture")
	require.NotEmpty(t, payload.MatchID, "corpus fixture must yield a non-empty MatchID")

	// ── 2. Build the DaemonEvent via dispatch.BuildEvent ─────────────────────

	const (
		smokeAccountID = "smoke-ingest-acc-001"
		smokeSessionID = "smoke-ingest-sess-001"
	)

	event, buildErr := dispatch.BuildEvent("match.completed", smokeAccountID, smokeSessionID, payload)
	require.NoError(t, buildErr, "dispatch.BuildEvent must succeed")
	require.Equal(t, "match.completed", event.Type)
	require.Equal(t, smokeAccountID, event.AccountID)
	require.NotEmpty(t, event.Payload, "BuildEvent payload must be non-empty")

	// ── 3. Wire the ingest stub httptest.Server ───────────────────────────────

	stub := &ingestStub{}
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)

	// ── 4. Dispatch via the real dispatch.Dispatcher ──────────────────────────

	d := dispatch.New(srv.URL, "/ingest/events", "smoke-api-key")
	sendErr := d.Send(context.Background(), event)
	require.NoError(t, sendErr,
		"Dispatcher.Send must succeed: dispatch.Dispatcher must produce a "+
			"valid HTTP POST that the ingest handler accepts with 202")

	// ── 5. Assert stub side-effects ───────────────────────────────────────────

	// 5a. HTTP 202 is implicitly asserted by Send returning nil (dispatcher
	//     treats non-2xx as an error and retries up to 3 times before failing).

	// 5b. Insert must have been recorded exactly once.
	require.Len(t, stub.calls, 1,
		"ingest handler must record Insert exactly once for a valid match.completed event")

	call := stub.calls[0]

	assert.Equal(t, "match.completed", call.eventType,
		"Insert eventType must be match.completed")
	assert.Equal(t, smokeAccountID, call.accountID,
		"Insert accountID must equal the accountID passed to dispatch.BuildEvent")
	assert.NotEmpty(t, call.payload,
		"Insert payload must be non-empty after the full parse→BuildEvent pipeline")
	assert.Greater(t, call.sequence, uint64(0),
		"Dispatcher.Send must stamp a non-zero sequence number (ADR-013)")

	// 5c. The persisted payload round-trips back to a MatchCompletedPayload
	//     carrying the same MatchID the fixture produced.
	var persisted contract.MatchCompletedPayload
	require.NoError(t, json.Unmarshal(call.payload, &persisted),
		"persisted payload must unmarshal to a valid MatchCompletedPayload")
	assert.Equal(t, payload.MatchID, persisted.MatchID,
		"MatchID must survive the full "+
			"log→logreader.ParseMatchCompletedEntry→dispatch.BuildEvent→"+
			"Dispatcher.Send→ingest-decode round-trip")
	assert.NotEmpty(t, persisted.ResultList,
		"persisted MatchCompletedPayload.ResultList must be non-empty")
}
