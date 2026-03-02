package handlers

import (
	"encoding/json"
	"testing"
)

// TestDraftFilterRequest_JSONFieldNames verifies that the frontend's snake_case
// field names are correctly deserialized into the Go struct.
func TestDraftFilterRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"set_code":"ECL","format":"PremierDraft","status":"active","limit":10}`

	var req DraftFilterRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SetCode == nil || *req.SetCode != "ECL" {
		t.Errorf("expected SetCode='ECL', got %v", req.SetCode)
	}
	if req.DraftType == nil || *req.DraftType != "PremierDraft" {
		t.Errorf("expected DraftType='PremierDraft', got %v", req.DraftType)
	}
	if req.Status == nil || *req.Status != "active" {
		t.Errorf("expected Status='active', got %v", req.Status)
	}
	if req.Limit != 10 {
		t.Errorf("expected Limit=10, got %d", req.Limit)
	}
}

// TestDraftStatsRequest_JSONFieldNames verifies snake_case deserialization.
func TestDraftStatsRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"set_code":"FDN","format":"QuickDraft"}`

	var req DraftStatsRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SetCode == nil || *req.SetCode != "FDN" {
		t.Errorf("expected SetCode='FDN', got %v", req.SetCode)
	}
	if req.DraftType == nil || *req.DraftType != "QuickDraft" {
		t.Errorf("expected DraftType='QuickDraft', got %v", req.DraftType)
	}
}

// TestGradePickRequest_JSONFieldNames verifies snake_case deserialization.
func TestGradePickRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"session_id":"draft-abc-123","pack_number":2,"pick_number":5}`

	var req GradePickRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SessionID != "draft-abc-123" {
		t.Errorf("expected SessionID='draft-abc-123', got '%s'", req.SessionID)
	}
	if req.PackNumber != 2 {
		t.Errorf("expected PackNumber=2, got %d", req.PackNumber)
	}
	if req.PickNumber != 5 {
		t.Errorf("expected PickNumber=5, got %d", req.PickNumber)
	}
}

// TestDraftInsightsRequest_JSONFieldNames verifies snake_case deserialization.
func TestDraftInsightsRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"set_code":"ECL","format":"PremierDraft"}`

	var req DraftInsightsRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SetCode != "ECL" {
		t.Errorf("expected SetCode='ECL', got '%s'", req.SetCode)
	}
	if req.DraftFormat != "PremierDraft" {
		t.Errorf("expected DraftFormat='PremierDraft', got '%s'", req.DraftFormat)
	}
}

// TestWinProbabilityRequest_JSONFieldNames verifies snake_case deserialization.
func TestWinProbabilityRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"session_id":"draft-win-test"}`

	var req WinProbabilityRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SessionID != "draft-win-test" {
		t.Errorf("expected SessionID='draft-win-test', got '%s'", req.SessionID)
	}
}

// TestTemporalTrendsRequest_JSONFieldNames verifies snake_case deserialization.
func TestTemporalTrendsRequest_JSONFieldNames(t *testing.T) {
	setCode := "ECL"
	jsonBody := `{"period_type":"weekly","num_periods":8,"set_code":"ECL"}`

	var req TemporalTrendsRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.PeriodType != "weekly" {
		t.Errorf("expected PeriodType='weekly', got '%s'", req.PeriodType)
	}
	if req.NumPeriods != 8 {
		t.Errorf("expected NumPeriods=8, got %d", req.NumPeriods)
	}
	if req.SetCode == nil || *req.SetCode != setCode {
		t.Errorf("expected SetCode='%s', got %v", setCode, req.SetCode)
	}
}

// TestCommunityComparisonRequest_JSONFieldNames verifies snake_case deserialization.
func TestCommunityComparisonRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"set_code":"ECL","draft_format":"PremierDraft"}`

	var req CommunityComparisonRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SetCode != "ECL" {
		t.Errorf("expected SetCode='ECL', got '%s'", req.SetCode)
	}
	if req.DraftFormat != "PremierDraft" {
		t.Errorf("expected DraftFormat='PremierDraft', got '%s'", req.DraftFormat)
	}
}

// TestRecalculateSetGradesRequest_JSONFieldNames verifies snake_case deserialization.
func TestRecalculateSetGradesRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"set_code":"ECL"}`

	var req RecalculateSetGradesRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SetCode != "ECL" {
		t.Errorf("expected SetCode='ECL', got '%s'", req.SetCode)
	}
}

// TestArchetypeCardsRequest_JSONFieldNames verifies snake_case deserialization.
func TestArchetypeCardsRequest_JSONFieldNames(t *testing.T) {
	jsonBody := `{"set_code":"ECL","draft_format":"PremierDraft","colors":"WU"}`

	var req ArchetypeCardsRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.SetCode != "ECL" {
		t.Errorf("expected SetCode='ECL', got '%s'", req.SetCode)
	}
	if req.DraftFormat != "PremierDraft" {
		t.Errorf("expected DraftFormat='PremierDraft', got '%s'", req.DraftFormat)
	}
	if req.Colors != "WU" {
		t.Errorf("expected Colors='WU', got '%s'", req.Colors)
	}
}

// TestDraftRequests_OldCamelCaseIgnored verifies that the old camelCase field names
// are no longer accepted (they should result in zero-value fields).
func TestDraftRequests_OldCamelCaseIgnored(t *testing.T) {
	// Old format: camelCase field names should NOT work anymore
	jsonBody := `{"sessionID":"draft-123","packNumber":1,"pickNumber":3}`

	var req GradePickRequest
	if err := json.Unmarshal([]byte(jsonBody), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// All fields should be zero values since the old field names don't match
	if req.SessionID != "" {
		t.Errorf("expected SessionID to be empty with old camelCase key, got '%s'", req.SessionID)
	}
	if req.PackNumber != 0 {
		t.Errorf("expected PackNumber to be 0 with old camelCase key, got %d", req.PackNumber)
	}
	if req.PickNumber != 0 {
		t.Errorf("expected PickNumber to be 0 with old camelCase key, got %d", req.PickNumber)
	}
}
