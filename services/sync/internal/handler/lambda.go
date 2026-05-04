// Package handler provides the AWS Lambda handler for the mtga-sync service.
// AWS EventBridge Scheduler invokes this handler on a configurable cron schedule,
// replacing the long-running ticker loop that was used for local/server deployments.
package handler

import (
	"context"
	"log"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

// Fetcher retrieves card ratings from an external source.
type Fetcher interface {
	FetchCardRatings(ctx context.Context, setCode, format string) ([]seventeenlands.CardRating, error)
}

// SyncHandler is the Lambda handler that fetches card ratings for all active sets
// and persists them to Postgres. Each invocation performs a single full refresh.
type SyncHandler struct {
	fetcher      Fetcher
	store        datasets.Store
	overrideSets []string // non-empty when caller provides an explicit set list
}

// New creates a SyncHandler. overrideSets may be nil/empty to use DB-driven active sets.
func New(fetcher Fetcher, store datasets.Store, overrideSets []string) *SyncHandler {
	return &SyncHandler{
		fetcher:      fetcher,
		store:        store,
		overrideSets: overrideSets,
	}
}

// Handle is the Lambda handler function. It is invoked by EventBridge Scheduler
// and performs a single ratings refresh across all active sets.
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

	log.Printf("[sync] fetching ratings for %d set(s): %v", len(sets), sets)

	for _, setCode := range sets {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ratings, err := h.fetcher.FetchCardRatings(ctx, setCode, "PremierDraft")
		if err != nil {
			log.Printf("[sync] fetch %s: %v", setCode, err)
			continue
		}

		if len(ratings) == 0 {
			log.Printf("[sync] WARNING: 0 cards returned for %s/PremierDraft — set code may not match 17Lands expansion code", setCode)
			continue
		}

		sr := draftdata.SetRatings{
			SetCode:     setCode,
			DraftFormat: "PremierDraft",
			Cards:       ratings,
		}

		if err := h.store.UpsertRatings(ctx, sr); err != nil {
			log.Printf("[sync] upsert %s: %v", setCode, err)
			continue
		}

		log.Printf("[sync] refreshed %s: %d cards", setCode, len(ratings))
	}

	return nil
}

func (h *SyncHandler) activeSets(ctx context.Context) ([]string, error) {
	if len(h.overrideSets) > 0 {
		return h.overrideSets, nil
	}

	return h.store.GetActiveSets(ctx)
}
