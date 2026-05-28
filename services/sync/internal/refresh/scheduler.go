package refresh

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/draftdata"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
)

const defaultRefreshHour = 2

// defaultFormats mirrors handler.defaultFormats — PremierDraft and QuickDraft.
// Sealed is opt-in via SYNC_FORMATS.
var defaultFormats = []string{"PremierDraft", "QuickDraft"}

// Fetcher retrieves card and color ratings from an external source.
type Fetcher interface {
	FetchCardRatings(ctx context.Context, setCode, format string) ([]seventeenlands.CardRating, error)
	FetchColorRatings(ctx context.Context, setCode, format, startDate, endDate string) ([]seventeenlands.ColorRating, error)
}

// SetFetcher retrieves active standard set metadata from an external source.
type SetFetcher interface {
	FetchSets(ctx context.Context) ([]scryfall.ScryfallSet, error)
}

// Scheduler runs a daily fetch of card ratings for all active sets.
type Scheduler struct {
	fetcher      Fetcher
	setFetcher   SetFetcher
	store        datasets.Store
	overrideSets []string // non-empty when SYNC_ACTIVE_SETS is set; bypasses DB lookup
	formats      []string // draft formats to sync per set
	refreshHour  int
	newTicker    func(d time.Duration) (<-chan time.Time, func())
}

// New creates a Scheduler reading configuration from environment variables.
//
// SYNC_REFRESH_HOUR — hour of day (0–23) to run the refresh (default 2).
// SYNC_ACTIVE_SETS  — optional override; comma-separated set codes (e.g. "FDN,BLB").
//
//	When unset, active sets are queried from the database each run using
//	is_standard_legal = TRUE so the scheduler stays current without redeployment.
//
// SYNC_FORMATS — optional override; comma-separated 17Lands format names
//
//	(e.g. "PremierDraft,QuickDraft,Sealed"). Defaults to PremierDraft,QuickDraft.
func New(setFetcher SetFetcher, fetcher Fetcher, store datasets.Store) *Scheduler {
	hour := defaultRefreshHour
	if v := os.Getenv("SYNC_REFRESH_HOUR"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h >= 0 && h <= 23 {
			hour = h
		} else {
			log.Printf("[sync] invalid SYNC_REFRESH_HOUR=%q: falling back to default %d", v, defaultRefreshHour)
		}
	}

	var override []string
	if v := os.Getenv("SYNC_ACTIVE_SETS"); v != "" {
		for _, s := range strings.Split(v, ",") {
			if t := strings.TrimSpace(s); t != "" {
				override = append(override, t)
			}
		}
	}

	formats := defaultFormats
	if v := os.Getenv("SYNC_FORMATS"); v != "" {
		var parsed []string
		for _, f := range strings.Split(v, ",") {
			if t := strings.TrimSpace(f); t != "" {
				parsed = append(parsed, t)
			}
		}
		if len(parsed) > 0 {
			formats = parsed
		}
	}

	return &Scheduler{
		fetcher:      fetcher,
		setFetcher:   setFetcher,
		store:        store,
		overrideSets: override,
		formats:      formats,
		refreshHour:  hour,
		newTicker:    defaultTicker,
	}
}

func defaultTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Start blocks until ctx is cancelled, running a fetch at the configured hour each day.
func (s *Scheduler) Start(ctx context.Context) {
	src := "db"
	if len(s.overrideSets) > 0 {
		src = "SYNC_ACTIVE_SETS override"
	}
	log.Printf("[sync] scheduler starting: refresh_hour=%d sets_source=%s formats=%v", s.refreshHour, src, s.formats)

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

// activeSets returns the SyncSets to process for this run.
// Override sets (SYNC_ACTIVE_SETS) default to ExpansionCode == Code.
func (s *Scheduler) activeSets(ctx context.Context) ([]datasets.SyncSet, error) {
	if len(s.overrideSets) > 0 {
		sets := make([]datasets.SyncSet, len(s.overrideSets))
		for i, code := range s.overrideSets {
			sets[i] = datasets.SyncSet{Code: code, ExpansionCode: code}
		}
		return sets, nil
	}
	return s.store.GetActiveSets(ctx)
}

func (s *Scheduler) runFetch(ctx context.Context) {
	// Sync Scryfall set metadata first so is_standard_legal stays current.
	scryfallSets, err := s.setFetcher.FetchSets(ctx)
	if err != nil {
		log.Printf("[sync] fetch scryfall sets: %v", err)
	} else if len(scryfallSets) > 0 {
		if err := s.store.UpsertSets(ctx, scryfallSets); err != nil {
			log.Printf("[sync] upsert scryfall sets: %v", err)
		} else {
			log.Printf("[sync] synced %d standard sets from Scryfall", len(scryfallSets))
		}
	}

	sets, err := s.activeSets(ctx)
	if err != nil {
		log.Printf("[sync] resolve active sets: %v", err)
		return
	}
	if len(sets) == 0 {
		log.Println("[sync] no standard-legal sets found, skipping fetch")
		return
	}

	codes := make([]string, len(sets))
	for i, ss := range sets {
		codes[i] = ss.Code
	}
	log.Printf("[sync] fetching ratings for %d sets x %d formats: sets=%v formats=%v", len(sets), len(s.formats), codes, s.formats)

	for _, set := range sets {
		for _, format := range s.formats {
			if ctx.Err() != nil {
				return
			}

			// Use the 17Lands expansion code for the API request.
			ratings, err := s.fetcher.FetchCardRatings(ctx, set.ExpansionCode, format)
			if err != nil {
				log.Printf("[sync] fetch %s/%s: %v", set.Code, format, err)
				continue
			}

			if len(ratings) == 0 {
				log.Printf("[sync] WARNING: 0 cards returned for %s/%s (17Lands expansion=%s) — check expansion code or upstream outage",
					set.Code, format, set.ExpansionCode)
				continue
			}

			// DB write keyed on Scryfall Code.
			sr := draftdata.SetRatings{
				SetCode:     set.Code,
				DraftFormat: format,
				FetchedAt:   time.Now().UTC(),
				Cards:       ratings,
			}

			if err := s.store.UpsertRatings(ctx, sr); err != nil {
				log.Printf("[sync] upsert %s/%s: %v", set.Code, format, err)
				continue
			}

			log.Printf("[sync] refreshed %s/%s: %d cards", set.Code, format, len(ratings))

			// Fetch and persist per-color-combination win rates. A failure here is
			// non-fatal — card ratings are already stored and color data is best-effort.
			if ctx.Err() != nil {
				return
			}

			// Rolling 2-year date window — mirrors the Lambda handler approach.
			now := time.Now().UTC()
			startDate := now.AddDate(-2, 0, 0).Format("2006-01-02")
			endDate := now.Format("2006-01-02")

			// Use the 17Lands expansion code for the color ratings request.
			colorRatings, err := s.fetcher.FetchColorRatings(ctx, set.ExpansionCode, format, startDate, endDate)
			if err != nil {
				log.Printf("[sync] fetch color ratings %s/%s: %v", set.Code, format, err)
				continue
			}

			var filtered []seventeenlands.ColorRating
			for _, cr := range colorRatings {
				if !cr.IsSummary {
					filtered = append(filtered, cr)
				}
			}

			if len(filtered) == 0 {
				log.Printf("[sync] no color ratings returned for %s/%s", set.Code, format)
				continue
			}

			// DB write for color ratings keyed on Scryfall Code.
			if err := s.store.UpsertColorRatings(ctx, set.Code, format, filtered); err != nil {
				log.Printf("[sync] upsert color ratings %s/%s: %v", set.Code, format, err)
				continue
			}

			log.Printf("[sync] refreshed color ratings %s/%s: %d combinations", set.Code, format, len(filtered))
		}
	}
}
