package daemon

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keychainTestService builds a minimal Service suitable for retryKeychain
// tests. It replaces keychainGet with the provided stub and sets keychainErr
// so retryKeychain is triggered.
func keychainTestService(getKey func() (string, error)) *Service {
	cfg := &config.Config{
		CloudAPIURL: "http://localhost",
		IngestPath:  "/v1/ingest/events",
		Keychain:    true,
	}
	svc := New(cfg)
	// Override the keychain getter and pre-set an error so retryKeychain runs.
	svc.keychainGet = getKey
	svc.keychainErr = errors.New("keychain locked")
	return svc
}

// TestRetryKeychain_SucceedsOnSecondAttempt verifies that retryKeychain
// returns nil when the keychain succeeds on the second attempt.
func TestRetryKeychain_SucceedsOnSecondAttempt(t *testing.T) {
	// Shorten backoff so the test completes in milliseconds.
	origBase := keychainRetryBase
	keychainRetryBase = 5 * time.Millisecond
	t.Cleanup(func() { keychainRetryBase = origBase })

	var calls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		n := calls.Add(1)
		if n == 1 {
			return "", errors.New("keychain still locked")
		}
		return "my-api-key", nil
	})

	keychainErrorSet := false
	keychainErrorCleared := false
	svc.trayHooks = TrayHooks{
		SetKeychainError: func(show bool) {
			if show {
				keychainErrorSet = true
			} else {
				keychainErrorCleared = true
			}
		},
	}

	ctx := context.Background()
	err := svc.retryKeychain(ctx)

	require.NoError(t, err, "expected success on second attempt")
	assert.Equal(t, int32(2), calls.Load(), "expected exactly 2 keychainGet calls")
	assert.Nil(t, svc.keychainErr, "keychainErr must be cleared on success")
	assert.True(t, keychainErrorSet, "tray must be put into keychain-error state")
	assert.True(t, keychainErrorCleared, "tray must be cleared after success")
}

// TestRetryKeychain_ExhaustsRetries verifies that retryKeychain returns a
// non-nil error when all attempts fail.
func TestRetryKeychain_ExhaustsRetries(t *testing.T) {
	origBase := keychainRetryBase
	origMax := keychainMaxRetries
	keychainRetryBase = 5 * time.Millisecond
	keychainMaxRetries = 3
	t.Cleanup(func() {
		keychainRetryBase = origBase
		keychainMaxRetries = origMax
	})

	var calls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		calls.Add(1)
		return "", errors.New("keychain unavailable")
	})

	ctx := context.Background()
	err := svc.retryKeychain(ctx)

	require.Error(t, err, "expected error after exhausting all retries")
	assert.Equal(t, int32(keychainMaxRetries), calls.Load(),
		"keychainGet must be called exactly maxRetries times")
	assert.Contains(t, err.Error(), "3 retries")
}

// TestRetryKeychain_TryAgainSkipsBackoff verifies that a signal on TryAgain
// causes an immediate retry without waiting for the backoff timer.
func TestRetryKeychain_TryAgainSkipsBackoff(t *testing.T) {
	// Use a very long backoff so the test would hang if TryAgain doesn't fire.
	origBase := keychainRetryBase
	origMax := keychainMaxRetries
	keychainRetryBase = 10 * time.Second
	keychainMaxRetries = 3
	t.Cleanup(func() {
		keychainRetryBase = origBase
		keychainMaxRetries = origMax
	})

	var calls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		n := calls.Add(1)
		if n < 2 {
			return "", errors.New("locked")
		}
		return "my-api-key", nil
	})

	// Pre-load TryAgain with 2 signals so both attempts fire immediately
	// (attempt 1 fails → attempt 2 fires TryAgain → succeeds).
	tryAgainCh := make(chan struct{}, 2)
	tryAgainCh <- struct{}{}
	tryAgainCh <- struct{}{}
	svc.trayHooks = TrayHooks{
		TryAgain: tryAgainCh,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := svc.retryKeychain(ctx)

	require.NoError(t, err, "TryAgain signal must cause immediate retry and succeed")
	assert.Equal(t, int32(2), calls.Load(), "expected 2 keychainGet calls (fail + succeed)")
}

// TestRetryKeychain_ContextCancelledExitsEarly verifies that retryKeychain
// returns the context error when the context is cancelled mid-wait.
func TestRetryKeychain_ContextCancelledExitsEarly(t *testing.T) {
	origBase := keychainRetryBase
	keychainRetryBase = 10 * time.Second // long enough that only cancel fires
	t.Cleanup(func() { keychainRetryBase = origBase })

	var calls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		calls.Add(1)
		return "", errors.New("still locked")
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately — the first backoff select should pick ctx.Done().
	cancel()

	err := svc.retryKeychain(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(0), calls.Load(), "no keychainGet call expected when context cancelled before backoff")
}

// ---------------------------------------------------------------------------
// #2136: Keychain unavailable handling — graceful degradation
// ---------------------------------------------------------------------------

// TestRetryKeychain_ErrNotFound_ShortCircuits verifies that when s.keychainErr
// is keychain.ErrNotFound, retryKeychain:
//   - Returns ErrSetupRequired immediately (no loop entered)
//   - Does NOT call SetKeychainError (tray must not enter error state)
//   - Clears s.keychainErr (so computeAuthStatus → "setup_required", not "keychain_error")
//   - Makes zero calls to the keychainGet function
func TestRetryKeychain_ErrNotFound_ShortCircuits(t *testing.T) {
	origBase := keychainRetryBase
	keychainRetryBase = 10 * time.Second // would hang if loop is entered
	t.Cleanup(func() { keychainRetryBase = origBase })

	var getCalls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		getCalls.Add(1)
		return "", errors.New("unexpected keychainGet call — should have short-circuited")
	})

	// Override the pre-set error with the permanent ErrNotFound sentinel.
	svc.keychainErr = keychain.ErrNotFound

	keychainErrorSet := false
	svc.trayHooks = TrayHooks{
		SetKeychainError: func(show bool) {
			if show {
				keychainErrorSet = true
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := svc.retryKeychain(ctx)

	require.ErrorIs(t, err, ErrSetupRequired, "must return ErrSetupRequired for ErrNotFound")
	assert.Equal(t, int32(0), getCalls.Load(), "keychainGet must not be called for ErrNotFound")
	assert.False(t, keychainErrorSet, "SetKeychainError(true) must not be called for ErrNotFound")
	assert.Nil(t, svc.keychainErr, "keychainErr must be cleared so computeAuthStatus → setup_required")
}

// TestRetryKeychain_TransientError_FullRetries verifies that a transient
// (non-ErrNotFound) keychain error exhausts all keychainMaxRetries attempts.
// Uses a shortened keychainRetryBase to keep the test fast.
func TestRetryKeychain_TransientError_FullRetries(t *testing.T) {
	origBase := keychainRetryBase
	origMax := keychainMaxRetries
	keychainRetryBase = 5 * time.Millisecond
	keychainMaxRetries = 3
	t.Cleanup(func() {
		keychainRetryBase = origBase
		keychainMaxRetries = origMax
	})

	var getCalls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		getCalls.Add(1)
		return "", errors.New("keychain transient OS error")
	})

	ctx := context.Background()
	err := svc.retryKeychain(ctx)

	require.Error(t, err, "expected error after exhausting all retries")
	assert.NotErrorIs(t, err, ErrSetupRequired, "transient error must not return ErrSetupRequired")
	assert.Equal(t, int32(keychainMaxRetries), getCalls.Load(),
		"keychainGet must be called exactly keychainMaxRetries times for transient errors")
}

// TestRetryKeychain_TransientThenSuccess_ClearsErr verifies that when the
// keychain fails transiently then succeeds, s.keychainErr is cleared to nil
// on the successful attempt.
func TestRetryKeychain_TransientThenSuccess_ClearsErr(t *testing.T) {
	origBase := keychainRetryBase
	keychainRetryBase = 5 * time.Millisecond
	t.Cleanup(func() { keychainRetryBase = origBase })

	var getCalls atomic.Int32
	svc := keychainTestService(func() (string, error) {
		n := getCalls.Add(1)
		if n < 3 {
			return "", errors.New("transient lock")
		}
		return "my-api-key", nil
	})

	ctx := context.Background()
	err := svc.retryKeychain(ctx)

	require.NoError(t, err, "expected success on third attempt")
	assert.Nil(t, svc.keychainErr, "keychainErr must be cleared after successful retry")
	assert.Equal(t, int32(3), getCalls.Load(), "expected 3 keychainGet calls (fail, fail, succeed)")
}
