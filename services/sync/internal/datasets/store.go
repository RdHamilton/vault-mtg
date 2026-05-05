package datasets

import (
	"context"

	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

// Store persists and retrieves draft card ratings.
type Store interface {
	// GetActiveSets returns set codes where is_draft_active = TRUE.
	// This covers all Arena-draftable sets including masters, alchemy, and
	// draft_innovation types that are not necessarily standard-legal.
	GetActiveSets(ctx context.Context) ([]string, error)
	UpsertRatings(ctx context.Context, ratings draftdata.SetRatings) error
	GetRatings(ctx context.Context, setCode, draftFormat string) (*draftdata.SetRatings, error)
	// UpsertSets upserts set metadata and marks each as draft-active.
	UpsertSets(ctx context.Context, sets []scryfall.ScryfallSet) error
	// UpsertColorRatings replaces all color-combination ratings for the given
	// set/format in draft_color_ratings.
	UpsertColorRatings(ctx context.Context, setCode, draftFormat string, ratings []seventeenlands.ColorRating) error
}
