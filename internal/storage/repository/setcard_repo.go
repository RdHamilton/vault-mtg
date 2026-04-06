package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards/search"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// MetadataStaleness contains counts of fresh/stale metadata.
type MetadataStaleness struct {
	Total     int
	Fresh     int
	Stale     int
	VeryStale int
}

// StaleCard represents a card with stale metadata.
type StaleCard struct {
	ArenaID     string
	SetCode     string
	LastUpdated string
}

// SetRarityCount represents card counts for a set and rarity.
type SetRarityCount struct {
	SetCode string
	SetName string
	Rarity  string
	Total   int
}

// CardSetInfo represents the set/rarity info for a card.
type CardSetInfo struct {
	ArenaID string
	SetCode string
	Rarity  string
}

// SetCardRepository provides methods for managing set cards cached from Scryfall.
type SetCardRepository interface {
	// SaveCard saves a set card to the database.
	SaveCard(ctx context.Context, card *models.SetCard) error

	// SaveCards saves multiple set cards in a batch.
	SaveCards(ctx context.Context, cards []*models.SetCard) error

	// GetCardByArenaID retrieves a card by its Arena ID.
	GetCardByArenaID(ctx context.Context, arenaID string) (*models.SetCard, error)

	// GetCardsBySet retrieves all cards for a given set code.
	GetCardsBySet(ctx context.Context, setCode string) ([]*models.SetCard, error)

	// SearchCards searches for cards by name, oracle text, or type across all cached sets.
	// Supports prefix syntax: t:type, o:text, k:keyword. Optionally filter by set codes.
	// Returns up to limit results.
	SearchCards(ctx context.Context, query string, setCodes []string, limit int) ([]*models.SetCard, error)

	// IsSetCached checks if a set has been cached.
	IsSetCached(ctx context.Context, setCode string) (bool, error)

	// GetCachedSets returns a list of all cached set codes.
	GetCachedSets(ctx context.Context) ([]string, error)

	// DeleteSet removes all cards for a given set (for cache invalidation).
	DeleteSet(ctx context.Context, setCode string) error

	// Staleness tracking methods
	// GetMetadataStaleness returns counts of fresh, stale, and very stale cards.
	GetMetadataStaleness(ctx context.Context, staleAgeSeconds, veryStaleAgeSeconds int) (*MetadataStaleness, error)

	// GetStaleCards returns cards with stale metadata, ordered by oldest first.
	GetStaleCards(ctx context.Context, staleAgeSeconds, limit int) ([]*StaleCard, error)

	// Set completion methods
	// GetSetRarityCounts returns card counts grouped by set and rarity, with set names.
	GetSetRarityCounts(ctx context.Context) ([]*SetRarityCount, error)

	// GetAllCardSetInfo returns arena_id, set_code, and rarity for all cards.
	GetAllCardSetInfo(ctx context.Context) ([]*CardSetInfo, error)

	// GetSetCardCount returns the number of cards cached for a given set.
	GetSetCardCount(ctx context.Context, setCode string) (int, error)
}

type setCardRepository struct {
	db *sql.DB
}

// NewSetCardRepository creates a new set card repository.
func NewSetCardRepository(db *sql.DB) SetCardRepository {
	return &setCardRepository{db: db}
}

// SaveCard saves a set card to the database.
func (r *setCardRepository) SaveCard(ctx context.Context, card *models.SetCard) error {
	// Marshal arrays to JSON
	typesJSON, err := json.Marshal(card.Types)
	if err != nil {
		return err
	}

	colorsJSON, err := json.Marshal(card.Colors)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO set_cards (
			set_code, arena_id, scryfall_id, name, mana_cost, cmc, types, colors,
			rarity, text, power, toughness, image_url, image_url_small, image_url_art, fetched_at,
			price_usd, price_usd_foil, price_eur, price_eur_foil, price_tix, prices_updated_at, legalities
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(set_code, arena_id) DO UPDATE SET
			scryfall_id = excluded.scryfall_id,
			name = excluded.name,
			mana_cost = excluded.mana_cost,
			cmc = excluded.cmc,
			types = excluded.types,
			colors = excluded.colors,
			rarity = excluded.rarity,
			text = excluded.text,
			power = excluded.power,
			toughness = excluded.toughness,
			image_url = excluded.image_url,
			image_url_small = excluded.image_url_small,
			image_url_art = excluded.image_url_art,
			fetched_at = excluded.fetched_at,
			price_usd = excluded.price_usd,
			price_usd_foil = excluded.price_usd_foil,
			price_eur = excluded.price_eur,
			price_eur_foil = excluded.price_eur_foil,
			price_tix = excluded.price_tix,
			prices_updated_at = excluded.prices_updated_at,
			legalities = excluded.legalities
	`
	result, err := r.db.ExecContext(ctx, query,
		card.SetCode,
		card.ArenaID,
		card.ScryfallID,
		card.Name,
		card.ManaCost,
		card.CMC,
		string(typesJSON),
		string(colorsJSON),
		card.Rarity,
		card.Text,
		card.Power,
		card.Toughness,
		card.ImageURL,
		card.ImageURLSmall,
		card.ImageURLArt,
		card.FetchedAt,
		card.PriceUSD,
		card.PriceUSDFoil,
		card.PriceEUR,
		card.PriceEURFoil,
		card.PriceTIX,
		card.PricesUpdatedAt,
		card.Legalities,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err == nil {
		card.ID = int(id)
	}

	return nil
}

// SaveCards saves multiple set cards in a batch.
func (r *setCardRepository) SaveCards(ctx context.Context, cards []*models.SetCard) error {
	// Start a transaction for batch insert
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO set_cards (
			set_code, arena_id, scryfall_id, name, mana_cost, cmc, types, colors,
			rarity, text, power, toughness, image_url, image_url_small, image_url_art, fetched_at,
			price_usd, price_usd_foil, price_eur, price_eur_foil, price_tix, prices_updated_at, legalities
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(set_code, arena_id) DO UPDATE SET
			scryfall_id = excluded.scryfall_id,
			name = excluded.name,
			mana_cost = excluded.mana_cost,
			cmc = excluded.cmc,
			types = excluded.types,
			colors = excluded.colors,
			rarity = excluded.rarity,
			text = excluded.text,
			power = excluded.power,
			toughness = excluded.toughness,
			image_url = excluded.image_url,
			image_url_small = excluded.image_url_small,
			image_url_art = excluded.image_url_art,
			fetched_at = excluded.fetched_at,
			price_usd = excluded.price_usd,
			price_usd_foil = excluded.price_usd_foil,
			price_eur = excluded.price_eur,
			price_eur_foil = excluded.price_eur_foil,
			price_tix = excluded.price_tix,
			prices_updated_at = excluded.prices_updated_at,
			legalities = excluded.legalities
	`)
	if err != nil {
		return err
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, card := range cards {
		typesJSON, err := json.Marshal(card.Types)
		if err != nil {
			return err
		}

		colorsJSON, err := json.Marshal(card.Colors)
		if err != nil {
			return err
		}

		_, err = stmt.ExecContext(ctx,
			card.SetCode,
			card.ArenaID,
			card.ScryfallID,
			card.Name,
			card.ManaCost,
			card.CMC,
			string(typesJSON),
			string(colorsJSON),
			card.Rarity,
			card.Text,
			card.Power,
			card.Toughness,
			card.ImageURL,
			card.ImageURLSmall,
			card.ImageURLArt,
			card.FetchedAt,
			card.PriceUSD,
			card.PriceUSDFoil,
			card.PriceEUR,
			card.PriceEURFoil,
			card.PriceTIX,
			card.PricesUpdatedAt,
			card.Legalities,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetCardByArenaID retrieves a card by its Arena ID.
func (r *setCardRepository) GetCardByArenaID(ctx context.Context, arenaID string) (*models.SetCard, error) {
	query := `
		SELECT id, set_code, arena_id, scryfall_id, name, mana_cost, cmc, types, colors,
			   rarity, text, power, toughness, image_url, image_url_small, image_url_art, fetched_at,
			   price_usd, price_usd_foil, price_eur, price_eur_foil, price_tix, prices_updated_at, legalities
		FROM set_cards
		WHERE arena_id = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, arenaID)

	card := &models.SetCard{}
	var typesJSON, colorsJSON string
	var legalities sql.NullString

	err := row.Scan(
		&card.ID,
		&card.SetCode,
		&card.ArenaID,
		&card.ScryfallID,
		&card.Name,
		&card.ManaCost,
		&card.CMC,
		&typesJSON,
		&colorsJSON,
		&card.Rarity,
		&card.Text,
		&card.Power,
		&card.Toughness,
		&card.ImageURL,
		&card.ImageURLSmall,
		&card.ImageURLArt,
		&card.FetchedAt,
		&card.PriceUSD,
		&card.PriceUSDFoil,
		&card.PriceEUR,
		&card.PriceEURFoil,
		&card.PriceTIX,
		&card.PricesUpdatedAt,
		&legalities,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Parse JSON arrays
	if err := json.Unmarshal([]byte(typesJSON), &card.Types); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(colorsJSON), &card.Colors); err != nil {
		return nil, err
	}
	if legalities.Valid {
		card.Legalities = legalities.String
	}

	return card, nil
}

// GetCardsBySet retrieves all cards for a given set code.
func (r *setCardRepository) GetCardsBySet(ctx context.Context, setCode string) ([]*models.SetCard, error) {
	query := `
		SELECT id, set_code, arena_id, scryfall_id, name, mana_cost, cmc, types, colors,
			   rarity, text, power, toughness, image_url, image_url_small, image_url_art, fetched_at,
			   price_usd, price_usd_foil, price_eur, price_eur_foil, price_tix, prices_updated_at, legalities
		FROM set_cards
		WHERE set_code = ?
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query, setCode)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	cards := []*models.SetCard{}
	for rows.Next() {
		card := &models.SetCard{}
		var typesJSON, colorsJSON string
		var legalities sql.NullString

		err := rows.Scan(
			&card.ID,
			&card.SetCode,
			&card.ArenaID,
			&card.ScryfallID,
			&card.Name,
			&card.ManaCost,
			&card.CMC,
			&typesJSON,
			&colorsJSON,
			&card.Rarity,
			&card.Text,
			&card.Power,
			&card.Toughness,
			&card.ImageURL,
			&card.ImageURLSmall,
			&card.ImageURLArt,
			&card.FetchedAt,
			&card.PriceUSD,
			&card.PriceUSDFoil,
			&card.PriceEUR,
			&card.PriceEURFoil,
			&card.PriceTIX,
			&card.PricesUpdatedAt,
			&legalities,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON arrays
		if err := json.Unmarshal([]byte(typesJSON), &card.Types); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(colorsJSON), &card.Colors); err != nil {
			return nil, err
		}
		if legalities.Valid {
			card.Legalities = legalities.String
		}

		cards = append(cards, card)
	}

	return cards, rows.Err()
}

// IsSetCached checks if a set has been cached.
func (r *setCardRepository) IsSetCached(ctx context.Context, setCode string) (bool, error) {
	query := `SELECT COUNT(*) FROM set_cards WHERE set_code = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, setCode).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetCachedSets returns a list of all cached set codes.
func (r *setCardRepository) GetCachedSets(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT set_code FROM set_cards ORDER BY set_code`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	sets := []string{}
	for rows.Next() {
		var setCode string
		if err := rows.Scan(&setCode); err != nil {
			return nil, err
		}
		sets = append(sets, setCode)
	}

	return sets, rows.Err()
}

// DeleteSet removes all cards for a given set (for cache invalidation).
func (r *setCardRepository) DeleteSet(ctx context.Context, setCode string) error {
	query := `DELETE FROM set_cards WHERE set_code = ?`
	_, err := r.db.ExecContext(ctx, query, setCode)
	return err
}

// SearchCards searches for cards by name, oracle text, or type across all cached sets.
// Supports prefix syntax: t:type, o:text, k:keyword. If no prefix is used, all fields are searched.
// If setCodes is empty, searches all sets. Returns up to limit results.
// Cards matching by name are prioritized over those matching only by text or type.
func (r *setCardRepository) SearchCards(ctx context.Context, query string, setCodes []string, limit int) ([]*models.SetCard, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 200 {
		limit = 200 // Max limit to prevent performance issues
	}

	parsed := search.Parse(query)
	if parsed.IsEmpty() {
		return []*models.SetCard{}, nil
	}

	var conditions []string
	var args []interface{}

	for _, term := range parsed.Terms {
		pattern := "%" + term.Value + "%"
		switch term.Field {
		case search.FieldAll:
			conditions = append(conditions, "(name LIKE ? OR text LIKE ? OR types LIKE ?)")
			args = append(args, pattern, pattern, pattern)
		case search.FieldType:
			conditions = append(conditions, "types LIKE ?")
			args = append(args, pattern)
		case search.FieldText, search.FieldKeyword:
			conditions = append(conditions, "text LIKE ?")
			args = append(args, pattern)
		}
	}

	whereClause := strings.Join(conditions, " AND ")

	// Set code filter
	if len(setCodes) > 0 {
		placeholders := make([]string, len(setCodes))
		for i, code := range setCodes {
			placeholders[i] = "?"
			args = append(args, code)
		}
		whereClause += " AND set_code IN (" + strings.Join(placeholders, ",") + ")"
	}

	// ORDER BY: prioritize name matches for the first FieldAll term
	orderClause := "name"
	for _, term := range parsed.Terms {
		if term.Field == search.FieldAll {
			namePattern := "%" + term.Value + "%"
			orderClause = "CASE WHEN name LIKE ? THEN 0 ELSE 1 END, name"
			args = append(args, namePattern)
			break
		}
	}

	args = append(args, limit)

	sqlQuery := `
		SELECT id, set_code, arena_id, scryfall_id, name, mana_cost, cmc, types, colors,
			   rarity, text, power, toughness, image_url, image_url_small, image_url_art, fetched_at,
			   price_usd, price_usd_foil, price_eur, price_eur_foil, price_tix, prices_updated_at, legalities
		FROM set_cards
		WHERE ` + whereClause + `
		ORDER BY ` + orderClause + `
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	cards := []*models.SetCard{}
	for rows.Next() {
		card := &models.SetCard{}
		var typesJSON, colorsJSON string
		var legalities sql.NullString

		err := rows.Scan(
			&card.ID,
			&card.SetCode,
			&card.ArenaID,
			&card.ScryfallID,
			&card.Name,
			&card.ManaCost,
			&card.CMC,
			&typesJSON,
			&colorsJSON,
			&card.Rarity,
			&card.Text,
			&card.Power,
			&card.Toughness,
			&card.ImageURL,
			&card.ImageURLSmall,
			&card.ImageURLArt,
			&card.FetchedAt,
			&card.PriceUSD,
			&card.PriceUSDFoil,
			&card.PriceEUR,
			&card.PriceEURFoil,
			&card.PriceTIX,
			&card.PricesUpdatedAt,
			&legalities,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON arrays
		if err := json.Unmarshal([]byte(typesJSON), &card.Types); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(colorsJSON), &card.Colors); err != nil {
			return nil, err
		}
		if legalities.Valid {
			card.Legalities = legalities.String
		}

		cards = append(cards, card)
	}

	return cards, rows.Err()
}

// GetMetadataStaleness returns counts of fresh, stale, and very stale cards.
func (r *setCardRepository) GetMetadataStaleness(ctx context.Context, staleAgeSeconds, veryStaleAgeSeconds int) (*MetadataStaleness, error) {
	query := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN fetched_at >= datetime('now', '-' || ? || ' seconds') THEN 1 ELSE 0 END), 0) as fresh,
			COALESCE(SUM(CASE WHEN fetched_at < datetime('now', '-' || ? || ' seconds')
				AND fetched_at >= datetime('now', '-' || ? || ' seconds') THEN 1 ELSE 0 END), 0) as stale,
			COALESCE(SUM(CASE WHEN fetched_at < datetime('now', '-' || ? || ' seconds') THEN 1 ELSE 0 END), 0) as very_stale
		FROM set_cards
		WHERE fetched_at IS NOT NULL
	`

	var result MetadataStaleness
	err := r.db.QueryRowContext(ctx, query,
		staleAgeSeconds,
		staleAgeSeconds,
		veryStaleAgeSeconds,
		veryStaleAgeSeconds,
	).Scan(&result.Total, &result.Fresh, &result.Stale, &result.VeryStale)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetStaleCards returns cards with stale metadata, ordered by oldest first.
func (r *setCardRepository) GetStaleCards(ctx context.Context, staleAgeSeconds, limit int) ([]*StaleCard, error) {
	query := `
		SELECT arena_id, set_code, fetched_at
		FROM set_cards
		WHERE fetched_at < datetime('now', '-' || ? || ' seconds')
		ORDER BY fetched_at ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, staleAgeSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cards []*StaleCard
	for rows.Next() {
		card := &StaleCard{}
		if err := rows.Scan(&card.ArenaID, &card.SetCode, &card.LastUpdated); err != nil {
			continue
		}
		cards = append(cards, card)
	}

	return cards, rows.Err()
}

// GetSetRarityCounts returns card counts grouped by set and rarity, with set names.
func (r *setCardRepository) GetSetRarityCounts(ctx context.Context) ([]*SetRarityCount, error) {
	query := `
		SELECT
			sc.set_code,
			COALESCE(st.name, UPPER(sc.set_code)) as set_name,
			sc.rarity,
			COUNT(*) as total
		FROM set_cards sc
		LEFT JOIN sets st ON sc.set_code = st.code
		GROUP BY sc.set_code, sc.rarity
		ORDER BY sc.set_code, sc.rarity
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var counts []*SetRarityCount
	for rows.Next() {
		count := &SetRarityCount{}
		if err := rows.Scan(&count.SetCode, &count.SetName, &count.Rarity, &count.Total); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}

	return counts, rows.Err()
}

// GetAllCardSetInfo returns arena_id, set_code, and rarity for all cards.
func (r *setCardRepository) GetAllCardSetInfo(ctx context.Context) ([]*CardSetInfo, error) {
	query := `
		SELECT arena_id, set_code, rarity
		FROM set_cards
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cardInfos []*CardSetInfo
	for rows.Next() {
		card := &CardSetInfo{}
		if err := rows.Scan(&card.ArenaID, &card.SetCode, &card.Rarity); err != nil {
			return nil, err
		}
		cardInfos = append(cardInfos, card)
	}

	return cardInfos, rows.Err()
}

// GetSetCardCount returns the number of cards cached for a given set.
func (r *setCardRepository) GetSetCardCount(ctx context.Context, setCode string) (int, error) {
	query := `SELECT COUNT(*) FROM set_cards WHERE set_code = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, setCode).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
