// Package handler provides the AWS Lambda handler for the mtga-sync service.
// AWS EventBridge Scheduler invokes this handler on a configurable cron schedule,
// replacing the long-running ticker loop that was used for local/server deployments.
package handler

import (
	"context"
	"log"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

// defaultFormats are the draft formats synced on every invocation.
// Sealed is opt-in — it consumes separate 17Lands quota and is less commonly
// needed for draft-oriented features.
var defaultFormats = []string{"PremierDraft", "QuickDraft"}

// Fetcher retrieves card and color ratings from an external source.
type Fetcher interface {
	FetchCardRatings(ctx context.Context, setCode, format string) ([]seventeenlands.CardRating, error)
	FetchColorRatings(ctx context.Context, setCode, format string) ([]seventeenlands.ColorRating, error)
}

// SyncHandler is the Lambda handler that fetches card ratings for all active sets
// and persists them to Postgres. Each invocation performs a single full refresh.
type SyncHandler struct {
	fetcher      Fetcher
	store        datasets.Store
	overrideSets []string // non-empty when caller provides an explicit set list
	formats      []string // draft formats to sync per set
}

// New creates a SyncHandler. overrideSets may be nil/empty to use DB-driven active sets.
func New(fetcher Fetcher, store datasets.Store, overrideSets []string) *SyncHandler {
	return &SyncHandler{
		fetcher:      fetcher,
		store:        store,
		overrideSets: overrideSets,
		formats:      defaultFormats,
	}
}

// Handle is the Lambda handler function. It is invoked by EventBridge Scheduler
// and performs a single ratings refresh across all active sets and formats.
//
// The event payload is ignored — EventBridge scheduled events carry no
// application-level data. Any invocation triggers a full sync.
func (h *SyncHandler) Handle(ctx context.Context, _ any) error {
	sets, err := h.activeSets(ctx)
	if err != nil {
		return err
	}

	if len(sets) == 0 {
		log.Println("[sync] no standard-legal sets found, skipping fetch")
		return nil
	}

	log.Printf("[sync] fetching ratings for %d set(s) x %d format(s): sets=%v formats=%v", len(sets), len(h.formats), sets, h.formats)

	for _, setCode := range sets {
		for _, format := range h.formats {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			ratings, err := h.fetcher.FetchCardRatings(ctx, setCode, format)
			if err != nil {
				log.Printf("[sync] fetch %s/%s: %v", setCode, format, err)
				continue
			}

			if len(ratings) == 0 {
				log.Printf("[sync] WARNING: 0 cards returned for %s/%s — set code may not match 17Lands expansion code", setCode, format)
				continue
			}

			sr := draftdata.SetRatings{
				SetCode:     setCode,
				DraftFormat: format,
				FetchedAt:   time.Now().UTC(),
				Cards:       ratings,
			}

			if err := h.store.UpsertRatings(ctx, sr); err != nil {
				log.Printf("[sync] upsert %s/%s: %v", setCode, format, err)
				continue
			}

			log.Printf("[sync] refreshed %s/%s: %d cards", setCode, format, len(ratings))

			// Fetch and persist per-color-combination win rates. A failure here is
			// non-fatal — card ratings are already stored and color data is best-effort.
			if ctx.Err() != nil {
				return ctx.Err()
			}

			colorRatings, err := h.fetcher.FetchColorRatings(ctx, setCode, format)
			if err != nil {
				log.Printf("[sync] fetch color ratings %s/%s: %v", setCode, format, err)
				continue
			}

			if len(colorRatings) == 0 {
				log.Printf("[sync] no color ratings returned for %s/%s", setCode, format)
				continue
			}

			if err := h.store.UpsertColorRatings(ctx, setCode, format, colorRatings); err != nil {
				log.Printf("[sync] upsert color ratings %s/%s: %v", setCode, format, err)
				continue
			}

			log.Printf("[sync] refreshed color ratings %s/%s: %d combinations", setCode, format, len(colorRatings))
		}
	}

	return nil
}

func (h *SyncHandler) activeSets(ctx context.Context) ([]string, error) {
	if len(h.overrideSets) > 0 {
		return h.overrideSets, nil
	}

	return h.store.GetActiveSets(ctx)
}
