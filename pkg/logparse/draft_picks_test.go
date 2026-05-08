package logparse

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseDraftPicks_LegacyHumanDraftEvent tests the pre-2026.58 humanDraftEvent format.
func TestParseDraftPicks_LegacyHumanDraftEvent(t *testing.T) {
	tests := []struct {
		name    string
		entries []*LogEntry
		want    []*DraftPicks
		wantNil bool
	}{
		{
			name: "no draft pick data",
			entries: []*LogEntry{
				{
					IsJSON: true,
					JSON: map[string]interface{}{
						"otherEvent": map[string]interface{}{},
					},
				},
			},
			wantNil: true,
		},
		{
			name: "legacy format with integer card IDs converted to strings",
			entries: []*LogEntry{
				{
					IsJSON:    true,
					Timestamp: "[UnityCrossThreadLogger]2024-01-15 10:30:45",
					JSON: map[string]interface{}{
						"humanDraftEvent": map[string]interface{}{
							"CourseId":     "course-1",
							"SelfPack":     float64(1),
							"SelfPick":     float64(1),
							"PackCards":    []interface{}{float64(12345), float64(12346), float64(12347)},
							"SelectedCard": float64(12345),
						},
					},
				},
			},
			want: []*DraftPicks{
				{
					CourseID: "course-1",
					Picks: []DraftPick{
						{
							CourseID:       "course-1",
							PackNumber:     1,
							PickNumber:     1,
							AvailableCards: []string{"12345", "12346", "12347"},
							SelectedCard:   "12345",
						},
					},
				},
			},
		},
		{
			name: "2026.58+ PickNumber field (SelfPick removed)",
			entries: []*LogEntry{
				{
					IsJSON:    true,
					Timestamp: "[UnityCrossThreadLogger]2024-01-15 10:30:45",
					JSON: map[string]interface{}{
						"humanDraftEvent": map[string]interface{}{
							"CourseId":     "course-2",
							"SelfPack":     float64(1),
							"PickNumber":   float64(3),
							"PackCards":    []interface{}{"97380", "97381"},
							"SelectedCard": "97380",
						},
					},
				},
			},
			want: []*DraftPicks{
				{
					CourseID: "course-2",
					Picks: []DraftPick{
						{
							CourseID:       "course-2",
							PackNumber:     1,
							PickNumber:     3,
							AvailableCards: []string{"97380", "97381"},
							SelectedCard:   "97380",
						},
					},
				},
			},
		},
		{
			name: "multiple picks for same course",
			entries: []*LogEntry{
				{
					IsJSON:    true,
					Timestamp: "[UnityCrossThreadLogger]2024-01-15 10:30:45",
					JSON: map[string]interface{}{
						"humanDraftEvent": map[string]interface{}{
							"CourseId":     "course-1",
							"SelfPack":     float64(1),
							"SelfPick":     float64(1),
							"PackCards":    []interface{}{float64(12345)},
							"SelectedCard": float64(12345),
						},
					},
				},
				{
					IsJSON:    true,
					Timestamp: "[UnityCrossThreadLogger]2024-01-15 10:31:00",
					JSON: map[string]interface{}{
						"humanDraftEvent": map[string]interface{}{
							"CourseId":     "course-1",
							"SelfPack":     float64(1),
							"SelfPick":     float64(2),
							"PackCards":    []interface{}{float64(12346)},
							"SelectedCard": float64(12346),
						},
					},
				},
			},
			want: []*DraftPicks{
				{
					CourseID: "course-1",
					Picks: []DraftPick{
						{
							CourseID:       "course-1",
							PackNumber:     1,
							PickNumber:     1,
							AvailableCards: []string{"12345"},
							SelectedCard:   "12345",
						},
						{
							CourseID:       "course-1",
							PackNumber:     1,
							PickNumber:     2,
							AvailableCards: []string{"12346"},
							SelectedCard:   "12346",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDraftPicks(tt.entries)
			if err != nil {
				t.Errorf("ParseDraftPicks() error = %v", err)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseDraftPicks() expected nil, got %v", got)
				}
				return
			}

			if got == nil {
				t.Errorf("ParseDraftPicks() expected picks, got nil")
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("ParseDraftPicks() course count = %d, want %d", len(got), len(tt.want))
				return
			}

			for i, wantPicks := range tt.want {
				if i >= len(got) {
					t.Errorf("ParseDraftPicks() missing course %s", wantPicks.CourseID)
					continue
				}

				gotPicks := got[i]
				if gotPicks.CourseID != wantPicks.CourseID {
					t.Errorf("ParseDraftPicks() course ID = %s, want %s", gotPicks.CourseID, wantPicks.CourseID)
				}

				if len(gotPicks.Picks) != len(wantPicks.Picks) {
					t.Errorf("ParseDraftPicks() pick count = %d, want %d", len(gotPicks.Picks), len(wantPicks.Picks))
					continue
				}

				for j, wantPick := range wantPicks.Picks {
					gotPick := gotPicks.Picks[j]
					if gotPick.PickNumber != wantPick.PickNumber {
						t.Errorf("pick[%d] PickNumber = %d, want %d", j, gotPick.PickNumber, wantPick.PickNumber)
					}
					if gotPick.PackNumber != wantPick.PackNumber {
						t.Errorf("pick[%d] PackNumber = %d, want %d", j, gotPick.PackNumber, wantPick.PackNumber)
					}
					if gotPick.SelectedCard != wantPick.SelectedCard {
						t.Errorf("pick[%d] SelectedCard = %s, want %s", j, gotPick.SelectedCard, wantPick.SelectedCard)
					}
					if len(gotPick.AvailableCards) != len(wantPick.AvailableCards) {
						t.Errorf("pick[%d] AvailableCards count = %d, want %d", j, len(gotPick.AvailableCards), len(wantPick.AvailableCards))
					} else {
						for k, wantCard := range wantPick.AvailableCards {
							if gotPick.AvailableCards[k] != wantCard {
								t.Errorf("pick[%d] AvailableCards[%d] = %s, want %s", j, k, gotPick.AvailableCards[k], wantCard)
							}
						}
					}
				}
			}
		})
	}
}

// TestParseDraftPicks_BotDraftWrapperFormat tests the Arena 2026.58+ BotDraft wrapper format.
// Fix #1: {"CurrentModule":"BotDraft","Payload":"<JSON-encoded-string>"}
// Card IDs are strings, CourseName→EventName, SelfPick removed/PickNumber used.
func TestParseDraftPicks_BotDraftWrapperFormat(t *testing.T) {
	// Simulate the Arena 2026.58+ BotDraftDraftStatus response.
	// The outer JSON is {"CurrentModule":"BotDraft","Payload":"<json-string>"}
	// The Payload contains the actual draft state with string card IDs.
	payloadJSON := `{"EventName":"QuickDraft_TDM_20251111","PackNumber":1,"PickNumber":2,"DraftPack":["97380","97381","97382"],"PickedCards":["97379"]}`

	entries := []*LogEntry{
		{
			IsJSON:    true,
			Timestamp: "[UnityCrossThreadLogger]2025-11-12 09:15:00",
			JSON: map[string]interface{}{
				"CurrentModule": "BotDraft",
				"Payload":       payloadJSON,
			},
		},
	}

	got, err := ParseDraftPicks(entries)
	if err != nil {
		t.Fatalf("ParseDraftPicks() error = %v", err)
	}
	if got == nil {
		t.Fatal("ParseDraftPicks() expected picks, got nil")
	}
	if len(got) != 1 {
		t.Fatalf("ParseDraftPicks() course count = %d, want 1", len(got))
	}

	picks := got[0]
	if picks.CourseID != "QuickDraft_TDM_20251111" {
		t.Errorf("CourseID = %q, want %q", picks.CourseID, "QuickDraft_TDM_20251111")
	}
	if len(picks.Picks) != 1 {
		t.Fatalf("pick count = %d, want 1", len(picks.Picks))
	}

	pick := picks.Picks[0]
	if pick.PackNumber != 1 {
		t.Errorf("PackNumber = %d, want 1", pick.PackNumber)
	}
	if pick.PickNumber != 2 {
		t.Errorf("PickNumber = %d, want 2", pick.PickNumber)
	}
	if len(pick.AvailableCards) != 3 {
		t.Errorf("AvailableCards count = %d, want 3", len(pick.AvailableCards))
	} else {
		if pick.AvailableCards[0] != "97380" {
			t.Errorf("AvailableCards[0] = %q, want %q", pick.AvailableCards[0], "97380")
		}
	}
}

func TestParseDraftPicks_FromLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_player.log")

	testData := `[UnityCrossThreadLogger]{"humanDraftEvent":{"CourseId":"course-1","SelfPack":1,"SelfPick":1,"PackCards":[12345,12346,12347],"SelectedCard":12345}}
`
	if err := os.WriteFile(logPath, []byte(testData), 0o644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	reader, err := NewReader(logPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("Error closing reader: %v", err)
		}
	}()

	entries, err := reader.ReadAllJSON()
	if err != nil {
		t.Fatalf("Failed to read entries: %v", err)
	}

	picks, err := ParseDraftPicks(entries)
	if err != nil {
		t.Fatalf("ParseDraftPicks() error = %v", err)
	}

	if picks == nil {
		t.Fatal("ParseDraftPicks() expected picks, got nil")
	}

	if len(picks) != 1 {
		t.Errorf("ParseDraftPicks() course count = %d, want 1", len(picks))
	}

	if len(picks[0].Picks) != 1 {
		t.Errorf("ParseDraftPicks() pick count = %d, want 1", len(picks[0].Picks))
	}

	// Verify card IDs are now strings.
	if picks[0].Picks[0].SelectedCard != "12345" {
		t.Errorf("SelectedCard = %q, want %q", picks[0].Picks[0].SelectedCard, "12345")
	}
}

func TestToCardIDString(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{"97380", "97380"},
		{float64(12345), "12345"},
		{int(99), "99"},
		{nil, ""},
	}
	for _, tt := range tests {
		got := toCardIDString(tt.input)
		if got != tt.want {
			t.Errorf("toCardIDString(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
