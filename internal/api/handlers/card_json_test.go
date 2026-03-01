package handlers

import (
	"encoding/json"
	"testing"
)

// TestFetchRatingsRequest_JSONFieldNames verifies that the frontend's "format"
// field name is correctly deserialized into the Go struct.
func TestFetchRatingsRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"format":"PremierDraft"}`

	var req FetchRatingsRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.DraftFormat != "PremierDraft" {
		t.Errorf("expected DraftFormat='PremierDraft', got '%s'", req.DraftFormat)
	}
}

// TestFetchRatingsRequest_OldCamelCaseIgnored verifies the old "draftFormat"
// field name is no longer accepted.
func TestFetchRatingsRequest_OldCamelCaseIgnored(t *testing.T) {
	jsonBody := `{"draftFormat":"PremierDraft"}`

	var req FetchRatingsRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.DraftFormat != "" {
		t.Errorf("expected DraftFormat to be empty with old camelCase key, got '%s'", req.DraftFormat)
	}
}
