package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// CardsRepository serves the Phase 2 /api/v1/cards/* surface — card
// metadata lookups, set catalog, and 17Lands + ChannelFireball ratings.
// Most endpoints are global (catalog data, no account scope).
// getCollectionQuantities + searchCardsWithCollection are the two
// account-scoped queries — they join card_inventory.
type CardsRepository struct {
	db DB
}

// NewCardsRepository returns a CardsRepository backed by db.
func NewCardsRepository(db DB) *CardsRepository {
	return &CardsRepository{db: db}
}

// SetCardRow mirrors a card metadata row joined from cards + sets +
// set_cards (for prices). Reused by /cards/{arenaId}, /cards search,
// /cards/sets/{setCode}/cards, and /cards/search-with-collection.
type SetCardRow struct {
	CardID        int
	ArenaID       int
	Name          string
	SetCode       string
	SetName       string
	Rarity        string
	ManaCost      string
	CMC           float64
	TypeLine      string
	Colors        string // JSON array TEXT
	ColorIdentity string
	ImageURIs     string
	Power         *string
	Toughness     *string
	PriceUSD      *float64
	PriceUSDFoil  *float64
	PriceEUR      *float64
	PricesUpdated *int64
}

// SearchCards runs a name-substring search over cards. Optional set/limit
// narrow the result; default limit is 50, max 200. Returns rows with the
// joined set/price metadata so the SPA can render rich result cards.
func (r *CardsRepository) SearchCards(ctx context.Context, query, setCode string, limit int) ([]SetCardRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	clauses := []string{"c.name ILIKE $1"}
	args := []any{"%" + strings.TrimSpace(query) + "%"}
	next := 2
	if setCode != "" {
		clauses = append(clauses, "lower(c.set_code) = lower($"+strconv.Itoa(next)+")")
		args = append(args, setCode)
		next++
	}
	q := `SELECT c.arena_id::INTEGER, c.arena_id::INTEGER AS card_id,
	             c.name, COALESCE(c.set_code, ''), COALESCE(s.name, ''),
	             COALESCE(c.rarity, ''), COALESCE(c.mana_cost, ''),
	             COALESCE(c.cmc, 0), COALESCE(c.types, ''),
	             COALESCE(c.colors, '[]'), '' AS color_identity,
	             '{}' AS image_uris,
	             c.power, c.toughness,
	             c.price_usd, c.price_usd_foil, c.price_eur,
	             EXTRACT(EPOCH FROM c.prices_updated_at)::BIGINT
	      FROM set_cards c
	      LEFT JOIN sets s ON s.code = c.set_code
	      WHERE c.arena_id IS NOT NULL AND ` + strings.Join(clauses, " AND ") + `
	      ORDER BY c.name
	      LIMIT $` + strconv.Itoa(next)
	args = append(args, limit)
	return r.scanSetCardRows(ctx, q, args...)
}

// CardByArenaID returns the card with the given arena id. Returns nil
// when not found.
func (r *CardsRepository) CardByArenaID(ctx context.Context, arenaID int) (*SetCardRow, error) {
	const q = `SELECT c.arena_id::INTEGER, c.arena_id::INTEGER AS card_id,
	                  c.name, COALESCE(c.set_code, ''), COALESCE(s.name, ''),
	                  COALESCE(c.rarity, ''), COALESCE(c.mana_cost, ''),
	                  COALESCE(c.cmc, 0), COALESCE(c.types, ''),
	                  COALESCE(c.colors, '[]'), '' AS color_identity,
	                  '{}' AS image_uris,
	                  c.power, c.toughness,
	                  c.price_usd, c.price_usd_foil, c.price_eur,
	                  EXTRACT(EPOCH FROM c.prices_updated_at)::BIGINT
	           FROM set_cards c
	           LEFT JOIN sets s ON s.code = c.set_code
	           WHERE c.arena_id::INTEGER = $1
	           LIMIT 1`
	rows, err := r.scanSetCardRows(ctx, q, arenaID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// CardsBySetCode returns every card in setCode, ordered by name.
func (r *CardsRepository) CardsBySetCode(ctx context.Context, setCode string) ([]SetCardRow, error) {
	const q = `SELECT c.arena_id::INTEGER, c.arena_id::INTEGER AS card_id,
	                  c.name, COALESCE(c.set_code, ''), COALESCE(s.name, ''),
	                  COALESCE(c.rarity, ''), COALESCE(c.mana_cost, ''),
	                  COALESCE(c.cmc, 0), COALESCE(c.types, ''),
	                  COALESCE(c.colors, '[]'), '' AS color_identity,
	                  '{}' AS image_uris,
	                  c.power, c.toughness,
	                  c.price_usd, c.price_usd_foil, c.price_eur,
	                  EXTRACT(EPOCH FROM c.prices_updated_at)::BIGINT
	           FROM set_cards c
	           LEFT JOIN sets s ON s.code = c.set_code
	           WHERE lower(c.set_code) = lower($1) AND c.arena_id IS NOT NULL
	           ORDER BY c.name
	           LIMIT 1000`
	return r.scanSetCardRows(ctx, q, setCode)
}

// SearchCardsWithCollection runs a name-substring search joined to the
// account's card_inventory; returns rows + per-card quantity. Limited to
// the requested limit (max 200).
type SetCardWithQty struct {
	SetCardRow
	Quantity int
}

// SearchCardsWithCollection runs a name search and joins card_inventory
// for the given account so results carry per-card quantities. Sets is an
// optional list of set codes to narrow the search.
func (r *CardsRepository) SearchCardsWithCollection(ctx context.Context, accountID int64, query string, sets []string, limit int) ([]SetCardWithQty, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	clauses := []string{"c.name ILIKE $1", "c.arena_id IS NOT NULL"}
	args := []any{"%" + strings.TrimSpace(query) + "%"}
	next := 2
	if len(sets) > 0 {
		clauses = append(clauses, "lower(c.set_code) = ANY($"+strconv.Itoa(next)+")")
		args = append(args, lowerSlice(sets))
		next++
	}
	q := `SELECT c.arena_id::INTEGER, c.arena_id::INTEGER AS card_id,
	             c.name, COALESCE(c.set_code, ''), COALESCE(s.name, ''),
	             COALESCE(c.rarity, ''), COALESCE(c.mana_cost, ''),
	             COALESCE(c.cmc, 0), COALESCE(c.types, ''),
	             COALESCE(c.colors, '[]'), '' AS color_identity,
	             '{}' AS image_uris,
	             c.power, c.toughness,
	             c.price_usd, c.price_usd_foil, c.price_eur,
	             EXTRACT(EPOCH FROM c.prices_updated_at)::BIGINT,
	             COALESCE(ci.count, 0) AS qty
	      FROM set_cards c
	      LEFT JOIN sets s ON s.code = c.set_code
	      LEFT JOIN card_inventory ci ON ci.account_id = $` + strconv.Itoa(next) + ` AND ci.card_id = c.arena_id::INTEGER
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY c.name
	      LIMIT $` + strconv.Itoa(next+1)
	args = append(args, accountID, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SetCardWithQty
	for rows.Next() {
		var row SetCardWithQty
		if err := rows.Scan(
			&row.ArenaID, &row.CardID, &row.Name, &row.SetCode, &row.SetName,
			&row.Rarity, &row.ManaCost, &row.CMC, &row.TypeLine,
			&row.Colors, &row.ColorIdentity, &row.ImageURIs,
			&row.Power, &row.Toughness,
			&row.PriceUSD, &row.PriceUSDFoil, &row.PriceEUR, &row.PricesUpdated,
			&row.Quantity,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// CollectionQuantities returns a map of arena_id → count for the requested
// arena ids, scoped to the account. Missing entries default to 0 and are
// not present in the returned map.
func (r *CardsRepository) CollectionQuantities(ctx context.Context, accountID int64, arenaIDs []int) (map[int]int, error) {
	if len(arenaIDs) == 0 {
		return map[int]int{}, nil
	}
	const q = `SELECT card_id, count
	           FROM card_inventory
	           WHERE account_id = $1 AND card_id = ANY($2) AND count > 0`
	rows, err := r.db.QueryContext(ctx, q, accountID, intSliceToInt64Slice(arenaIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int]int{}
	for rows.Next() {
		var id, n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// SetInfoRow mirrors gui.SetInfo.
type SetInfoRow struct {
	Code       string
	Name       string
	IconSvgURI string
	SetType    string
	ReleasedAt string
	CardCount  int
}

// AllSetInfo returns every row from sets, sorted by released_at DESC.
func (r *CardsRepository) AllSetInfo(ctx context.Context) ([]SetInfoRow, error) {
	const q = `SELECT code, name, COALESCE(icon_svg_uri, ''), COALESCE(set_type, ''),
	                  COALESCE(released_at, ''), COALESCE(card_count, 0)
	           FROM sets
	           ORDER BY released_at DESC NULLS LAST, code
	           LIMIT 500`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SetInfoRow
	for rows.Next() {
		var s SetInfoRow
		if err := rows.Scan(&s.Code, &s.Name, &s.IconSvgURI, &s.SetType, &s.ReleasedAt, &s.CardCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CardRatingRow mirrors a draft_card_ratings row.
type CardRatingRow struct {
	ArenaID  int
	Name     string
	Color    *string
	Rarity   *string
	GIHWR    *float64
	OHWR     *float64
	ALSA     *float64
	ATA      *float64
	GIHCount *int
	URL      *string
	URLBack  *string
	CachedAt time.Time
}

// CardRatings returns 17Lands draft_card_ratings for (set, format),
// ordered by gihwr DESC. Plus the dataset's cached_at + degraded flag.
func (r *CardsRepository) CardRatings(ctx context.Context, setCode, format string) ([]CardRatingRow, error) {
	const q = `SELECT arena_id, name, color, rarity, gihwr, ohwr, alsa, ata,
	                  gih_count, url, url_back, cached_at
	           FROM draft_card_ratings
	           WHERE lower(set_code) = lower($1) AND lower(draft_format) = lower($2)
	           ORDER BY gihwr DESC NULLS LAST, name`
	rows, err := r.db.QueryContext(ctx, q, setCode, format)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CardRatingRow
	for rows.Next() {
		var rr CardRatingRow
		if err := rows.Scan(
			&rr.ArenaID, &rr.Name, &rr.Color, &rr.Rarity,
			&rr.GIHWR, &rr.OHWR, &rr.ALSA, &rr.ATA, &rr.GIHCount,
			&rr.URL, &rr.URLBack, &rr.CachedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

// ColorRatingRow mirrors a draft_color_ratings row.
type ColorRatingRow struct {
	ColorCombination string
	WinRate          *float64
	GamesPlayed      *int
}

// ColorRatings returns 17Lands draft_color_ratings for the set, ordered
// by win_rate DESC.
func (r *CardsRepository) ColorRatings(ctx context.Context, setCode string) ([]ColorRatingRow, error) {
	const q = `SELECT color_combination, win_rate, games_played
	           FROM draft_color_ratings
	           WHERE lower(set_code) = lower($1)
	           ORDER BY win_rate DESC NULLS LAST`
	rows, err := r.db.QueryContext(ctx, q, setCode)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ColorRatingRow
	for rows.Next() {
		var c ColorRatingRow
		if err := rows.Scan(&c.ColorCombination, &c.WinRate, &c.GamesPlayed); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RatingsStaleness mirrors the dataset_metadata staleness summary.
type RatingsStalenessRow struct {
	CachedAt  *time.Time
	IsStale   bool
	CardCount int
}

// RatingsStaleness returns the cached_at timestamp + card count for a
// (set, format). isStale is true when cached_at is older than 7 days.
func (r *CardsRepository) RatingsStaleness(ctx context.Context, setCode, format string) (RatingsStalenessRow, error) {
	const q = `SELECT MAX(cached_at), COUNT(*)
	           FROM draft_card_ratings
	           WHERE lower(set_code) = lower($1) AND lower(draft_format) = lower($2)`
	var s RatingsStalenessRow
	if err := r.db.QueryRowContext(ctx, q, setCode, format).Scan(&s.CachedAt, &s.CardCount); err != nil {
		if err == sql.ErrNoRows {
			return RatingsStalenessRow{}, nil
		}
		return RatingsStalenessRow{}, err
	}
	if s.CachedAt != nil {
		s.IsStale = time.Since(*s.CachedAt) > 7*24*time.Hour
	} else {
		s.IsStale = true
	}
	return s, nil
}

// TouchRatingsCachedAt bumps cached_at for every draft_card_ratings row
// matching (set, format). Used by the refresh stub to acknowledge the
// SPA's "force refresh" click without actually scraping 17Lands.
func (r *CardsRepository) TouchRatingsCachedAt(ctx context.Context, setCode, format string) error {
	const q = `UPDATE draft_card_ratings
	           SET cached_at = NOW()
	           WHERE lower(set_code) = lower($1) AND lower(draft_format) = lower($2)`
	_, err := r.db.ExecContext(ctx, q, setCode, format)
	return err
}

// CFBRatingRow mirrors cfb_ratings.
type CFBRatingRow struct {
	ID                int64
	CardName          string
	SetCode           string
	ArenaID           *int
	LimitedRating     float64
	LimitedScore      float64
	ConstructedRating *string
	ConstructedScore  float64
	ArchetypeFit      *string
	Commentary        *string
	SourceURL         *string
	Author            *string
	ImportedAt        time.Time
	UpdatedAt         time.Time
}

// CFBRatingsBySet returns cfb_ratings for the set, ordered by
// limited_rating DESC.
func (r *CardsRepository) CFBRatingsBySet(ctx context.Context, setCode string) ([]CFBRatingRow, error) {
	const q = `SELECT id, card_name, set_code, arena_id, limited_rating, limited_score,
	                  constructed_rating, constructed_score, archetype_fit, commentary,
	                  source_url, author, imported_at, updated_at
	           FROM cfb_ratings
	           WHERE lower(set_code) = lower($1)
	           ORDER BY limited_rating DESC NULLS LAST, card_name`
	return r.scanCFBRows(ctx, q, setCode)
}

// CFBRatingByCard returns the cfb_ratings row matching (set, card_name).
// Returns nil when not found.
func (r *CardsRepository) CFBRatingByCard(ctx context.Context, setCode, cardName string) (*CFBRatingRow, error) {
	const q = `SELECT id, card_name, set_code, arena_id, limited_rating, limited_score,
	                  constructed_rating, constructed_score, archetype_fit, commentary,
	                  source_url, author, imported_at, updated_at
	           FROM cfb_ratings
	           WHERE lower(set_code) = lower($1) AND lower(card_name) = lower($2)
	           LIMIT 1`
	rows, err := r.scanCFBRows(ctx, q, setCode, cardName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// CFBRatingsCount returns the row count for the set's CFB ratings.
func (r *CardsRepository) CFBRatingsCount(ctx context.Context, setCode string) (int, error) {
	const q = `SELECT COUNT(*) FROM cfb_ratings WHERE lower(set_code) = lower($1)`
	var n int
	if err := r.db.QueryRowContext(ctx, q, setCode).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CFBImport is one row to upsert into cfb_ratings.
type CFBImport struct {
	CardName          string
	SetCode           string
	LimitedRating     float64
	ConstructedRating *string
	ArchetypeFit      *string
	Commentary        *string
	SourceURL         *string
	Author            *string
}

// ImportCFBRatings upserts (card_name, set_code, ...) rows. Returns the
// number of rows inserted/updated. Score is the rating divided by 5
// (rating is on a 0-5 scale, score is normalised 0-1).
func (r *CardsRepository) ImportCFBRatings(ctx context.Context, imports []CFBImport) (int, error) {
	if len(imports) == 0 {
		return 0, nil
	}
	const q = `INSERT INTO cfb_ratings (card_name, set_code, limited_rating, limited_score,
	                                    constructed_rating, archetype_fit, commentary,
	                                    source_url, author, imported_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	           ON CONFLICT (card_name, set_code) DO UPDATE SET
	             limited_rating = EXCLUDED.limited_rating,
	             limited_score  = EXCLUDED.limited_score,
	             constructed_rating = EXCLUDED.constructed_rating,
	             archetype_fit  = EXCLUDED.archetype_fit,
	             commentary     = EXCLUDED.commentary,
	             source_url     = EXCLUDED.source_url,
	             author         = EXCLUDED.author,
	             updated_at     = NOW()`
	count := 0
	for _, im := range imports {
		score := im.LimitedRating / 5.0
		if _, err := r.db.ExecContext(
			ctx, q,
			im.CardName, im.SetCode, im.LimitedRating, score,
			im.ConstructedRating, im.ArchetypeFit, im.Commentary,
			im.SourceURL, im.Author,
		); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// LinkCFBArenaIds populates arena_id on cfb_ratings rows by joining
// cards on lower(name). Returns the number of rows updated.
func (r *CardsRepository) LinkCFBArenaIds(ctx context.Context, setCode string) (int, error) {
	const q = `UPDATE cfb_ratings cr
	           SET arena_id = c.arena_id::INTEGER
	           FROM set_cards c
	           WHERE lower(cr.set_code) = lower($1)
	             AND cr.arena_id IS NULL
	             AND lower(c.name) = lower(cr.card_name)
	             AND c.arena_id IS NOT NULL`
	res, err := r.db.ExecContext(ctx, q, setCode)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// DeleteCFBRatings removes every cfb_ratings row for the set. Returns the
// number of rows deleted.
func (r *CardsRepository) DeleteCFBRatings(ctx context.Context, setCode string) (int, error) {
	const q = `DELETE FROM cfb_ratings WHERE lower(set_code) = lower($1)`
	res, err := r.db.ExecContext(ctx, q, setCode)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// scanSetCardRows centralises the row-scan boilerplate shared by every
// SetCard read query.
func (r *CardsRepository) scanSetCardRows(ctx context.Context, q string, args ...any) ([]SetCardRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SetCardRow
	for rows.Next() {
		var row SetCardRow
		if err := rows.Scan(
			&row.ArenaID, &row.CardID, &row.Name, &row.SetCode, &row.SetName,
			&row.Rarity, &row.ManaCost, &row.CMC, &row.TypeLine,
			&row.Colors, &row.ColorIdentity, &row.ImageURIs,
			&row.Power, &row.Toughness,
			&row.PriceUSD, &row.PriceUSDFoil, &row.PriceEUR, &row.PricesUpdated,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// scanCFBRows centralises the row-scan boilerplate shared by CFB queries.
func (r *CardsRepository) scanCFBRows(ctx context.Context, q string, args ...any) ([]CFBRatingRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CFBRatingRow
	for rows.Next() {
		var c CFBRatingRow
		if err := rows.Scan(
			&c.ID, &c.CardName, &c.SetCode, &c.ArenaID,
			&c.LimitedRating, &c.LimitedScore,
			&c.ConstructedRating, &c.ConstructedScore,
			&c.ArchetypeFit, &c.Commentary,
			&c.SourceURL, &c.Author,
			&c.ImportedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// intSliceToInt64Slice converts an int slice to int64 slice for use with
// pq array binding.
func intSliceToInt64Slice(in []int) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}
