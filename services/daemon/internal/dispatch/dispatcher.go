// Package dispatch handles encoding and posting contract.DaemonEvent payloads to the BFF.
package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
)

// ErrReauthRequired is returned by a Refresher when the token cannot be
// refreshed automatically and user interaction is required (e.g. keychain
// mode where re-authentication must be triggered via the tray icon). The
// dispatcher treats this sentinel as a hard stop: it breaks the retry loop
// immediately after the first attempt and propagates the error to the caller.
var ErrReauthRequired = errors.New("reauth required: user interaction needed")

const (
	maxAttempts = 3
	retryBase   = 500 * time.Millisecond
)

// Refresher is implemented by any component that can obtain a fresh daemon JWT.
// The dispatcher calls it when the BFF returns 401 before retrying the request.
type Refresher interface {
	Refresh(ctx context.Context) (newToken string, err error)
}

// Dispatcher POSTs DaemonEvents to the BFF ingest endpoint.
// It maintains a per-session monotonic sequence counter that is assigned to
// each event before dispatch (ADR-013).  The counter starts at 1 and resets
// to 0 when the Dispatcher is created (i.e. on daemon restart).
type Dispatcher struct {
	cloudAPIURL string
	ingestPath  string
	// apiKey is the current bearer token. Protected by apiKeyMu so that
	// SetToken (called from the re-auth goroutine in AC-3, #2135) and Token /
	// doSend (called from concurrent Send goroutines) do not race.
	apiKeyMu  sync.RWMutex
	apiKey    string
	client    *http.Client
	refresher Refresher
	// buffer is the optional ring buffer wired via WithBuffer. When non-nil,
	// SendOrBuffer enqueues pre-marshaled bytes after retry exhaustion rather
	// than returning an error to the caller.
	buffer *RingBuffer
	// onBFFFailure is an optional callback invoked once when SendOrBuffer
	// exhausts all retry attempts and buffers the event (terminal failure path
	// only). statusCode is the last HTTP status returned by the BFF, or 0 for
	// transport-level failures. The callback must NOT be invoked on intermediate
	// retry attempts or on context-cancellation buffering — only on the
	// "all attempts failed" branch. Set via WithOnBFFFailure; nil is safe.
	onBFFFailure func(statusCode int)
	// onBFFSuccess is an optional callback invoked when SendOrBuffer successfully
	// delivers an event to the BFF (HTTP 2xx). Called before draining the buffer.
	// Set via WithOnBFFSuccess; nil is safe.
	onBFFSuccess func()
	// seq is the per-session sequence counter.  Incremented atomically so
	// Send is safe for concurrent callers.  Reset to 0 on daemon restart
	// because the Dispatcher itself is recreated on restart.
	seq atomic.Uint64
}

// New creates a Dispatcher.
//
// cloudAPIURL: base URL of the cloud API / BFF, e.g. "https://api.example.com"
// ingestPath: path of the ingest endpoint, e.g. "/v1/ingest/events"
// apiKey: bearer token for Authorization header
func New(cloudAPIURL, ingestPath, apiKey string) *Dispatcher {
	return &Dispatcher{
		cloudAPIURL: cloudAPIURL,
		ingestPath:  ingestPath,
		apiKey:      apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// WithRefresher attaches a Refresher that will be called when the BFF returns 401.
// This enables automatic JWT re-registration without restarting the daemon.
func (d *Dispatcher) WithRefresher(r Refresher) *Dispatcher {
	d.refresher = r
	return d
}

// WithBuffer attaches a RingBuffer that SendOrBuffer will use to store
// pre-marshaled event bytes when all retry attempts are exhausted. The buffer
// is per-Dispatcher; concurrent callers share the same RingBuffer instance.
func (d *Dispatcher) WithBuffer(b *RingBuffer) *Dispatcher {
	d.buffer = b
	return d
}

// WithOnBFFFailure registers an optional callback that is invoked exactly once
// when SendOrBuffer exhausts all retry attempts and buffers the event. The
// callback receives the HTTP status code from the last BFF attempt (0 for
// transport-level failures). It is NOT called on intermediate retries or when
// buffering occurs due to context cancellation. Set to nil to disable.
func (d *Dispatcher) WithOnBFFFailure(cb func(statusCode int)) *Dispatcher {
	d.onBFFFailure = cb
	return d
}

// WithOnBFFSuccess registers an optional callback invoked when SendOrBuffer
// successfully delivers an event (HTTP 2xx). Called before buffer drain.
// Used by the service to reset the consecutive-failure counter.
func (d *Dispatcher) WithOnBFFSuccess(cb func()) *Dispatcher {
	d.onBFFSuccess = cb
	return d
}

// SetToken updates the bearer token used for subsequent requests.
// Safe to call concurrently with Send, SendOrBuffer, and Token.
// Called after successful re-registration or in-process re-auth (AC-3, #2135).
func (d *Dispatcher) SetToken(token string) {
	d.apiKeyMu.Lock()
	d.apiKey = token
	d.apiKeyMu.Unlock()
}

// Token returns the current bearer token. Safe to call concurrently.
// Used when building a transient dispatcher that needs the same credentials
// as the primary dispatcher.
func (d *Dispatcher) Token() string {
	d.apiKeyMu.RLock()
	defer d.apiKeyMu.RUnlock()
	return d.apiKey
}

// Send assigns the next per-session sequence number to the event, encodes it
// as JSON, and POSTs it to the BFF with up to 3 attempts.
// Retries on transport errors or non-2xx responses with 500ms * attempt backoff.
// On a 401 response, calls the Refresher (if set) to obtain a new token before
// the next retry.
func (d *Dispatcher) Send(ctx context.Context, event contract.DaemonEvent) error {
	// Assign per-session sequence (ADR-013).  Add(1) returns the new value, so
	// the first call yields 1 — matching the "starts at 1" requirement.
	event.Sequence = d.seq.Add(1)

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var statusCode int
		statusCode, lastErr = d.doSend(ctx, body)
		if lastErr == nil {
			log.Printf("[dispatch] event %q sent (session=%s)", event.Type, event.SessionID)
			return nil
		}
		// On 401, attempt to refresh the token before retrying.
		if statusCode == http.StatusUnauthorized && d.refresher != nil {
			log.Printf("[dispatch] 401 received; attempting token refresh")
			newToken, refreshErr := d.refresher.Refresh(ctx)
			if errors.Is(refreshErr, ErrReauthRequired) {
				log.Printf("[dispatch] reauth required; aborting retry loop")
				return ErrReauthRequired
			}
			if refreshErr != nil {
				log.Printf("[dispatch] token refresh failed: %v", refreshErr)
			} else {
				d.SetToken(newToken)
				log.Printf("[dispatch] token refreshed; retrying")
			}
		}
		if attempt < maxAttempts {
			backoff := retryBase * time.Duration(attempt)
			log.Printf("[dispatch] attempt %d/%d failed: %v; retrying in %s", attempt, maxAttempts, lastErr, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// SendOrBuffer behaves like Send but, when a buffer has been wired via
// WithBuffer, silently enqueues the pre-marshaled event bytes on retry
// exhaustion instead of returning an error.
//
// This satisfies ADR-013 Option C: the sequence number is stamped into the
// marshaled bytes at emission time (inside Send's seq.Add(1) call), so
// bytes stored in the buffer carry their original sequence and are replayed
// verbatim without re-numbering.
//
// When no buffer is attached, SendOrBuffer is identical to Send.
func (d *Dispatcher) SendOrBuffer(ctx context.Context, event contract.DaemonEvent) error {
	// Stamp sequence and marshal before calling doSend so the bytes are
	// ready to buffer if needed — same as Send's internal flow, but we need
	// the marshaled bytes to hand to the ring buffer on failure.
	event.Sequence = d.seq.Add(1)

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	var lastErr error
	var lastStatusCode int
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var statusCode int
		statusCode, lastErr = d.doSend(ctx, body)
		if lastErr == nil {
			log.Printf("[dispatch] event %q sent (session=%s)", event.Type, event.SessionID)
			// Notify the service of a confirmed BFF success before draining the
			// buffer. This allows the service to reset failure counters at the
			// earliest possible moment (before replay events potentially fail).
			if d.onBFFSuccess != nil {
				d.onBFFSuccess()
			}
			// AC-3: drain buffered events on first successful send (best-effort;
			// per ADR-013 amendment Q1/OQ-1 a failed drain item is logged and
			// discarded — no re-enqueue to avoid thundering-herd / livelock).
			if d.buffer != nil {
				for _, item := range d.buffer.Drain() {
					if _, drainErr := d.doSend(ctx, item); drainErr != nil {
						log.Printf("[dispatch] drain replay failed: %v", drainErr)
					}
				}
			}
			return nil
		}
		lastStatusCode = statusCode
		// On 401, attempt to refresh the token before retrying.
		if statusCode == http.StatusUnauthorized && d.refresher != nil {
			log.Printf("[dispatch] 401 received; attempting token refresh")
			newToken, refreshErr := d.refresher.Refresh(ctx)
			if errors.Is(refreshErr, ErrReauthRequired) {
				log.Printf("[dispatch] reauth required; aborting retry loop")
				if d.buffer != nil {
					d.buffer.Enqueue(body)
					log.Printf("[dispatch] reauth required; buffered event seq=%d", event.Sequence)
				}
				return ErrReauthRequired
			}
			if refreshErr != nil {
				log.Printf("[dispatch] token refresh failed: %v", refreshErr)
			} else {
				d.SetToken(newToken)
				log.Printf("[dispatch] token refreshed; retrying")
			}
		}
		if attempt < maxAttempts {
			backoff := retryBase * time.Duration(attempt)
			log.Printf("[dispatch] attempt %d/%d failed: %v; retrying in %s", attempt, maxAttempts, lastErr, backoff)
			select {
			case <-ctx.Done():
				if d.buffer != nil {
					d.buffer.Enqueue(body)
					log.Printf("[dispatch] context cancelled; buffered event seq=%d", event.Sequence)
				}
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	if d.buffer != nil {
		d.buffer.Enqueue(body)
		log.Printf("[dispatch] all %d attempts failed; buffered event seq=%d (dropped_total=%d)",
			maxAttempts, event.Sequence, d.buffer.Dropped())
		// Notify the caller that a terminal BFF failure occurred. The last
		// known HTTP status code is passed (0 for transport-level failures)
		// so the service can record it for the dispatch_degraded counter.
		// This fires ONLY on the "all retries exhausted" path — NOT on
		// intermediate retries and NOT on context-cancellation buffering.
		if d.onBFFFailure != nil {
			d.onBFFFailure(lastStatusCode)
		}
		return nil
	}
	return fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// doSend performs a single POST of body to the ingest endpoint.
// Returns the HTTP status code (0 on transport failure) and any error.
func (d *Dispatcher) doSend(ctx context.Context, body []byte) (int, error) {
	url := d.cloudAPIURL + d.ingestPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := d.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("BFF returned %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// BuildEvent constructs a contract.DaemonEvent from raw log entry data.
//
// eventType: semantic event type, e.g. "draft.pick"
// accountID: MTGA account ID
// sessionID: current monitoring session ID
// payload: any JSON-serialisable value
func BuildEvent(eventType, accountID, sessionID string, payload interface{}) (contract.DaemonEvent, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return contract.DaemonEvent{}, fmt.Errorf("marshal payload: %w", err)
	}
	return contract.DaemonEvent{
		Type:       eventType,
		AccountID:  accountID,
		SessionID:  sessionID,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(raw),
	}, nil
}
