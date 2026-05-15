package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestArchetype inserts a row into mtgzone_archetypes and returns its id.
// The row (and any cascade children) are cleaned up via t.Cleanup.
func insertTestArchetype(t *testing.T, db *sql.DB, name, format string, tier *string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(
		context.Background(),
		`INSERT INTO mtgzone_archetypes (name, format, tier, last_updated)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (name, format) DO UPDATE SET tier = EXCLUDED.tier
		 RETURNING id`,
		name, format, tier,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestArchetype %q/%q: %v", name, format, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM mtgzone_archetypes WHERE id = $1`, id)
	})
	return id
}

// insertTestArchetypeCard inserts a row into mtgzone_archetype_cards.
// Cleanup is handled by the parent archetype's ON DELETE CASCADE.
func insertTestArchetypeCard(t *testing.T, db *sql.DB, archetypeID int64, cardName, role string, copies int) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO mtgzone_archetype_cards (archetype_id, card_name, role, copies, last_updated)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (archetype_id, card_name) DO NOTHING`,
		archetypeID, cardName, role, copies,
	)
	if err != nil {
		t.Fatalf("insertTestArchetypeCard archetype=%d card=%q: %v", archetypeID, cardName, err)
	}
}

func strPtr(s string) *string { return &s }

// ----------------------------------------------------------------------------
// MetaRepository.ListArchetypesByFormat
// ----------------------------------------------------------------------------

// TestMetaRepository_ListArchetypesByFormat_HappyPath verifies that
// ListArchetypesByFormat returns archetypes for the queried format and
// excludes archetypes belonging to a different format.
func TestMetaRepository_ListArchetypesByFormat_HappyPath(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	tier1 := "1"
	tier2 := "2"
	insertTestArchetype(t, db, "meta-repo-mono-red", "standard", &tier1)
	insertTestArchetype(t, db, "meta-repo-esper-control", "standard", &tier2)
	// Different format — must NOT appear in results.
	insertTestArchetype(t, db, "meta-repo-uw-control", "historic", nil)

	rows, err := repo.ListArchetypesByFormat(ctx, "standard", 0)
	if err != nil {
		t.Fatalf("ListArchetypesByFormat: %v", err)
	}
	found := map[string]bool{}
	for _, a := range rows {
		found[a.Name] = true
		if a.Format != "standard" {
			t.Errorf("unexpected format %q for archetype %q", a.Format, a.Name)
		}
	}
	if !found["meta-repo-mono-red"] {
		t.Error("expected meta-repo-mono-red in results")
	}
	if !found["meta-repo-esper-control"] {
		t.Error("expected meta-repo-esper-control in results")
	}
	if found["meta-repo-uw-control"] {
		t.Error("meta-repo-uw-control (historic) must not appear in standard results")
	}
}

// TestMetaRepository_ListArchetypesByFormat_TierFilter verifies that passing
// tier > 0 narrows results to only that tier value.
func TestMetaRepository_ListArchetypesByFormat_TierFilter(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	tier1 := "1"
	tier2 := "2"
	insertTestArchetype(t, db, "meta-repo-tier1-deck", "pioneer", &tier1)
	insertTestArchetype(t, db, "meta-repo-tier2-deck", "pioneer", &tier2)

	rows, err := repo.ListArchetypesByFormat(ctx, "pioneer", 1)
	if err != nil {
		t.Fatalf("ListArchetypesByFormat (tier=1): %v", err)
	}
	for _, a := range rows {
		if a.Tier == nil || *a.Tier != "1" {
			t.Errorf("tier filter failed: got archetype %q with tier %v", a.Name, a.Tier)
		}
	}
	found := false
	for _, a := range rows {
		if a.Name == "meta-repo-tier1-deck" {
			found = true
		}
	}
	if !found {
		t.Error("expected meta-repo-tier1-deck in tier-1 results")
	}
}

// TestMetaRepository_ListArchetypesByFormat_EmptyTable verifies that an empty
// table returns an empty slice (not an error and not nil).
func TestMetaRepository_ListArchetypesByFormat_EmptyTable(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	// Use a format that is very unlikely to have seeded rows.
	rows, err := repo.ListArchetypesByFormat(ctx, "format-that-does-not-exist-xyz", 0)
	if err != nil {
		t.Fatalf("ListArchetypesByFormat (empty): %v", err)
	}
	// Nil slice and empty slice are both acceptable; length must be 0.
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d: %v", len(rows), rows)
	}
}

// TestMetaRepository_ListArchetypesByFormat_CaseInsensitive verifies that the
// format filter is case-insensitive (lower(format) = lower($1)).
func TestMetaRepository_ListArchetypesByFormat_CaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	insertTestArchetype(t, db, "meta-repo-rg-aggro", "Standard", nil)

	// Query with lowercase — should match "Standard" stored in the DB.
	rows, err := repo.ListArchetypesByFormat(ctx, "standard", 0)
	if err != nil {
		t.Fatalf("ListArchetypesByFormat (case): %v", err)
	}
	found := false
	for _, a := range rows {
		if a.Name == "meta-repo-rg-aggro" {
			found = true
		}
	}
	if !found {
		t.Error("case-insensitive format filter failed: meta-repo-rg-aggro not found with lowercase query")
	}
}

// ----------------------------------------------------------------------------
// MetaRepository.LatestArchetypeUpdate
// ----------------------------------------------------------------------------

// TestMetaRepository_LatestArchetypeUpdate_HappyPath verifies that
// LatestArchetypeUpdate returns (ts, true, nil) when rows exist for the format.
func TestMetaRepository_LatestArchetypeUpdate_HappyPath(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	before := time.Now().UTC().Truncate(time.Second)
	insertTestArchetype(t, db, "meta-repo-latest-deck", "modern", nil)

	ts, ok, err := repo.LatestArchetypeUpdate(ctx, "modern")
	if err != nil {
		t.Fatalf("LatestArchetypeUpdate: %v", err)
	}
	if !ok {
		t.Fatal("LatestArchetypeUpdate: expected ok=true, got false")
	}
	if ts.Before(before) {
		t.Errorf("LatestArchetypeUpdate: ts %v is before seed time %v", ts, before)
	}
}

// TestMetaRepository_LatestArchetypeUpdate_NoRows verifies that
// LatestArchetypeUpdate returns (zero, false, nil) when no archetypes exist for
// the format (MAX returns NULL).
func TestMetaRepository_LatestArchetypeUpdate_NoRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	ts, ok, err := repo.LatestArchetypeUpdate(ctx, "format-no-rows-xyz")
	if err != nil {
		t.Fatalf("LatestArchetypeUpdate (no rows): %v", err)
	}
	if ok {
		t.Errorf("expected ok=false for empty format, got true (ts=%v)", ts)
	}
	if !ts.IsZero() {
		t.Errorf("expected zero time, got %v", ts)
	}
}

// ----------------------------------------------------------------------------
// MetaRepository.ArchetypeByName
// ----------------------------------------------------------------------------

// TestMetaRepository_ArchetypeByName_Found verifies that ArchetypeByName
// returns the correct row when the archetype exists.
func TestMetaRepository_ArchetypeByName_Found(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	insertTestArchetype(t, db, "meta-repo-found-deck", "explorer", strPtr("1"))

	row, err := repo.ArchetypeByName(ctx, "explorer", "meta-repo-found-deck")
	if err != nil {
		t.Fatalf("ArchetypeByName: %v", err)
	}
	if row == nil {
		t.Fatal("ArchetypeByName: expected non-nil, got nil")
	}
	if row.Name != "meta-repo-found-deck" {
		t.Errorf("Name: got %q, want %q", row.Name, "meta-repo-found-deck")
	}
	if row.Format != "explorer" {
		t.Errorf("Format: got %q, want %q", row.Format, "explorer")
	}
	if row.Tier == nil || *row.Tier != "1" {
		t.Errorf("Tier: got %v, want \"1\"", row.Tier)
	}
}

// TestMetaRepository_ArchetypeByName_NotFound verifies that ArchetypeByName
// returns (nil, nil) when the archetype does not exist.
func TestMetaRepository_ArchetypeByName_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	row, err := repo.ArchetypeByName(ctx, "standard", "archetype-does-not-exist-xyz")
	if err != nil {
		t.Fatalf("ArchetypeByName (not found): %v", err)
	}
	if row != nil {
		t.Errorf("expected nil, got %+v", row)
	}
}

// TestMetaRepository_ArchetypeByName_CaseInsensitive verifies that the lookup
// is case-insensitive for both format and name.
func TestMetaRepository_ArchetypeByName_CaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	insertTestArchetype(t, db, "meta-repo-ub-midrange", "Standard", nil)

	// Mixed-case query — should match.
	row, err := repo.ArchetypeByName(ctx, "STANDARD", "Meta-Repo-UB-Midrange")
	if err != nil {
		t.Fatalf("ArchetypeByName (case-insensitive): %v", err)
	}
	if row == nil {
		t.Fatal("ArchetypeByName: case-insensitive lookup returned nil")
	}
}

// ----------------------------------------------------------------------------
// MetaRepository.ArchetypeCardsByID
// ----------------------------------------------------------------------------

// TestMetaRepository_ArchetypeCardsByID_HappyPath verifies that
// ArchetypeCardsByID returns all cards for a given archetype id with the
// correct fields populated.
func TestMetaRepository_ArchetypeCardsByID_HappyPath(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	archetypeID := insertTestArchetype(t, db, "meta-repo-card-test-deck", "standard", nil)
	insertTestArchetypeCard(t, db, archetypeID, "Goblin Guide", "Creature", 4)
	insertTestArchetypeCard(t, db, archetypeID, "Lightning Bolt", "Removal", 4)
	insertTestArchetypeCard(t, db, archetypeID, "Mountain", "Common", 20)

	cards, err := repo.ArchetypeCardsByID(ctx, archetypeID)
	if err != nil {
		t.Fatalf("ArchetypeCardsByID: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(cards))
	}
	// Verify each expected card is present.
	byName := map[string]repository.ArchetypeCardRow{}
	for _, c := range cards {
		byName[c.CardName] = c
	}
	if _, ok := byName["Goblin Guide"]; !ok {
		t.Error("Goblin Guide missing from results")
	}
	if _, ok := byName["Lightning Bolt"]; !ok {
		t.Error("Lightning Bolt missing from results")
	}
	if _, ok := byName["Mountain"]; !ok {
		t.Error("Mountain missing from results")
	}
	if byName["Goblin Guide"].Role != "Creature" {
		t.Errorf("Goblin Guide Role: got %q, want %q", byName["Goblin Guide"].Role, "Creature")
	}
	if byName["Lightning Bolt"].Copies != 4 {
		t.Errorf("Lightning Bolt Copies: got %d, want 4", byName["Lightning Bolt"].Copies)
	}
}

// TestMetaRepository_ArchetypeCardsByID_NoCards verifies that
// ArchetypeCardsByID returns an empty (non-nil) slice when no cards are
// associated with the archetype.
func TestMetaRepository_ArchetypeCardsByID_NoCards(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	archetypeID := insertTestArchetype(t, db, "meta-repo-no-cards-deck", "standard", nil)

	cards, err := repo.ArchetypeCardsByID(ctx, archetypeID)
	if err != nil {
		t.Fatalf("ArchetypeCardsByID (no cards): %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("expected 0 cards, got %d: %v", len(cards), cards)
	}
}

// TestMetaRepository_ArchetypeCardsByID_CrossArchetypeIsolation verifies that
// ArchetypeCardsByID only returns cards for the requested archetype id and not
// cards belonging to a different archetype.
func TestMetaRepository_ArchetypeCardsByID_CrossArchetypeIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMetaRepository(db)
	ctx := context.Background()

	archetypeA := insertTestArchetype(t, db, "meta-repo-isolation-deck-a", "standard", nil)
	archetypeB := insertTestArchetype(t, db, "meta-repo-isolation-deck-b", "standard", nil)

	insertTestArchetypeCard(t, db, archetypeA, "Card A Only", "Creature", 4)
	insertTestArchetypeCard(t, db, archetypeB, "Card B Only", "Creature", 4)

	cards, err := repo.ArchetypeCardsByID(ctx, archetypeA)
	if err != nil {
		t.Fatalf("ArchetypeCardsByID (isolation): %v", err)
	}
	for _, c := range cards {
		if c.CardName == "Card B Only" {
			t.Errorf("cross-archetype isolation failure: Card B Only appeared in archetype A results")
		}
	}
	found := false
	for _, c := range cards {
		if c.CardName == "Card A Only" {
			found = true
		}
	}
	if !found {
		t.Error("Card A Only not found in archetype A results")
	}
}
