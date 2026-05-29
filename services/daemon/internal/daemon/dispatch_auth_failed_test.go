package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/pkce"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// classifyPKCEError — unit tests (Cases 1–4)
// ---------------------------------------------------------------------------

// TestClassifyPKCEError_TokenExchangeFailed_WrappedDeep verifies that an error
// wrapping pkce.ErrTokenExchange at the source (pkce.Run) maps to
// "pkce_token_exchange_failed" even after additional wrapping layers added by
// runInProcessReauth and keychainRefresherAdapter. This is the regression guard
// for Fix C (#2172): errors.Is must unwrap through multiple fmt.Errorf levels.
func TestClassifyPKCEError_TokenExchangeFailed_WrappedDeep(t *testing.T) {
	// Simulate the exact wrap chain produced in production:
	//   pkce.Run wraps:       "pkce: token exchange: %w: %w" (ErrTokenExchange, origErr)
	//   runInProcessReauth:   "in-process reauth: pkce flow: %w"
	origErr := errors.New("token endpoint returned 400: {\"error\":\"invalid_grant\"}")
	innerWrap := fmt.Errorf("pkce: token exchange: %w: %w", pkce.ErrTokenExchange, origErr)
	runWrap := fmt.Errorf("in-process reauth: pkce flow: %w", innerWrap)

	// classifyPKCEError must detect ErrTokenExchange through three levels.
	assert.Equal(t, "pkce_token_exchange_failed", classifyPKCEError(runWrap),
		"classifyPKCEError must return pkce_token_exchange_failed when ErrTokenExchange is wrapped 3 levels deep")

	// Sanity: errors.Is resolves correctly (validates the wrap chain itself).
	assert.True(t, errors.Is(runWrap, pkce.ErrTokenExchange),
		"errors.Is must find pkce.ErrTokenExchange through 3 wrap levels")
}

// TestClassifyPKCEError_Cancelled verifies that bare context.Canceled maps to
// "pkce_cancelled".
func TestClassifyPKCEError_Cancelled(t *testing.T) {
	assert.Equal(t, "pkce_cancelled", classifyPKCEError(context.Canceled))
}

// TestClassifyPKCEError_WrappedCancelled verifies that a wrapped context.Canceled
// (as produced by fmt.Errorf("pkce flow: %w", context.Canceled)) maps to
// "pkce_cancelled". errors.Is traverses the chain.
func TestClassifyPKCEError_WrappedCancelled(t *testing.T) {
	wrapped := fmt.Errorf("pkce flow: %w", context.Canceled)
	assert.Equal(t, "pkce_cancelled", classifyPKCEError(wrapped))
}

// TestClassifyPKCEError_Timeout verifies that the wall-clock expiry error
// string returned by pkce.waitForCode maps to "pkce_timeout".
func TestClassifyPKCEError_Timeout(t *testing.T) {
	err := errors.New("pkce: timed out waiting for OAuth callback (5 min)")
	assert.Equal(t, "pkce_timeout", classifyPKCEError(err))
}

// TestClassifyPKCEError_OtherError verifies that any non-cancel error (e.g. a
// port-bind failure) maps to "pkce_timeout" as the safe default.
func TestClassifyPKCEError_OtherError(t *testing.T) {
	err := errors.New("could not bind on ports 51423 or 51424")
	assert.Equal(t, "pkce_timeout", classifyPKCEError(err))
}

// ---------------------------------------------------------------------------
// dispatchAuthFailed direct call — unit tests (Cases 5–6)
// ---------------------------------------------------------------------------

// captureAuthFailedPayload starts a test HTTP server that captures the first
// daemon.auth_failed event payload dispatched by svc.dispatchAuthFailed.
// Returns the parsed authFailedPayload fields as a map.
func captureAuthFailedPayload(t *testing.T, reason string) map[string]interface{} {
	t.Helper()

	var (
		mu       sync.Mutex
		captured map[string]interface{}
		gotEvent = make(chan struct{}, 1)
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var evt contract.DaemonEvent
		if json.Unmarshal(body, &evt) == nil && evt.Type == "daemon.auth_failed" {
			var p map[string]interface{}
			if json.Unmarshal(evt.Payload, &p) == nil {
				mu.Lock()
				if captured == nil {
					captured = p
					select {
					case gotEvent <- struct{}{}:
					default:
					}
				}
				mu.Unlock()
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		AccountID:   "test_account_123",
		Keychain:    true,
	}
	svc := New(cfg)

	svc.dispatchAuthFailed(context.Background(), reason)

	select {
	case <-gotEvent:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for daemon.auth_failed event to be received by test server")
	}

	mu.Lock()
	defer mu.Unlock()
	return captured
}

// TestDispatchAuthFailed_PKCETimeout_CallsDispatch verifies that calling
// dispatchAuthFailed with "pkce_timeout" sends a daemon.auth_failed event with
// reason="pkce_timeout" and no bff_status_code field (omitempty, zero value).
func TestDispatchAuthFailed_PKCETimeout_CallsDispatch(t *testing.T) {
	payload := captureAuthFailedPayload(t, "pkce_timeout")

	if payload["reason"] != "pkce_timeout" {
		t.Errorf("reason=%v, want pkce_timeout", payload["reason"])
	}
	if v, ok := payload["bff_status_code"]; ok && v != nil {
		t.Errorf("bff_status_code must be absent for pkce_timeout reason (omitempty), got %v", v)
	}
}

// TestDispatchAuthFailed_PKCECancelled_CallsDispatch verifies that calling
// dispatchAuthFailed with "pkce_cancelled" sends a daemon.auth_failed event with
// reason="pkce_cancelled" and no bff_status_code field (omitempty, zero value).
func TestDispatchAuthFailed_PKCECancelled_CallsDispatch(t *testing.T) {
	payload := captureAuthFailedPayload(t, "pkce_cancelled")

	if payload["reason"] != "pkce_cancelled" {
		t.Errorf("reason=%v, want pkce_cancelled", payload["reason"])
	}
	if v, ok := payload["bff_status_code"]; ok && v != nil {
		t.Errorf("bff_status_code must be absent for pkce_cancelled reason (omitempty), got %v", v)
	}
}

// ---------------------------------------------------------------------------
// keychainRefresherAdapter integration — Cases 7–8
//
// Ray's implementation note: the inner go s.dispatchAuthFailed goroutine runs
// inside the outer reauthInProgress goroutine. To deterministically join it we
// use a captured channel that the test HTTP server closes on receipt of the
// auth_failed event — no fixed sleep, no race on the recorder.
// ---------------------------------------------------------------------------

// captureAuthFailedViaRefresher wires a Service whose BFF first returns 401
// (triggering keychainRefresherAdapter), then accepts subsequent events. The
// reauthFunc is set to return reauthErr. Returns the parsed payload of the first
// daemon.auth_failed event whose reason matches wantReason.
//
// The 401 response causes two auth_failed events to be dispatched:
//  1. "bff_rejected" — from handleEntry (line ~1189 of service.go), fired immediately
//     when the BFF returns 401.
//  2. The PKCE-classified reason — from the keychainRefresherAdapter goroutine,
//     fired after reauthFunc returns an error.
//
// We filter on wantReason so the test doesn't accidentally capture the bff_rejected
// event that always precedes the PKCE event on this code path.
func captureAuthFailedViaRefresher(t *testing.T, reauthErr error, wantReason string) map[string]interface{} {
	t.Helper()

	var (
		mu           sync.Mutex
		reqCount     int
		capturedBody map[string]interface{}
		authFailedCh = make(chan struct{}, 1)
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		mu.Lock()
		reqCount++
		n := reqCount
		mu.Unlock()

		if n == 1 {
			// First request: return 401 to trigger the keychainRefresherAdapter.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var evt contract.DaemonEvent
		if json.Unmarshal(body, &evt) == nil && evt.Type == "daemon.auth_failed" {
			var p map[string]interface{}
			if json.Unmarshal(evt.Payload, &p) == nil && p["reason"] == wantReason {
				mu.Lock()
				if capturedBody == nil {
					capturedBody = p
					select {
					case authFailedCh <- struct{}{}:
					default:
					}
				}
				mu.Unlock()
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		CloudAPIURL: srv.URL,
		IngestPath:  "/v1/ingest/events",
		AccountID:   "acc_refresher_test",
		Keychain:    true,
	}
	svc := New(cfg)

	svc.trayHooks = TrayHooks{
		SetReauthRequired: func(string) {},
	}

	svc.WithReauthFunc(func(ctx context.Context) error {
		return reauthErr
	})

	// Trigger the BFF send (returns 401) → keychainRefresherAdapter fires.
	entry := &logreader.LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"draftPack": []interface{}{"card1"}},
	}

	if err := svc.handleEntry(context.Background(), entry); err != nil {
		t.Fatalf("handleEntry returned unexpected error: %v", err)
	}

	// Wait for the inner dispatchAuthFailed goroutine to deliver the event.
	select {
	case <-authFailedCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for daemon.auth_failed event from keychainRefresherAdapter goroutine")
	}

	mu.Lock()
	defer mu.Unlock()
	return capturedBody
}

// TestKeychainRefresherAdapter_PKCETimeout_EmitsAuthFailed verifies that when
// the in-process PKCE reauth returns a non-cancel error (wall-clock timeout),
// a daemon.auth_failed event with reason="pkce_timeout" is dispatched.
func TestKeychainRefresherAdapter_PKCETimeout_EmitsAuthFailed(t *testing.T) {
	reauthErr := fmt.Errorf("pkce flow: %w",
		errors.New("pkce: timed out waiting for OAuth callback (5 min)"))
	payload := captureAuthFailedViaRefresher(t, reauthErr, "pkce_timeout")

	if payload == nil {
		t.Fatal("no auth_failed payload captured")
	}
	if payload["reason"] != "pkce_timeout" {
		t.Errorf("reason=%v, want pkce_timeout", payload["reason"])
	}
	if v, ok := payload["bff_status_code"]; ok && v != nil {
		t.Errorf("bff_status_code must be absent for pkce_timeout reason, got %v", v)
	}
}

// TestKeychainRefresherAdapter_PKCECancelled_EmitsAuthFailed verifies that when
// the in-process PKCE reauth returns context.Canceled (user dismissed the
// browser), a daemon.auth_failed event with reason="pkce_cancelled" is dispatched.
func TestKeychainRefresherAdapter_PKCECancelled_EmitsAuthFailed(t *testing.T) {
	reauthErr := fmt.Errorf("pkce flow: %w", context.Canceled)
	payload := captureAuthFailedViaRefresher(t, reauthErr, "pkce_cancelled")

	if payload == nil {
		t.Fatal("no auth_failed payload captured")
	}
	if payload["reason"] != "pkce_cancelled" {
		t.Errorf("reason=%v, want pkce_cancelled", payload["reason"])
	}
	if v, ok := payload["bff_status_code"]; ok && v != nil {
		t.Errorf("bff_status_code must be absent for pkce_cancelled reason, got %v", v)
	}
}
