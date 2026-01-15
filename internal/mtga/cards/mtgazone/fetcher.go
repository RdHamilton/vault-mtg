package mtgazone

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/repository"
)

// Fetcher fetches and stores MTG Arena Zone ratings.
type Fetcher struct {
	scraper     *Scraper
	cfbRepo     repository.CFBRatingsRepository
	setCardRepo repository.SetCardRepository
}

// FetcherOptions configures the fetcher.
type FetcherOptions struct {
	ScraperOptions ScraperOptions
}

// NewFetcher creates a new MTG Arena Zone fetcher.
func NewFetcher(cfbRepo repository.CFBRatingsRepository, setCardRepo repository.SetCardRepository, options FetcherOptions) *Fetcher {
	return &Fetcher{
		scraper:     NewScraper(options.ScraperOptions),
		cfbRepo:     cfbRepo,
		setCardRepo: setCardRepo,
	}
}

// FetchAndStoreRatings fetches ratings from MTG Arena Zone and stores them.
// Returns the number of ratings stored.
func (f *Fetcher) FetchAndStoreRatings(ctx context.Context, setCode string) (int, error) {
	setCode = strings.ToUpper(setCode)

	log.Printf("[MTGAZone Fetcher] Fetching ratings for set %s", setCode)

	// Scrape ratings from MTG Arena Zone
	scrapedRatings, err := f.scraper.GetSetRatings(ctx, setCode)
	if err != nil {
		return 0, fmt.Errorf("failed to scrape ratings: %w", err)
	}

	if len(scrapedRatings) == 0 {
		log.Printf("[MTGAZone Fetcher] No ratings found for set %s", setCode)
		return 0, nil
	}

	log.Printf("[MTGAZone Fetcher] Scraped %d ratings for set %s", len(scrapedRatings), setCode)

	// Convert to CFBRating format and store
	stored := 0
	now := time.Now()

	for _, sr := range scrapedRatings {
		rating := &models.CFBRating{
			CardName:      sr.CardName,
			SetCode:       setCode,
			LimitedRating: sr.Rating,
			LimitedScore:  models.LimitedRatingToScore(sr.Rating),
			SourceURL:     fmt.Sprintf("%s/%s-limited-set-review-%s/", BaseURL, f.getSetSlug(setCode), sr.Color),
			Author:        "MTG Arena Zone",
			ImportedAt:    now,
			UpdatedAt:     now,
		}

		if err := f.cfbRepo.UpsertRating(ctx, rating); err != nil {
			log.Printf("[MTGAZone Fetcher] Warning: failed to store rating for %s: %v", sr.CardName, err)
			continue
		}
		stored++
	}

	log.Printf("[MTGAZone Fetcher] Stored %d/%d ratings for set %s", stored, len(scrapedRatings), setCode)

	// Link Arena IDs after storing
	linked, err := f.LinkArenaIDs(ctx, setCode)
	if err != nil {
		log.Printf("[MTGAZone Fetcher] Warning: failed to link Arena IDs: %v", err)
	} else {
		log.Printf("[MTGAZone Fetcher] Linked %d ratings to Arena IDs for set %s", linked, setCode)
	}

	return stored, nil
}

// LinkArenaIDs links stored ratings to Arena IDs by matching card names.
func (f *Fetcher) LinkArenaIDs(ctx context.Context, setCode string) (int, error) {
	setCode = strings.ToUpper(setCode)

	// Get all set cards for this set
	cards, err := f.setCardRepo.GetCardsBySet(ctx, setCode)
	if err != nil {
		return 0, fmt.Errorf("failed to get set cards: %w", err)
	}

	if len(cards) == 0 {
		log.Printf("[MTGAZone Fetcher] No set cards found for %s - ratings won't be linked to Arena IDs", setCode)
		return 0, nil
	}

	// Build name to Arena ID map (case-insensitive)
	cardNameToArenaID := make(map[string]int)
	for _, card := range cards {
		normalizedName := strings.TrimSpace(strings.ToLower(card.Name))
		var arenaID int
		if _, err := fmt.Sscanf(card.ArenaID, "%d", &arenaID); err == nil {
			cardNameToArenaID[normalizedName] = arenaID
		}
	}

	// Get ratings for this set
	ratings, err := f.cfbRepo.GetRatingsForSet(ctx, setCode)
	if err != nil {
		return 0, fmt.Errorf("failed to get ratings: %w", err)
	}

	linked := 0
	for _, rating := range ratings {
		// Skip if already linked
		if rating.ArenaID != nil {
			continue
		}

		// Try to find matching Arena ID
		normalizedName := strings.TrimSpace(strings.ToLower(rating.CardName))
		if arenaID, found := cardNameToArenaID[normalizedName]; found {
			rating.ArenaID = &arenaID
			if err := f.cfbRepo.UpsertRating(ctx, rating); err != nil {
				log.Printf("[MTGAZone Fetcher] Warning: failed to link Arena ID for %s: %v", rating.CardName, err)
				continue
			}
			linked++
		}
	}

	return linked, nil
}

// getSetSlug returns the URL slug for a set code.
func (f *Fetcher) getSetSlug(setCode string) string {
	slug, ok := f.scraper.GetSetMapping(setCode)
	if !ok {
		return strings.ToLower(setCode)
	}
	return slug
}

// HasRatings checks if ratings exist for a set.
func (f *Fetcher) HasRatings(ctx context.Context, setCode string) (bool, error) {
	count, err := f.cfbRepo.GetRatingsCount(ctx, strings.ToUpper(setCode))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetRatingsAge returns how old the ratings are for a set.
func (f *Fetcher) GetRatingsAge(ctx context.Context, setCode string) (time.Duration, error) {
	ratings, err := f.cfbRepo.GetRatingsForSet(ctx, strings.ToUpper(setCode))
	if err != nil {
		return 0, err
	}
	if len(ratings) == 0 {
		return 0, fmt.Errorf("no ratings found for set %s", setCode)
	}

	// Find the oldest imported_at timestamp
	var oldest time.Time
	for _, r := range ratings {
		if oldest.IsZero() || r.ImportedAt.Before(oldest) {
			oldest = r.ImportedAt
		}
	}

	return time.Since(oldest), nil
}

// ShouldRefresh determines if ratings should be refreshed.
// Ratings are considered stale after 7 days during active set season,
// or 30 days for older sets.
func (f *Fetcher) ShouldRefresh(ctx context.Context, setCode string, isActiveSeason bool) (bool, error) {
	age, err := f.GetRatingsAge(ctx, setCode)
	if err != nil {
		// No ratings - should fetch
		return true, nil
	}

	staleThreshold := 30 * 24 * time.Hour // 30 days for older sets
	if isActiveSeason {
		staleThreshold = 7 * 24 * time.Hour // 7 days for active sets
	}

	return age > staleThreshold, nil
}

// RefreshIfNeeded fetches ratings if they don't exist or are stale.
func (f *Fetcher) RefreshIfNeeded(ctx context.Context, setCode string, isActiveSeason bool) (int, bool, error) {
	setCode = strings.ToUpper(setCode)

	shouldRefresh, err := f.ShouldRefresh(ctx, setCode, isActiveSeason)
	if err != nil {
		// Error checking - try to fetch anyway
		shouldRefresh = true
	}

	if !shouldRefresh {
		return 0, false, nil
	}

	count, err := f.FetchAndStoreRatings(ctx, setCode)
	return count, true, err
}

// AddSetMapping adds a custom set code to URL slug mapping.
func (f *Fetcher) AddSetMapping(setCode, slug string) {
	f.scraper.AddSetMapping(setCode, slug)
}
