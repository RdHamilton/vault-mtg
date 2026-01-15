package cards

import (
	"testing"
)

func TestGetArenaExclusiveCard(t *testing.T) {
	tests := []struct {
		name     string
		arenaID  int
		wantName string
		wantNil  bool
	}{
		{
			name:     "known card - Swamp 81181",
			arenaID:  81181,
			wantName: "Swamp",
			wantNil:  false,
		},
		{
			name:     "known card - Starting Town 96172",
			arenaID:  96172,
			wantName: "Starting Town",
			wantNil:  false,
		},
		{
			name:    "unknown card returns nil",
			arenaID: 999999,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetArenaExclusiveCard(tt.arenaID)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetArenaExclusiveCard(%d) = %v, want nil", tt.arenaID, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("GetArenaExclusiveCard(%d) = nil, want card", tt.arenaID)
			}
			if got.Name != tt.wantName {
				t.Errorf("GetArenaExclusiveCard(%d).Name = %q, want %q", tt.arenaID, got.Name, tt.wantName)
			}
		})
	}
}

func TestArenaExclusiveCard_ToCard(t *testing.T) {
	t.Run("card with ProducedMana (basic land)", func(t *testing.T) {
		aec := &ArenaExclusiveCard{
			ArenaID:      81181,
			Name:         "Swamp",
			TypeLine:     "Basic Land — Swamp",
			SetCode:      "JMP",
			SetName:      "Jumpstart",
			ProducedMana: []string{"B"},
		}

		card := aec.ToCard()

		if card.ArenaID != 81181 {
			t.Errorf("ToCard().ArenaID = %d, want 81181", card.ArenaID)
		}
		if card.Name != "Swamp" {
			t.Errorf("ToCard().Name = %q, want %q", card.Name, "Swamp")
		}
		if card.TypeLine != "Basic Land — Swamp" {
			t.Errorf("ToCard().TypeLine = %q, want %q", card.TypeLine, "Basic Land — Swamp")
		}
		if card.SetCode != "JMP" {
			t.Errorf("ToCard().SetCode = %q, want %q", card.SetCode, "JMP")
		}
		if card.SetName != "Jumpstart" {
			t.Errorf("ToCard().SetName = %q, want %q", card.SetName, "Jumpstart")
		}
		if len(card.Colors) != 1 || card.Colors[0] != "B" {
			t.Errorf("ToCard().Colors = %v, want [B]", card.Colors)
		}
		if len(card.ColorIdentity) != 1 || card.ColorIdentity[0] != "B" {
			t.Errorf("ToCard().ColorIdentity = %v, want [B]", card.ColorIdentity)
		}
		if card.Rarity != "common" {
			t.Errorf("ToCard().Rarity = %q, want %q", card.Rarity, "common")
		}
		if card.Layout != "normal" {
			t.Errorf("ToCard().Layout = %q, want %q", card.Layout, "normal")
		}
	})

	t.Run("card without ProducedMana (non-basic land)", func(t *testing.T) {
		aec := &ArenaExclusiveCard{
			ArenaID:  96172,
			Name:     "Starting Town",
			TypeLine: "Land",
			SetCode:  "FIN",
			SetName:  "Final Fantasy",
			// No ProducedMana - Starting Town doesn't have a color
		}

		card := aec.ToCard()

		if card.ArenaID != 96172 {
			t.Errorf("ToCard().ArenaID = %d, want 96172", card.ArenaID)
		}
		if card.Name != "Starting Town" {
			t.Errorf("ToCard().Name = %q, want %q", card.Name, "Starting Town")
		}
		if card.TypeLine != "Land" {
			t.Errorf("ToCard().TypeLine = %q, want %q", card.TypeLine, "Land")
		}
		if card.SetCode != "FIN" {
			t.Errorf("ToCard().SetCode = %q, want %q", card.SetCode, "FIN")
		}
		if card.SetName != "Final Fantasy" {
			t.Errorf("ToCard().SetName = %q, want %q", card.SetName, "Final Fantasy")
		}
		// Colors and ColorIdentity should be nil/empty for colorless cards
		if len(card.Colors) != 0 {
			t.Errorf("ToCard().Colors = %v, want empty", card.Colors)
		}
		if len(card.ColorIdentity) != 0 {
			t.Errorf("ToCard().ColorIdentity = %v, want empty", card.ColorIdentity)
		}
		if card.Rarity != "common" {
			t.Errorf("ToCard().Rarity = %q, want %q", card.Rarity, "common")
		}
		if card.Layout != "normal" {
			t.Errorf("ToCard().Layout = %q, want %q", card.Layout, "normal")
		}
	})
}
