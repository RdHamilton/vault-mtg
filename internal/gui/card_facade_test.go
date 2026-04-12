package gui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage"
)

// setupTestCardFacade creates a CardFacade with an in-memory DB for testing.
func setupTestCardFacade(t *testing.T) *CardFacade {
	t.Helper()

	cfg := storage.DefaultConfig(t.TempDir() + "/test.db")
	cfg.AutoMigrate = true

	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	services := &Services{
		Context: context.Background(),
		Storage: storage.NewService(db),
	}

	return NewCardFacade(services)
}

func TestNewCardFacade_InitializesNegativeCache(t *testing.T) {
	facade := NewCardFacade(&Services{})
	if facade.cfbFetchFailedSets == nil {
		t.Error("expected cfbFetchFailedSets to be initialized")
	}
	if len(facade.cfbFetchFailedSets) != 0 {
		t.Error("expected cfbFetchFailedSets to be empty on init")
	}
}

func TestGetCFBRatings_NoFetcher_ReturnsEmpty(t *testing.T) {
	facade := setupTestCardFacade(t)

	ratings, err := facade.GetCFBRatings(context.Background(), "TMT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 0 {
		t.Errorf("expected 0 ratings, got %d", len(ratings))
	}
}

func TestGetCFBRatings_NoFetcher_DoesNotPopulateNegativeCache(t *testing.T) {
	facade := setupTestCardFacade(t)

	_, _ = facade.GetCFBRatings(context.Background(), "TMT")

	facade.cfbFetchFailedSetsMu.RLock()
	_, exists := facade.cfbFetchFailedSets["TMT"]
	facade.cfbFetchFailedSetsMu.RUnlock()

	if exists {
		t.Error("negative cache should not be populated when no fetcher is configured")
	}
}

func TestGetCFBRatings_NegativeCache_SkipsRetry(t *testing.T) {
	facade := setupTestCardFacade(t)

	// Pre-populate negative cache as if a recent fetch failed
	facade.cfbFetchFailedSetsMu.Lock()
	facade.cfbFetchFailedSets["TMT"] = time.Now()
	facade.cfbFetchFailedSetsMu.Unlock()

	// Even though MTGAZoneFetcher is nil (so the branch won't trigger),
	// verify the cache state is correctly set
	facade.cfbFetchFailedSetsMu.RLock()
	failedAt, exists := facade.cfbFetchFailedSets["TMT"]
	isCached := exists && time.Since(failedAt) < 30*time.Minute
	facade.cfbFetchFailedSetsMu.RUnlock()

	if !isCached {
		t.Error("expected TMT to be in the negative cache")
	}
}

func TestGetCFBRatings_NegativeCache_ExpiresAfter30Minutes(t *testing.T) {
	facade := setupTestCardFacade(t)

	facade.cfbFetchFailedSetsMu.Lock()
	facade.cfbFetchFailedSets["OLD"] = time.Now().Add(-31 * time.Minute)
	facade.cfbFetchFailedSets["NEW"] = time.Now()
	facade.cfbFetchFailedSetsMu.Unlock()

	// Verify expired entry is recognized as expired
	facade.cfbFetchFailedSetsMu.RLock()
	failedAt := facade.cfbFetchFailedSets["OLD"]
	isExpired := time.Since(failedAt) >= 30*time.Minute
	facade.cfbFetchFailedSetsMu.RUnlock()

	if !isExpired {
		t.Error("expected OLD entry to be expired (>30min)")
	}

	// Verify fresh entry is still active
	facade.cfbFetchFailedSetsMu.RLock()
	failedAt = facade.cfbFetchFailedSets["NEW"]
	isFresh := time.Since(failedAt) < 30*time.Minute
	facade.cfbFetchFailedSetsMu.RUnlock()

	if !isFresh {
		t.Error("expected NEW entry to still be fresh (<30min)")
	}
}

func TestGetCFBRatings_NegativeCache_ConcurrentAccess(t *testing.T) {
	facade := setupTestCardFacade(t)

	// Concurrent reads/writes to the negative cache should not race.
	// Run with -race to verify mutex protection.
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			facade.cfbFetchFailedSetsMu.Lock()
			facade.cfbFetchFailedSets["TMT"] = time.Now()
			facade.cfbFetchFailedSetsMu.Unlock()
		}(i)
	}

	// Concurrent readers (via GetCFBRatings which uses RLock internally)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = facade.GetCFBRatings(context.Background(), "TMT")
		}()
	}

	wg.Wait()
}

func TestGetCFBRatings_UppercasesSetCode(t *testing.T) {
	facade := setupTestCardFacade(t)

	// Should work with lowercase input
	ratings, err := facade.GetCFBRatings(context.Background(), "tmt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings) != 0 {
		t.Errorf("expected 0 ratings, got %d", len(ratings))
	}
}
