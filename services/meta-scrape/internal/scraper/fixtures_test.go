package scraper

import (
	"os"
	"path/filepath"
	"testing"
)

// readFixture loads a testdata HTML snapshot. The fixtures live two directories
// up from this package, under services/meta-scrape/testdata/.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// TestFixture_GoldfishStandardParses verifies the reference Goldfish Standard
// snapshot parses with the live archetype-tile patterns, so the snapshot stays
// in sync with the regex (ground-truth check, not a live fetch).
func TestFixture_GoldfishStandardParses(t *testing.T) {
	client := NewGoldfishClient(nil)
	meta := client.parseMetaPage(readFixture(t, "goldfish_standard.html"), "standard")
	if len(meta.Decks) != 3 {
		t.Fatalf("expected 3 decks from goldfish_standard.html, got %d", len(meta.Decks))
	}
	want := map[string]float64{
		"Izzet Cauldron": 18.2,
		"Dimir Midrange": 14.7,
		"Mono Red Aggro": 9.3,
	}
	for _, d := range meta.Decks {
		if share, ok := want[d.Name]; ok && d.MetaShare != share {
			t.Errorf("%s: expected share %.1f, got %.1f", d.Name, share, d.MetaShare)
		}
	}
}

// TestFixture_GoldfishHistoricParses verifies the Historic snapshot parses.
func TestFixture_GoldfishHistoricParses(t *testing.T) {
	client := NewGoldfishClient(nil)
	meta := client.parseMetaPage(readFixture(t, "goldfish_historic.html"), "historic")
	if len(meta.Decks) != 2 {
		t.Fatalf("expected 2 decks from goldfish_historic.html, got %d", len(meta.Decks))
	}
}

// TestFixture_Top8StandardParses verifies the MTGTop8 Standard snapshot parses
// its archetype summary table and event titles.
func TestFixture_Top8StandardParses(t *testing.T) {
	client := NewTop8Client(nil)
	meta := client.parseTournamentPage(readFixture(t, "mtgtop8_standard.html"), "standard")
	if len(meta.ArchetypeStats) != 4 {
		t.Fatalf("expected 4 archetype stats from mtgtop8_standard.html, got %d", len(meta.ArchetypeStats))
	}
	if len(meta.Tournaments) != 3 {
		t.Fatalf("expected 3 tournaments from mtgtop8_standard.html, got %d", len(meta.Tournaments))
	}
	if stats := meta.ArchetypeStats["izzet cauldron"]; stats == nil || stats.Top8Count != 37 {
		t.Errorf("expected Izzet Cauldron Top8Count 37, got %+v", stats)
	}
}

// TestFixture_Top8PioneerParses verifies the MTGTop8 Pioneer snapshot parses.
func TestFixture_Top8PioneerParses(t *testing.T) {
	client := NewTop8Client(nil)
	meta := client.parseTournamentPage(readFixture(t, "mtgtop8_pioneer.html"), "pioneer")
	if len(meta.ArchetypeStats) != 3 {
		t.Fatalf("expected 3 archetype stats from mtgtop8_pioneer.html, got %d", len(meta.ArchetypeStats))
	}
	if len(meta.Tournaments) != 2 {
		t.Fatalf("expected 2 tournaments from mtgtop8_pioneer.html, got %d", len(meta.Tournaments))
	}
}
