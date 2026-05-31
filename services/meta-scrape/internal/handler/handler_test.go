package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/scraper"
	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/store"
	"github.com/jackc/pgx/v5"
)

// fakeRefresher stands in for *scraper.Service. It records which formats were
// refreshed and returns a canned aggregated result (with optional card lists)
// per format.
type fakeRefresher struct {
	formats     []string
	metaByFmt   map[string]*scraper.AggregatedMeta
	cardsByFmt  map[string][]scraper.ArchetypeCardList
	failFormats map[string]error
	refreshed   []string
}

func (f *fakeRefresher) GetSupportedFormats() []string { return f.formats }

func (f *fakeRefresher) RefreshAll(_ context.Context, format string) (*scraper.AggregatedMeta, error) {
	f.refreshed = append(f.refreshed, format)
	if err := f.failFormats[format]; err != nil {
		return nil, err
	}
	return f.metaByFmt[format], nil
}

func (f *fakeRefresher) CardListsFromMeta(meta *scraper.AggregatedMeta) []scraper.ArchetypeCardList {
	if meta == nil {
		return nil
	}
	return f.cardsByFmt[meta.Format]
}

// fakeCardStore records id lookups and card upserts.
type fakeCardStore struct {
	idByKey    map[string]int64
	missing    map[string]bool
	upserts    int
	lastCards  []store.ArchetypeCard
	upsertFail error
}

func (s *fakeCardStore) ArchetypeIDByKey(_ context.Context, name, format string) (int64, error) {
	key := name + "|" + format
	if s.missing[key] {
		return 0, pgx.ErrNoRows
	}
	id, ok := s.idByKey[key]
	if !ok {
		return 0, pgx.ErrNoRows
	}
	return id, nil
}

func (s *fakeCardStore) UpsertArchetypeCards(_ context.Context, _ int64, cards []store.ArchetypeCard) error {
	if s.upsertFail != nil {
		return s.upsertFail
	}
	s.upserts++
	s.lastCards = cards
	return nil
}

func metaWith(format string, archetypeNames ...string) *scraper.AggregatedMeta {
	arch := make([]*scraper.AggregatedArchetype, 0, len(archetypeNames))
	for _, n := range archetypeNames {
		arch = append(arch, &scraper.AggregatedArchetype{Name: n})
	}
	return &scraper.AggregatedMeta{Format: format, Archetypes: arch}
}

func TestRun_AllFormatsSucceed_WiresCards(t *testing.T) {
	ref := &fakeRefresher{
		formats: []string{"standard", "modern"},
		metaByFmt: map[string]*scraper.AggregatedMeta{
			"standard": metaWith("standard", "Mono Red Aggro"),
			"modern":   metaWith("modern", "Amulet Titan"),
		},
		cardsByFmt: map[string][]scraper.ArchetypeCardList{
			"standard": {{Name: "Mono Red Aggro", Format: "standard", Cards: []store.ArchetypeCard{{CardName: "Monastery Swiftspear", Role: "mainboard", Copies: 4}}}},
			"modern":   {{Name: "Amulet Titan", Format: "modern", Cards: []store.ArchetypeCard{{CardName: "Primeval Titan", Role: "mainboard", Copies: 4}}}},
		},
	}
	cards := &fakeCardStore{idByKey: map[string]int64{
		"Mono Red Aggro|standard": 1,
		"Amulet Titan|modern":     2,
	}}

	h := New(ref, cards)
	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FormatsSucceeded != 2 || res.FormatsTotal != 2 {
		t.Fatalf("formats: got %d/%d", res.FormatsSucceeded, res.FormatsTotal)
	}
	if res.ArchetypesWritten != 2 {
		t.Errorf("archetypes: got %d want 2", res.ArchetypesWritten)
	}
	if res.CardListsWritten != 2 {
		t.Errorf("card lists: got %d want 2", res.CardListsWritten)
	}
	if cards.upserts != 2 {
		t.Errorf("card upserts: got %d want 2", cards.upserts)
	}
}

func TestRun_PartialFailure_StillSucceeds(t *testing.T) {
	ref := &fakeRefresher{
		formats: []string{"standard", "modern"},
		metaByFmt: map[string]*scraper.AggregatedMeta{
			"standard": metaWith("standard", "Mono Red Aggro"),
		},
		failFormats: map[string]error{"modern": errors.New("source down")},
	}
	h := New(ref, &fakeCardStore{idByKey: map[string]int64{"Mono Red Aggro|standard": 1}})

	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("partial failure must not error: %v", err)
	}
	if res.FormatsSucceeded != 1 {
		t.Errorf("succeeded: got %d want 1", res.FormatsSucceeded)
	}
}

func TestRun_AllFail_ReturnsError(t *testing.T) {
	ref := &fakeRefresher{
		formats:     []string{"standard", "modern"},
		failFormats: map[string]error{"standard": errors.New("x"), "modern": errors.New("y")},
	}
	h := New(ref, &fakeCardStore{})

	res, err := h.Run(context.Background())
	if err == nil {
		t.Fatal("expected error when all formats fail")
	}
	if res.FormatsSucceeded != 0 {
		t.Errorf("succeeded: got %d want 0", res.FormatsSucceeded)
	}
}

func TestRun_MissingArchetypeAfterUpsert_SkipsCardsNotFatal(t *testing.T) {
	ref := &fakeRefresher{
		formats:   []string{"standard"},
		metaByFmt: map[string]*scraper.AggregatedMeta{"standard": metaWith("standard", "Ghost Archetype")},
		cardsByFmt: map[string][]scraper.ArchetypeCardList{
			"standard": {{Name: "Ghost Archetype", Format: "standard", Cards: []store.ArchetypeCard{{CardName: "X", Role: "mainboard", Copies: 1}}}},
		},
	}
	cards := &fakeCardStore{missing: map[string]bool{"Ghost Archetype|standard": true}}

	h := New(ref, cards)
	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("missing-after-upsert must not abort the run: %v", err)
	}
	if res.CardListsWritten != 0 {
		t.Errorf("card lists: got %d want 0", res.CardListsWritten)
	}
	if cards.upserts != 0 {
		t.Errorf("no card upsert expected, got %d", cards.upserts)
	}
}

func TestRun_NilCardStore_SkipsCardWiring(t *testing.T) {
	ref := &fakeRefresher{
		formats:   []string{"standard"},
		metaByFmt: map[string]*scraper.AggregatedMeta{"standard": metaWith("standard", "A")},
	}
	h := New(ref, nil)
	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CardListsWritten != 0 {
		t.Errorf("card lists must be 0 with nil store, got %d", res.CardListsWritten)
	}
}
