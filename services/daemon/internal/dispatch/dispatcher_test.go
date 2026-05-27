package dispatch_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDispatcherSendsValidDaemonEvent verifies that the dispatcher POSTs a correctly
// structured contract.DaemonEvent to the BFF /v1/ingest/events endpoint.
func TestDispatcherSendsValidDaemonEvent(t *testing.T) {
	var received contract.DaemonEvent
	var authHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/ingest/events", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		authHeader = r.Header.Get("Authorization")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "test-api-key")

	payload := map[string]interface{}{"draftPack": []string{"card1", "card2"}}
	evt, err := dispatch.BuildEvent("draft.pack", "account-123", "session-abc", payload)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, d.Send(ctx, evt))

	assert.Equal(t, "Bearer test-api-key", authHeader)
	assert.Equal(t, "draft.pack", received.Type)
	assert.Equal(t, "account-123", received.AccountID)
	assert.Equal(t, "session-abc", received.SessionID)
	assert.False(t, received.OccurredAt.IsZero())
	assert.NotEmpty(t, received.Payload)
	// First Send from a new Dispatcher must assign sequence=1 (ADR-013).
	assert.Equal(t, uint64(1), received.Sequence, "first event must have sequence=1")
}

// TestDispatcherSequenceMonotonicallyIncreases verifies that consecutive Send
// calls on the same Dispatcher assign strictly increasing sequence numbers
// starting at 1 (ADR-013).
func TestDispatcherSequenceMonotonicallyIncreases(t *testing.T) {
	var mu sync.Mutex
	var sequences []uint64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var evt contract.DaemonEvent
		require.NoError(t, json.Unmarshal(body, &evt))
		mu.Lock()
		sequences = append(sequences, evt.Sequence)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "tok")

	const n = 5
	for i := range n {
		evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]int{"i": i})
		require.NoError(t, err)
		require.NoError(t, d.Send(context.Background(), evt))
	}

	require.Len(t, sequences, n)
	for i, seq := range sequences {
		want := uint64(i + 1)
		assert.Equal(t, want, seq, "event %d: sequence mismatch", i)
	}
}

// TestDispatcherHandlesBFFError verifies that non-2xx responses are returned as errors.
// With retry logic the dispatcher will attempt 3 times before returning an error.
func TestDispatcherHandlesBFFError(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "")
	evt, err := dispatch.BuildEvent("test.event", "", "sess", map[string]string{})
	require.NoError(t, err)

	err = d.Send(context.Background(), evt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.EqualValues(t, 3, requestCount.Load(), "expected 3 attempts before giving up")
}

// TestDispatcherRetriesOnFailure verifies the dispatcher retries exactly 3 times on
// server errors before returning an error, and that the server received all 3 requests.
func TestDispatcherRetriesOnFailure(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "test-api-key")
	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{"k": "v"})
	require.NoError(t, err)

	err = d.Send(context.Background(), evt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 attempts failed")
	assert.EqualValues(t, 3, requestCount.Load(), "server should have received exactly 3 requests")
}

// TestBuildEvent verifies that BuildEvent correctly populates all fields.
func TestBuildEvent(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	evt, err := dispatch.BuildEvent("match.completed", "acc-1", "sess-1", payload)
	require.NoError(t, err)

	assert.Equal(t, "match.completed", evt.Type)
	assert.Equal(t, "acc-1", evt.AccountID)
	assert.Equal(t, "sess-1", evt.SessionID)
	assert.WithinDuration(t, time.Now().UTC(), evt.OccurredAt, 5*time.Second)

	// Payload should contain the marshalled data
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(evt.Payload, &decoded))
	assert.Equal(t, "value", decoded["key"])
}

// ---- 401 re-registration ----

// mockRefresher is a test double for dispatch.Refresher.
type mockRefresher struct {
	token string
	err   error
	calls int
}

func (m *mockRefresher) Refresh(_ context.Context) (string, error) {
	m.calls++
	return m.token, m.err
}

// TestDispatcher401TriggersRefresh verifies that a single 401 causes the
// dispatcher to call Refresh and swap in the new token, then succeed on retry.
func TestDispatcher401TriggersRefresh(t *testing.T) {
	var requestCount atomic.Int32
	var authHeaders []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		if n == 1 {
			// First request: return 401 to trigger refresh.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Second request: succeed.
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	ref := &mockRefresher{token: "refreshed-jwt"}
	d := dispatch.New(srv.URL, "/v1/ingest/events", "old-token").WithRefresher(ref)

	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)

	require.NoError(t, d.Send(context.Background(), evt))
	assert.Equal(t, 1, ref.calls, "Refresh should have been called exactly once")
	assert.EqualValues(t, 2, requestCount.Load())
	// Second request should carry the refreshed token.
	if len(authHeaders) >= 2 {
		assert.Equal(t, "Bearer refreshed-jwt", authHeaders[1])
	}
}

// TestDispatcher401WithoutRefresherRetriesWithoutTokenChange verifies that when no
// Refresher is set, a 401 is retried normally (without any token swap).
func TestDispatcher401WithoutRefresherRetriesWithoutTokenChange(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "key")
	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)

	err = d.Send(context.Background(), evt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.EqualValues(t, 3, requestCount.Load(), "should retry all 3 times")
}

// TestDispatcher401RefreshFailureContinuesRetry verifies that if Refresh returns
// an error, the dispatcher still retries with the old token.
func TestDispatcher401RefreshFailureContinuesRetry(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ref := &mockRefresher{err: errors.New("registration unavailable")}
	d := dispatch.New(srv.URL, "/v1/ingest/events", "key").WithRefresher(ref)

	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)

	err = d.Send(context.Background(), evt)
	require.Error(t, err)
	// All 3 attempts exhausted despite refresh failure.
	assert.EqualValues(t, 3, requestCount.Load())
}

// TestDispatcher_ErrReauthRequiredBreaksRetryLoop verifies that when a Refresher
// returns ErrReauthRequired the dispatcher breaks the retry loop immediately
// after the first BFF hit and surfaces ErrReauthRequired to the caller.
// The BFF must receive exactly 1 request — no retries.
func TestDispatcher_ErrReauthRequiredBreaksRetryLoop(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ref := &mockRefresher{err: dispatch.ErrReauthRequired}
	d := dispatch.New(srv.URL, "/v1/ingest/events", "old-token").WithRefresher(ref)

	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)

	sendErr := d.Send(context.Background(), evt)
	require.Error(t, sendErr)
	assert.True(t, errors.Is(sendErr, dispatch.ErrReauthRequired),
		"error must wrap ErrReauthRequired")
	// Sentinel breaks after 1 attempt — no retries.
	assert.EqualValues(t, 1, requestCount.Load(),
		"BFF must be hit exactly once when refresher returns ErrReauthRequired")
	// Refresher called exactly once.
	assert.Equal(t, 1, ref.calls, "Refresh must be called exactly once")
}

// TestSetToken verifies that SetToken updates the bearer token used on next send.
func TestSetToken(t *testing.T) {
	var lastAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d := dispatch.New(srv.URL, "/v1/ingest/events", "original")
	d.SetToken("updated-token")

	evt, err := dispatch.BuildEvent("test.event", "acc", "sess", map[string]string{})
	require.NoError(t, err)
	require.NoError(t, d.Send(context.Background(), evt))
	assert.Equal(t, "Bearer updated-token", lastAuth)
}
