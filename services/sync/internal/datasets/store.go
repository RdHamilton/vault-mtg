package datasets

import (
	"context"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/draftdata"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
)

// SyncSet holds the Scryfall set code and the 17Lands expansion code for a set.
// Code is the canonical Scryfall code — used as the key for all DB writes
// (draft_card_ratings.set_code, draft_color_ratings.set_code, sync_hashes).
// ExpansionCode is the code sent to the 17Lands API. For most sets these are
// identical; seventeenlands_code in the sets table is NULL for those sets and
// COALESCE falls back to the Scryfall code. Sets like Aetherdrift (AED / DFT)
// have an explicit expansion code stored in the column.
type SyncSet struct {
	Code          string // Scryfall code — keyed in DB
	ExpansionCode string // 17Lands expansion code — used in API requests
}

// Store persists and retrieves draft card ratings.
type Store interface {
	// GetActiveSets returns sets where is_draft_active = TRUE.
	// Each SyncSet carries the Scryfall Code (DB key) and the ExpansionCode
	// to use in 17Lands API requests (COALESCE(seventeenlands_code, code)).
	GetActiveSets(ctx context.Context) ([]SyncSet, error)
	UpsertRatings(ctx context.Context, ratings draftdata.SetRatings) error
	GetRatings(ctx context.Context, setCode, draftFormat string) (*draftdata.SetRatings, error)
	// UpsertSets upserts set metadata and marks each as draft-active.
	UpsertSets(ctx context.Context, sets []scryfall.ScryfallSet) error
	// UpsertColorRatings replaces all color-combination ratings for the given
	// set/format in draft_color_ratings.
	UpsertColorRatings(ctx context.Context, setCode, draftFormat string, ratings []seventeenlands.ColorRating) error
	// GetHash returns the stored hash for the given key (e.g. a set code).
	// Returns ("", nil) when no hash has been stored for that key.
	GetHash(ctx context.Context, key string) (string, error)
	// SetHash stores a hash for the given key, replacing any existing value.
	SetHash(ctx context.Context, key string, hash string) error
	// UpsertSetCards upserts per-set card entries into set_cards keyed on
	// (set_code, arena_id). arena_id is stored as TEXT in set_cards so a
	// ::TEXT cast is required when writing from the integer ArenaID field.
	// This is the sole Scryfall card write path — the retired cards table
	// (dropped in migration 000025) is not written.
	UpsertSetCards(ctx context.Context, cards []scryfall.ScryfallCard) error
}
