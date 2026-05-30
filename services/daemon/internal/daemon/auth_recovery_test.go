package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/ratingsclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// PropagateKeychainToken tests (#275)
//
// Fix A (Faults 1 & 2): after a Retry-Setup PKCE success, the new api_key
// stored in the OS keychain must be wired into the long-lived dispatcher and
// ratings client before Run() starts. Without PropagateKeychainToken(), Run()
// launches with an empty bearer token and every ingest call returns 401.
// ---------------------------------------------------------------------------

// TestPropagateKeychainToken_Success verifies the happy path:
//   - keychainGet returns a non-empty key with nil error
//   - dispatcher.SetToken is called with the key (verified via dispatcher.Token())
//   - ratings.SetToken is called (verified by capturing the next request's
//     Authorization header via a stub ratings server)
//   - keychainErr is cleared to nil
func TestPropagateKeychainToken_Success(t *testing.T) {
	const wantToken = "api-key-from-keychain"

	// Ratings stub: capture Authorization header so we can verify SetToken.
	var capturedAuth string
	ratingsStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNotFound) // rating fetch fails — we only care about the header
	}))
	defer ratingsStub.Close()

	cfg := &config.Config{
		CloudAPIURL: ratingsStub.URL,
		IngestPath:  "/v1/ingest/events",
		Keychain:    true,
		AccountID:   "acc-propagate",
	}
	svc := New(cfg)

	// Pre-set a keychainErr to simulate a previous startup failure.
	svc.setKeychainErr(errors.New("keychain locked at startup"))

	// Override keychainGet with a stub that returns the expected key.
	svc.keychainGet = func() (string, error) {
		return wantToken, nil
	}

	// Rebuild the ratings client pointing at the stub so we can observe SetToken.
	svc.ratings = ratingsclient.New(ratingsclient.Config{
		BFFURL: ratingsStub.URL,
		Token:  "",
	})

	err := svc.PropagateKeychainToken()
	require.NoError(t, err, "PropagateKeychainToken must succeed when keychain returns a valid key")

	// Dispatcher must have the new token.
	assert.Equal(t, wantToken, svc.dispatcher.Token(),
		"dispatcher token must be updated by PropagateKeychainToken")

	// keychainErr must be cleared.
	assert.Nil(t, svc.getKeychainErr(),
		"keychainErr must be nil after successful PropagateKeychainToken")

	// Verify ratings client uses the new token by triggering a Warm() fetch
	// and inspecting the Authorization header captured by the stub.
	_ = svc.ratings.Warm(context.Background(), "BLB", "PremierDraft")
	assert.Equal(t, "Bearer "+wantToken, capturedAuth,
		"ratings client must send the new token after PropagateKeychainToken")
}

// TestPropagateKeychainToken_NilRatings verifies A1 guard:
// PropagateKeychainToken must not panic when s.ratings is nil.
// New() does not guarantee ratings is non-nil in all test configurations
// (it is currently always set, but the guard makes the invariant explicit).
func TestPropagateKeychainToken_NilRatings(t *testing.T) {
	cfg := &config.Config{
		CloudAPIURL: "http://localhost",
		IngestPath:  "/v1/ingest/events",
		Keychain:    true,
	}
	svc := New(cfg)
	svc.ratings = nil // force nil to exercise the A1 guard

	svc.keychainGet = func() (string, error) {
		return "my-api-key", nil
	}

	// Must not panic even with nil ratings.
	require.NotPanics(t, func() {
		err := svc.PropagateKeychainToken()
		assert.NoError(t, err)
	}, "PropagateKeychainToken must not panic when s.ratings is nil (A1 guard)")

	// Dispatcher must still be updated.
	assert.Equal(t, "my-api-key", svc.dispatcher.Token(),
		"dispatcher token must be set even when ratings is nil")
}

// TestPropagateKeychainToken_EmptyKey verifies A2 guard:
// an empty key returned with a nil error must be treated as an error —
// the empty string must NOT be silently written to the dispatcher as the bearer
// token. keychainErr must be set to the returned error.
func TestPropagateKeychainToken_EmptyKey(t *testing.T) {
	cfg := &config.Config{
		CloudAPIURL: "http://localhost",
		IngestPath:  "/v1/ingest/events",
		Keychain:    true,
	}
	svc := New(cfg)

	// Manually set a known token so we can verify it is NOT overwritten.
	const preCallToken = "token-before-propagate"
	svc.dispatcher.SetToken(preCallToken)

	// keychainGet returns an empty key with no error — same guard as retryKeychain.
	svc.keychainGet = func() (string, error) {
		return "", nil
	}

	err := svc.PropagateKeychainToken()
	require.Error(t, err, "PropagateKeychainToken must return an error when keychain returns empty key (A2 guard)")

	// keychainErr must be set so computeAuthStatus routes to keychain_error.
	assert.NotNil(t, svc.getKeychainErr(),
		"keychainErr must be set when PropagateKeychainToken fails (A2 guard)")

	// Dispatcher must NOT have been overwritten by the empty key.
	// The pre-call token must be retained unchanged.
	assert.Equal(t, preCallToken, svc.dispatcher.Token(),
		"dispatcher token must not be overwritten when keychain returns empty key")
}

// TestPropagateKeychainToken_FailureThenRetryKeychainRecovery verifies T1:
//
//	PropagateKeychainToken fails → keychainErr is set → Run() phase-0
//	detects keychainErr != nil → retryKeychain fallback runs and succeeds.
//
// This closes the AC#4 error-branch coverage gap: the daemon must recover
// fully (reach the event loop) even when PropagateKeychainToken returns an
// error, as long as retryKeychain can subsequently obtain the key.
func TestPropagateKeychainToken_FailureThenRetryKeychainRecovery(t *testing.T) {
	// Shorten keychain retry backoff so the test completes quickly.
	origBase := keychainRetryBase
	origMax := keychainMaxRetries
	keychainRetryBase = 5 * time.Millisecond
	keychainMaxRetries = 3
	t.Cleanup(func() {
		keychainRetryBase = origBase
		keychainMaxRetries = origMax
	})

	// BFF stub: accept all ingest events.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		Keychain:    true,
		AccountID:   "acc-recovery",
		LogPath:     "/dev/null",
	}
	svc := New(cfg)

	// Step 1: PropagateKeychainToken fails with empty key (A2).
	svc.keychainGet = func() (string, error) {
		return "", nil
	}
	propErr := svc.PropagateKeychainToken()
	require.Error(t, propErr, "PropagateKeychainToken must return an error for empty key")

	// Confirm keychainErr is now set — Run() phase-0 will detect this.
	require.NotNil(t, svc.getKeychainErr(),
		"keychainErr must be set after PropagateKeychainToken failure")

	// Step 2: swap keychainGet so retryKeychain can succeed on the first attempt.
	svc.keychainGet = func() (string, error) {
		return "recovered-api-key", nil
	}

	// Step 3: Run() — phase-0 detects keychainErr != nil, calls retryKeychain.
	// retryKeychain succeeds → clears keychainErr → event loop starts → Run
	// runs until ctx is cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	runErr := svc.Run(ctx)

	// Run() must return nil (context cancelled = clean exit, not a fatal error).
	// If retryKeychain failed, Run would return a non-nil error wrapping
	// "keychain unavailable after retries".
	assert.NoError(t, runErr,
		"Run must exit cleanly after retryKeychain recovers from PropagateKeychainToken failure")

	// After Run exits, keychainErr must be nil (cleared by retryKeychain).
	assert.Nil(t, svc.getKeychainErr(),
		"keychainErr must be nil after retryKeychain succeeds inside Run")
}
