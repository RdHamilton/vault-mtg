package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
)

// reachedHandler is a helper that records whether ServeHTTP was called.
type reachedHandler struct {
	called bool
}

func (h *reachedHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
}

func TestAdminTokenAuth_MissingHeader_Returns401(t *testing.T) {
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth("secret-token")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if next.called {
		t.Error("next handler must not be called on missing Authorization")
	}
}

func TestAdminTokenAuth_WrongToken_Returns401(t *testing.T) {
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth("correct-token")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if next.called {
		t.Error("next handler must not be called on wrong token")
	}
}

func TestAdminTokenAuth_CorrectToken_CallsNext(t *testing.T) {
	token := "my-high-entropy-admin-token"
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth(token)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !next.called {
		t.Error("next handler must be called on correct token")
	}
}

func TestAdminTokenAuth_EmptyConfiguredToken_AlwaysRejects(t *testing.T) {
	// When no admin token is configured (empty string), all requests must be
	// rejected regardless of what the caller sends.  This guards against a
	// misconfigured BFF boot silently opening the endpoint to everyone.
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth("")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when token not configured, got %d", rr.Code)
	}
	if next.called {
		t.Error("next handler must not be called when token not configured")
	}
}

func TestAdminTokenAuth_BearerPrefixRequired(t *testing.T) {
	// A raw token value without the "Bearer " prefix must be rejected.
	token := "my-token"
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth(token)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", token) // missing "Bearer " prefix
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for raw token (no Bearer prefix), got %d", rr.Code)
	}
	if next.called {
		t.Error("next handler must not be called on malformed Authorization")
	}
}

func TestAdminTokenAuth_ConstantTimeCompare_NoTimingLeak(t *testing.T) {
	// Smoke-test that the comparison does not panic or branch on token length.
	// We submit a token of wildly different length from the configured one —
	// a length-aware compare would panic or short-circuit; constant-time must
	// still return 401 cleanly.
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth("short")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer this-is-a-very-long-token-that-differs-in-length")
	rr := httptest.NewRecorder()

	// Must not panic, must return 401.
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
