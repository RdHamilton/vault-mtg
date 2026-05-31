package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/posthog/posthog-go"
	"golang.org/x/crypto/bcrypt"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

const (
	// daemonAPIKeyPrefix is prepended to every minted daemon API key.
	daemonAPIKeyPrefix = "sk_live_"

	// daemonRateLimitWindow is the sliding window for per-account rate limiting.
	daemonRateLimitWindow = time.Hour

	// daemonRateLimitMax is the maximum number of /v1/daemon/register calls
	// allowed per account per daemonRateLimitWindow.
	daemonRateLimitMax = 5
)

// daemonAPIKeyUpsertRepo is the subset of DaemonAPIKeyRepository used by
// DaemonRegisterHandler. GetByAccountAndDevice supports the ADR-031 §5 +
// ADR-028 revoked-row-resurrection guard (a daemon replaying a stale,
// revoked device_id MUST receive a freshly-minted server-issued device_id,
// not a resurrection of the revoked row).
type daemonAPIKeyUpsertRepo interface {
	UpsertKey(ctx context.Context, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer string) (*repository.DaemonAPIKey, bool, error)
	GetByAccountAndDevice(ctx context.Context, accountID, deviceID string) (*repository.DaemonAPIKey, error)
}

// userUpserter is the subset of UserRepository used to ensure a users row
// exists for the Clerk user_id before issuing a daemon api_key. Without this
// the subsequent ingest auth path (DaemonAPIKeyAuth) cannot resolve the key
// back to an int64 users.id.
type userUpserter interface {
	UpsertByClerkUserID(ctx context.Context, clerkUserID string) (*repository.User, error)
}

// rateEntry tracks register call timestamps for one account.
type rateEntry struct {
	mu        sync.Mutex
	callTimes []time.Time
}

// allow returns true if the call is within the rate limit, false otherwise.
func (e *rateEntry) allow() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-daemonRateLimitWindow)

	// Prune stale timestamps.
	filtered := e.callTimes[:0]
	for _, t := range e.callTimes {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	e.callTimes = filtered

	if len(e.callTimes) >= daemonRateLimitMax {
		return false
	}
	e.callTimes = append(e.callTimes, now)
	return true
}

// DaemonRegisterHandler handles POST /v1/daemon/register.
//
// It accepts a Clerk JWT (verified by RequireClerkAuth middleware), mints
// (or retrieves) a per-account API key scoped to the Clerk user's account_id,
// and returns it to the daemon.  Rate limited at 5 req/hour per account_id
// using in-memory state (no Redis required for beta volume).
//
// Required request body fields: device_id (UUID), platform (string), daemon_ver (semver string).
//
// Response:
//   - 201 Created — new key minted; api_key field contains the plaintext key.
//   - 200 OK — existing key returned; api_key is empty (daemon must use
//     its locally stored keychain value).
type DaemonRegisterHandler struct {
	repo       daemonAPIKeyUpsertRepo
	userRepo   userUpserter
	postHog    PostHogClient
	rateMu     sync.Mutex
	rateByAcct map[string]*rateEntry
}

// NewDaemonRegisterHandler returns a handler backed by the given repositories.
// userRepo may be nil in tests; in production it must be wired so the user row
// is JIT-provisioned for the Clerk identity before the api_key is issued.
func NewDaemonRegisterHandler(repo daemonAPIKeyUpsertRepo, userRepo userUpserter) *DaemonRegisterHandler {
	return &DaemonRegisterHandler{
		repo:       repo,
		userRepo:   userRepo,
		postHog:    noopPostHogClient{},
		rateByAcct: make(map[string]*rateEntry),
	}
}

// WithPostHogClient wires a PostHog client for analytics events.
func (h *DaemonRegisterHandler) WithPostHogClient(ph PostHogClient) *DaemonRegisterHandler {
	h.postHog = ph
	return h
}

// daemonRegisterRequest is the JSON request body for POST /v1/daemon/register.
type daemonRegisterRequest struct {
	// DeviceID is a UUID uniquely identifying this daemon installation.
	DeviceID string `json:"device_id"`
	// Platform is the OS the daemon is running on (e.g. "darwin", "windows").
	Platform string `json:"platform"`
	// DaemonVer is the semantic version string of the daemon binary (e.g. "0.3.1").
	DaemonVer string `json:"daemon_ver"`
}

// daemonRegisterResponse is the JSON response body for POST /v1/daemon/register.
type daemonRegisterResponse struct {
	// APIKey is the plaintext API key — present only on 201 (new key created).
	// On 200 (existing key) this field is empty; the daemon uses its keychain copy.
	APIKey    string `json:"api_key"`
	AccountID string `json:"account_id"`
	// DeviceID is the server-authoritative UUID for this daemon installation.
	// Echoed from the repo row on both 201 (new) and 200 (existing) responses
	// per ADR-028 and ADR-034 §1.
	DeviceID string `json:"device_id"`
}

// Register handles POST /v1/daemon/register.
// RequireClerkAuth middleware must run first.
func (h *DaemonRegisterHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Clerk user ID is placed on context by RequireClerkAuth.
	accountID, ok := middleware.ClerkUserIDFromContext(r)
	if !ok || accountID == "" {
		log.Printf("[daemon_register] missing Clerk user ID — RequireClerkAuth not applied")
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// In-memory rate limit: 5 req/hour per account_id.
	if !h.rateAllow(accountID) {
		writeJSONError(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Decode request body for device metadata.
	var reqBody daemonRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("[daemon_register] decode body: %v", err)
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// device_id handling per ADR-028:
	//   - Empty → BFF mints a fresh server-issued UUIDv4 (first-install path).
	//   - Non-empty, valid UUID → pass through to UpsertKey (cached value from daemon.json).
	//   - Non-empty, malformed → 400 (tampered-daemon defense per ADR-028 §"Implementation Notes").
	if reqBody.DeviceID == "" {
		reqBody.DeviceID = uuid.NewString()
	} else if _, err := uuid.Parse(reqBody.DeviceID); err != nil {
		writeJSONError(w, "device_id must be a valid UUID", http.StatusBadRequest)
		return
	}
	if reqBody.Platform == "" {
		writeJSONError(w, "platform is required", http.StatusBadRequest)
		return
	}
	if reqBody.DaemonVer == "" {
		writeJSONError(w, "daemon_ver is required", http.StatusBadRequest)
		return
	}

	// Revoked-row-resurrection guard (ADR-031 §5 + ADR-028 "no resurrection").
	// If the daemon-submitted device_id maps to an existing revoked row, the
	// daemon is replaying a stale cached device_id. The UNIQUE(account_id,
	// device_id) constraint would otherwise either resurrect the revoked
	// credential (wrong) or trip a duplicate-key error. Clear DeviceID so
	// the ADR-028 first-pair path mints a fresh server-issued UUID — the
	// new row carries a new device_id, leaving the original revoked row
	// intact for audit.
	existing, err := h.repo.GetByAccountAndDevice(r.Context(), accountID, reqBody.DeviceID)
	if err != nil && err != repository.ErrDaemonAPIKeyNotFound {
		log.Printf("[daemon_register] GetByAccountAndDevice account=%s device=%s: %v", accountID, reqBody.DeviceID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if existing != nil && existing.RevokedAt != nil {
		// Mint a fresh server-issued UUIDv4 for the new row. ADR-028's
		// full first-pair-when-empty path lives in #2631; the minimal
		// inline mint here is what makes the resurrection guard
		// observably correct (the alternative would be UpsertKey failing
		// against the DB's NOT NULL constraint on device_id).
		reqBody.DeviceID = uuid.NewString()
	}

	// Already-paired, non-revoked device re-register guard (P0 fix):
	// If the (account_id, device_id) pair already has an active row,
	// return 200 + empty api_key so the daemon reuses its keychain copy.
	// Skipping bcrypt mint and UpsertKey prevents the UNIQUE(account_id,
	// device_id) violation (pg 23505) that previously caused HTTP 500.
	// Per daemon consumer contract (main.go:677-685): 200+empty api_key
	// means "already registered, reuse keychain copy".
	if existing != nil && existing.RevokedAt == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(daemonRegisterResponse{
			APIKey:    "",
			AccountID: accountID,
			DeviceID:  existing.DeviceID,
		}); err != nil {
			log.Printf("[daemon_register] encode (active re-register): %v", err)
		}
		return
	}

	// Generate a new candidate key: "sk_live_" + 32 random bytes hex-encoded.
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		log.Printf("[daemon_register] rand.Read: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	plaintextKey := daemonAPIKeyPrefix + hex.EncodeToString(rawBytes)
	keyPrefix := plaintextKey[:16]

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextKey), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[daemon_register] bcrypt: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// JIT-provision the users row for this Clerk identity so DaemonAPIKeyAuth
	// can resolve the key's account_id (Clerk user_id) back to users.id
	// (int64) on subsequent ingest calls. Skipped when userRepo is nil
	// (test-only path).
	var userCreatedAt time.Time
	if h.userRepo != nil {
		user, err := h.userRepo.UpsertByClerkUserID(r.Context(), accountID)
		if err != nil {
			log.Printf("[daemon_register] UpsertByClerkUserID account=%s: %v", accountID, err)
			writeJSONError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if user != nil {
			userCreatedAt = user.CreatedAt
		}
	}

	rec, created, err := h.repo.UpsertKey(r.Context(), accountID, string(hash), keyPrefix, reqBody.DeviceID, reqBody.Platform, reqBody.DaemonVer)
	if err != nil {
		// Defense-in-depth: if a concurrent first-pair race caused a pg 23505
		// unique-violation on (account_id, device_id), re-fetch the row the
		// concurrent winner just inserted and return 200+empty rather than 500.
		// This closes the TOCTOU gap between GetByAccountAndDevice and UpsertKey.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			refetched, fetchErr := h.repo.GetByAccountAndDevice(r.Context(), accountID, reqBody.DeviceID)
			if fetchErr == nil && refetched != nil && refetched.RevokedAt == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if encErr := json.NewEncoder(w).Encode(daemonRegisterResponse{
					APIKey:    "",
					AccountID: accountID,
					DeviceID:  refetched.DeviceID,
				}); encErr != nil {
					log.Printf("[daemon_register] encode (23505 re-fetch): %v", encErr)
				}
				return
			}
		}
		log.Printf("[daemon_register] UpsertKey account=%s: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// When returning an existing key the plaintext is not available — the daemon
	// must use its stored keychain value. Return empty api_key with 200.
	var responseKey string
	if created {
		responseKey = plaintextKey
	}

	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}

	// Emit PostHog daemon_paired event on first pairing (ADR-027 §3).
	if created {
		go func(acct, deviceID, platform, daemonVer string, signupTime time.Time) {
			var timeSinceSignup float64
			if !signupTime.IsZero() {
				timeSinceSignup = time.Since(signupTime).Seconds()
			}
			if err := h.postHog.Enqueue(posthog.Capture{
				DistinctId: hashAccountID(acct),
				Event:      "daemon_paired",
				Properties: posthog.NewProperties().
					Set("device_id", deviceID).
					Set("account_id_hash", hashAccountID(acct)).
					Set("platform", platform).
					Set("daemon_ver", daemonVer).
					Set("time_since_signup_seconds", timeSinceSignup),
			}); err != nil {
				log.Printf("[daemon_register] posthog enqueue: %v", err)
			}
		}(accountID, rec.DeviceID, reqBody.Platform, reqBody.DaemonVer, userCreatedAt)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Echo rec.DeviceID (the authoritative server-stored value, not reqBody.DeviceID)
	// per ADR-028 and ADR-034 §1. On 201: the newly-minted value; on 200: the
	// existing row's value. api_key is empty on 200 (daemon uses keychain copy).
	if err := json.NewEncoder(w).Encode(daemonRegisterResponse{
		APIKey:    responseKey,
		AccountID: accountID,
		DeviceID:  rec.DeviceID,
	}); err != nil {
		log.Printf("[daemon_register] encode: %v", err)
	}
}

// rateAllow checks and records a rate-limit call for accountID.
func (h *DaemonRegisterHandler) rateAllow(accountID string) bool {
	h.rateMu.Lock()
	entry, ok := h.rateByAcct[accountID]
	if !ok {
		entry = &rateEntry{}
		h.rateByAcct[accountID] = entry
	}
	h.rateMu.Unlock()
	return entry.allow()
}
