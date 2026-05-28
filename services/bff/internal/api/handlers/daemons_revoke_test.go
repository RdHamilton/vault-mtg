package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
)

// stubDaemonRevokeRepo is a test double for the daemonAPIKeyRevokeRepo
// interface. It records the (accountID, deviceID) it was called with so
// tests can verify cross-tenancy invariants (no row matches for User B's
// attempt against User A's device).
type stubDaemonRevokeRepo struct {
	// owners maps (account_id, device_id) → exists-and-active. Anything
	// not in the map is treated as not found.
	owners map[string]map[string]bool
	err    error

	// recorded captures the last revoke call so tests can assert what was
	// passed to the repo.
	lastAccountID string
	lastDeviceID  string
}

func (s *stubDaemonRevokeRepo) RevokeByAccountIDAndDeviceID(_ context.Context, accountID, deviceID string) (bool, error) {
	s.lastAccountID = accountID
	s.lastDeviceID = deviceID
	if s.err != nil {
		return false, s.err
	}
	devices, ok := s.owners[accountID]
	if !ok {
		return false, nil
	}
	active, ok := devices[deviceID]
	if !ok || !active {
		return false, nil
	}
	// Mark inactive on successful revoke.
	devices[deviceID] = false
	return true, nil
}

// newRevokeRequest builds a DELETE /api/v1/daemons/{device_id} request with
// chi URL parameters wired up so chi.URLParam(r, "device_id") returns the
// expected value.
func newRevokeRequest(accountID, deviceID string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/daemons/"+deviceID, nil)
	if accountID != "" {
		req = middleware.WithClerkUserID(req, accountID)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("device_id", deviceID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

// TestDaemonsRevoke_HappyPath_Returns204 verifies AC2: a user revoking their
// own device receives 204 No Content with no response body. Per ADR-031 §3.
func TestDaemonsRevoke_HappyPath_Returns204(t *testing.T) {
	deviceID := "11111111-1111-1111-1111-111111111111"
	repo := &stubDaemonRevokeRepo{
		owners: map[string]map[string]bool{
			"user_A": {deviceID: true},
		},
	}
	h := handlers.NewDaemonsRevokeHandler(repo)

	req := newRevokeRequest("user_A", deviceID)
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Errorf("204 response MUST have an empty body; got: %s", rr.Body.String())
	}
	if repo.lastAccountID != "user_A" || repo.lastDeviceID != deviceID {
		t.Errorf("handler passed wrong (account, device) to repo: got (%q, %q)", repo.lastAccountID, repo.lastDeviceID)
	}
}

// TestDaemonsRevoke_CrossTenancy_Returns404 is the LOAD-BEARING assertion for
// ADR-031 §3 + AC4: User B attempting to revoke User A's device receives 404
// (NOT 403 — 404 prevents cross-tenant device_id enumeration). The handler
// MUST scope by the caller's Clerk user_id, not a client-supplied account_id.
func TestDaemonsRevoke_CrossTenancy_Returns404(t *testing.T) {
	deviceID := "22222222-2222-2222-2222-222222222222"
	repo := &stubDaemonRevokeRepo{
		owners: map[string]map[string]bool{
			"user_A": {deviceID: true},
		},
	}
	h := handlers.NewDaemonsRevokeHandler(repo)

	// User B attacks User A's device.
	req := newRevokeRequest("user_B", deviceID)
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (NOT 403) for cross-tenant revoke; got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Code == http.StatusForbidden {
		t.Errorf("403 leaks cross-tenant existence — ADR-031 §3 Fitness Function §2 violation")
	}
	// Verify the handler scoped by user_B (the authenticated caller), not user_A.
	if repo.lastAccountID != "user_B" {
		t.Errorf("handler MUST scope by authenticated caller (user_B), got %q", repo.lastAccountID)
	}
	// Repo state: User A's device must still be active.
	if !repo.owners["user_A"][deviceID] {
		t.Errorf("User A's device MUST remain active after User B's cross-tenant attempt")
	}
}

// TestDaemonsRevoke_AlreadyRevoked_Returns404 verifies AC6 + ADR-031 §3
// 404-collapse: a second DELETE for an already-revoked device returns 404
// (because the repo's WHERE revoked_at IS NULL filter matches zero rows).
func TestDaemonsRevoke_AlreadyRevoked_Returns404(t *testing.T) {
	deviceID := "33333333-3333-3333-3333-333333333333"
	repo := &stubDaemonRevokeRepo{
		owners: map[string]map[string]bool{
			"user_A": {deviceID: false}, // already revoked → not active
		},
	}
	h := handlers.NewDaemonsRevokeHandler(repo)

	req := newRevokeRequest("user_A", deviceID)
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for already-revoked device; got %d", rr.Code)
	}
}

// TestDaemonsRevoke_NonExistent_Returns404 verifies a DELETE against a
// device_id that doesn't exist at all returns 404 — indistinguishable from
// "not yours" or "already revoked" per the §3 collapse.
func TestDaemonsRevoke_NonExistent_Returns404(t *testing.T) {
	repo := &stubDaemonRevokeRepo{owners: map[string]map[string]bool{}}
	h := handlers.NewDaemonsRevokeHandler(repo)

	req := newRevokeRequest("user_A", "99999999-9999-9999-9999-999999999999")
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent device_id; got %d", rr.Code)
	}
}

// TestDaemonsRevoke_MalformedDeviceID_Returns400 verifies a non-UUID path
// segment is rejected with 400 before the repo is touched. Defence in depth.
func TestDaemonsRevoke_MalformedDeviceID_Returns400(t *testing.T) {
	repo := &stubDaemonRevokeRepo{owners: map[string]map[string]bool{"user_A": {}}}
	h := handlers.NewDaemonsRevokeHandler(repo)

	req := newRevokeRequest("user_A", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed device_id; got %d: %s", rr.Code, rr.Body.String())
	}
	// Repo must not have been called.
	if repo.lastDeviceID != "" {
		t.Errorf("repo MUST NOT be called for a malformed device_id; got call with deviceID=%q", repo.lastDeviceID)
	}
}

// TestDaemonsRevoke_MissingClerkAuth_Returns401 verifies the handler returns
// 401 when no Clerk session is attached to the request context.
func TestDaemonsRevoke_MissingClerkAuth_Returns401(t *testing.T) {
	repo := &stubDaemonRevokeRepo{}
	h := handlers.NewDaemonsRevokeHandler(repo)

	deviceID := "44444444-4444-4444-4444-444444444444"
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/daemons/"+deviceID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("device_id", deviceID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestDaemonsRevoke_RepoError_Returns500 verifies a repo-layer error surfaces
// as 500 (and does NOT echo the underlying error to the response body).
func TestDaemonsRevoke_RepoError_Returns500(t *testing.T) {
	repo := &stubDaemonRevokeRepo{err: errors.New("db gone")}
	h := handlers.NewDaemonsRevokeHandler(repo)

	deviceID := "55555555-5555-5555-5555-555555555555"
	req := newRevokeRequest("user_A", deviceID)
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "db gone") {
		t.Errorf("response body MUST NOT echo the underlying error; body: %s", rr.Body.String())
	}
}
