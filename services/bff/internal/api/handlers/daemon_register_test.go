package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// ─── stub repo ────────────────────────────────────────────────────────────────

type stubDaemonAPIKeyRepo struct {
	existing *repository.DaemonAPIKey
	err      error

	// existingByDevice maps device_id → an existing row (possibly revoked).
	// Used by GetByAccountAndDevice to simulate the revoked-row-resurrection
	// guard's lookup path. When nil, every GetByAccountAndDevice call
	// returns ErrDaemonAPIKeyNotFound.
	existingByDevice map[string]*repository.DaemonAPIKey

	// upsertCalls captures every UpsertKey invocation so tests can assert
	// what device_id the handler ultimately submitted to the repo (the
	// resurrection guard rewrites device_id="" so the ADR-028 first-pair
	// path mints a new UUID).
	upsertCalls []upsertCall
}

type upsertCall struct {
	AccountID string
	DeviceID  string
}

func (s *stubDaemonAPIKeyRepo) UpsertKey(_ context.Context, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer string) (*repository.DaemonAPIKey, bool, error) {
	s.upsertCalls = append(s.upsertCalls, upsertCall{AccountID: accountID, DeviceID: deviceID})
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

func (s *stubDaemonAPIKeyRepo) GetByAccountAndDevice(_ context.Context, _, deviceID string) (*repository.DaemonAPIKey, error) {
	if s.existingByDevice == nil {
		return nil, repository.ErrDaemonAPIKeyNotFound
	}
	rec, ok := s.existingByDevice[deviceID]
	if !ok {
		return nil, repository.ErrDaemonAPIKeyNotFound
	}
	return rec, nil
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
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequest("user_err")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDaemonRegister_RateLimit_Returns429(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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

// TestDaemonRegister_EmptyDeviceID_MintsAndReturns201 verifies that when the
// daemon sends an empty device_id (first install, no cached value), the BFF
// mints a fresh server-issued UUIDv4, returns 201, and the response body
// contains a parseable device_id.
// Per ADR-028: empty device_id is no longer 400; the BFF is now the source of truth.
func TestDaemonRegister_EmptyDeviceID_MintsAndReturns201(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequestWithBody("user_firstinstall", map[string]string{
		"device_id":  "",
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for empty device_id (BFF mints), got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Response must include a valid device_id minted by the BFF.
	deviceID, _ := resp["device_id"].(string)
	if deviceID == "" {
		t.Fatal("expected non-empty device_id in response when BFF mints")
	}
	if _, err := uuid.Parse(deviceID); err != nil {
		t.Errorf("server-minted device_id must be a valid UUID, got %q: %v", deviceID, err)
	}

	// The stub repo must have received the minted device_id (not empty string).
	if len(repo.upsertCalls) != 1 {
		t.Fatalf("expected exactly 1 UpsertKey call, got %d", len(repo.upsertCalls))
	}
	if repo.upsertCalls[0].DeviceID == "" {
		t.Error("UpsertKey must receive the minted device_id, not empty string")
	}
	if _, err := uuid.Parse(repo.upsertCalls[0].DeviceID); err != nil {
		t.Errorf("minted device_id passed to UpsertKey must be a valid UUID, got %q", repo.upsertCalls[0].DeviceID)
	}
}

// TestDaemonRegister_TwoEmptyDeviceIDCalls_TwoDistinctUUIDs verifies that two
// sequential requests with empty device_id from the same account_id each receive
// a distinct server-minted device_id. This confirms multi-device pairing semantics
// per ADR-028: each empty-device_id call produces a new installation identity.
func TestDaemonRegister_TwoEmptyDeviceIDCalls_TwoDistinctUUIDs(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	body := map[string]string{
		"device_id":  "",
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	}

	rr1 := httptest.NewRecorder()
	h.Register(rr1, newRegisterRequestWithBody("user_multidevice", body))
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first call: expected 201, got %d: %s", rr1.Code, rr1.Body.String())
	}

	rr2 := httptest.NewRecorder()
	h.Register(rr2, newRegisterRequestWithBody("user_multidevice", body))
	if rr2.Code != http.StatusCreated {
		t.Fatalf("second call: expected 201, got %d: %s", rr2.Code, rr2.Body.String())
	}

	if len(repo.upsertCalls) != 2 {
		t.Fatalf("expected 2 UpsertKey calls, got %d", len(repo.upsertCalls))
	}
	id1 := repo.upsertCalls[0].DeviceID
	id2 := repo.upsertCalls[1].DeviceID
	if id1 == id2 {
		t.Errorf("two empty device_id calls from the same account must produce distinct UUIDs; both got %q", id1)
	}
	if _, err := uuid.Parse(id1); err != nil {
		t.Errorf("first minted device_id must be valid UUID, got %q", id1)
	}
	if _, err := uuid.Parse(id2); err != nil {
		t.Errorf("second minted device_id must be valid UUID, got %q", id2)
	}
}

// TestDaemonRegister_MalformedDeviceID_Returns400 verifies that a non-empty,
// non-UUID device_id is rejected with 400. This is the tampered-daemon defense
// per ADR-028 §"Implementation Notes" item 1 bullet 4.
func TestDaemonRegister_MalformedDeviceID_Returns400(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequestWithBody("user_tampered", map[string]string{
		"device_id":  "not-a-uuid",
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed device_id, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	errMsg, _ := resp["error"].(string)
	if errMsg != "device_id must be a valid UUID" {
		t.Errorf("expected error message %q, got %q", "device_id must be a valid UUID", errMsg)
	}
}

func TestDaemonRegister_MissingPlatform_Returns400(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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
	h := handlers.NewDaemonRegisterHandler(repo, nil)

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

// TestDaemonRegister_RevokedRowResurrectionGuard verifies the ADR-031 §5 +
// ADR-028 invariant: a daemon replaying a stale device_id that maps to a
// revoked row MUST NOT resurrect that row. The handler detects the revoked
// row via GetByAccountAndDevice and clears reqBody.DeviceID="" so the
// ADR-028 first-pair path mints a fresh server-issued UUID — the resulting
// new row carries a new device_id, leaving the original revoked row intact.
//
// This is the load-bearing test that proves a revoked daemon cannot
// resurrect itself by replaying its cached device_id.
func TestDaemonRegister_RevokedRowResurrectionGuard(t *testing.T) {
	staleDeviceID := "550e8400-e29b-41d4-a716-446655440099"
	revokedAt := time.Now().UTC().Add(-1 * time.Hour)
	repo := &stubDaemonAPIKeyRepo{
		existingByDevice: map[string]*repository.DaemonAPIKey{
			staleDeviceID: {
				ID:        "uuid-old-revoked",
				AccountID: "user_resurrect",
				DeviceID:  staleDeviceID,
				RevokedAt: &revokedAt,
			},
		},
	}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	// Daemon replays the stale, revoked device_id.
	req := newRegisterRequestWithBody("user_resurrect", map[string]string{
		"device_id":  staleDeviceID,
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 (new row minted), got %d: %s", rr.Code, rr.Body.String())
	}

	// CRITICAL: the handler must have rewritten device_id to "" before
	// calling UpsertKey, so the ADR-028 first-pair path mints a new UUID.
	// If the handler passed the stale device_id through to UpsertKey, the
	// resurrection guard is broken.
	if len(repo.upsertCalls) != 1 {
		t.Fatalf("expected exactly 1 UpsertKey call, got %d", len(repo.upsertCalls))
	}
	if repo.upsertCalls[0].DeviceID == staleDeviceID {
		t.Errorf("resurrection guard FAILED — UpsertKey received the stale revoked device_id %q; should have been replaced by a freshly-minted UUID", staleDeviceID)
	}
	// The replacement device_id MUST be a valid UUID (so the DB's NOT NULL
	// UUID column accepts it). Empty string would also fail this assertion,
	// guarding against a future refactor that drops the inline mint.
	if _, err := uuid.Parse(repo.upsertCalls[0].DeviceID); err != nil {
		t.Errorf("resurrection guard MUST mint a valid UUID; got %q (err: %v)", repo.upsertCalls[0].DeviceID, err)
	}
}

// TestDaemonRegister_ActiveRowReplay_PassesThroughDeviceID verifies the
// guard is narrow: an ACTIVE (non-revoked) existing row for the same
// (account, device) submission must NOT trigger the rewrite — the
// UNIQUE(account_id, device_id) constraint will trip and surface as the
// duplicate-key error that #2631 owns mapping to 409. The guard only fires
// on revoked rows.
func TestDaemonRegister_ActiveRowReplay_PassesThroughDeviceID(t *testing.T) {
	activeDeviceID := "550e8400-e29b-41d4-a716-446655440100"
	repo := &stubDaemonAPIKeyRepo{
		existingByDevice: map[string]*repository.DaemonAPIKey{
			activeDeviceID: {
				ID:        "uuid-active",
				AccountID: "user_active",
				DeviceID:  activeDeviceID,
				RevokedAt: nil,
			},
		},
	}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequestWithBody("user_active", map[string]string{
		"device_id":  activeDeviceID,
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if len(repo.upsertCalls) != 1 {
		t.Fatalf("expected exactly 1 UpsertKey call, got %d", len(repo.upsertCalls))
	}
	if repo.upsertCalls[0].DeviceID != activeDeviceID {
		t.Errorf("active row replay MUST pass device_id through unchanged; got %q", repo.upsertCalls[0].DeviceID)
	}
}

// TestDaemonRegister_DeviceIDEchoedOn201 verifies that a 201 response body
// includes the server-authoritative device_id read from the repo row (rec.DeviceID),
// not from the request body. Per ADR-034 §1 and Ray's Q4 verdict.
func TestDaemonRegister_DeviceIDEchoedOn201(t *testing.T) {
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequest("user_echo201")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	deviceID, _ := resp["device_id"].(string)
	if deviceID == "" {
		t.Fatal("device_id must be non-empty in 201 response per ADR-034 §1")
	}
	if _, err := uuid.Parse(deviceID); err != nil {
		t.Errorf("device_id in 201 response must be a valid UUID, got %q", deviceID)
	}
}

// TestDaemonRegister_DeviceIDEchoedOn200 verifies that a 200 (already-registered)
// response body includes the device_id from the existing repo row. Per ADR-034 §1:
// "The response always carries the resolved device_id ... api_key is empty on 200."
func TestDaemonRegister_DeviceIDEchoedOn200(t *testing.T) {
	const existingDeviceID = "550e8400-e29b-41d4-a716-446655440002"
	existing := &repository.DaemonAPIKey{
		ID:        "uuid-existing-echo",
		AccountID: "user_echo200",
		KeyHash:   "hash",
		KeyPrefix: "sk_live_abc",
		DeviceID:  existingDeviceID,
		Platform:  "darwin",
		DaemonVer: "0.3.1",
		CreatedAt: time.Now().UTC(),
	}
	repo := &stubDaemonAPIKeyRepo{existing: existing}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequest("user_echo200")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// api_key must be empty on 200 (ADR-034 §1).
	apiKey, _ := resp["api_key"].(string)
	if apiKey != "" {
		t.Errorf("api_key must be empty on 200 response, got %q", apiKey)
	}

	// device_id must be echoed from the repo row (ADR-034 §1).
	deviceID, _ := resp["device_id"].(string)
	if deviceID != existingDeviceID {
		t.Errorf("device_id in 200 response must equal repo row value %q, got %q", existingDeviceID, deviceID)
	}
}
