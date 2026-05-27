// Package dispatch handles encoding and posting contract.DaemonEvent payloads to the BFF.
package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
)

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
	apiKey      string
	client      *http.Client
	refresher   Refresher
	// buffer is the optional ring buffer wired via WithBuffer. When non-nil,
	// SendOrBuffer enqueues pre-marshaled bytes after retry exhaustion rather
	// than returning an error to the caller.
	buffer *RingBuffer
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

// SetToken updates the bearer token used for subsequent requests.
// Called after successful re-registration to swap in the new JWT.
func (d *Dispatcher) SetToken(token string) {
	d.apiKey = token
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
	if d.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
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
