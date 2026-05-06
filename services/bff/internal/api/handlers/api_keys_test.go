package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// stubAPIKeyCreator is a test double for the apiKeyCreator interface.
type stubAPIKeyCreator struct {
	key *repository.APIKey
	err error
}

func (s *stubAPIKeyCreator) Create(_ context.Context, userID int64, keyHash string) (*repository.APIKey, error) {
	if s.err != nil {
		return nil, s.err
	}

	if s.key != nil {
		return s.key, nil
	}

	return &repository.APIKey{
		ID:        1,
		UserID:    userID,
		KeyHash:   keyHash,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// errAPIKeyCreator always returns an error from Create.
type errAPIKeyCreator struct{}

func (e *errAPIKeyCreator) Create(_ context.Context, _ int64, _ string) (*repository.APIKey, error) {
	return nil, context.DeadlineExceeded
}

// newAPIKeysRequest builds a POST /api/keys request. When userID > 0 the
// APIKeyAuth context value is populated to simulate middleware having
// validated an API key.
func newAPIKeysRequest(userID int64) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/keys", nil)
	if userID > 0 {
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	}

	return req
}

func TestCreateAPIKey_Success(t *testing.T) {
	h := handlers.NewAPIKeysHandler(&stubAPIKeyCreator{})

	req := newAPIKeysRequest(42)
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	key, ok := resp["key"].(string)
	if !ok || len(key) != 64 {
		t.Errorf("expected 64-char hex key, got %q", key)
	}

	if _, ok := resp["created_at"]; !ok {
		t.Error("response missing created_at")
	}
}

func TestCreateAPIKey_MissingUserID(t *testing.T) {
	h := handlers.NewAPIKeysHandler(&stubAPIKeyCreator{})

	// No user ID in context (APIKeyAuth middleware not applied).
	req := httptest.NewRequest(http.MethodPost, "/api/keys", nil)
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	assertJSONError(t, rr, "unauthorized")
}

func TestCreateAPIKey_ZeroUserID(t *testing.T) {
	h := handlers.NewAPIKeysHandler(&stubAPIKeyCreator{})

	// A zero user_id must also be rejected (invalid).
	req := httptest.NewRequest(http.MethodPost, "/api/keys", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 0))
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCreateAPIKey_DBError(t *testing.T) {
	h := handlers.NewAPIKeysHandler(&errAPIKeyCreator{})

	req := newAPIKeysRequest(1)
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestCreateAPIKey_KeyIsUnique(t *testing.T) {
	h := handlers.NewAPIKeysHandler(&stubAPIKeyCreator{})

	rr1, rr2 := httptest.NewRecorder(), httptest.NewRecorder()
	h.CreateAPIKey(rr1, newAPIKeysRequest(1))
	h.CreateAPIKey(rr2, newAPIKeysRequest(1))

	var r1, r2 map[string]any
	_ = json.NewDecoder(rr1.Body).Decode(&r1)
	_ = json.NewDecoder(rr2.Body).Decode(&r2)

	k1, _ := r1["key"].(string)
	k2, _ := r2["key"].(string)

	if k1 == k2 {
		t.Error("two successive key generations returned identical plaintext keys")
	}
}

// assertJSONError checks that the response body is {"error": msg}.
func assertJSONError(t *testing.T, rr *httptest.ResponseRecorder, msg string) {
	t.Helper()

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if body["error"] != msg {
		t.Errorf("expected error %q, got %q", msg, body["error"])
	}
}
