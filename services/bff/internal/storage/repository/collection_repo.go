package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

// CollectionRepository serves the Phase 2 /api/v1/collection/* read paths.
// All queries are scoped by account_id and join card_inventory (the active
// inventory table) against set_cards (per-set cache that carries metadata and
// prices). The legacy `cards` table was dropped in migration 25 and replaced
// by set_cards; the legacy `collection` table is intentionally not touched.
type CollectionRepository struct {
	db DB
}

// NewCollectionRepository returns a CollectionRepository backed by db.
func NewCollectionRepository(db DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

// CollectionFilter narrows a collection list query. Zero-valued fields mean
// "no filter on that dimension" so a partial struct is fine.
type CollectionFilter struct {
	SetCode     string
	Rarity      string
	Colors      []string // any-of match against the set_cards.colors JSON array
	OwnedOnly   bool     // require count > 0 (always true today; kept for API symmetry)
	MissingOnly bool     // require count = 0 (rare — an account row only exists for owned cards, so this returns nothing today)
}

// CollectionItem is a single card in the collection-list response — joined
// metadata from set_cards + sets.
type CollectionItem struct {
	CardID        int
	ArenaID       int
	Quantity      int
	Name          string
	SetCode       string
	SetName       string
	Rarity        string
	ManaCost      string
	CMC           float64
	TypeLine      string
	Colors        string // JSON array stored as TEXT
	ColorIdentity string // JSON array stored as TEXT (empty: not available in set_cards)
	ImageURIs     string // JSON object stored as TEXT
	Power         *string
	Toughness     *string
	PriceUSD      *float64
	PriceUSDFoil  *float64
	PriceEUR      *float64
	PricesUpdated *int64 // unix seconds; nil when no price data
}

// ListCollection returns the joined collection rows for the account, filtered
// per f and ordered by name. The handler is responsible for translating the
// rows into the SPA's CollectionCard wire shape (image URI extraction,
// JSON-array parsing, etc.).
func (r *CollectionRepository) ListCollection(ctx context.Context, accountID int64, f CollectionFilter) ([]CollectionItem, error) {
	// ci predicates: always applied in the outer WHERE.
	ciClauses := []string{"ci.account_id = $1"}
	args := []any{accountID}
	next := 2

	if !f.MissingOnly {
		// OwnedOnly is the default: card_inventory only stores owned cards
		// today, but be explicit so future "phantom" rows (count=0 markers)
		// don't sneak in.
		ciClauses = append(ciClauses, "ci.count > 0")
	} else {
		ciClauses = append(ciClauses, "ci.count = 0")
	}

	// sc predicates are pushed into the CTE WHERE so that DISTINCT ON selects
	// among matching printings only — not the globally lowest-id printing.
	// Without this, filtering by SetCode='S2' on a card whose lowest-id row is
	// in S1 would silently exclude the card even though the user owns it in S2.
	// When any sc predicate is active we also add sc.arena_id IS NOT NULL to the
	// outer WHERE so that cards with no matching printing are excluded.
	var scClauses []string
	if f.SetCode != "" {
		scClauses = append(scClauses, "lower(set_code) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.SetCode)
		next++
	}
	if f.Rarity != "" {
		scClauses = append(scClauses, "lower(rarity) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Rarity)
		next++
	}
	if len(f.Colors) > 0 {
		// set_cards.colors is stored as a JSON array of color letters ('W','U','B','R','G').
		// Use ILIKE on the raw JSON text so we don't have to depend on a JSONB cast.
		ors := make([]string, 0, len(f.Colors))
		for _, color := range f.Colors {
			ors = append(ors, "colors ILIKE $"+strconv.Itoa(next))
			args = append(args, "%\""+strings.ToUpper(strings.TrimSpace(color))+"\"%")
			next++
		}
		scClauses = append(scClauses, "("+strings.Join(ors, " OR ")+")")
	}
	scWhere := ""
	if len(scClauses) > 0 {
		scWhere = "\n\t\t\t\tWHERE " + strings.Join(scClauses, " AND ")
		ciClauses = append(ciClauses, "sc.arena_id IS NOT NULL")
	}

	// DISTINCT ON (arena_id) guards against reprints: set_cards has UNIQUE(set_code,
	// arena_id) not UNIQUE(arena_id), so the same arena_id can appear in multiple
	// sets.  sc predicates in the CTE WHERE ensure the dedup picks the correct
	// printing when a filter is active.
	q := `
		WITH sc AS (
			SELECT DISTINCT ON (arena_id) *
			FROM set_cards` + scWhere + `
			ORDER BY arena_id, id
		)
		SELECT
			ci.card_id,
			COALESCE(sc.arena_id::INT, ci.card_id),
			ci.count,
			COALESCE(sc.name, ''),
			COALESCE(sc.set_code, ''),
			COALESCE(s.name, ''),
			COALESCE(sc.rarity, ''),
			COALESCE(sc.mana_cost, ''),
			COALESCE(sc.cmc, 0),
			COALESCE(sc.types, ''),
			COALESCE(sc.colors, '[]'),
			'[]',
			CASE WHEN sc.image_url IS NOT NULL THEN json_build_object('normal', sc.image_url)::TEXT ELSE '{}' END,
			sc.power,
			sc.toughness,
			sc.price_usd,
			sc.price_usd_foil,
			sc.price_eur,
			EXTRACT(EPOCH FROM sc.prices_updated_at)::BIGINT
		FROM card_inventory ci
		LEFT JOIN sc ON sc.arena_id = ci.card_id::TEXT
		LEFT JOIN sets s ON s.code = sc.set_code
		WHERE ` + strings.Join(ciClauses, " AND ") + `
		ORDER BY sc.name NULLS LAST, ci.card_id
		LIMIT 5000`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []CollectionItem
	for rows.Next() {
		var ci CollectionItem
		if err := rows.Scan(
			&ci.CardID, &ci.ArenaID, &ci.Quantity, &ci.Name, &ci.SetCode, &ci.SetName,
			&ci.Rarity, &ci.ManaCost, &ci.CMC, &ci.TypeLine, &ci.Colors, &ci.ColorIdentity,
			&ci.ImageURIs, &ci.Power, &ci.Toughness,
			&ci.PriceUSD, &ci.PriceUSDFoil, &ci.PriceEUR, &ci.PricesUpdated,
		); err != nil {
			return nil, err
		}
		out = append(out, ci)
	}
	return out, rows.Err()
}

// CollectionCounts is the aggregated total + unique count for the account.
// Per-rarity counts come from CollectionStatsByRarity below.
type CollectionCounts struct {
	UniqueCards int
	TotalCards  int
}

// CountCollection returns total/unique card counts for the account.
func (r *CollectionRepository) CountCollection(ctx context.Context, accountID int64) (CollectionCounts, error) {
	const q = `SELECT COUNT(*) AS unique_cards,
	                  COALESCE(SUM(count), 0) AS total_cards
	           FROM card_inventory
	           WHERE account_id = $1 AND count > 0`
	row := r.db.QueryRowContext(ctx, q, accountID)
	var c CollectionCounts
	if err := row.Scan(&c.UniqueCards, &c.TotalCards); err != nil {
		return CollectionCounts{}, err
	}
	return c, nil
}

// RarityCount aggregates copies of cards at a single rarity. Used by
// /api/v1/collection/stats.
type RarityCount struct {
	Rarity     string
	TotalCards int // sum of card_inventory.count
}

// CountByRarity returns per-rarity copy counts for the account, joining
// card_inventory → set_cards to pull rarity. Cards with no metadata are
// bucketed under empty-string rarity which the handler can fold into "unknown".
func (r *CollectionRepository) CountByRarity(ctx context.Context, accountID int64) ([]RarityCount, error) {
	const q = `SELECT COALESCE(sc.rarity, '') AS rarity,
	                  COALESCE(SUM(ci.count), 0) AS total_cards
	           FROM card_inventory ci
	           LEFT JOIN (SELECT DISTINCT ON (arena_id) arena_id, rarity FROM set_cards ORDER BY arena_id, id) sc
	                  ON sc.arena_id = ci.card_id::TEXT
	           WHERE ci.account_id = $1 AND ci.count > 0
	           GROUP BY sc.rarity
	           ORDER BY rarity`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RarityCount
	for rows.Next() {
		var rc RarityCount
		if err := rows.Scan(&rc.Rarity, &rc.TotalCards); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

// SetCompletionRow is one set's completion summary. Per-rarity breakdown is a
// separate query — the handler stitches them together.
type SetCompletionRow struct {
	SetCode    string
	SetName    string
	TotalCards int
	OwnedCards int
}

// SetCompletion returns completion ratios for every set with at least one
// card the account owns. Joins set_cards (the canonical "this set's cards"
// list) against card_inventory.
func (r *CollectionRepository) SetCompletion(ctx context.Context, accountID int64) ([]SetCompletionRow, error) {
	// We compute total per set from set_cards (distinct arena_id) and owned
	// from card_inventory joined on arena_id (cast to TEXT to match
	// set_cards.arena_id's type).
	const q = `
		WITH per_set_total AS (
			SELECT set_code, COUNT(DISTINCT arena_id) AS total_cards
			FROM set_cards
			GROUP BY set_code
		),
		per_set_owned AS (
			SELECT sc.set_code, COUNT(DISTINCT ci.card_id) AS owned_cards
			FROM card_inventory ci
			JOIN set_cards sc ON sc.arena_id = ci.card_id::TEXT
			WHERE ci.account_id = $1 AND ci.count > 0
			GROUP BY sc.set_code
		)
		SELECT
			t.set_code,
			COALESCE(s.name, t.set_code) AS set_name,
			t.total_cards,
			COALESCE(o.owned_cards, 0) AS owned_cards
		FROM per_set_total t
		LEFT JOIN per_set_owned o ON o.set_code = t.set_code
		LEFT JOIN sets s          ON s.code = t.set_code
		WHERE COALESCE(o.owned_cards, 0) > 0
		ORDER BY t.set_code`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SetCompletionRow
	for rows.Next() {
		var s SetCompletionRow
		if err := rows.Scan(&s.SetCode, &s.SetName, &s.TotalCards, &s.OwnedCards); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetRarityRow is per-set, per-rarity totals + owned. Used to build the
// SetCompletion.RarityBreakdown maps.
type SetRarityRow struct {
	SetCode string
	Rarity  string
	Total   int
	Owned   int
}

// SetRarityBreakdown returns total/owned counts per (set, rarity). Used by
// the /collection/sets handler to populate SetCompletion.RarityBreakdown.
func (r *CollectionRepository) SetRarityBreakdown(ctx context.Context, accountID int64) ([]SetRarityRow, error) {
	const q = `
		WITH set_rarity_totals AS (
			SELECT set_code, COALESCE(rarity, '') AS rarity, COUNT(DISTINCT arena_id) AS total
			FROM set_cards
			GROUP BY set_code, rarity
		),
		set_rarity_owned AS (
			SELECT sc.set_code, COALESCE(sc.rarity, '') AS rarity, COUNT(DISTINCT ci.card_id) AS owned
			FROM card_inventory ci
			JOIN set_cards sc ON sc.arena_id = ci.card_id::TEXT
			WHERE ci.account_id = $1 AND ci.count > 0
			GROUP BY sc.set_code, sc.rarity
		)
		SELECT
			t.set_code, t.rarity, t.total, COALESCE(o.owned, 0)
		FROM set_rarity_totals t
		LEFT JOIN set_rarity_owned o
		    ON o.set_code = t.set_code AND o.rarity = t.rarity
		ORDER BY t.set_code, t.rarity`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SetRarityRow
	for rows.Next() {
		var s SetRarityRow
		if err := rows.Scan(&s.SetCode, &s.Rarity, &s.Total, &s.Owned); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CardValueRow is a single (card, quantity, price) tuple used by the value
// aggregator. Cards without a price row are omitted (the handler counts them
// separately to surface "uniqueCardsWithPrice" honestly).
type CardValueRow struct {
	CardID   int
	Name     string
	SetCode  string
	Rarity   string
	Quantity int
	PriceUSD float64
	PriceEUR float64
}

// ValueRows returns every owned card with a known USD price. Caller iterates
// to compute totals + top-N. Cards without a price are excluded from the
// total but their existence is reflected in CountWithoutPrice.
func (r *CollectionRepository) ValueRows(ctx context.Context, accountID int64) ([]CardValueRow, int, error) {
	// Two queries: one for priced rows, one to count owned-but-unpriced.
	const priced = `
		WITH sc AS (
			SELECT DISTINCT ON (arena_id) arena_id, name, set_code, rarity, price_usd, price_eur
			FROM set_cards
			ORDER BY arena_id, id
		)
		SELECT
			ci.card_id,
			COALESCE(sc.name, ''),
			COALESCE(sc.set_code, ''),
			COALESCE(sc.rarity, ''),
			ci.count,
			COALESCE(sc.price_usd, 0),
			COALESCE(sc.price_eur, 0)
		FROM card_inventory ci
		JOIN sc ON sc.arena_id = ci.card_id::TEXT
		WHERE ci.account_id = $1 AND ci.count > 0 AND sc.price_usd IS NOT NULL AND sc.price_usd > 0`

	rows, err := r.db.QueryContext(ctx, priced, accountID)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []CardValueRow
	for rows.Next() {
		var v CardValueRow
		if err := rows.Scan(
			&v.CardID, &v.Name, &v.SetCode, &v.Rarity, &v.Quantity, &v.PriceUSD, &v.PriceEUR,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Count owned-but-unpriced (informational; not currently surfaced on the
	// CollectionValue response but cheap to compute).
	const unpriced = `
		SELECT COUNT(*)
		FROM card_inventory ci
		LEFT JOIN set_cards sc ON sc.arena_id = ci.card_id::TEXT
		WHERE ci.account_id = $1 AND ci.count > 0
		  AND (sc.price_usd IS NULL OR sc.price_usd = 0)`
	var unpricedCount int
	if err := r.db.QueryRowContext(ctx, unpriced, accountID).Scan(&unpricedCount); err != nil {
		return nil, 0, err
	}
	return out, unpricedCount, nil
}

// LastPriceUpdate returns the most recent set_cards.prices_updated_at for the
// account's owned cards (so the SPA can show a "prices last refreshed" hint).
func (r *CollectionRepository) LastPriceUpdate(ctx context.Context, accountID int64) (int64, error) {
	const q = `SELECT EXTRACT(EPOCH FROM MAX(sc.prices_updated_at))::BIGINT
	           FROM card_inventory ci
	           JOIN set_cards sc ON sc.arena_id = ci.card_id::TEXT
	           WHERE ci.account_id = $1 AND ci.count > 0`
	var ts sql.NullInt64
	if err := r.db.QueryRowContext(ctx, q, accountID).Scan(&ts); err != nil {
		return 0, err
	}
	if !ts.Valid {
		return 0, nil
	}
	return ts.Int64, nil
}
