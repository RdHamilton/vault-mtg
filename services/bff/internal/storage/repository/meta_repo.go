package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// MetaRepository serves the Phase 2 /api/v1/meta/* read paths. The data
// originates from external scrapes (MTGGoldfish / MTGTop8 / 17lands) that
// the projection worker stores in the mtgzone_* tables. The projection /
// scrape pipeline lives outside this PR — we only read here.
type MetaRepository struct {
	db DB
}

// NewMetaRepository returns a MetaRepository backed by db.
func NewMetaRepository(db DB) *MetaRepository {
	return &MetaRepository{db: db}
}

// ArchetypeRow mirrors a row in mtgzone_archetypes plus optional aggregates
// from related tables. Counts and rates that the SPA's ArchetypeInfo expects
// (metaShare, tournamentTop8s, tournamentWins, confidenceScore,
// trendDirection) aren't tracked in mtgzone_* yet — the handler emits 0 /
// "stable" for those until the scrape pipeline backfills them. Tier in the
// schema is TEXT (e.g. "S","A","1","2"); the handler parses it to int as
// best-effort.
type ArchetypeRow struct {
	ID          int64
	Name        string
	Format      string
	Tier        *string
	Description *string
	PlayStyle   *string
	SourceURL   *string
	LastUpdated time.Time
}

// ListArchetypesByFormat returns mtgzone archetypes for a given format,
// optionally narrowed to a specific tier (passing tier=0 disables the
// filter). Ordered by tier ascending (S/1 first), then name.
func (r *MetaRepository) ListArchetypesByFormat(ctx context.Context, format string, tier int) ([]ArchetypeRow, error) {
	clauses := []string{"lower(format) = lower($1)"}
	args := []any{format}
	next := 2
	if tier > 0 {
		clauses = append(clauses, "tier = $"+strconv.Itoa(next))
		args = append(args, strconv.Itoa(tier))
		next++
	}
	q := `SELECT id, name, format, tier, description, play_style, source_url, last_updated
	      FROM mtgzone_archetypes
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY tier NULLS LAST, name
	      LIMIT 200`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchetypeRow
	for rows.Next() {
		var a ArchetypeRow
		if err := rows.Scan(&a.ID, &a.Name, &a.Format, &a.Tier, &a.Description, &a.PlayStyle, &a.SourceURL, &a.LastUpdated); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LatestArchetypeUpdate returns MAX(last_updated) across mtgzone_archetypes
// for the given format. Used by /meta/refresh to populate
// MetaDashboardResponse.lastUpdated when no fresh scrape has happened.
// Returns the zero time + ok=false when no archetypes exist for the format.
func (r *MetaRepository) LatestArchetypeUpdate(ctx context.Context, format string) (time.Time, bool, error) {
	const q = `SELECT MAX(last_updated) FROM mtgzone_archetypes WHERE lower(format) = lower($1)`
	var ts *time.Time
	if err := r.db.QueryRowContext(ctx, q, format).Scan(&ts); err != nil {
		return time.Time{}, false, err
	}
	if ts == nil {
		return time.Time{}, false, nil
	}
	return *ts, true, nil
}

// ArchetypeByName returns the mtgzone_archetypes row matching (format, name)
// case-insensitively. Returns nil when the archetype does not exist for the
// format.
func (r *MetaRepository) ArchetypeByName(ctx context.Context, format, name string) (*ArchetypeRow, error) {
	const q = `SELECT id, name, format, tier, description, play_style, source_url, last_updated
	           FROM mtgzone_archetypes
	           WHERE lower(format) = lower($1) AND lower(name) = lower($2)
	           LIMIT 1`
	var a ArchetypeRow
	err := r.db.QueryRowContext(ctx, q, format, name).Scan(
		&a.ID, &a.Name, &a.Format, &a.Tier, &a.Description, &a.PlayStyle, &a.SourceURL, &a.LastUpdated,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ArchetypeCardRow mirrors a row in mtgzone_archetype_cards. Role is the
// free-form bucket label the scrape pipeline assigned (e.g.,
// "Creatures", "Removal", "Commons"); the handler groups by role to build
// the ArchetypeCards top_creatures / top_removal / top_commons lists.
type ArchetypeCardRow struct {
	CardName   string
	Role       string
	Copies     int
	Importance *string
	Notes      *string
}

// ArchetypeCardsByID returns every card associated with archetypeID, ordered
// by importance / role / name so the handler's bucket-by-role loop is
// deterministic.
func (r *MetaRepository) ArchetypeCardsByID(ctx context.Context, archetypeID int64) ([]ArchetypeCardRow, error) {
	const q = `SELECT card_name, role, copies, importance, notes
	           FROM mtgzone_archetype_cards
	           WHERE archetype_id = $1
	           ORDER BY role, copies DESC, card_name
	           LIMIT 500`
	rows, err := r.db.QueryContext(ctx, q, archetypeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchetypeCardRow
	for rows.Next() {
		var c ArchetypeCardRow
		if err := rows.Scan(&c.CardName, &c.Role, &c.Copies, &c.Importance, &c.Notes); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
