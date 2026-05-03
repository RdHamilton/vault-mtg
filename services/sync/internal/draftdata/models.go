package draftdata

import (
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

// SetRatings holds the fetched card ratings for a single MTG set.
type SetRatings struct {
	SetCode     string
	DraftFormat string
	FetchedAt   time.Time
	Cards       []seventeenlands.CardRating
}
