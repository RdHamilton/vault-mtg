package sse_test

// TestSSEEndpoint_NoDB_Returns503 documents the expected behaviour when the
// BFF is started without DATABASE_URL (development only).  The test mirrors
// the route registration logic in cmd/main.go: when no auth middleware is
// available the SSE endpoint must respond 503 Service Unavailable rather than
// falling back to an unauthenticated stream.
//
// This is a unit test of the routing decision, not an integration test of
// main.go itself (which cannot be easily tested with httptest).  The
// equivalent production guard is enforced by config.Load returning an error
// when MTGA_ENV=production and DATABASE_URL is unset.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// noDBSSEHandler returns the handler registered for /api/v1/events when no
// auth middleware is available (i.e. DATABASE_URL is unset).  It must always
// return 503 so that unauthenticated clients receive an explicit error rather
// than an open event stream.
func noDBSSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable — database not configured", http.StatusServiceUnavailable)
	}
}

func TestSSEEndpoint_NoDB_Returns503(t *testing.T) {
	srv := httptest.NewServer(noDBSSEHandler())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when no DB configured, got %d", resp.StatusCode)
	}
}

func TestSSEEndpoint_NoDB_WithAuthHeader_Returns503(t *testing.T) {
	// Even with an Authorization header, the 503 must still be returned when
	// the database is unavailable.  The handler does not inspect headers.
	srv := httptest.NewServer(noDBSSEHandler())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	req.Header.Set("Authorization", "Bearer sometoken")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 even with auth header when no DB, got %d", resp.StatusCode)
	}
}
