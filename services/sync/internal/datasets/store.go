package datasets

import (
	"context"

	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
)

// Store persists and retrieves draft card ratings.
type Store interface {
	UpsertRatings(ctx context.Context, ratings draftdata.SetRatings) error
	GetRatings(ctx context.Context, setCode, draftFormat string) (*draftdata.SetRatings, error)
}
