package contract_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
)

// TestDaemonEventRoundTrip verifies that a DaemonEvent can be marshaled and
// unmarshaled without data loss or type assertions.
func TestDaemonEventRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	payload := contract.SyncRatingsPayload{
		SetCode:      "BLB",
		CardsUpdated: 42,
		Source:       "17lands",
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	original := contract.DaemonEvent{
		Type:       "sync:ratings",
		AccountID:  "acct_abc123",
		SessionID:  "sess_xyz789",
		OccurredAt: now,
		Payload:    rawPayload,
	}

	// Serialize.
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal DaemonEvent: %v", err)
	}

	// Deserialize.
	var decoded contract.DaemonEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal DaemonEvent: %v", err)
	}

	// Validate envelope fields.
	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.AccountID != original.AccountID {
		t.Errorf("AccountID: got %q, want %q", decoded.AccountID, original.AccountID)
	}
	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID: got %q, want %q", decoded.SessionID, original.SessionID)
	}
	if !decoded.OccurredAt.Equal(original.OccurredAt) {
		t.Errorf("OccurredAt: got %v, want %v", decoded.OccurredAt, original.OccurredAt)
	}

	// Validate payload can be decoded without reflection or type assertions.
	var decodedPayload contract.SyncRatingsPayload
	if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
		t.Fatalf("unmarshal SyncRatingsPayload: %v", err)
	}
	if decodedPayload.SetCode != payload.SetCode {
		t.Errorf("payload.SetCode: got %q, want %q", decodedPayload.SetCode, payload.SetCode)
	}
	if decodedPayload.CardsUpdated != payload.CardsUpdated {
		t.Errorf("payload.CardsUpdated: got %d, want %d", decodedPayload.CardsUpdated, payload.CardsUpdated)
	}
	if decodedPayload.Source != payload.Source {
		t.Errorf("payload.Source: got %q, want %q", decodedPayload.Source, payload.Source)
	}
}

// TestSyncCardMetadataPayloadRoundTrip validates SyncCardMetadataPayload.
func TestSyncCardMetadataPayloadRoundTrip(t *testing.T) {
	payload := contract.SyncCardMetadataPayload{
		SetCode:      "DSK",
		CardsAdded:   100,
		CardsUpdated: 5,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded contract.SyncCardMetadataPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SetCode != payload.SetCode {
		t.Errorf("SetCode: got %q, want %q", decoded.SetCode, payload.SetCode)
	}
	if decoded.CardsAdded != payload.CardsAdded {
		t.Errorf("CardsAdded: got %d, want %d", decoded.CardsAdded, payload.CardsAdded)
	}
	if decoded.CardsUpdated != payload.CardsUpdated {
		t.Errorf("CardsUpdated: got %d, want %d", decoded.CardsUpdated, payload.CardsUpdated)
	}
}

// TestDaemonEventRoundTripDraftPayload validates nesting DraftEventPayload.
func TestDaemonEventRoundTripDraftPayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	inner := contract.DraftEventPayload{
		DraftID:    "draft_001",
		SetCode:    "BLB",
		PackNumber: 2,
		PickNumber: 7,
	}

	rawInner, err := json.Marshal(inner)
	if err != nil {
		t.Fatalf("marshal inner: %v", err)
	}

	event := contract.DaemonEvent{
		Type:       "draft:pick",
		AccountID:  "acct_draft",
		SessionID:  "sess_draft",
		OccurredAt: now,
		Payload:    rawInner,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var decoded contract.DaemonEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	var decodedInner contract.DraftEventPayload
	if err := json.Unmarshal(decoded.Payload, &decodedInner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}

	if decodedInner.DraftID != inner.DraftID {
		t.Errorf("DraftID: got %q, want %q", decodedInner.DraftID, inner.DraftID)
	}
	if decodedInner.PackNumber != inner.PackNumber {
		t.Errorf("PackNumber: got %d, want %d", decodedInner.PackNumber, inner.PackNumber)
	}
	if decodedInner.PickNumber != inner.PickNumber {
		t.Errorf("PickNumber: got %d, want %d", decodedInner.PickNumber, inner.PickNumber)
	}
}

// TestMatchEventPayloadRoundTrip validates MatchEventPayload.
func TestMatchEventPayloadRoundTrip(t *testing.T) {
	payload := contract.MatchEventPayload{
		MatchID:      "match_xyz",
		Format:       "Draft",
		OpponentName: "Opponent",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded contract.MatchEventPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.MatchID != payload.MatchID {
		t.Errorf("MatchID: got %q, want %q", decoded.MatchID, payload.MatchID)
	}
	if decoded.Format != payload.Format {
		t.Errorf("Format: got %q, want %q", decoded.Format, payload.Format)
	}
	if decoded.OpponentName != payload.OpponentName {
		t.Errorf("OpponentName: got %q, want %q", decoded.OpponentName, payload.OpponentName)
	}
}
