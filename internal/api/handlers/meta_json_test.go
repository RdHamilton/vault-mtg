package handlers

import (
	"encoding/json"
	"testing"
)

// TestIdentifyArchetypeRequest_JSONFieldNames verifies that the frontend's
// "cardIds" (camelCase) field name is correctly deserialized.
func TestIdentifyArchetypeRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"colors":["W","U"],"cardIds":[100,200,300],"format":"standard"}`

	var req IdentifyArchetypeRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(req.Colors) != 2 || req.Colors[0] != "W" || req.Colors[1] != "U" {
		t.Errorf("expected Colors=['W','U'], got %v", req.Colors)
	}
	if len(req.CardIDs) != 3 || req.CardIDs[0] != 100 {
		t.Errorf("expected CardIDs=[100,200,300], got %v", req.CardIDs)
	}
	if req.Format != "standard" {
		t.Errorf("expected Format='standard', got '%s'", req.Format)
	}
}

// TestIdentifyArchetypeRequest_OldSnakeCaseIgnored verifies that the old
// "card_ids" (snake_case) field name is no longer accepted.
func TestIdentifyArchetypeRequest_OldSnakeCaseIgnored(t *testing.T) {
	jsonBody := `{"colors":["R"],"card_ids":[100,200],"format":"standard"}`

	var req IdentifyArchetypeRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(req.CardIDs) != 0 {
		t.Errorf("expected CardIDs to be empty with old snake_case key, got %v", req.CardIDs)
	}
}
