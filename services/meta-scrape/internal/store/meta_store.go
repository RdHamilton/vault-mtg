// Package store provides a write-side persistence layer for MTGZone metagame
// data. It connects directly to Postgres as the mtga_sync role and writes to
// mtgzone_archetypes and mtgzone_archetype_cards.
//
// This package is intentionally self-contained: it does not import services/bff
// or services/sync. The shape mirrors services/sync/internal/datasets/postgres_store.go.
package store

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Archetype is the write-side model for a scraped archetype record.
// It maps to a row in mtgzone_archetypes. Pointer fields are nullable — a nil
// pointer means "do not overwrite" is NOT applicable here; the ON CONFLICT DO
// UPDATE will set the column to NULL when the pointer is nil. Callers must
// populate all fields that should have a value.
type Archetype struct {
	Name            string
	Format          string
	Tier            *string
	Description     *string
	PlayStyle       *string
	SourceURL       *string
	MetaShare       *float32
	TournamentTop8s *int
	TournamentWins  *int
	ConfidenceScore *float32
	TrendDirection  *string
}

// ArchetypeCard is the write-side model for a card associated with a scraped
// archetype. It maps to a row in mtgzone_archetype_cards.
type ArchetypeCard struct {
	CardName   string
	Role       string
	Copies     int
	Importance *string
	Notes      *string
}

// MetaStore persists metagame data scraped from external sources. It wraps a
// pgxpool.Pool and writes to mtgzone_archetypes and mtgzone_archetype_cards
// using the mtga_sync Postgres role credentials supplied at pool construction.
type MetaStore struct {
	pool *pgxpool.Pool
}

// NewMetaStore creates a MetaStore wrapping the given pool.
func NewMetaStore(pool *pgxpool.Pool) *MetaStore {
	return &MetaStore{pool: pool}
}

// UpsertArchetypes upserts a batch of archetypes into mtgzone_archetypes.
// Conflict target: UNIQUE(name, format) — from migration 000041.
// The method is a no-op when archetypes is nil or empty (AC3: source-failure
// resilience). There is no DELETE path — rows from prior healthy runs are never
// removed.
func (s *MetaStore) UpsertArchetypes(ctx context.Context, archetypes []Archetype) error {
	if len(archetypes) == 0 {
		return nil
	}

	const q = `
		INSERT INTO mtgzone_archetypes
		    (name, format, tier, description, play_style, source_url,
		     meta_share, tournament_top8s, tournament_wins, confidence_score,
		     trend_direction, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (name, format) DO UPDATE SET
		    tier             = EXCLUDED.tier,
		    description      = EXCLUDED.description,
		    play_style       = EXCLUDED.play_style,
		    source_url       = EXCLUDED.source_url,
		    meta_share       = EXCLUDED.meta_share,
		    tournament_top8s = EXCLUDED.tournament_top8s,
		    tournament_wins  = EXCLUDED.tournament_wins,
		    confidence_score = EXCLUDED.confidence_score,
		    trend_direction  = EXCLUDED.trend_direction,
		    last_updated     = NOW()
	`

	for _, a := range archetypes {
		if _, err := s.pool.Exec(
			ctx, q,
			a.Name, a.Format, a.Tier, a.Description, a.PlayStyle, a.SourceURL,
			a.MetaShare, a.TournamentTop8s, a.TournamentWins, a.ConfidenceScore,
			a.TrendDirection,
		); err != nil {
			return fmt.Errorf("upsert archetype %q/%q: %w", a.Name, a.Format, err)
		}
	}

	log.Printf("[meta-scrape] UpsertArchetypes: upserted %d archetypes", len(archetypes))
	return nil
}

// UpsertArchetypeCards upserts the card list for a single archetype.
// archetypeID is the PK from mtgzone_archetypes — callers obtain it via
// ArchetypeIDByKey after calling UpsertArchetypes.
// Conflict target: UNIQUE(archetype_id, card_name) — from migration 000041.
// The method is a no-op when cards is nil or empty (AC3). No DELETE path.
func (s *MetaStore) UpsertArchetypeCards(ctx context.Context, archetypeID int64, cards []ArchetypeCard) error {
	if len(cards) == 0 {
		return nil
	}

	const q = `
		INSERT INTO mtgzone_archetype_cards
		    (archetype_id, card_name, role, copies, importance, notes, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (archetype_id, card_name) DO UPDATE SET
		    role         = EXCLUDED.role,
		    copies       = EXCLUDED.copies,
		    importance   = EXCLUDED.importance,
		    notes        = EXCLUDED.notes,
		    last_updated = NOW()
	`

	for _, c := range cards {
		if _, err := s.pool.Exec(
			ctx, q,
			archetypeID, c.CardName, c.Role, c.Copies, c.Importance, c.Notes,
		); err != nil {
			return fmt.Errorf("upsert archetype card %q (archetype_id=%d): %w", c.CardName, archetypeID, err)
		}
	}

	log.Printf("[meta-scrape] UpsertArchetypeCards: upserted %d cards for archetype_id=%d", len(cards), archetypeID)
	return nil
}

// ArchetypeIDByKey returns the id of the mtgzone_archetypes row for the given
// (name, format) natural key.
//
// Returns pgx.ErrNoRows when no matching row exists. This is the #177 contract:
// after UpsertArchetypes succeeds the key will always exist; ErrNoRows signals a
// genuine error to be surfaced to Sentry by the Lambda handler.
func (s *MetaStore) ArchetypeIDByKey(ctx context.Context, name, format string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(
		ctx,
		`SELECT id FROM mtgzone_archetypes WHERE name = $1 AND format = $2`,
		name, format,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, pgx.ErrNoRows
		}
		return 0, fmt.Errorf("look up archetype id for %q/%q: %w", name, format, err)
	}
	return id, nil
}
