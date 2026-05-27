// Package handlers provides HTTP request handlers for the BFF service.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	bffmiddleware "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	contract "github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/posthog/posthog-go"
)

// EventBroadcaster is implemented by any type that can broadcast a daemon event
// to connected clients (e.g. an SSE broker).  userID scopes delivery to the
// authenticated user's SSE subscribers only — preventing cross-tenant leakage.
type EventBroadcaster interface {
	BroadcastDaemonEvent(userID int64, event contract.DaemonEvent)
}

// DaemonEventInserter is implemented by any type that can persist a daemon event
// to durable storage.  It is satisfied by *repository.DaemonEventsRepository.
type DaemonEventInserter interface {
	Insert(ctx context.Context, userID int64, accountID string, eventType string, payload json.RawMessage, occurredAt time.Time, eventID string, sequence uint64) error
}

// PostHogClient is a mockable interface for server-side PostHog event capture.
// It is satisfied by the real posthog.Client and by test doubles.
type PostHogClient interface {
	Enqueue(msg posthog.Message) error
}

// noopPostHogClient is a no-op PostHogClient used when POSTHOG_API_KEY is empty.
type noopPostHogClient struct{}

func (noopPostHogClient) Enqueue(posthog.Message) error { return nil }

// IngestHandler accepts daemon events posted by the daemon service and
// broadcasts them to connected frontend clients via the broadcaster.
// When a DaemonEventInserter is wired, each event is also persisted to the
// database before broadcasting.
type IngestHandler struct {
	broadcaster   EventBroadcaster
	repo          DaemonEventInserter
	gapDetector   *GapDetector
	postHogClient PostHogClient
}

// NewIngestHandler creates an IngestHandler that broadcasts received events
// through the provided broadcaster.  Pass nil for repo to run in
// broadcast-only mode (no persistence).
//
// A GapDetector is always initialised.  PostHog defaults to the no-op client
// until WithPostHogClient is called.
func NewIngestHandler(broadcaster EventBroadcaster) *IngestHandler {
	return &IngestHandler{
		broadcaster:   broadcaster,
		gapDetector:   &GapDetector{},
		postHogClient: noopPostHogClient{},
	}
}

// WithRepository returns a copy of h with repo wired for persistence.
// This enables optional dependency injection without changing the existing
// NewIngestHandler call-sites.
func (h *IngestHandler) WithRepository(repo DaemonEventInserter) *IngestHandler {
	return &IngestHandler{
		broadcaster:   h.broadcaster,
		repo:          repo,
		gapDetector:   h.gapDetector,
		postHogClient: h.postHogClient,
	}
}

// WithPostHogClient returns a copy of h with the given PostHog client wired.
// When not called, the handler uses a no-op client so the code path is always
// exercised without network calls.
func (h *IngestHandler) WithPostHogClient(client PostHogClient) *IngestHandler {
	return &IngestHandler{
		broadcaster:   h.broadcaster,
		repo:          h.repo,
		gapDetector:   h.gapDetector,
		postHogClient: client,
	}
}

// IngestEvent handles POST /v1/ingest/events.
// Authentication is enforced by APIKeyAuth middleware upstream.
// By the time this handler runs, UserIDFromContext is set on the request context.
func (h *IngestHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var event contract.DaemonEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if event.Type == "" {
		http.Error(w, "event type is required", http.StatusBadRequest)
		return
	}

	// Persist the event before broadcasting. A persistence failure is logged
	// but does not drop the live event — the broadcast still proceeds so the
	// frontend receives the event even when the database is degraded.
	if h.repo != nil {
		if err := h.repo.Insert(r.Context(), userID, event.AccountID, event.Type, event.Payload, event.OccurredAt, event.EventID, event.Sequence); err != nil {
			slog.Error(
				"[IngestHandler] ERROR persisting event",
				"type", event.Type,
				"userID", userID,
				"account_id_hash", hashAccountID(event.AccountID),
				"err", err,
			)
		}
	}

	// Gap detection: check for sequence discontinuities.
	// This never blocks or discards events — it is observability only.
	if event.Sequence > 0 {
		if isGap, expected := h.gapDetector.Check(event.AccountID, event.SessionID, event.Sequence); isGap {
			slog.Warn(
				"[IngestHandler] sequence gap detected",
				"account_id_hash", hashAccountID(event.AccountID),
				"session_id", event.SessionID,
				"expected_sequence", expected,
				"received_sequence", event.Sequence,
			)

			hashedAccountID := hashAccountID(event.AccountID)
			_ = h.postHogClient.Enqueue(posthog.Capture{
				DistinctId: hashedAccountID,
				Event:      "daemon_event_gap_detected",
				Properties: posthog.NewProperties().
					Set("account_id_hash", hashedAccountID).
					Set("session_id", event.SessionID).
					Set("expected_sequence", expected).
					Set("received_sequence", event.Sequence),
			})
		}
	}

	// Heartbeat drift detection: if parse_failure_count > 0, the daemon
	// encountered typed-parse failures in this window. Emit daemon.log_format_drift
	// to PostHog so Faye can detect MTGA log-format changes within one heartbeat
	// window (≤30 s). PostHog emission is BFF-only per ADR-027 §OQ-5.
	if event.Type == "daemon.heartbeat" {
		var hb struct {
			ParseFailureCount uint32   `json:"parse_failure_count"`
			SampleLineHash    string   `json:"sample_line_hash,omitempty"`
			FailedEventTypes  []string `json:"failed_event_types,omitempty"`
		}
		if err := json.Unmarshal(event.Payload, &hb); err == nil && hb.ParseFailureCount > 0 {
			hashedAccountID := hashAccountID(event.AccountID)
			_ = h.postHogClient.Enqueue(posthog.Capture{
				DistinctId: hashedAccountID,
				Event:      "daemon.log_format_drift",
				Properties: posthog.NewProperties().
					Set("account_id_hash", hashedAccountID).
					Set("parse_failure_count", hb.ParseFailureCount).
					Set("sample_line_hash", hb.SampleLineHash).
					Set("failed_event_types", hb.FailedEventTypes),
			})
		}
	}

	if h.broadcaster != nil {
		h.broadcaster.BroadcastDaemonEvent(userID, event)
	}

	slog.Info(
		"[IngestHandler] Received event",
		"type", event.Type,
		"seq", event.Sequence,
		"account_id_hash", hashAccountID(event.AccountID),
		"userID", userID,
	)

	w.WriteHeader(http.StatusAccepted)
}
