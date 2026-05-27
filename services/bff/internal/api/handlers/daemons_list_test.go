package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// stubDaemonListRepo is a test double for the daemonAPIKeyListRepo interface.
type stubDaemonListRepo struct {
	byAccount map[string][]repository.DaemonAPIKey
	err       error
}

func (s *stubDaemonListRepo) ListByAccountID(_ context.Context, accountID string) ([]repository.DaemonAPIKey, error) {
	if s.err != nil {
		return nil, s.err
	}
	out, ok := s.byAccount[accountID]
	if !ok {
		return []repository.DaemonAPIKey{}, nil
	}
	return out, nil
}

func newListRequest(accountID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemons", nil)
	if accountID != "" {
		req = middleware.WithClerkUserID(req, accountID)
	}
	return req
}

// TestDaemonsList_HappyPath_Returns200WithDevices verifies AC1: a logged-in
// user with paired devices receives 200 + a JSON array of device records.
func TestDaemonsList_HappyPath_Returns200WithDevices(t *testing.T) {
	paired := time.Now().UTC().Add(-1 * time.Hour)
	repo := &stubDaemonListRepo{
		byAccount: map[string][]repository.DaemonAPIKey{
			"user_A": {
				{DeviceID: "dev-A1", Platform: "darwin", DaemonVer: "0.3.3", PairedAt: paired.Add(1 * time.Hour)},
				{DeviceID: "dev-A2", Platform: "windows", DaemonVer: "0.3.3", PairedAt: paired},
			},
		},
	}
	h := handlers.NewDaemonsListHandler(repo)

	req := newListRequest("user_A")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(resp.Devices))
	}
	if resp.Devices[0]["device_id"] != "dev-A1" {
		t.Errorf("expected first device dev-A1, got %v", resp.Devices[0]["device_id"])
	}
	if resp.Devices[1]["device_id"] != "dev-A2" {
		t.Errorf("expected second device dev-A2, got %v", resp.Devices[1]["device_id"])
	}
}

// TestDaemonsList_EmptyState_Returns200WithEmptyArray verifies AC4 / ADR-031
// §4 empty-state: a user with zero paired devices receives 200 + an empty
// JSON array (NOT null, NOT an error).
func TestDaemonsList_EmptyState_Returns200WithEmptyArray(t *testing.T) {
	repo := &stubDaemonListRepo{byAccount: map[string][]repository.DaemonAPIKey{}}
	h := handlers.NewDaemonsListHandler(repo)

	req := newListRequest("user_empty")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Assert raw JSON: must include "devices":[], not "devices":null.
	body := rr.Body.String()
	if !strings.Contains(body, `"devices":[]`) {
		t.Errorf("expected empty array marshalled as []; got body: %s", body)
	}
}

// TestDaemonsList_SensitiveFieldExclusion is the ADR-031 Fitness Function §4
// load-bearing assertion: raw response bytes must not contain key_hash or
// key_prefix. Even if the struct shape changes in the future, this test
// guards against accidental exposure of the bcrypt hash or its prefix.
func TestDaemonsList_SensitiveFieldExclusion(t *testing.T) {
	repo := &stubDaemonListRepo{
		byAccount: map[string][]repository.DaemonAPIKey{
			"user_sens": {
				{
					ID:        "should-not-appear-id",
					AccountID: "user_sens",
					KeyHash:   "$2a$10$THIS_HASH_MUST_NEVER_APPEAR",
					KeyPrefix: "sk_live_SHOULDNT",
					DeviceID:  "dev-sens-1",
					Platform:  "darwin",
					DaemonVer: "0.3.3",
					PairedAt:  time.Now().UTC(),
				},
			},
		},
	}
	h := handlers.NewDaemonsListHandler(repo)

	req := newListRequest("user_sens")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	forbidden := []string{
		"key_hash", "keyHash", "KeyHash",
		"key_prefix", "keyPrefix", "KeyPrefix",
		"THIS_HASH_MUST_NEVER_APPEAR",
		"sk_live_SHOULDNT",
		"should-not-appear-id",
	}
	for _, s := range forbidden {
		if strings.Contains(body, s) {
			t.Errorf("response body MUST NOT contain %q; body: %s", s, body)
		}
	}
}

// TestDaemonsList_CrossTenancy verifies the cross-tenancy guarantee at the
// handler layer: the handler MUST pass the caller's Clerk user_id to the
// repo, never a client-supplied account_id. Two users with separate devices
// each see only their own.
func TestDaemonsList_CrossTenancy(t *testing.T) {
	repo := &stubDaemonListRepo{
		byAccount: map[string][]repository.DaemonAPIKey{
			"user_A": {{DeviceID: "dev-A1", Platform: "darwin", DaemonVer: "0.3.3", PairedAt: time.Now()}},
			"user_B": {{DeviceID: "dev-B1", Platform: "windows", DaemonVer: "0.3.3", PairedAt: time.Now()}},
		},
	}
	h := handlers.NewDaemonsListHandler(repo)

	// User B asks for the list — must see ONLY dev-B1, never dev-A1.
	req := newListRequest("user_B")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "dev-A1") {
		t.Errorf("User B's response MUST NOT contain User A's device dev-A1; body: %s", body)
	}
	if !strings.Contains(body, "dev-B1") {
		t.Errorf("User B's response should contain dev-B1; body: %s", body)
	}
}

// TestDaemonsList_MissingClerkAuth_Returns401 verifies the handler returns
// 401 when no Clerk session is attached to the request context (defence in
// depth — the RequireClerkAuth middleware should reject first, but the
// handler must also fail safe).
func TestDaemonsList_MissingClerkAuth_Returns401(t *testing.T) {
	repo := &stubDaemonListRepo{}
	h := handlers.NewDaemonsListHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemons", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestDaemonsList_RepoError_Returns500 verifies a repo-layer error surfaces
// as a 500 (and importantly: does NOT leak the underlying error to the body).
func TestDaemonsList_RepoError_Returns500(t *testing.T) {
	repo := &stubDaemonListRepo{err: errors.New("db gone")}
	h := handlers.NewDaemonsListHandler(repo)

	req := newListRequest("user_err")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "db gone") {
		t.Errorf("response body MUST NOT echo the underlying error; body: %s", rr.Body.String())
	}
}
