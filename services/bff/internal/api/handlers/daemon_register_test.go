package handlers_test

import (
	"bytes"
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

// ─── stub repo ────────────────────────────────────────────────────────────────

type stubDaemonAPIKeyRepo struct {
	existing *repository.DaemonAPIKey
	err      error
}

func (s *stubDaemonAPIKeyRepo) UpsertKey(_ context.Context, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer string) (*repository.DaemonAPIKey, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	if s.existing != nil {
		return s.existing, false, nil
	}
	now := time.Now().UTC()
	return &repository.DaemonAPIKey{
		ID:        "uuid-test-1",
		AccountID: accountID,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		DeviceID:  deviceID,
		Platform:  platform,
		DaemonVer: daemonVer,
		CreatedAt: now,
	}, true, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// newRegisterRequest builds a POST /api/v1/daemon/register request with a JSON body.
// When accountID is non-empty it simulates RequireClerkAuth having verified a JWT.
func newRegisterRequest(accountID string) *http.Request {
	body := map[string]string{
		"device_id":  "550e8400-e29b-41d4-a716-446655440001",
		"platform":   "darwin",
		"daemon_ver": "0.3.1",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/daemon/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if accountID != "" {
		req = middleware.WithClerkUserID(req, accountID)
	}
	return req
}

// newRegisterRequestWithBody builds a POST /api/v1/daemon/register request with a custom JSON body.
func newRegisterRequestWithBody(accountID string, body map[string]string) *http.Request {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/daemon/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if accountID != "" {
		req = middleware.WithClerkUserID(req, accountID)
	}
	return req
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestDaemonRegister_NewKey_Returns201(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequest("user_test_123")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	apiKey, _ := resp["api_key"].(string)
	if len(apiKey) == 0 {
		t.Error("expected non-empty api_key on new key creation")
	}
	if len(apiKey) < 16 {
		t.Errorf("api_key too short: %q", apiKey)
	}

	accountID, _ := resp["account_id"].(string)
	if accountID != "user_test_123" {
		t.Errorf("expected account_id=user_test_123, got %q", accountID)
	}
}

func TestDaemonRegister_ExistingKey_Returns200_EmptyAPIKey(t *testing.T) {
	existing := &repository.DaemonAPIKey{
		ID:        "uuid-existing",
		AccountID: "user_existing",
		KeyHash:   "hash",
		KeyPrefix: "sk_live_abc",
		DeviceID:  "550e8400-e29b-41d4-a716-446655440002",
		Platform:  "windows",
		DaemonVer: "0.3.1",
		CreatedAt: time.Now().UTC(),
	}
	repo := &stubDaemonAPIKeyRepo{existing: existing}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequest("user_existing")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Existing key: api_key must be empty (daemon uses its keychain copy).
	apiKey, _ := resp["api_key"].(string)
	if apiKey != "" {
		t.Errorf("expected empty api_key for existing key, got %q", apiKey)
	}

	accountID, _ := resp["account_id"].(string)
	if accountID != "user_existing" {
		t.Errorf("expected account_id=user_existing, got %q", accountID)
	}
}

func TestDaemonRegister_MissingClerkAuth_Returns401(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	// No Clerk user ID set on context.
	body := map[string]string{
		"device_id":  "550e8400-e29b-41d4-a716-446655440003",
		"platform":   "darwin",
		"daemon_ver": "0.3.1",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/daemon/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestDaemonRegister_RepoError_Returns500(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{err: context.DeadlineExceeded}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequest("user_err")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDaemonRegister_RateLimit_Returns429(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	// Send 5 requests (should succeed).
	for i := 0; i < 5; i++ {
		req := newRegisterRequest("user_ratelimit")
		rr := httptest.NewRecorder()
		h.Register(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			t.Fatalf("rate limit hit too early on request %d", i+1)
		}
	}

	// 6th request should be rate limited.
	req := newRegisterRequest("user_ratelimit")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on 6th request, got %d", rr.Code)
	}
}

func TestDaemonRegister_APIKeyFormat(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequest("user_format")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	apiKey, _ := resp["api_key"].(string)
	if len(apiKey) == 0 {
		t.Fatal("api_key is empty")
	}
	// "sk_live_" (8) + 64 hex chars = 72 chars total.
	const expected = 8 + 64
	if len(apiKey) != expected {
		t.Errorf("api_key length: want %d, got %d (%q)", expected, len(apiKey), apiKey)
	}
}

func TestDaemonRegister_MissingDeviceID_Returns400(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequestWithBody("user_nodevice", map[string]string{
		"platform":   "darwin",
		"daemon_ver": "0.3.1",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing device_id, got %d", rr.Code)
	}
}

func TestDaemonRegister_MissingPlatform_Returns400(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequestWithBody("user_noplat", map[string]string{
		"device_id":  "550e8400-e29b-41d4-a716-446655440004",
		"daemon_ver": "0.3.1",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing platform, got %d", rr.Code)
	}
}

func TestDaemonRegister_MissingDaemonVer_Returns400(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo)

	req := newRegisterRequestWithBody("user_nover", map[string]string{
		"device_id": "550e8400-e29b-41d4-a716-446655440005",
		"platform":  "windows",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing daemon_ver, got %d", rr.Code)
	}
}
