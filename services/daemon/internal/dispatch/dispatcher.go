// Package dispatch handles encoding and posting contract.DaemonEvent payloads to the BFF.
package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
)

const (
	maxAttempts = 3
	retryBase   = 500 * time.Millisecond
)

// Dispatcher POSTs DaemonEvents to the BFF ingest endpoint.
type Dispatcher struct {
	cloudAPIURL string
	ingestPath  string
	apiKey      string
	client      *http.Client
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

// Send encodes event as JSON and POSTs it to the BFF with up to 3 attempts.
// Retries on transport errors or non-2xx responses with 500ms * attempt backoff.
func (d *Dispatcher) Send(ctx context.Context, event contract.DaemonEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = d.doSend(ctx, body)
		if lastErr == nil {
			log.Printf("[dispatch] event %q sent (session=%s)", event.Type, event.SessionID)
			return nil
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

// doSend performs a single POST of body to the ingest endpoint.
func (d *Dispatcher) doSend(ctx context.Context, body []byte) error {
	url := d.cloudAPIURL + d.ingestPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if d.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("post event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("BFF returned %d", resp.StatusCode)
	}

	return nil
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
