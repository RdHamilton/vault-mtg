package storage

import "testing"

func TestNormalizeMatchFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Generic queue types -> Standard
		{"Play returns Standard", "Play", "Standard"},
		{"Ladder returns Standard", "Ladder", "Standard"},
		{"Traditional_Play returns Standard", "Traditional_Play", "Standard"},
		{"Traditional_Ladder returns Standard", "Traditional_Ladder", "Standard"},

		// Format-specific events
		{"Alchemy returns Alchemy", "Alchemy", "Alchemy"},
		{"Alchemy_Ladder returns Alchemy", "Alchemy_Ladder", "Alchemy"},
		{"Alchemy_Play returns Alchemy", "Alchemy_Play", "Alchemy"},
		{"Historic returns Historic", "Historic", "Historic"},
		{"Historic_Ladder returns Historic", "Historic_Ladder", "Historic"},
		{"Explorer returns Explorer", "Explorer", "Explorer"},
		{"Explorer_Play returns Explorer", "Explorer_Play", "Explorer"},
		{"Timeless returns Timeless", "Timeless", "Timeless"},
		{"Timeless_Ladder returns Timeless", "Timeless_Ladder", "Timeless"},
		{"HistoricBrawl returns HistoricBrawl", "HistoricBrawl", "HistoricBrawl"},
		{"HistoricBrawl_Play returns HistoricBrawl", "HistoricBrawl_Play", "HistoricBrawl"},
		{"Brawl returns Brawl", "Brawl", "Brawl"},

		// Traditional Standard (stored as "Standard" in deck formats)
		{"TraditionalStandard returns Standard", "TraditionalStandard", "Standard"},
		{"TraditionalStandard_Ladder returns Standard", "TraditionalStandard_Ladder", "Standard"},
		{"Traditional_Standard returns Standard", "Traditional_Standard", "Standard"},

		// Draft formats -> Limited
		{"QuickDraft_TLA returns Limited", "QuickDraft_TLA_20251127", "Limited"},
		{"PremierDraft_MKM returns Limited", "PremierDraft_MKM", "Limited"},
		{"TradDraft_DSK returns Limited", "TradDraft_DSK", "Limited"},
		{"SealedDeck_BLB returns Limited", "SealedDeck_BLB", "Limited"},

		// Unknown format returns as-is
		{"Unknown format returns as-is", "SomeUnknownFormat", "SomeUnknownFormat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMatchFormat(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeMatchFormat(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
