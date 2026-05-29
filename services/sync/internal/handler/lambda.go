// Package handler provides the AWS Lambda handler for the mtga-sync service.
// AWS EventBridge Scheduler invokes this handler on a configurable cron schedule,
// replacing the long-running ticker loop that was used for local/server deployments.
package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/draftdata"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
)

// defaultFormats is the canonical list of 17Lands draft formats synced per set.
// Sealed is omitted by default — it has far fewer games logged and the data is
// lower confidence. Set SYNC_FORMATS to override (e.g. "PremierDraft,QuickDraft,Sealed").
var defaultFormats = []string{"PremierDraft", "QuickDraft"}

const (
	// defaultMaxRetries is the number of retry attempts per fetch/upsert on transient
	// errors. A value of 2 means up to 3 total attempts (1 initial + 2 retries).
	defaultMaxRetries = 2

	// defaultMaxConsecutiveSkipDays is the number of consecutive daily invocations that
	// must return 0 cards before the skip guard emits a WARNING log for that set.
	// The guard is non-fatal: threshold breaches are observable via CloudWatch Logs
	// Insights but never abort the Lambda invocation or cause EventBridge retries.
	// See: docs/runbooks/sync-dlq-alarms.md
	defaultMaxConsecutiveSkipDays = 3

	// defaultInterRequestSleepMs is the inter-request pause injected between
	// consecutive set×format API calls in the sync loop. 150 ms is a conservative
	// courtesy delay that keeps us well under 17Lands' undocumented rate limit while
	// adding only ~30 s to a typical sync run (2 formats × 10 active sets).
	// Override with SYNC_INTER_REQUEST_SLEEP_MS.
	defaultInterRequestSleepMs = 150

	// skipHashPrefix namespaces the consecutive-skip counter inside the sync_hashes
	// table so it cannot collide with ADR-005 payload hashes (which use set/format keys).
	skipHashPrefix = "skip_count:"
)

// Fetcher retrieves card and color ratings from an external source.
type Fetcher interface {
	FetchCardRatings(ctx context.Context, setCode, format string) ([]seventeenlands.CardRating, error)
	FetchColorRatings(ctx context.Context, setCode, format, startDate, endDate string) ([]seventeenlands.ColorRating, error)
}

// CardFetcher retrieves Scryfall card metadata from the bulk-data endpoint.
// The returned slice contains only Arena-tagged cards (non-null arena_id).
type CardFetcher interface {
	FetchBulkDefaultCards(ctx context.Context) ([]scryfall.ScryfallCard, error)
}

// SyncHandler is the Lambda handler that fetches card ratings for all active sets
// and persists them to Postgres. Each invocation performs a single full refresh.
type SyncHandler struct {
	fetcher             Fetcher
	cardFetcher         CardFetcher // fetches Scryfall bulk card metadata; may be nil (skips syncCards)
	store               datasets.Store
	overrideSets        []string      // non-empty when caller provides an explicit set list
	formats             []string      // draft formats to sync; read from SYNC_FORMATS env var
	maxRetries          int           // per-fetch/upsert retry attempts (0 = no retries)
	maxConsecutiveSkips int           // zero-card invocations before WARNING log (0 = disabled; non-fatal)
	interRequestSleep   time.Duration // courtesy pause between consecutive set×format API calls
	// retryBackoff returns the duration to sleep before attempt n (1-indexed).
	// Defaults to exponentialBackoff. Injectable for tests to use noBackoff.
	retryBackoff func(attempt int) time.Duration
}

// New creates a SyncHandler. overrideSets may be nil/empty to use DB-driven active sets.
//
// cardFetcher may be nil; when nil, the syncCards step is skipped this invocation.
//
// The formats list is read from SYNC_FORMATS (comma-separated). If unset, defaultFormats
// is used: PremierDraft and QuickDraft.
//
// Retry counts are read from SYNC_MAX_RETRIES and SYNC_MAX_CONSECUTIVE_SKIP_DAYS env vars;
// defaults are defaultMaxRetries and defaultMaxConsecutiveSkipDays respectively.
func New(fetcher Fetcher, cardFetcher CardFetcher, store datasets.Store, overrideSets []string) *SyncHandler {
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

	maxRetries := defaultMaxRetries
	if v := os.Getenv("SYNC_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			maxRetries = n
		}
	}

	maxConsecutiveSkips := defaultMaxConsecutiveSkipDays
	if v := os.Getenv("SYNC_MAX_CONSECUTIVE_SKIP_DAYS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			maxConsecutiveSkips = n
		}
	}

	interRequestSleep := time.Duration(defaultInterRequestSleepMs) * time.Millisecond
	if v := os.Getenv("SYNC_INTER_REQUEST_SLEEP_MS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			interRequestSleep = time.Duration(n) * time.Millisecond
		}
	}

	return &SyncHandler{
		fetcher:             fetcher,
		cardFetcher:         cardFetcher,
		store:               store,
		overrideSets:        overrideSets,
		formats:             formats,
		maxRetries:          maxRetries,
		maxConsecutiveSkips: maxConsecutiveSkips,
		interRequestSleep:   interRequestSleep,
		retryBackoff:        exponentialBackoff,
	}
}

// NewWithFormats creates a SyncHandler with an explicit formats list, bypassing the
// SYNC_FORMATS env var. Intended for tests that need deterministic format control.
//
// cardFetcher is nil (syncCards step disabled) so existing test expectations are
// preserved. maxRetries, maxConsecutiveSkips, and interRequestSleep are all 0 (disabled).
func NewWithFormats(fetcher Fetcher, store datasets.Store, overrideSets, formats []string) *SyncHandler {
	return &SyncHandler{
		fetcher:             fetcher,
		cardFetcher:         nil,
		store:               store,
		overrideSets:        overrideSets,
		formats:             formats,
		maxRetries:          0,
		maxConsecutiveSkips: 0,
		interRequestSleep:   0,
		retryBackoff:        exponentialBackoff,
	}
}

// NewWithOptions creates a SyncHandler with fully explicit configuration.
// Intended for tests that need fine-grained control over retry, skip-guard, and
// inter-request sleep behaviour.
//
// cardFetcher may be nil; when nil, the syncCards step is skipped.
func NewWithOptions(
	fetcher Fetcher,
	cardFetcher CardFetcher,
	store datasets.Store,
	overrideSets, formats []string,
	maxRetries, maxConsecutiveSkips int,
	interRequestSleep time.Duration,
	backoff func(attempt int) time.Duration,
) *SyncHandler {
	if backoff == nil {
		backoff = exponentialBackoff
	}
	return &SyncHandler{
		fetcher:             fetcher,
		cardFetcher:         cardFetcher,
		store:               store,
		overrideSets:        overrideSets,
		formats:             formats,
		maxRetries:          maxRetries,
		maxConsecutiveSkips: maxConsecutiveSkips,
		interRequestSleep:   interRequestSleep,
		retryBackoff:        backoff,
	}
}

// Handle is the Lambda handler function. It is invoked by EventBridge Scheduler
// and performs a single ratings refresh across all active sets and formats.
//
// The event payload is ignored — EventBridge scheduled events carry no
// application-level data. Any invocation triggers a full sync.
//
// The consecutive-skip guard is non-fatal: when a set trips the threshold,
// updateSkipGuard emits a WARNING log but never returns an error, so Handle
// continues syncing all remaining sets and always returns nil for skip-guard
// conditions. Handle can still return a non-nil error for context cancellation
// or a DB-query failure in activeSets.
func (h *SyncHandler) Handle(ctx context.Context, _ any) error {
	// Step 1: sync Scryfall card metadata into cards and set_cards tables.
	// Non-fatal: a failure here is logged and the ratings sync continues.
	h.syncCards(ctx)

	// Step 2: sync 17Lands ratings for all active sets.
	sets, err := h.activeSets(ctx)
	if err != nil {
		return err
	}

	if len(sets) == 0 {
		log.Println("[sync] no standard-legal sets found, skipping fetch")
		return nil
	}

	codes := make([]string, len(sets))
	for i, s := range sets {
		codes[i] = s.Code
	}
	log.Printf("[sync] fetching ratings for %d set(s) x %d format(s): sets=%v formats=%v",
		len(sets), len(h.formats), codes, h.formats)

	var firstErr error
	for _, set := range sets {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := h.syncSet(ctx, set); err != nil {
			log.Printf("[sync] syncSet %s: %v", set.Code, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// syncCards fetches the Scryfall default-cards bulk file and writes all
// Arena-tagged cards into set_cards (the sole write target — the retired
// cards table was dropped in migration 000025). It is intentionally non-fatal:
// any failure is logged and the caller continues with the 17Lands ratings sync.
//
// When h.cardFetcher is nil the step is skipped silently (e.g. in tests
// constructed with NewWithFormats / NewWithOptions(cardFetcher=nil)).
func (h *SyncHandler) syncCards(ctx context.Context) {
	if h.cardFetcher == nil {
		return
	}

	cards, err := h.cardFetcher.FetchBulkDefaultCards(ctx)
	if err != nil {
		log.Printf("[sync] syncCards: fetch bulk-data: %v", err)
		return
	}

	log.Printf("[sync] syncCards: fetched %d Arena cards from Scryfall bulk-data", len(cards))

	if err := h.store.UpsertSetCards(ctx, cards); err != nil {
		log.Printf("[sync] syncCards: UpsertSetCards: %v", err)
		return
	}

	log.Printf("[sync] syncCards: upserted %d cards into set_cards", len(cards))
}

// syncSet fetches and upserts ratings for all formats of a single set. It never
// returns an error due to the consecutive-skip guard — the guard is non-fatal and
// only emits a WARNING log at threshold. syncSet can still return a non-nil error
// for context cancellation. A courtesy inter-request pause (h.interRequestSleep)
// is injected between each set×format API call to stay within 17Lands'
// undocumented rate limit.
//
// set.ExpansionCode is sent to the 17Lands API; set.Code is used for all DB
// writes so ratings remain keyed on the stable Scryfall code.
func (h *SyncHandler) syncSet(ctx context.Context, set datasets.SyncSet) error {
	var firstErr error
	for i, format := range h.formats {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Inject inter-request sleep before every call except the first.
		if i > 0 && h.interRequestSleep > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.interRequestSleep):
			}
		}
		if err := h.syncFormat(ctx, set, format); err != nil {
			log.Printf("[sync] %s/%s: %v", set.Code, format, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// syncFormat fetches and upserts ratings for one (set, format) pair with retry.
// Returns a non-nil error only when the consecutive-skip guard trips. Transient
// fetch/upsert errors are retried and swallowed after exhausting retries.
//
// set.ExpansionCode is used for all 17Lands API requests.
// set.Code (Scryfall) is used for all DB writes so ratings are keyed stably.
// The skip guard also keys on set.Code so the counter key never changes when
// the 17Lands expansion code differs from the Scryfall code.
func (h *SyncHandler) syncFormat(ctx context.Context, set datasets.SyncSet, format string) error {
	var (
		ratings  []seventeenlands.CardRating
		fetchErr error
	)

	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.retryBackoff(attempt)):
			}
		}

		// Use the 17Lands expansion code for the API request.
		ratings, fetchErr = h.fetcher.FetchCardRatings(ctx, set.ExpansionCode, format)
		if fetchErr == nil {
			break
		}
		log.Printf("[sync] fetch %s/%s attempt %d/%d: %v", set.Code, format, attempt+1, h.maxRetries+1, fetchErr)
	}

	if fetchErr != nil {
		// All fetch attempts failed — non-fatal; caller logs at syncSet level if needed.
		return nil
	}

	if len(ratings) == 0 {
		log.Printf("[sync] WARNING: 0 cards returned for %s/%s (17Lands expansion=%s) — check expansion code or upstream outage",
			set.Code, format, set.ExpansionCode)
		// Skip guard keyed on Scryfall Code — stable across expansion code changes.
		// Non-fatal: increments the counter and logs a warning at threshold, but
		// never aborts the run so other sets continue syncing.
		h.updateSkipGuard(ctx, set.Code)
		return nil
	}

	// Successful card response: reset the skip counter (keyed on Scryfall Code).
	h.resetSkipGuard(ctx, set.Code)

	// ADR-005: compute a SHA-256 hash over the sorted payload and skip the
	// upsert when the hash matches the previously stored value in sync_hashes.
	// Hash key uses the Scryfall Code so it remains stable.
	hashKey := set.Code + "/" + format
	newHash, hashErr := computeRatingsHash(ratings)
	if hashErr != nil {
		log.Printf("[sync] hash compute %s/%s: %v — proceeding with upsert", set.Code, format, hashErr)
		newHash = ""
	} else {
		storedHash, getErr := h.store.GetHash(ctx, hashKey)
		if getErr != nil {
			log.Printf("[sync] get hash %s/%s: %v — proceeding with upsert", set.Code, format, getErr)
			storedHash = ""
		}
		if storedHash != "" && storedHash == newHash {
			log.Printf("[sync] skipped %s/%s: payload unchanged (hash=%s)", set.Code, format, newHash[:8])
			return nil
		}
	}

	// DB write keyed on Scryfall Code.
	sr := draftdata.SetRatings{
		SetCode:     set.Code,
		DraftFormat: format,
		FetchedAt:   time.Now().UTC(),
		Cards:       ratings,
	}

	var upsertErr error
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.retryBackoff(attempt)):
			}
		}
		upsertErr = h.store.UpsertRatings(ctx, sr)
		if upsertErr == nil {
			break
		}
		log.Printf("[sync] upsert %s/%s attempt %d/%d: %v", set.Code, format, attempt+1, h.maxRetries+1, upsertErr)
	}

	if upsertErr != nil {
		log.Printf("[sync] upsert %s/%s failed after all retries: %v", set.Code, format, upsertErr)
		return nil
	}

	// Store the hash only after a successful upsert.
	if newHash != "" {
		if err := h.store.SetHash(ctx, hashKey, newHash); err != nil {
			// Non-fatal: the upsert succeeded; log and continue so the next run
			// simply re-upserts rather than silently losing data.
			log.Printf("[sync] set hash %s/%s: %v", set.Code, format, err)
		}
	}

	log.Printf("[sync] refreshed %s/%s: %d cards (hash=%s)", set.Code, format, len(ratings), newHash[:8])

	// Fetch and persist per-color-combination win rates. A failure here is
	// non-fatal — card ratings are already stored and color data is best-effort.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Rolling 2-year date window: avoids a Store interface change and covers all
	// active draft sets (none are older than two years). See vault-mtg-tickets#46.
	now := time.Now().UTC()
	startDate := now.AddDate(-2, 0, 0).Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	// Use the 17Lands expansion code for the color ratings request as well.
	colorRatings, err := h.fetcher.FetchColorRatings(ctx, set.ExpansionCode, format, startDate, endDate)
	if err != nil {
		log.Printf("[sync] fetch color ratings %s/%s: %v", set.Code, format, err)
		return nil
	}

	// Filter out is_summary rows — these are aggregate rows returned by the API
	// with integer short_name values that do not represent playable color pairs.
	var filtered []seventeenlands.ColorRating
	for _, cr := range colorRatings {
		if !cr.IsSummary {
			filtered = append(filtered, cr)
		}
	}

	if len(filtered) == 0 {
		log.Printf("[sync] no color ratings returned for %s/%s", set.Code, format)
		return nil
	}

	// DB write for color ratings keyed on Scryfall Code.
	if err := h.store.UpsertColorRatings(ctx, set.Code, format, filtered); err != nil {
		log.Printf("[sync] upsert color ratings %s/%s: %v", set.Code, format, err)
		return nil
	}

	log.Printf("[sync] refreshed color ratings %s/%s: %d combinations", set.Code, format, len(filtered))
	return nil
}

// activeSets returns the SyncSets to process for this invocation.
// When overrideSets is non-empty (SYNC_ACTIVE_SETS env var), each code is
// returned with ExpansionCode == Code — the caller is responsible for knowing
// the correct 17Lands expansion code in override mode (debug/dev path only).
// In normal operation, active sets are queried from the DB where
// COALESCE(seventeenlands_code, code) handles the translation.
func (h *SyncHandler) activeSets(ctx context.Context) ([]datasets.SyncSet, error) {
	if len(h.overrideSets) > 0 {
		sets := make([]datasets.SyncSet, len(h.overrideSets))
		for i, code := range h.overrideSets {
			sets[i] = datasets.SyncSet{Code: code, ExpansionCode: code}
		}
		return sets, nil
	}

	return h.store.GetActiveSets(ctx)
}

// updateSkipGuard increments the consecutive-zero-card counter for setCode in
// the sync_hashes table (using a "skip_count:" prefix). It is intentionally
// non-fatal: when the counter reaches h.maxConsecutiveSkips it emits an
// elevated WARNING log so operators can observe the condition via CloudWatch,
// but it never returns an error. A single set returning 0 cards must not abort
// the entire invocation or cause EventBridge to route to the DLQ.
//
// If h.maxConsecutiveSkips == 0, the guard is disabled and this is a no-op.
func (h *SyncHandler) updateSkipGuard(ctx context.Context, setCode string) {
	if h.maxConsecutiveSkips <= 0 {
		return
	}

	key := skipHashPrefix + setCode
	stored, err := h.store.GetHash(ctx, key)
	if err != nil {
		log.Printf("[sync] skip guard: GetHash %s: %v — skipping guard check", setCode, err)
		return
	}

	count := 0
	if stored != "" {
		if n, parseErr := strconv.Atoi(stored); parseErr == nil {
			count = n
		}
	}
	count++

	log.Printf("[sync] skip guard: set %s returned 0 cards for %d consecutive invocation(s)", setCode, count)

	if setErr := h.store.SetHash(ctx, key, strconv.Itoa(count)); setErr != nil {
		log.Printf("[sync] skip guard: SetHash %s: %v", setCode, setErr)
	}

	if count >= h.maxConsecutiveSkips {
		// Log at WARNING level so this appears in CloudWatch Logs Insights
		// queries and can trigger a metric filter alarm without aborting the run.
		log.Printf("[sync] skip guard WARNING: set %s returned 0 cards for %d consecutive invocations (threshold=%d) — check 17Lands expansion code or upstream outage",
			setCode, count, h.maxConsecutiveSkips)
	}
}

// resetSkipGuard clears the consecutive-zero-card counter for setCode when a
// successful (non-empty) card response is received.
func (h *SyncHandler) resetSkipGuard(ctx context.Context, setCode string) {
	if h.maxConsecutiveSkips <= 0 {
		return
	}

	key := skipHashPrefix + setCode
	stored, err := h.store.GetHash(ctx, key)
	if err != nil || stored == "" || stored == "0" {
		return
	}

	if err := h.store.SetHash(ctx, key, "0"); err != nil {
		log.Printf("[sync] skip guard reset: SetHash %s: %v", setCode, err)
	}
}

// computeRatingsHash returns a deterministic SHA-256 hex string over the given
// card ratings slice. Cards are sorted by MtgaID ascending before marshalling so
// that ordering differences in upstream responses do not produce false cache misses.
func computeRatingsHash(ratings []seventeenlands.CardRating) (string, error) {
	sorted := make([]seventeenlands.CardRating, len(ratings))
	copy(sorted, ratings)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MtgaID < sorted[j].MtgaID
	})

	b, err := json.Marshal(sorted)
	if err != nil {
		return "", fmt.Errorf("marshal ratings for hash: %w", err)
	}

	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum), nil
}

// exponentialBackoff returns the backoff duration for a given attempt number (1-indexed).
// Durations: attempt 1 → 2s, 2 → 4s, 3 → 8s, capped at 30s.
func exponentialBackoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}
