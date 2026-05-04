package registrar_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/registrar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper that unmarshals the request body sent by the client.
func readRegisterRequest(t *testing.T, r *http.Request) registrar.RegisterRequest {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var req registrar.RegisterRequest
	require.NoError(t, json.Unmarshal(body, &req))
	return req
}

// TestRegisterSuccess verifies the happy path: BFF returns token + daemon_id.
func TestRegisterSuccess(t *testing.T) {
	var capturedAuth string
	var capturedReq registrar.RegisterRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/daemon/register", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		capturedAuth = r.Header.Get("Authorization")
		capturedReq = readRegisterRequest(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":     "eyJhbGciOiJIUzI1NiJ9.test.sig",
			"daemon_id": "550e8400-e29b-41d4-a716-446655440000",
		})
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	resp, err := client.Register(context.Background(), "user-api-key-123", 42)
	require.NoError(t, err)

	assert.Equal(t, "Bearer user-api-key-123", capturedAuth)
	assert.Equal(t, 42, capturedReq.UserID)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.test.sig", resp.Token)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", resp.DaemonID)
}

// TestRegisterSuccessNoAPIKey verifies the client omits Authorization when apiKey is empty.
func TestRegisterSuccessNoAPIKey(t *testing.T) {
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":     "tok",
			"daemon_id": "did",
		})
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(context.Background(), "", 1)
	require.NoError(t, err)
	assert.Empty(t, capturedAuth)
}

// TestRegister400 verifies that a 400 response surfaces as ErrHTTP with correct status.
func TestRegister400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid user_id"}`))
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(context.Background(), "key", 0)
	require.Error(t, err)

	var httpErr *registrar.ErrHTTP
	require.True(t, errors.As(err, &httpErr), "expected *ErrHTTP, got %T: %v", err, err)
	assert.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
	assert.Contains(t, httpErr.Body, "invalid user_id")
	assert.Contains(t, err.Error(), "400")
}

// TestRegister500 verifies that a 500 response surfaces as ErrHTTP with correct status.
func TestRegister500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(context.Background(), "key", 7)
	require.Error(t, err)

	var httpErr *registrar.ErrHTTP
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
	assert.Contains(t, err.Error(), "500")
}

// TestRegisterNetworkTimeout verifies that a context deadline exceeded is
// returned as a wrapped network error (not an ErrHTTP).
func TestRegisterNetworkTimeout(t *testing.T) {
	// Server that hangs indefinitely.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the client disconnects.
		select {
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(ctx, "key", 1)
	require.Error(t, err)

	// Must NOT be an ErrHTTP — the transport never received a response.
	var httpErr *registrar.ErrHTTP
	assert.False(t, errors.As(err, &httpErr), "timeout should not surface as ErrHTTP")
	assert.Contains(t, err.Error(), "network error")
}

// TestRegisterConnectionRefused verifies behaviour when the BFF is unreachable.
func TestRegisterConnectionRefused(t *testing.T) {
	// Bind a port, immediately close it so the address is definitely not listening.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	l.Close()

	client := registrar.NewClient("http://" + addr)
	_, connErr := client.Register(context.Background(), "key", 3)
	require.Error(t, connErr)

	var httpErr *registrar.ErrHTTP
	assert.False(t, errors.As(connErr, &httpErr))
	assert.Contains(t, connErr.Error(), "network error")
}

// TestRegisterEmptyTokenReturnsError verifies that a 200 response with an empty
// token field is treated as an error.
func TestRegisterEmptyTokenReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"","daemon_id":"did-123"}`))
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(context.Background(), "key", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
}

// TestRegisterEmptyDaemonIDReturnsError verifies that a 200 response with an
// empty daemon_id field is treated as an error.
func TestRegisterEmptyDaemonIDReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"tok-abc","daemon_id":""}`))
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(context.Background(), "key", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty daemon_id")
}

// TestRegisterMalformedJSONReturnsError verifies that an unparseable 200 body
// is returned as an error.
func TestRegisterMalformedJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	client := registrar.NewClient(srv.URL)
	_, err := client.Register(context.Background(), "key", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

// TestNewClientWithHTTP verifies that the injected http.Client is used,
// allowing callers to control timeout/transport.
func TestNewClientWithHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":     "tok-custom",
			"daemon_id": "did-custom",
		})
	}))
	defer srv.Close()

	custom := &http.Client{Timeout: 5 * time.Second}
	client := registrar.NewClientWithHTTP(srv.URL, custom)
	resp, err := client.Register(context.Background(), "key", 99)
	require.NoError(t, err)
	assert.Equal(t, "tok-custom", resp.Token)
	assert.Equal(t, "did-custom", resp.DaemonID)
}
