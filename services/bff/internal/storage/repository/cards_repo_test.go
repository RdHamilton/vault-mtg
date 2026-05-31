package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// seedSetCard inserts a minimal set_cards row keyed by (set_code, arena_id).
// Cleaned up via t.Cleanup.
func seedSetCard(t *testing.T, arenaID int, setCode, name, rarity string) {
	t.Helper()

	db := openTestDB(t)
	arenaIDText := fmt.Sprintf("%d", arenaID)

	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO set_cards (set_code, arena_id, scryfall_id, name, mana_cost, cmc, types,
		                       colors, rarity, text, power, toughness, image_url,
		                       image_url_small, image_url_art)
		VALUES ($1, $2, $3, $4, '1W', 2, 'Creature', '["W"]', $5,
		        'Flying.', '2', '2',
		        'https://img.scryfall.com/normal.jpg',
		        'https://img.scryfall.com/small.jpg',
		        'https://img.scryfall.com/art_crop.jpg')
		ON CONFLICT (set_code, arena_id) DO UPDATE SET name = EXCLUDED.name, rarity = EXCLUDED.rarity`,
		setCode, arenaIDText, "scryfall-"+arenaIDText, name, rarity,
	)
	if err != nil {
		t.Fatalf("seedSetCard arena_id=%d: %v", arenaID, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM set_cards WHERE set_code = $1 AND arena_id = $2`,
			setCode, arenaIDText,
		)
	})
}

// seedSet inserts a minimal sets row so LEFT JOIN sets resolves a set name.
// Cleaned up via t.Cleanup.
func seedSet(t *testing.T, code, name string) {
	t.Helper()

	db := openTestDB(t)
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO sets (code, name, set_type, card_count, last_updated)
		VALUES ($1, $2, 'expansion', 10, NOW())
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name`,
		code, name,
	)
	if err != nil {
		t.Fatalf("seedSet %q: %v", code, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM sets WHERE code = $1`, code)
	})
}

// TestCardsRepository_SearchCards verifies that SearchCards returns rows from
// set_cards (not the retired cards table).
func TestCardsRepository_SearchCards(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	seedSet(t, "sc1", "Test Set One")
	seedSetCard(t, 700001, "sc1", "Angelic Guardian", "rare")
	seedSetCard(t, 700002, "sc1", "Angelic Herald", "uncommon")
	seedSetCard(t, 700003, "sc1", "Fire Bolt", "common")

	rows, err := repo.SearchCards(ctx, "Angelic", "", 50)
	if err != nil {
		t.Fatalf("SearchCards: %v", err)
	}

	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows matching 'Angelic', got %d", len(rows))
	}

	for _, row := range rows {
		if row.ArenaID == 0 {
			t.Errorf("ArenaID must not be zero; got row: %+v", row)
		}
		if row.Name == "" {
			t.Error("Name must not be empty")
		}
	}
}

// TestCardsRepository_SearchCards_FilterBySet verifies set-code filtering.
func TestCardsRepository_SearchCards_FilterBySet(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	seedSet(t, "sc1", "Test Set One")
	seedSet(t, "sc2", "Test Set Two")
	seedSetCard(t, 700010, "sc1", "Lightning Dragon", "rare")
	seedSetCard(t, 700011, "sc2", "Lightning Wolf", "uncommon")

	rows, err := repo.SearchCards(ctx, "Lightning", "sc1", 50)
	if err != nil {
		t.Fatalf("SearchCards with set filter: %v", err)
	}

	for _, row := range rows {
		if row.SetCode != "sc1" {
			t.Errorf("expected SetCode=sc1, got %q", row.SetCode)
		}
	}

	// sc2 card must not appear.
	for _, row := range rows {
		if row.ArenaID == 700011 {
			t.Error("sc2 card must not appear when filtering by sc1")
		}
	}
}

// TestCardsRepository_CardByArenaID verifies that CardByArenaID reads from set_cards
// and returns the correct row.
func TestCardsRepository_CardByArenaID(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	seedSet(t, "sc1", "Test Set One")
	seedSetCard(t, 700020, "sc1", "Coastal Wizard", "mythic")

	row, err := repo.CardByArenaID(ctx, 700020)
	if err != nil {
		t.Fatalf("CardByArenaID: %v", err)
	}
	if row == nil {
		t.Fatal("expected non-nil row for seeded arena_id=700020")
	}

	if row.ArenaID != 700020 {
		t.Errorf("expected ArenaID=700020, got %d", row.ArenaID)
	}
	if row.Name != "Coastal Wizard" {
		t.Errorf("expected Name=%q, got %q", "Coastal Wizard", row.Name)
	}
	if row.SetCode != "sc1" {
		t.Errorf("expected SetCode=sc1, got %q", row.SetCode)
	}
}

// TestCardsRepository_CardByArenaID_NotFound verifies nil is returned for a
// missing arena_id.
func TestCardsRepository_CardByArenaID_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	row, err := repo.CardByArenaID(ctx, 999999999)
	if err != nil {
		t.Fatalf("CardByArenaID (not found): %v", err)
	}
	if row != nil {
		t.Errorf("expected nil for missing arena_id, got %+v", row)
	}
}

// TestCardsRepository_CardsBySetCode verifies that CardsBySetCode returns all
// cards for the given set from set_cards.
func TestCardsRepository_CardsBySetCode(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	seedSet(t, "sc3", "Test Set Three")
	seedSetCard(t, 700030, "sc3", "Forest Spirit", "common")
	seedSetCard(t, 700031, "sc3", "Mountain Drake", "uncommon")

	rows, err := repo.CardsBySetCode(ctx, "sc3")
	if err != nil {
		t.Fatalf("CardsBySetCode: %v", err)
	}

	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows for set sc3, got %d", len(rows))
	}

	for _, row := range rows {
		if row.SetCode != "sc3" {
			t.Errorf("expected SetCode=sc3, got %q", row.SetCode)
		}
		if row.ArenaID == 0 {
			t.Error("ArenaID must not be zero")
		}
	}
}

// TestCardsRepository_CardsBySetCode_CaseInsensitive verifies set code matching
// is case-insensitive.
func TestCardsRepository_CardsBySetCode_CaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	seedSet(t, "sc4", "Test Set Four")
	seedSetCard(t, 700040, "sc4", "Island Turtle", "common")

	rows, err := repo.CardsBySetCode(ctx, "SC4")
	if err != nil {
		t.Fatalf("CardsBySetCode (case-insensitive): %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("expected at least 1 row for uppercase set code SC4")
	}
}

// TestCardsRepository_ImageURLsPopulated verifies that the 3 discrete image URL
// columns (image_url, image_url_small, image_url_art) are returned by all card
// read paths instead of the retired '{}' blob.
func TestCardsRepository_ImageURLsPopulated(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardsRepository(db)
	ctx := context.Background()

	seedSet(t, "img1", "Image Test Set")
	seedSetCard(t, 710001, "img1", "Image Test Card", "rare")

	t.Run("SearchCards returns image URLs", func(t *testing.T) {
		rows, err := repo.SearchCards(ctx, "Image Test Card", "img1", 10)
		if err != nil {
			t.Fatalf("SearchCards: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected at least 1 row")
		}
		assertImageURLs(t, rows[0])
	})

	t.Run("CardByArenaID returns image URLs", func(t *testing.T) {
		row, err := repo.CardByArenaID(ctx, 710001)
		if err != nil {
			t.Fatalf("CardByArenaID: %v", err)
		}
		if row == nil {
			t.Fatal("expected non-nil row")
		}
		assertImageURLs(t, *row)
	})

	t.Run("CardsBySetCode returns image URLs", func(t *testing.T) {
		rows, err := repo.CardsBySetCode(ctx, "img1")
		if err != nil {
			t.Fatalf("CardsBySetCode: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected at least 1 row")
		}
		assertImageURLs(t, rows[0])
	})

	t.Run("SearchCardsWithCollection returns image URLs", func(t *testing.T) {
		rows, err := repo.SearchCardsWithCollection(ctx, 0, "Image Test Card", []string{"img1"}, 10)
		if err != nil {
			t.Fatalf("SearchCardsWithCollection: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected at least 1 row")
		}
		assertImageURLs(t, rows[0].SetCardRow)
	})
}

// assertImageURLs fails t if any of the 3 image URL fields are empty or still
// hold the retired '{}' sentinel value.
func assertImageURLs(t *testing.T, row repository.SetCardRow) {
	t.Helper()
	if row.ImageURL == "" {
		t.Errorf("ImageURL must not be empty; got SetCardRow: %+v", row)
	}
	if row.ImageURLSmall == "" {
		t.Errorf("ImageURLSmall must not be empty; got SetCardRow: %+v", row)
	}
	if row.ImageURLArt == "" {
		t.Errorf("ImageURLArt must not be empty; got SetCardRow: %+v", row)
	}
	if row.ImageURL == "{}" || row.ImageURLSmall == "{}" || row.ImageURLArt == "{}" {
		t.Errorf("image URL field still holds retired '{}' sentinel; got SetCardRow: %+v", row)
	}
}
