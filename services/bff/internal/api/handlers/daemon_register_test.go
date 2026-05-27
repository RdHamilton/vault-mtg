package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/posthog/posthog-go"

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

// wgPostHogClient wraps mockPostHogClient and signals a WaitGroup on each
// Enqueue call so the test can deterministically wait for the goroutine that
// fires the PostHog event without sleeping.
type wgPostHogClient struct {
	inner *mockPostHogClient
	wg    *sync.WaitGroup
}

func (w *wgPostHogClient) Enqueue(msg posthog.Message) error {
	defer w.wg.Done()
	return w.inner.Enqueue(msg)
}

// testHashAccountID mirrors handlers.hashAccountID (SHA-256 hex[:16]) so the
// test can compute the expected DistinctId without exporting the production
// helper.
func testHashAccountID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%x", sum)[:16]
}

// TestDaemonRegister_PostHogDistinctIDIsHashed verifies that the daemon_paired
// PostHog event emitted on first pairing uses a hashed account_id as the
// DistinctId, never the raw Clerk user_id (PII).
//
// The goroutine wrapping the Enqueue call is waited on deterministically via a
// sync.WaitGroup — no time.Sleep per Ray's Q1 amendment.
func TestDaemonRegister_PostHogDistinctIDIsHashed(t *testing.T) {
	const clerkUserID = "user_test123"
	wantDistinctID := testHashAccountID(clerkUserID)

	inner := &mockPostHogClient{}
	var wg sync.WaitGroup
	wg.Add(1)
	ph := &wgPostHogClient{inner: inner, wg: &wg}

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequest(clerkUserID)
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Wait for the goroutine to call Enqueue before asserting.
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog Enqueue call, got %d", len(inner.calls))
	}

	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	// DistinctId must be the hashed value, not the raw Clerk user_id.
	if capture.DistinctId == clerkUserID {
		t.Errorf("DistinctId must be hashed, got raw Clerk user_id %q — PII leak", capture.DistinctId)
	}
	if capture.DistinctId != wantDistinctID {
		t.Errorf("DistinctId=%q, want hashed value %q", capture.DistinctId, wantDistinctID)
	}
}

// ── R2/R3 reinstall path tests (ADR-034 §3) ──────────────────────────────────

// TestDaemonRegister_R2R3_ReinstallEmptyDeviceID_Returns201WithNewKey verifies
// the R2/R3 reinstall contract: an account that was previously registered sends
// an empty device_id (daemon.json was deleted on reinstall). The BFF mints a
// fresh server-issued UUIDv4, persists a new row, and returns 201 + plaintext
// api_key so the daemon can re-populate the OS keychain.
//
// This test confirms: the BFF does NOT return 200 + empty key (already-registered
// signal) when device_id is empty — it always treats empty as a new install.
func TestDaemonRegister_R2R3_ReinstallEmptyDeviceID_Returns201WithNewKey(t *testing.T) {
	// No existing row (existing == nil) — fresh stub, simulates the BFF having
	// no matching row for an empty device_id.
	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	req := newRegisterRequestWithBody("user_reinstall_r2r3", map[string]string{
		"device_id":  "", // daemon.json was deleted → empty
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	// Must be 201 with a fresh key — NOT 200 (which would mean alreadyRegistered).
	if rr.Code != http.StatusCreated {
		t.Fatalf("reinstall with empty device_id: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// 201 must carry a non-empty api_key (plaintext, to be stored in the OS keychain).
	apiKey, _ := resp["api_key"].(string)
	if apiKey == "" {
		t.Error("201 reinstall response must include a non-empty api_key for keychain storage")
	}

	// 201 must carry a server-minted device_id.
	deviceID, _ := resp["device_id"].(string)
	if deviceID == "" {
		t.Fatal("201 reinstall response must include a server-minted device_id")
	}
	if _, err := uuid.Parse(deviceID); err != nil {
		t.Errorf("server-minted device_id must be a valid UUID, got %q", deviceID)
	}
}

// ─── stub user repo ───────────────────────────────────────────────────────────

// stubUserRepo implements userUpserter for tests that need a user with a
// known CreatedAt so time_since_signup_seconds can be asserted.
type stubUserRepo struct {
	user *repository.User
	err  error
}

func (s *stubUserRepo) UpsertByClerkUserID(_ context.Context, _ string) (*repository.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

// ─── ADR-027 conformance tests ────────────────────────────────────────────────

// makeWGPostHog returns a wgPostHogClient that calls wg.Done() on each Enqueue.
func makeWGPostHog(wg *sync.WaitGroup) (*wgPostHogClient, *mockPostHogClient) {
	inner := &mockPostHogClient{}
	return &wgPostHogClient{inner: inner, wg: wg}, inner
}

// TestDaemonRegister_PostHogEvent_HasDeviceID verifies that the daemon_paired
// PostHog event includes a "device_id" property set to the server-authoritative
// device UUID per ADR-027 §3.
func TestDaemonRegister_PostHogEvent_HasDeviceID(t *testing.T) {
	const clerkUserID = "user_deviceid_check"
	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequest(clerkUserID)
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	deviceID, exists := capture.Properties["device_id"]
	if !exists {
		t.Fatal("daemon_paired event must include 'device_id' property per ADR-027 §3")
	}
	deviceIDStr, _ := deviceID.(string)
	if _, err := uuid.Parse(deviceIDStr); err != nil {
		t.Errorf("device_id property must be a valid UUID, got %q", deviceIDStr)
	}
}

// TestDaemonRegister_PostHogEvent_HasAccountIDHashProperty verifies that the
// daemon_paired event includes an explicit "account_id_hash" property per
// ADR-027 §3. The DistinctId being hashed is tested separately.
func TestDaemonRegister_PostHogEvent_HasAccountIDHashProperty(t *testing.T) {
	const clerkUserID = "user_acct_hash_prop"
	wantHash := testHashAccountID(clerkUserID)

	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequest(clerkUserID)
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	hashProp, exists := capture.Properties["account_id_hash"]
	if !exists {
		t.Fatal("daemon_paired event must include explicit 'account_id_hash' property per ADR-027 §3")
	}
	hashStr, _ := hashProp.(string)
	if hashStr != wantHash {
		t.Errorf("account_id_hash property: got %q, want %q", hashStr, wantHash)
	}
}

// TestDaemonRegister_PostHogEvent_OmitsKeyID_AndSource verifies that the
// daemon_paired event does NOT contain "key_id" or "source" properties.
// These were present in the legacy emission and are non-conformant with ADR-027 §3.
func TestDaemonRegister_PostHogEvent_OmitsKeyID_AndSource(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequest("user_omit_check")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if _, exists := capture.Properties["key_id"]; exists {
		t.Error("daemon_paired event must NOT include 'key_id' property — non-conformant per ADR-027 §3")
	}
	if _, exists := capture.Properties["source"]; exists {
		t.Error("daemon_paired event must NOT include 'source' property — non-conformant per ADR-027 §3")
	}
}

// TestDaemonRegister_PostHogEvent_HasTimeSinceSignup verifies that the
// daemon_paired event includes a "time_since_signup_seconds" property derived
// from the user's CreatedAt field per Ray's Option A decision.
func TestDaemonRegister_PostHogEvent_HasTimeSinceSignup(t *testing.T) {
	const clerkUserID = "user_signup_time"
	signupTime := time.Now().UTC().Add(-72 * time.Hour) // signed up 72 hours ago

	userRepo := &stubUserRepo{
		user: &repository.User{
			ID:          42,
			ClerkUserID: func() *string { s := clerkUserID; return &s }(),
			CreatedAt:   signupTime,
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, userRepo).WithPostHogClient(ph)

	req := newRegisterRequest(clerkUserID)
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	raw, exists := capture.Properties["time_since_signup_seconds"]
	if !exists {
		t.Fatal("daemon_paired event must include 'time_since_signup_seconds' property per ADR-027 §3")
	}
	secondsFloat, ok := raw.(float64)
	if !ok {
		t.Fatalf("time_since_signup_seconds must be numeric, got %T", raw)
	}
	// 72h = 259200s; allow ±5s for execution jitter.
	const expectedSeconds = 72 * 3600
	if secondsFloat < expectedSeconds-5 || secondsFloat > expectedSeconds+5 {
		t.Errorf("time_since_signup_seconds: got %.0f, want ~%d (±5s)", secondsFloat, expectedSeconds)
	}
}

// TestDaemonRegister_PostHogDistinctIDIsHashed_RawAccountIDAbsent extends
// the existing PII test to also assert that the raw account_id is absent from
// ALL event properties (not just DistinctId).
func TestDaemonRegister_PostHogDistinctIDIsHashed_RawAccountIDAbsent(t *testing.T) {
	const clerkUserID = "user_pii_absent_check"

	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequest(clerkUserID)
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	// DistinctId must not be the raw Clerk user_id.
	if capture.DistinctId == clerkUserID {
		t.Errorf("DistinctId must be hashed, got raw Clerk user_id %q — PII leak", capture.DistinctId)
	}

	// No property may contain the raw account_id string value.
	for k, v := range capture.Properties {
		if strVal, ok := v.(string); ok && strVal == clerkUserID {
			t.Errorf("PostHog property %q contains raw account_id %q — PII leak", k, clerkUserID)
		}
	}
}

// TestDaemonRegister_PostHogEvent_HasPlatform verifies that the daemon_paired
// PostHog event includes a "platform" property with the value sent by the
// daemon in the request body, per ADR-027 §3.
func TestDaemonRegister_PostHogEvent_HasPlatform(t *testing.T) {
	const clerkUserID = "user_platform_check"
	const wantPlatform = "darwin"

	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequestWithBody(clerkUserID, map[string]string{
		"device_id":  "550e8400-e29b-41d4-a716-446655440200",
		"platform":   wantPlatform,
		"daemon_ver": "0.3.1",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	got, exists := capture.Properties["platform"]
	if !exists {
		t.Fatal("daemon_paired event must include 'platform' property per ADR-027 §3")
	}
	if got != wantPlatform {
		t.Errorf("platform property: got %q, want %q", got, wantPlatform)
	}
}

// TestDaemonRegister_PostHogEvent_DaemonVerKeyName verifies that the
// daemon_paired PostHog event uses the key name "daemon_ver" (not
// "daemon_version"), per ADR-027 §3's pinned schema.
func TestDaemonRegister_PostHogEvent_DaemonVerKeyName(t *testing.T) {
	const clerkUserID = "user_daemonver_key"
	const wantVer = "0.3.1"

	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequestWithBody(clerkUserID, map[string]string{
		"device_id":  "550e8400-e29b-41d4-a716-446655440201",
		"platform":   "windows",
		"daemon_ver": wantVer,
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	// The key must be exactly "daemon_ver", not "daemon_version".
	if _, exists := capture.Properties["daemon_version"]; exists {
		t.Error("daemon_paired event must NOT use key 'daemon_version' — ADR-027 §3 pins the name as 'daemon_ver'")
	}
	got, exists := capture.Properties["daemon_ver"]
	if !exists {
		t.Fatal("daemon_paired event must include 'daemon_ver' property per ADR-027 §3")
	}
	if got != wantVer {
		t.Errorf("daemon_ver property: got %q, want %q", got, wantVer)
	}
}

// TestDaemonRegister_PostHogEvent_NoAppVersion verifies that the daemon_paired
// PostHog event does NOT contain an "app_version" property. That property is
// not part of the ADR-027 §3 pinned schema (it belongs to frontend events).
// Per Najah's PM-constraint §C2, out-of-spec properties are banned without an
// ADR update.
func TestDaemonRegister_PostHogEvent_NoAppVersion(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ph, inner := makeWGPostHog(&wg)

	repo := &stubDaemonAPIKeyRepo{}
	h := handlers.NewDaemonRegisterHandler(repo, nil).WithPostHogClient(ph)

	req := newRegisterRequest("user_no_app_version")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	wg.Wait()

	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(inner.calls))
	}
	capture, ok := inner.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if _, exists := capture.Properties["app_version"]; exists {
		t.Error("daemon_paired event must NOT include 'app_version' property — not in ADR-027 §3 pinned schema (Najah PM-constraint §C2)")
	}
}

// TestDaemonRegister_R2R3_RevokedDeviceRe_Register_Returns201 verifies the
// revoked-row resurrection guard for the R3 path: daemon presents a device_id
// that maps to a revoked row. The BFF must NOT resurrect that row — instead it
// mints a fresh device_id and returns 201 + new api_key. The revoked row is
// left intact for audit.
//
// This test mirrors the AC in ADR-034 §3 and ADR-036 I-3: "a revoked device_id
// must never be resurrected."
func TestDaemonRegister_R2R3_RevokedDeviceRe_Register_Returns201(t *testing.T) {
	const revokedDeviceID = "550e8400-e29b-41d4-a716-446655440300"
	revokedAt := time.Now().UTC().Add(-2 * time.Hour)

	repo := &stubDaemonAPIKeyRepo{
		existingByDevice: map[string]*repository.DaemonAPIKey{
			revokedDeviceID: {
				ID:        "uuid-revoked-r3",
				AccountID: "user_r3_reinstall",
				DeviceID:  revokedDeviceID,
				RevokedAt: &revokedAt,
			},
		},
	}
	h := handlers.NewDaemonRegisterHandler(repo, nil)

	// Daemon (after recovery DELETE) re-registers with empty device_id.
	req := newRegisterRequestWithBody("user_r3_reinstall", map[string]string{
		"device_id":  "", // cleared by daemon recovery flow
		"platform":   "darwin",
		"daemon_ver": "0.3.3",
	})
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("R3 re-register with empty device_id: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	apiKey, _ := resp["api_key"].(string)
	if apiKey == "" {
		t.Error("R3 201 response must include a non-empty api_key")
	}

	newDeviceID, _ := resp["device_id"].(string)
	if newDeviceID == revokedDeviceID {
		t.Errorf("R3: new device_id must NOT equal the revoked device_id %q — resurrection guard failed", revokedDeviceID)
	}
	if _, err := uuid.Parse(newDeviceID); err != nil {
		t.Errorf("R3: new device_id must be a valid UUID, got %q", newDeviceID)
	}
}
