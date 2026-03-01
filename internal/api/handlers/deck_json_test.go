package handlers

import (
	"encoding/json"
	"testing"
)

// TestBuildAroundSeedRequest_JSONFieldNames verifies that the frontend's snake_case
// field names are correctly deserialized into the Go struct.
func TestBuildAroundSeedRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{
		"seed_card_id": 12345,
		"max_results": 20,
		"budget_mode": true,
		"set_restriction": "multiple",
		"allowed_sets": ["ECL", "FDN"]
	}`

	var req BuildAroundSeedRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SeedCardID != 12345 {
		t.Errorf("expected SeedCardID=12345, got %d", req.SeedCardID)
	}
	if req.MaxResults != 20 {
		t.Errorf("expected MaxResults=20, got %d", req.MaxResults)
	}
	if !req.BudgetMode {
		t.Error("expected BudgetMode=true, got false")
	}
	if req.SetRestriction != "multiple" {
		t.Errorf("expected SetRestriction='multiple', got '%s'", req.SetRestriction)
	}
	if len(req.AllowedSets) != 2 || req.AllowedSets[0] != "ECL" || req.AllowedSets[1] != "FDN" {
		t.Errorf("expected AllowedSets=['ECL','FDN'], got %v", req.AllowedSets)
	}
}

// TestIterativeBuildAroundRequest_JSONFieldNames verifies snake_case deserialization.
func TestIterativeBuildAroundRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{
		"seed_card_id": 99999,
		"deck_card_ids": [100, 200, 300],
		"max_results": 10,
		"budget_mode": false,
		"set_restriction": "single",
		"allowed_sets": ["ECL"]
	}`

	var req IterativeBuildAroundRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SeedCardID != 99999 {
		t.Errorf("expected SeedCardID=99999, got %d", req.SeedCardID)
	}
	if len(req.DeckCardIDs) != 3 || req.DeckCardIDs[0] != 100 {
		t.Errorf("expected DeckCardIDs=[100,200,300], got %v", req.DeckCardIDs)
	}
	if req.MaxResults != 10 {
		t.Errorf("expected MaxResults=10, got %d", req.MaxResults)
	}
	if req.BudgetMode {
		t.Error("expected BudgetMode=false, got true")
	}
	if req.SetRestriction != "single" {
		t.Errorf("expected SetRestriction='single', got '%s'", req.SetRestriction)
	}
}

// TestGenerateCompleteDeckRequest_JSONFieldNames verifies snake_case deserialization.
func TestGenerateCompleteDeckRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{
		"seed_card_id": 55555,
		"archetype": "aggro",
		"budget_mode": true,
		"set_restriction": "all",
		"allowed_sets": []
	}`

	var req GenerateCompleteDeckRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SeedCardID != 55555 {
		t.Errorf("expected SeedCardID=55555, got %d", req.SeedCardID)
	}
	if req.Archetype != "aggro" {
		t.Errorf("expected Archetype='aggro', got '%s'", req.Archetype)
	}
	if !req.BudgetMode {
		t.Error("expected BudgetMode=true, got false")
	}
	if req.SetRestriction != "all" {
		t.Errorf("expected SetRestriction='all', got '%s'", req.SetRestriction)
	}
}

// TestCloneDeckRequest_JSONFieldNames verifies that "name" field is accepted.
func TestCloneDeckRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"name":"My Cloned Deck"}`

	var req CloneDeckRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.NewName != "My Cloned Deck" {
		t.Errorf("expected NewName='My Cloned Deck', got '%s'", req.NewName)
	}
}

// TestExportSuggestedDeckRequest_JSONFieldNames verifies snake_case deserialization.
func TestExportSuggestedDeckRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"suggestion":null,"deck_name":"Test Export Deck"}`

	var req ExportSuggestedDeckRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.DeckName != "Test Export Deck" {
		t.Errorf("expected DeckName='Test Export Deck', got '%s'", req.DeckName)
	}
}

// TestDeckRequests_OldCamelCaseIgnored verifies that old camelCase field names
// are no longer accepted (they should result in zero-value fields).
func TestDeckRequests_OldCamelCaseIgnored(t *testing.T) {
	// Old format: camelCase field names should NOT work anymore
	jsonBody := `{"seedCardID":12345,"maxResults":20,"budgetMode":true}`

	var req BuildAroundSeedRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// All fields should be zero values since the old field names don't match
	if req.SeedCardID != 0 {
		t.Errorf("expected SeedCardID to be 0 with old camelCase key, got %d", req.SeedCardID)
	}
	if req.MaxResults != 0 {
		t.Errorf("expected MaxResults to be 0 with old camelCase key, got %d", req.MaxResults)
	}
	if req.BudgetMode {
		t.Error("expected BudgetMode to be false with old camelCase key, got true")
	}
}
