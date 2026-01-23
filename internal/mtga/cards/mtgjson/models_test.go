package mtgjson

import (
	"encoding/json"
	"testing"
)

func TestCard_HasArenaID(t *testing.T) {
	tests := []struct {
		name     string
		card     Card
		expected bool
	}{
		{
			name: "card with Arena ID",
			card: Card{
				Identifiers: CardIdentifiers{
					MtgArenaId: "12345",
				},
			},
			expected: true,
		},
		{
			name: "card without Arena ID",
			card: Card{
				Identifiers: CardIdentifiers{
					ScryfallId: "abc-123",
				},
			},
			expected: false,
		},
		{
			name:     "card with empty identifiers",
			card:     Card{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.HasArenaID()
			if got != tt.expected {
				t.Errorf("HasArenaID() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCard_IsAvailableInArena(t *testing.T) {
	tests := []struct {
		name     string
		card     Card
		expected bool
	}{
		{
			name: "card available in arena",
			card: Card{
				Availability: []string{"paper", "arena", "mtgo"},
			},
			expected: true,
		},
		{
			name: "card only in paper",
			card: Card{
				Availability: []string{"paper"},
			},
			expected: false,
		},
		{
			name: "card in paper and mtgo",
			card: Card{
				Availability: []string{"paper", "mtgo"},
			},
			expected: false,
		},
		{
			name:     "card with empty availability",
			card:     Card{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.card.IsAvailableInArena()
			if got != tt.expected {
				t.Errorf("IsAvailableInArena() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCardIdentifiers_Unmarshal(t *testing.T) {
	jsonData := `{
		"mtgArenaId": "98387",
		"scryfallId": "fa940e68-010e-4b68-be8a-555d7068f7b4",
		"scryfallOracleId": "abc-oracle-123",
		"tcgplayerProductId": "12345"
	}`

	var identifiers CardIdentifiers
	err := json.Unmarshal([]byte(jsonData), &identifiers)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if identifiers.MtgArenaId != "98387" {
		t.Errorf("MtgArenaId = %q, want %q", identifiers.MtgArenaId, "98387")
	}
	if identifiers.ScryfallId != "fa940e68-010e-4b68-be8a-555d7068f7b4" {
		t.Errorf("ScryfallId = %q, want %q", identifiers.ScryfallId, "fa940e68-010e-4b68-be8a-555d7068f7b4")
	}
}

func TestSetData_Unmarshal(t *testing.T) {
	jsonData := `{
		"baseSetSize": 286,
		"code": "ECL",
		"name": "Lorwyn Eclipsed",
		"releaseDate": "2025-01-17",
		"type": "expansion",
		"cards": [
			{
				"uuid": "test-uuid-1",
				"name": "Test Card",
				"manaCost": "{2}{W}",
				"manaValue": 3,
				"type": "Creature - Human",
				"types": ["Creature"],
				"subtypes": ["Human"],
				"colors": ["W"],
				"colorIdentity": ["W"],
				"power": "2",
				"toughness": "2",
				"rarity": "common",
				"setCode": "ECL",
				"identifiers": {
					"mtgArenaId": "12345",
					"scryfallId": "abc-123"
				},
				"availability": ["paper", "arena"]
			}
		]
	}`

	var setData SetData
	err := json.Unmarshal([]byte(jsonData), &setData)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if setData.Code != "ECL" {
		t.Errorf("Code = %q, want %q", setData.Code, "ECL")
	}
	if setData.Name != "Lorwyn Eclipsed" {
		t.Errorf("Name = %q, want %q", setData.Name, "Lorwyn Eclipsed")
	}
	if len(setData.Cards) != 1 {
		t.Fatalf("len(Cards) = %d, want 1", len(setData.Cards))
	}

	card := setData.Cards[0]
	if card.Name != "Test Card" {
		t.Errorf("card.Name = %q, want %q", card.Name, "Test Card")
	}
	if !card.HasArenaID() {
		t.Error("card should have Arena ID")
	}
	if !card.IsAvailableInArena() {
		t.Error("card should be available in Arena")
	}
}

func TestLegalities_Unmarshal(t *testing.T) {
	jsonData := `{
		"standard": "legal",
		"historic": "legal",
		"pioneer": "not_legal",
		"modern": "legal",
		"commander": "legal"
	}`

	var legalities Legalities
	err := json.Unmarshal([]byte(jsonData), &legalities)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if legalities.Standard != "legal" {
		t.Errorf("Standard = %q, want %q", legalities.Standard, "legal")
	}
	if legalities.Pioneer != "not_legal" {
		t.Errorf("Pioneer = %q, want %q", legalities.Pioneer, "not_legal")
	}
}
