package repository

import (
	"context"
	"database/sql"
	"time"
)

// CardRating is a single row from draft_card_ratings.
type CardRating struct {
	ArenaID  int
	Name     string
	Color    string
	Rarity   string
	GIHWR    *float64
	OHWR     *float64
	ALSA     *float64
	ATA      *float64
	GIHCount *int
}

// ColorRating is a single row from draft_color_ratings.
type ColorRating struct {
	ColorCombination string
	WinRate          *float64
	GamesPlayed      *int
}

// DraftRatingsResult holds all ratings for a set/format pair, plus the freshness
// timestamp from MAX(cached_at) across the card ratings rows.
type DraftRatingsResult struct {
	SetCode      string
	DraftFormat  string
	CachedAt     time.Time
	CardRatings  []CardRating
	ColorRatings []ColorRating
}

// DraftRatingsRepository reads draft ratings from the database.
type DraftRatingsRepository struct {
	db DB
}

// NewDraftRatingsRepository returns a new DraftRatingsRepository backed by db.
func NewDraftRatingsRepository(db DB) *DraftRatingsRepository {
	return &DraftRatingsRepository{db: db}
}

// GetRatings returns all draft_card_ratings and draft_color_ratings rows for the
// given setCode and draftFormat.  The CachedAt field is set to MAX(cached_at)
// from the card ratings rows.
//
// color and rarity are resolved by LEFT JOIN-ing against set_cards so that the
// sync service does not need to duplicate Scryfall metadata.
//
// set_cards.arena_id is TEXT (migration 000014); draft_card_ratings.arena_id is
// INTEGER (migration 000015) — the join casts set_cards.arena_id to INTEGER.
// DISTINCT ON guards against the same arena_id appearing in multiple sets.
//
// Returns (nil, nil) when no rows exist for the requested set/format so that the
// caller can distinguish a missing result from a database error.
func (r *DraftRatingsRepository) GetRatings(ctx context.Context, setCode, draftFormat string) (*DraftRatingsResult, error) {
	const cardQuery = `
		SELECT
			dcr.arena_id,
			dcr.name,
			COALESCE(c.colors, ''),
			COALESCE(c.rarity, ''),
			dcr.gihwr,
			dcr.ohwr,
			dcr.alsa,
			dcr.ata,
			dcr.gih_count,
			MAX(dcr.cached_at) OVER () AS max_cached_at
		FROM draft_card_ratings dcr
		LEFT JOIN (
			SELECT DISTINCT ON (arena_id) arena_id, colors, rarity
			FROM set_cards
			ORDER BY arena_id, id
		) c ON c.arena_id::INTEGER = dcr.arena_id
		WHERE dcr.set_code = $1 AND dcr.draft_format = $2`

	rows, err := r.db.QueryContext(ctx, cardQuery, setCode, draftFormat)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var (
		cards    []CardRating
		cachedAt time.Time
	)

	for rows.Next() {
		var (
			c         CardRating
			maxCached time.Time
		)

		if err := rows.Scan(
			&c.ArenaID, &c.Name, &c.Color, &c.Rarity,
			&c.GIHWR, &c.OHWR, &c.ALSA, &c.ATA, &c.GIHCount,
			&maxCached,
		); err != nil {
			return nil, err
		}

		cachedAt = maxCached
		cards = append(cards, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// No rows found — return nil so the handler can return 404.
	if len(cards) == 0 {
		return nil, nil //nolint:nilnil
	}

	// Fetch color ratings (best-effort; missing rows are not an error).
	const colorQuery = `
		SELECT
			color_combination,
			win_rate,
			games_played
		FROM draft_color_ratings
		WHERE set_code = $1 AND draft_format = $2`

	cRows, err := r.db.QueryContext(ctx, colorQuery, setCode, draftFormat)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cRows.Close() }()

	var colors []ColorRating

	for cRows.Next() {
		var cr ColorRating
		if err := cRows.Scan(&cr.ColorCombination, &cr.WinRate, &cr.GamesPlayed); err != nil {
			return nil, err
		}

		colors = append(colors, cr)
	}

	if err := cRows.Err(); err != nil {
		return nil, err
	}

	return &DraftRatingsResult{
		SetCode:      setCode,
		DraftFormat:  draftFormat,
		CachedAt:     cachedAt,
		CardRatings:  cards,
		ColorRatings: colors,
	}, nil
}

// GetMaxCachedAt returns MAX(cached_at) from draft_card_ratings for the given
// setCode and draftFormat without fetching all rows.  Returns (zero time, nil)
// when no rows exist.
func (r *DraftRatingsRepository) GetMaxCachedAt(ctx context.Context, setCode, draftFormat string) (time.Time, error) {
	const q = `
		SELECT MAX(cached_at)
		FROM draft_card_ratings
		WHERE set_code = $1 AND draft_format = $2`

	var t sql.NullTime

	if err := r.db.QueryRowContext(ctx, q, setCode, draftFormat).Scan(&t); err != nil {
		return time.Time{}, err
	}

	if !t.Valid {
		return time.Time{}, nil
	}

	return t.Time, nil
}
