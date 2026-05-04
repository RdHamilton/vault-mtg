package dispatch_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-daemon/internal/dispatch"
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
