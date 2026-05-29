package handlers_test

import (
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
)

func TestIsKnownFormat(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		// Exact lowercase — all known formats.
		{"standard lowercase", "standard", true},
		{"historic lowercase", "historic", true},
		{"brawl lowercase", "brawl", true},
		{"limited lowercase", "limited", true},
		{"draft lowercase", "draft", true},
		{"sealed lowercase", "sealed", true},
		{"alchemy lowercase", "alchemy", true},
		{"explorer lowercase", "explorer", true},
		{"timeless lowercase", "timeless", true},
		{"gladiator lowercase", "gladiator", true},
		{"pauper lowercase", "pauper", true},
		// Case-insensitive variants.
		{"Standard title-case", "Standard", true},
		{"HISTORIC all-caps", "HISTORIC", true},
		{"Draft title-case", "Draft", true},
		{"Timeless title-case", "Timeless", true},
		{"ALCHEMY all-caps", "ALCHEMY", true},
		// Unknown / invalid inputs.
		{"empty string", "", false},
		{"unknown format", "vintage", false},
		{"partial match", "stan", false},
		{"numeric", "1234", false},
		{"spaces", "standard ", false},
		{"mixed unknown", "FakeFormat", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := handlers.IsKnownFormat(tc.input)
			if got != tc.want {
				t.Errorf("IsKnownFormat(%q) = %v; want %v", tc.input, got, tc.want)
			}
		})
	}
}
