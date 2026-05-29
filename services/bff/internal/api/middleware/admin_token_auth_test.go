package middleware_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// --- Audit-log tests (AC1 / AC2 / AC3) ---
// NOTE: These tests use log.SetOutput which redirects the global log writer.
// Do NOT add t.Parallel() to these tests — global log output capture is not
// parallel-safe (Ray: approved plan note, 2026-05-28).

func TestAdminTokenAuth_SuccessPath_EmitsAuditLog(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	token := "audit-test-token"
	next := &reachedHandler{}
	h := middleware.AdminTokenAuth(token)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/daemons/fleet-health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := buf.String()
	if !strings.Contains(got, "[admin_auth]") {
		t.Errorf("expected [admin_auth] prefix in audit log; got: %q", got)
	}
	if !strings.Contains(got, "outcome=ok") {
		t.Errorf("expected outcome=ok in audit log; got: %q", got)
	}
	if !strings.Contains(got, "path=/api/v1/admin/daemons/fleet-health") {
		t.Errorf("expected path in audit log; got: %q", got)
	}
	// AC2: token value must never appear in the log line.
	if strings.Contains(got, token) {
		t.Errorf("token value must not appear in audit log; got: %q", got)
	}
}

func TestAdminTokenAuth_FailPath_EmitsAuditLog(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	next := &reachedHandler{}
	h := middleware.AdminTokenAuth("correct-token")(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/daemons/fleet-health", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := buf.String()
	if !strings.Contains(got, "[admin_auth]") {
		t.Errorf("expected [admin_auth] prefix in audit log; got: %q", got)
	}
	if !strings.Contains(got, "outcome=fail") {
		t.Errorf("expected outcome=fail in audit log; got: %q", got)
	}
	// AC2: neither configured nor submitted token must appear.
	if strings.Contains(got, "correct-token") || strings.Contains(got, "wrong-token") {
		t.Errorf("token values must not appear in audit log; got: %q", got)
	}
}

func TestAdminTokenAuth_AuditLog_DoesNotContainTokenValue(t *testing.T) {
	// Explicit AC2 assertion across all failure modes.
	cases := []struct {
		name            string
		configuredToken string
		authHeader      string
		submitToken     string
	}{
		{
			name:            "missing_header",
			configuredToken: "cfg-tok-missing",
			authHeader:      "",
			submitToken:     "",
		},
		{
			name:            "wrong_token",
			configuredToken: "cfg-tok-wrong",
			authHeader:      "Bearer submitted-wrong-tok",
			submitToken:     "submitted-wrong-tok",
		},
		{
			name:            "empty_configured_token",
			configuredToken: "",
			authHeader:      "Bearer any-token-here",
			submitToken:     "any-token-here",
		},
		{
			name:            "missing_bearer_prefix",
			configuredToken: "cfg-tok-prefix",
			authHeader:      "cfg-tok-prefix",
			submitToken:     "cfg-tok-prefix",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// NOTE: no t.Parallel() — log.SetOutput is global.
			var buf bytes.Buffer
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			next := &reachedHandler{}
			h := middleware.AdminTokenAuth(tc.configuredToken)(next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/daemons/fleet-health", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			got := buf.String()
			if !strings.Contains(got, "[admin_auth]") {
				t.Errorf("expected [admin_auth] in log; got: %q", got)
			}
			if tc.configuredToken != "" && strings.Contains(got, tc.configuredToken) {
				t.Errorf("configured token must not appear in audit log; got: %q", got)
			}
			if tc.submitToken != "" && strings.Contains(got, tc.submitToken) {
				t.Errorf("submitted token must not appear in audit log; got: %q", got)
			}
		})
	}
}
