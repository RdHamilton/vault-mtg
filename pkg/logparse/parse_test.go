package logparse

import (
	"testing"
)

// --- Fix #2 + #3: PlayerInventory PascalCase fields + InventoryInfo wrapper ---

func TestParseInventory_PascalCaseWrapped(t *testing.T) {
	// Arena 2026.58+: inventory under "InventoryInfo" key, PascalCase field names.
	entries := []*LogEntry{
		{
			IsJSON: true,
			JSON: map[string]interface{}{
				"InventoryInfo": map[string]interface{}{
					"Gems":               float64(1500),
					"Gold":               float64(2000),
					"TotalVaultProgress": float64(42),
					"WildCardCommons":    float64(10),
					"WildCardUnCommons":  float64(5),
					"WildCardRares":      float64(3),
					"WildCardMythics":    float64(1),
					"Boosters": []interface{}{
						map[string]interface{}{
							"CollationId": float64(100),
							"SetCode":     "TDM",
							"Count":       float64(3),
						},
					},
					"CustomTokens": map[string]interface{}{
						"PlayerLevelToken": float64(7),
					},
				},
			},
		},
	}

	inv, err := ParseInventory(entries)
	if err != nil {
		t.Fatalf("ParseInventory() error = %v", err)
	}
	if inv == nil {
		t.Fatal("ParseInventory() expected non-nil, got nil")
	}

	if inv.Gems != 1500 {
		t.Errorf("Gems = %d, want 1500", inv.Gems)
	}
	if inv.Gold != 2000 {
		t.Errorf("Gold = %d, want 2000", inv.Gold)
	}
	if inv.TotalVaultProgress != 42 {
		t.Errorf("TotalVaultProgress = %d, want 42", inv.TotalVaultProgress)
	}
	if inv.WildCardCommons != 10 {
		t.Errorf("WildCardCommons = %d, want 10", inv.WildCardCommons)
	}
	if inv.WildCardUncommons != 5 {
		t.Errorf("WildCardUncommons = %d, want 5", inv.WildCardUncommons)
	}
	if inv.WildCardRares != 3 {
		t.Errorf("WildCardRares = %d, want 3", inv.WildCardRares)
	}
	if inv.WildCardMythics != 1 {
		t.Errorf("WildCardMythics = %d, want 1", inv.WildCardMythics)
	}

	// Fix #3: booster PascalCase fields
	if len(inv.Boosters) != 1 {
		t.Fatalf("Boosters count = %d, want 1", len(inv.Boosters))
	}
	b := inv.Boosters[0]
	if b.SetCode != "TDM" {
		t.Errorf("Booster.SetCode = %q, want %q", b.SetCode, "TDM")
	}
	if b.Count != 3 {
		t.Errorf("Booster.Count = %d, want 3", b.Count)
	}
	if b.CollationId != 100 {
		t.Errorf("Booster.CollationId = %d, want 100", b.CollationId)
	}
}

func TestParseInventory_NoInventoryInfo_ReturnsNil(t *testing.T) {
	// Without InventoryInfo wrapper, ParseInventory should return nil.
	entries := []*LogEntry{
		{
			IsJSON: true,
			JSON: map[string]interface{}{
				"SomeOtherKey": map[string]interface{}{
					"Gems": float64(999),
				},
			},
		},
	}

	inv, err := ParseInventory(entries)
	if err != nil {
		t.Fatalf("ParseInventory() error = %v", err)
	}
	if inv != nil {
		t.Errorf("ParseInventory() = %v, want nil (no InventoryInfo wrapper)", inv)
	}
}

// --- Fix #4: PlayerRank classifier uses constructedLevel + limitedLevel ---

func TestParseRank_BothLevelFieldsRequired(t *testing.T) {
	// Fix #4: rank entries must have BOTH constructedLevel AND limitedLevel.
	entries := []*LogEntry{
		{
			IsJSON: true,
			JSON: map[string]interface{}{
				"constructedLevel":         float64(3),
				"constructedClass":         "Gold",
				"constructedSeasonOrdinal": float64(12),
				"constructedPercentile":    float64(75.5),
				"constructedStep":          float64(2),
				"limitedLevel":             float64(2),
				"limitedClass":             "Silver",
				"limitedSeasonOrdinal":     float64(12),
				"limitedPercentile":        float64(50.0),
				"limitedStep":              float64(1),
				"limitedMatchesWon":        float64(15),
				"limitedMatchesLost":       float64(8),
			},
		},
	}

	rank, err := ParseRank(entries)
	if err != nil {
		t.Fatalf("ParseRank() error = %v", err)
	}
	if rank == nil {
		t.Fatal("ParseRank() expected non-nil, got nil")
	}

	if rank.ConstructedLevel != 3 {
		t.Errorf("ConstructedLevel = %d, want 3", rank.ConstructedLevel)
	}
	if rank.ConstructedClass != "Gold" {
		t.Errorf("ConstructedClass = %q, want Gold", rank.ConstructedClass)
	}
	if rank.LimitedLevel != 2 {
		t.Errorf("LimitedLevel = %d, want 2", rank.LimitedLevel)
	}
	if rank.LimitedClass != "Silver" {
		t.Errorf("LimitedClass = %q, want Silver", rank.LimitedClass)
	}
	if rank.LimitedMatchesWon != 15 {
		t.Errorf("LimitedMatchesWon = %d, want 15", rank.LimitedMatchesWon)
	}
	if rank.LimitedMatchesLost != 8 {
		t.Errorf("LimitedMatchesLost = %d, want 8", rank.LimitedMatchesLost)
	}
}

func TestParseRank_MissingOneLevelField_ReturnsNil(t *testing.T) {
	// Only constructedLevel without limitedLevel → should NOT match.
	entries := []*LogEntry{
		{
			IsJSON: true,
			JSON: map[string]interface{}{
				"constructedLevel": float64(3),
				"constructedClass": "Gold",
				// No limitedLevel
			},
		},
	}

	rank, err := ParseRank(entries)
	if err != nil {
		t.Fatalf("ParseRank() error = %v", err)
	}
	if rank != nil {
		t.Errorf("ParseRank() = %v, want nil when only one level field present", rank)
	}
}

func TestParseRank_OldRankClassKey_NotMatched(t *testing.T) {
	// Old "rankClass" top-level key should NOT trigger rank detection in 2026.58+.
	entries := []*LogEntry{
		{
			IsJSON: true,
			JSON: map[string]interface{}{
				"rankClass": "Gold",
				"rankTier":  float64(3),
			},
		},
	}

	rank, err := ParseRank(entries)
	if err != nil {
		t.Fatalf("ParseRank() error = %v", err)
	}
	if rank != nil {
		t.Errorf("ParseRank() = %v, want nil (rankClass key is not the rank detector)", rank)
	}
}

// --- Existing tests carried over ---

func TestParseDraftHistory(t *testing.T) {
	tests := []struct {
		name      string
		entries   []*LogEntry
		wantNil   bool
		wantCount int
	}{
		{
			name:    "no draft events",
			entries: []*LogEntry{},
			wantNil: true,
		},
		{
			name: "no JSON entries",
			entries: []*LogEntry{
				{Raw: "Plain text line", IsJSON: false},
			},
			wantNil: true,
		},
		{
			name: "constructed events only",
			entries: []*LogEntry{
				{
					Raw:    `{"Courses": [{"CourseId": "test-1", "InternalEventName": "Ladder"}]}`,
					IsJSON: true,
					JSON: map[string]interface{}{
						"Courses": []interface{}{
							map[string]interface{}{
								"CourseId":          "test-1",
								"InternalEventName": "Ladder",
							},
						},
					},
				},
			},
			wantNil: true,
		},
		{
			name: "single draft event",
			entries: []*LogEntry{
				{
					Raw:    `{"Courses": [{"CourseId": "draft-1", "InternalEventName": "PremierDraft_BLB"}]}`,
					IsJSON: true,
					JSON: map[string]interface{}{
						"Courses": []interface{}{
							map[string]interface{}{
								"CourseId":          "draft-1",
								"InternalEventName": "PremierDraft_BLB",
								"CurrentModule":     "DeckBuild",
								"CurrentWins":       float64(3),
								"CurrentLosses":     float64(1),
								"CourseDeck": map[string]interface{}{
									"MainDeck": []interface{}{
										map[string]interface{}{
											"cardId":   float64(12345),
											"quantity": float64(2),
										},
									},
								},
								"CourseDeckSummary": map[string]interface{}{
									"Name": "BLB Draft Deck",
								},
							},
						},
					},
				},
			},
			wantNil:   false,
			wantCount: 1,
		},
		{
			name: "multiple draft events",
			entries: []*LogEntry{
				{
					Raw:    `{"Courses": [...]}`,
					IsJSON: true,
					JSON: map[string]interface{}{
						"Courses": []interface{}{
							map[string]interface{}{
								"CourseId":          "draft-1",
								"InternalEventName": "PremierDraft_BLB",
								"CurrentModule":     "CreateMatch",
								"CurrentWins":       float64(7),
							},
							map[string]interface{}{
								"CourseId":          "draft-2",
								"InternalEventName": "QuickDraft_FDN",
								"CurrentModule":     "DeckBuild",
								"CurrentWins":       float64(2),
							},
							map[string]interface{}{
								"CourseId":          "constructed-1",
								"InternalEventName": "Ladder",
							},
						},
					},
				},
			},
			wantNil:   false,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history, err := ParseDraftHistory(tt.entries)
			if err != nil {
				t.Errorf("ParseDraftHistory() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if history != nil {
					t.Errorf("ParseDraftHistory() expected nil, got %v", history)
				}
				return
			}

			if history == nil {
				t.Error("ParseDraftHistory() expected non-nil result")
				return
			}

			if len(history.Drafts) != tt.wantCount {
				t.Errorf("ParseDraftHistory() got %d drafts, want %d", len(history.Drafts), tt.wantCount)
			}

			if tt.wantCount == 1 && len(history.Drafts) > 0 {
				draft := history.Drafts[0]
				if draft.EventID != "draft-1" {
					t.Errorf("Draft EventID = %s, want draft-1", draft.EventID)
				}
				if draft.EventName != "PremierDraft_BLB" {
					t.Errorf("Draft EventName = %s, want PremierDraft_BLB", draft.EventName)
				}
				if draft.Wins != 3 {
					t.Errorf("Draft Wins = %d, want 3", draft.Wins)
				}
				if draft.Losses != 1 {
					t.Errorf("Draft Losses = %d, want 1", draft.Losses)
				}
				if draft.Deck.Name != "BLB Draft Deck" {
					t.Errorf("Deck Name = %s, want BLB Draft Deck", draft.Deck.Name)
				}
				if len(draft.Deck.MainDeck) != 1 {
					t.Errorf("MainDeck length = %d, want 1", len(draft.Deck.MainDeck))
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"PremierDraft_BLB", "Draft", true},
		{"QuickDraft_FDN", "Draft", true},
		{"Sealed_BLB", "Sealed", true},
		{"Ladder", "Draft", false},
		{"Play", "Draft", false},
		{"", "Draft", false},
		{"Draft", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_contains_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestParseArenaStats(t *testing.T) {
	tests := []struct {
		name             string
		entries          []*LogEntry
		wantNil          bool
		wantTotalMatches int
		wantMatchWins    int
		wantMatchLosses  int
		wantTotalGames   int
		wantGameWins     int
		wantGameLosses   int
		wantFormatCount  int
	}{
		{
			name:    "no match events",
			entries: []*LogEntry{},
			wantNil: true,
		},
		{
			name: "no JSON entries",
			entries: []*LogEntry{
				{Raw: "Plain text line", IsJSON: false},
			},
			wantNil: true,
		},
		{
			name: "match without final result",
			entries: []*LogEntry{
				{
					Raw:    `{"matchGameRoomStateChangedEvent": {"gameRoomInfo": {}}}`,
					IsJSON: true,
					JSON: map[string]interface{}{
						"matchGameRoomStateChangedEvent": map[string]interface{}{
							"gameRoomInfo": map[string]interface{}{
								"stateType": "MatchGameRoomStateType_Playing",
							},
						},
					},
				},
			},
			wantNil: true,
		},
		{
			name: "single match win (player team 1)",
			entries: []*LogEntry{
				{
					Raw:    `{"matchGameRoomStateChangedEvent": {...}}`,
					IsJSON: true,
					JSON: map[string]interface{}{
						"matchGameRoomStateChangedEvent": map[string]interface{}{
							"gameRoomInfo": map[string]interface{}{
								"finalMatchResult": map[string]interface{}{
									"matchId": "match-1",
									"resultList": []interface{}{
										map[string]interface{}{
											"scope":         "MatchScope_Match",
											"winningTeamId": float64(1),
											"result":        "ResultType_WinLoss",
										},
										map[string]interface{}{
											"scope":         "MatchScope_Game",
											"winningTeamId": float64(1),
											"result":        "ResultType_WinLoss",
										},
									},
								},
								"gameRoomConfig": map[string]interface{}{
									"matchId": "match-1",
									"reservedPlayers": []interface{}{
										map[string]interface{}{
											"userId":  "player1",
											"teamId":  float64(1),
											"eventId": "Play",
										},
									},
								},
							},
						},
					},
				},
			},
			wantNil:          false,
			wantTotalMatches: 1,
			wantMatchWins:    1,
			wantMatchLosses:  0,
			wantTotalGames:   1,
			wantGameWins:     1,
			wantGameLosses:   0,
			wantFormatCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, err := ParseArenaStats(tt.entries)
			if err != nil {
				t.Errorf("ParseArenaStats() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if stats != nil {
					t.Errorf("ParseArenaStats() expected nil, got %v", stats)
				}
				return
			}

			if stats == nil {
				t.Error("ParseArenaStats() expected non-nil result")
				return
			}

			if stats.TotalMatches != tt.wantTotalMatches {
				t.Errorf("TotalMatches = %d, want %d", stats.TotalMatches, tt.wantTotalMatches)
			}
			if stats.MatchWins != tt.wantMatchWins {
				t.Errorf("MatchWins = %d, want %d", stats.MatchWins, tt.wantMatchWins)
			}
			if stats.MatchLosses != tt.wantMatchLosses {
				t.Errorf("MatchLosses = %d, want %d", stats.MatchLosses, tt.wantMatchLosses)
			}
			if stats.TotalGames != tt.wantTotalGames {
				t.Errorf("TotalGames = %d, want %d", stats.TotalGames, tt.wantTotalGames)
			}
			if stats.GameWins != tt.wantGameWins {
				t.Errorf("GameWins = %d, want %d", stats.GameWins, tt.wantGameWins)
			}
			if stats.GameLosses != tt.wantGameLosses {
				t.Errorf("GameLosses = %d, want %d", stats.GameLosses, tt.wantGameLosses)
			}
			if len(stats.FormatStats) != tt.wantFormatCount {
				t.Errorf("FormatStats count = %d, want %d", len(stats.FormatStats), tt.wantFormatCount)
			}
		})
	}
}
