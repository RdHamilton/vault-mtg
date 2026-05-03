package refresh

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

const defaultRefreshHour = 2

// Fetcher retrieves card ratings from an external source.
type Fetcher interface {
	FetchCardRatings(ctx context.Context, setCode, format string) ([]seventeenlands.CardRating, error)
}

// Scheduler runs a daily fetch of card ratings for all active sets.
type Scheduler struct {
	fetcher     Fetcher
	store       datasets.Store
	activeSets  []string
	refreshHour int
	newTicker   func(d time.Duration) (<-chan time.Time, func())
}

// New creates a Scheduler reading configuration from environment variables.
//
// SYNC_REFRESH_HOUR — hour of day (0–23) to run the refresh (default 2).
// SYNC_ACTIVE_SETS  — comma-separated set codes, e.g. "FDN,BLB,DSK".
func New(fetcher Fetcher, store datasets.Store) *Scheduler {
	hour := defaultRefreshHour
	if v := os.Getenv("SYNC_REFRESH_HOUR"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h >= 0 && h <= 23 {
			hour = h
		}
	}

	var sets []string
	if v := os.Getenv("SYNC_ACTIVE_SETS"); v != "" {
		for _, s := range strings.Split(v, ",") {
			if t := strings.TrimSpace(s); t != "" {
				sets = append(sets, t)
			}
		}
	}

	return &Scheduler{
		fetcher:     fetcher,
		store:       store,
		activeSets:  sets,
		refreshHour: hour,
		newTicker:   defaultTicker,
	}
}

func defaultTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Start blocks until ctx is cancelled, running a fetch at the configured hour each day.
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("[sync] scheduler starting: refresh_hour=%d active_sets=%v", s.refreshHour, s.activeSets)

	// Run an immediate fetch on startup so the first day isn't missed.
	s.runFetch(ctx)

	ch, stop := s.newTicker(1 * time.Hour)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[sync] scheduler stopped")
			return
		case t := <-ch:
			if t.UTC().Hour() == s.refreshHour {
				s.runFetch(ctx)
			}
		}
	}
}

func (s *Scheduler) runFetch(ctx context.Context) {
	if len(s.activeSets) == 0 {
		log.Println("[sync] no active sets configured, skipping fetch")
		return
	}

	for _, setCode := range s.activeSets {
		ratings, err := s.fetcher.FetchCardRatings(ctx, setCode, "PremierDraft")
		if err != nil {
			log.Printf("[sync] fetch %s: %v", setCode, err)
			continue
		}

		sr := draftdata.SetRatings{
			SetCode:     setCode,
			DraftFormat: "PremierDraft",
			FetchedAt:   time.Now().UTC(),
			Cards:       ratings,
		}

		if err := s.store.UpsertRatings(ctx, sr); err != nil {
			log.Printf("[sync] upsert %s: %v", setCode, err)
			continue
		}

		log.Printf("[sync] refreshed %s: %d cards", setCode, len(ratings))
	}
}
