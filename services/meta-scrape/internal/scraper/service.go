package scraper

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/store"
)

// archetypeUpserter is the write-side dependency PersistMeta needs from the
// store package (#176). *store.MetaStore satisfies it. Declaring it as an
// interface keeps the Service decoupled from a live pgxpool so PersistMeta and
// RefreshAll are unit-testable with a fake — no DB, no network.
//
// Only UpsertArchetypes is required here. UpsertArchetypeCards / ArchetypeIDByKey
// wiring (the card-list upsert) is deferred to #177's Lambda handler, which has
// the post-upsert archetype-ID loop context.
type archetypeUpserter interface {
	UpsertArchetypes(ctx context.Context, archetypes []store.Archetype) error
}

// Service aggregates meta data from multiple sources and persists the result
// to the MetaStore write side (#176).
type Service struct {
	goldfishClient *GoldfishClient
	top8Client     *Top8Client
	store          archetypeUpserter
	mu             sync.RWMutex
}

// ServiceConfig configures the meta service.
type ServiceConfig struct {
	GoldfishConfig *GoldfishConfig
	Top8Config     *Top8Config
}

// AggregatedMeta combines meta data from all sources.
type AggregatedMeta struct {
	Format          string                 `json:"format"`
	Archetypes      []*AggregatedArchetype `json:"archetypes"`
	TopDecks        []*MetaDeck            `json:"top_decks"`
	Tournaments     []*Tournament          `json:"tournaments,omitempty"`
	TotalArchetypes int                    `json:"total_archetypes"`
	LastUpdated     time.Time              `json:"last_updated"`
	Sources         []string               `json:"sources"`
}

// AggregatedArchetype combines archetype data from multiple sources.
type AggregatedArchetype struct {
	Name            string    `json:"name"`
	NormalizedName  string    `json:"normalized_name"`
	Colors          []string  `json:"colors"`
	MetaShare       float64   `json:"meta_share"`       // From MTGGoldfish
	TournamentTop8s int       `json:"tournament_top8s"` // From MTGTop8
	TournamentWins  int       `json:"tournament_wins"`  // From MTGTop8
	Tier            int       `json:"tier"`             // 1-4 based on combined data
	ConfidenceScore float64   `json:"confidence_score"` // How reliable the data is
	TrendDirection  string    `json:"trend_direction"`  // "up", "down", "stable"
	LastSeenInMeta  time.Time `json:"last_seen_in_meta"`
	LastSeenInTop8  time.Time `json:"last_seen_in_top8,omitempty"`
}

// NewService creates a new meta service. The store may be nil for read-only use
// (e.g. ad-hoc querying without a write path); PersistMeta and RefreshAll then
// skip persistence.
func NewService(config *ServiceConfig, st *store.MetaStore) *Service {
	if st == nil {
		return newService(config, nil)
	}
	return newService(config, st)
}

// NewServiceWithStore creates a service backed by any archetypeUpserter. It is
// the seam used by unit tests (fake upserter) and by #177's Lambda handler
// (real *store.MetaStore via NewService).
func NewServiceWithStore(config *ServiceConfig, st archetypeUpserter) *Service {
	return newService(config, st)
}

func newService(config *ServiceConfig, st archetypeUpserter) *Service {
	if config == nil {
		config = &ServiceConfig{}
	}

	return &Service{
		goldfishClient: NewGoldfishClient(config.GoldfishConfig),
		top8Client:     NewTop8Client(config.Top8Config),
		store:          st,
	}
}

// GetAggregatedMeta returns combined meta data from all sources.
func (s *Service) GetAggregatedMeta(ctx context.Context, format string) (*AggregatedMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Fetch from both sources concurrently
	type goldfishResult struct {
		meta *FormatMeta
		err  error
	}
	type top8Result struct {
		meta *TournamentMeta
		err  error
	}

	goldfishCh := make(chan goldfishResult, 1)
	top8Ch := make(chan top8Result, 1)

	go func() {
		meta, err := s.goldfishClient.GetMeta(ctx, format)
		goldfishCh <- goldfishResult{meta, err}
	}()

	go func() {
		meta, err := s.top8Client.GetTournamentMeta(ctx, format)
		top8Ch <- top8Result{meta, err}
	}()

	// Collect results
	var goldfishMeta *FormatMeta
	var top8Meta *TournamentMeta
	var sources []string

	gfResult := <-goldfishCh
	if gfResult.err == nil && gfResult.meta != nil {
		goldfishMeta = gfResult.meta
		sources = append(sources, "mtggoldfish")
	}

	t8Result := <-top8Ch
	if t8Result.err == nil && t8Result.meta != nil {
		top8Meta = t8Result.meta
		sources = append(sources, "mtgtop8")
	}

	if goldfishMeta == nil && top8Meta == nil {
		return nil, fmt.Errorf("failed to fetch meta from any source")
	}

	// Aggregate the data
	aggregated := s.aggregateData(format, goldfishMeta, top8Meta)
	aggregated.Sources = sources
	aggregated.LastUpdated = time.Now()

	return aggregated, nil
}

// aggregateData combines data from both sources.
func (s *Service) aggregateData(format string, goldfish *FormatMeta, top8 *TournamentMeta) *AggregatedMeta {
	archetypeMap := make(map[string]*AggregatedArchetype)

	// Process MTGGoldfish data
	if goldfish != nil {
		for _, deck := range goldfish.Decks {
			normalized := strings.ToLower(deck.ArchetypeName)
			if _, exists := archetypeMap[normalized]; !exists {
				archetypeMap[normalized] = &AggregatedArchetype{
					Name:           deck.Name,
					NormalizedName: normalized,
					Colors:         deck.Colors,
					Tier:           deck.Tier,
					LastSeenInMeta: deck.LastUpdated,
				}
			}
			archetypeMap[normalized].MetaShare = deck.MetaShare
			archetypeMap[normalized].LastSeenInMeta = deck.LastUpdated
		}
	}

	// Process MTGTop8 data
	if top8 != nil {
		for name, stats := range top8.ArchetypeStats {
			normalized := strings.ToLower(name)
			if existing, exists := archetypeMap[normalized]; exists {
				existing.TournamentTop8s = stats.Top8Count
				existing.TournamentWins = stats.WinCount
				existing.LastSeenInTop8 = stats.LastSeen
				if stats.TrendDirection != "" {
					existing.TrendDirection = stats.TrendDirection
				}
				// Merge colors if needed
				if len(existing.Colors) == 0 {
					existing.Colors = stats.Colors
				}
			} else {
				archetypeMap[normalized] = &AggregatedArchetype{
					Name:            stats.ArchetypeName,
					NormalizedName:  normalized,
					Colors:          stats.Colors,
					TournamentTop8s: stats.Top8Count,
					TournamentWins:  stats.WinCount,
					TrendDirection:  stats.TrendDirection,
					LastSeenInTop8:  stats.LastSeen,
				}
			}
		}
	}

	// Calculate confidence scores and finalize tiers
	archetypes := make([]*AggregatedArchetype, 0, len(archetypeMap))
	for _, arch := range archetypeMap {
		arch.ConfidenceScore = s.calculateConfidence(arch)
		if arch.Tier == 0 {
			arch.Tier = s.calculateTier(arch)
		}
		if arch.TrendDirection == "" {
			arch.TrendDirection = "stable"
		}
		archetypes = append(archetypes, arch)
	}

	// Sort by combined relevance (meta share + tournament presence)
	sort.Slice(archetypes, func(i, j int) bool {
		scoreI := archetypes[i].MetaShare + float64(archetypes[i].TournamentTop8s)*0.5
		scoreJ := archetypes[j].MetaShare + float64(archetypes[j].TournamentTop8s)*0.5
		return scoreI > scoreJ
	})

	// Build top decks list from goldfish
	var topDecks []*MetaDeck
	if goldfish != nil {
		topDecks = goldfish.Decks
	}

	// Build tournaments list from top8
	var tournaments []*Tournament
	if top8 != nil {
		tournaments = top8.Tournaments
	}

	return &AggregatedMeta{
		Format:          format,
		Archetypes:      archetypes,
		TopDecks:        topDecks,
		Tournaments:     tournaments,
		TotalArchetypes: len(archetypes),
	}
}

// calculateConfidence calculates how confident we are in the archetype data.
func (s *Service) calculateConfidence(arch *AggregatedArchetype) float64 {
	confidence := 0.0

	// Has meta share data
	if arch.MetaShare > 0 {
		confidence += 0.4
	}

	// Has tournament data
	if arch.TournamentTop8s > 0 {
		confidence += 0.3
		// More top 8s = more confidence
		if arch.TournamentTop8s >= 10 {
			confidence += 0.1
		}
		if arch.TournamentTop8s >= 20 {
			confidence += 0.1
		}
	}

	// Has color data
	if len(arch.Colors) > 0 {
		confidence += 0.1
	}

	return confidence
}

// calculateTier determines the tier based on available data.
func (s *Service) calculateTier(arch *AggregatedArchetype) int {
	// Use meta share if available
	if arch.MetaShare >= 5.0 {
		return 1
	}
	if arch.MetaShare >= 2.0 {
		return 2
	}
	if arch.MetaShare >= 0.5 {
		return 3
	}

	// Fall back to tournament data
	if arch.TournamentTop8s >= 20 {
		return 1
	}
	if arch.TournamentTop8s >= 10 {
		return 2
	}
	if arch.TournamentTop8s >= 5 {
		return 3
	}

	return 4 // Untiered
}

// PersistMeta maps the aggregated archetypes to the store write-side model and
// upserts them via UpsertArchetypes (#176). It is a no-op when the service has
// no store or when the archetype list is empty (control FH-1: an empty scrape
// result must never reach the store as a clear signal).
//
// Field mapping (per Ray's #175 review): scalar zero values map to nil pointers
// so the upsert writes SQL NULL for absent data. Description / PlayStyle /
// SourceURL are not present in AggregatedArchetype and are left nil here; #177
// may populate them from scraper metadata. Card-list upsert (UpsertArchetypeCards)
// is #177's Lambda-handler concern and is intentionally not called here.
func (s *Service) PersistMeta(ctx context.Context, format string, meta *AggregatedMeta) error {
	if s.store == nil {
		return nil
	}
	if meta == nil || len(meta.Archetypes) == 0 {
		return nil
	}

	archetypes := make([]store.Archetype, 0, len(meta.Archetypes))
	for _, a := range meta.Archetypes {
		archetypes = append(archetypes, store.Archetype{
			Name:            a.Name,
			Format:          format,
			Tier:            tierPtr(a.Tier),
			MetaShare:       float32Ptr(a.MetaShare),
			TournamentTop8s: intPtr(a.TournamentTop8s),
			TournamentWins:  intPtr(a.TournamentWins),
			ConfidenceScore: float32Ptr(a.ConfidenceScore),
			TrendDirection:  stringPtr(a.TrendDirection),
		})
	}

	if err := s.store.UpsertArchetypes(ctx, archetypes); err != nil {
		return fmt.Errorf("persist meta for format %q: %w", format, err)
	}
	return nil
}

// tierPtr converts a tier int to a string pointer, returning nil for tier 0
// (untiered / unknown) so the column is written as NULL.
func tierPtr(tier int) *string {
	if tier == 0 {
		return nil
	}
	s := strconv.Itoa(tier)
	return &s
}

// float32Ptr returns a *float32 for non-zero values, nil otherwise.
func float32Ptr(v float64) *float32 {
	if v == 0 {
		return nil
	}
	f := float32(v)
	return &f
}

// intPtr returns a *int for non-zero values, nil otherwise.
func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

// stringPtr returns a *string for non-empty values, nil otherwise.
func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// GetTopArchetypes returns the top N archetypes for a format.
func (s *Service) GetTopArchetypes(ctx context.Context, format string, limit int) ([]*AggregatedArchetype, error) {
	meta, err := s.GetAggregatedMeta(ctx, format)
	if err != nil {
		return nil, err
	}

	if limit <= 0 || limit > len(meta.Archetypes) {
		return meta.Archetypes, nil
	}

	return meta.Archetypes[:limit], nil
}

// GetArchetypeByName finds an archetype by name.
func (s *Service) GetArchetypeByName(ctx context.Context, format, name string) (*AggregatedArchetype, error) {
	meta, err := s.GetAggregatedMeta(ctx, format)
	if err != nil {
		return nil, err
	}

	nameLower := strings.ToLower(name)
	for _, arch := range meta.Archetypes {
		if arch.NormalizedName == nameLower || strings.Contains(arch.NormalizedName, nameLower) {
			return arch, nil
		}
	}

	return nil, fmt.Errorf("archetype not found: %s", name)
}

// GetTier1Archetypes returns all tier 1 archetypes for a format.
func (s *Service) GetTier1Archetypes(ctx context.Context, format string) ([]*AggregatedArchetype, error) {
	meta, err := s.GetAggregatedMeta(ctx, format)
	if err != nil {
		return nil, err
	}

	tier1 := make([]*AggregatedArchetype, 0)
	for _, arch := range meta.Archetypes {
		if arch.Tier == 1 {
			tier1 = append(tier1, arch)
		}
	}

	return tier1, nil
}

// GetArchetypesByColors returns archetypes matching the given colors.
func (s *Service) GetArchetypesByColors(ctx context.Context, format string, colors []string) ([]*AggregatedArchetype, error) {
	meta, err := s.GetAggregatedMeta(ctx, format)
	if err != nil {
		return nil, err
	}

	matching := make([]*AggregatedArchetype, 0)
	for _, arch := range meta.Archetypes {
		if s.colorsMatch(arch.Colors, colors) {
			matching = append(matching, arch)
		}
	}

	return matching, nil
}

// colorsMatch checks if archetype colors match the given colors.
func (s *Service) colorsMatch(archetypeColors, targetColors []string) bool {
	if len(targetColors) == 0 {
		return true
	}

	// Check if all target colors are in archetype colors
	for _, target := range targetColors {
		found := false
		for _, arch := range archetypeColors {
			if arch == target {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// GetRecentTournaments returns recent tournaments for a format.
func (s *Service) GetRecentTournaments(ctx context.Context, format string, limit int) ([]*Tournament, error) {
	return s.top8Client.GetRecentTournaments(ctx, format, limit)
}

// RefreshAll forces a refresh of all meta data for a format, then persists the
// aggregated result to the store. This is the entry point #177's Lambda handler
// invokes: a single call both fetches (cache-busting) and writes.
func (s *Service) RefreshAll(ctx context.Context, format string) (*AggregatedMeta, error) {
	// Clear caches under write lock
	s.mu.Lock()
	s.goldfishClient.ClearCache()
	s.top8Client.ClearCache()
	s.mu.Unlock()

	// GetAggregatedMeta handles its own locking.
	meta, err := s.GetAggregatedMeta(ctx, format)
	if err != nil {
		return nil, err
	}

	// Persist after aggregation (no-op when store is nil or result is empty).
	if err := s.PersistMeta(ctx, format, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

// ArchetypeCardList pairs an archetype's natural key (Name + Format) with the
// mainboard/sideboard card list scraped for it. #177's Lambda handler consumes
// this to wire store.UpsertArchetypeCards: it looks up the archetype id via
// store.ArchetypeIDByKey(Name, Format) and upserts the Cards under that id.
//
// This is the card-list seam that #176/#175 deferred to #177. The Service owns
// the scrape-to-card mapping (it has the *MetaDeck data); the handler owns the
// id-lookup + child-table write (it has the concrete *store.MetaStore).
type ArchetypeCardList struct {
	Name   string
	Format string
	Cards  []store.ArchetypeCard
}

// CardListsFromMeta derives the per-archetype card lists from an already
// aggregated meta result. It maps each MTGGoldfish top deck's mainboard and
// sideboard entries to store.ArchetypeCard rows, keyed by the deck's archetype
// name and the format. Decks with no card list are skipped (an empty card list
// is never written — control FH-1, mirrors UpsertArchetypeCards' no-op guard).
//
// Role is "mainboard" or "sideboard"; Copies is the scraped quantity. Importance
// and Notes are not available from the scrape and are left nil (SQL NULL).
func (s *Service) CardListsFromMeta(meta *AggregatedMeta) []ArchetypeCardList {
	if meta == nil || len(meta.TopDecks) == 0 {
		return nil
	}

	lists := make([]ArchetypeCardList, 0, len(meta.TopDecks))
	for _, deck := range meta.TopDecks {
		if deck == nil {
			continue
		}
		cards := make([]store.ArchetypeCard, 0, len(deck.MainboardCards)+len(deck.SideboardCards))
		for _, c := range deck.MainboardCards {
			if c.Name == "" {
				continue
			}
			cards = append(cards, store.ArchetypeCard{
				CardName: c.Name,
				Role:     "mainboard",
				Copies:   c.Quantity,
			})
		}
		for _, c := range deck.SideboardCards {
			if c.Name == "" {
				continue
			}
			cards = append(cards, store.ArchetypeCard{
				CardName: c.Name,
				Role:     "sideboard",
				Copies:   c.Quantity,
			})
		}
		if len(cards) == 0 {
			continue
		}
		lists = append(lists, ArchetypeCardList{
			Name:   deck.Name,
			Format: meta.Format,
			Cards:  cards,
		})
	}

	return lists
}

// GetSupportedFormats returns the list of supported formats.
func (s *Service) GetSupportedFormats() []string {
	return []string{
		"standard",
		"historic",
		"explorer",
		"pioneer",
		"modern",
		"legacy",
		"vintage",
		"pauper",
		"alchemy",
		"timeless",
	}
}

// IsFormatSupported checks if a format is supported.
func (s *Service) IsFormatSupported(format string) bool {
	formatLower := strings.ToLower(format)
	for _, f := range s.GetSupportedFormats() {
		if f == formatLower {
			return true
		}
	}
	return false
}
