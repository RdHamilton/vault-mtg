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
	"strings"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

// defaultFormats is the canonical list of 17Lands draft formats synced per set.
// Sealed is omitted by default — it has far fewer games logged and the data is
// lower confidence. Set SYNC_FORMATS to override (e.g. "PremierDraft,QuickDraft,Sealed").
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
	formats      []string // draft formats to sync; read from SYNC_FORMATS env var
}

// New creates a SyncHandler. overrideSets may be nil/empty to use DB-driven active sets.
//
// The formats list is read from SYNC_FORMATS (comma-separated). If unset, defaultFormats
// is used: PremierDraft and QuickDraft.
func New(fetcher Fetcher, store datasets.Store, overrideSets []string) *SyncHandler {
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

	return &SyncHandler{
		fetcher:      fetcher,
		store:        store,
		overrideSets: overrideSets,
		formats:      formats,
	}
}

// NewWithFormats creates a SyncHandler with an explicit formats list, bypassing the
// SYNC_FORMATS env var. Intended for tests that need deterministic format control.
func NewWithFormats(fetcher Fetcher, store datasets.Store, overrideSets, formats []string) *SyncHandler {
	return &SyncHandler{
		fetcher:      fetcher,
		store:        store,
		overrideSets: overrideSets,
		formats:      formats,
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

	log.Printf("[sync] fetching ratings for %d set(s) x %d format(s): sets=%v formats=%v",
		len(sets), len(h.formats), sets, h.formats)

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

			// ADR-005: compute a SHA-256 hash over the sorted payload and skip the
			// upsert when the hash matches the previously stored value in sync_hashes.
			hashKey := setCode + "/" + format
			newHash, err := computeRatingsHash(ratings)
			if err != nil {
				log.Printf("[sync] hash compute %s/%s: %v", setCode, format, err)
				continue
			}

			storedHash, err := h.store.GetHash(ctx, hashKey)
			if err != nil {
				log.Printf("[sync] get hash %s/%s: %v — proceeding with upsert", setCode, format, err)
				// Non-fatal: fall through and upsert anyway.
				storedHash = ""
			}

			if storedHash != "" && storedHash == newHash {
				log.Printf("[sync] skipped %s/%s: payload unchanged (hash=%s)", setCode, format, newHash[:8])
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

			if err := h.store.SetHash(ctx, hashKey, newHash); err != nil {
				// Non-fatal: the upsert succeeded; log and continue so the next run
				// simply re-upserts rather than silently losing data.
				log.Printf("[sync] set hash %s/%s: %v", setCode, format, err)
			}

			log.Printf("[sync] refreshed %s/%s: %d cards (hash=%s)", setCode, format, len(ratings), newHash[:8])

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
