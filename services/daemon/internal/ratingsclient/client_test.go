package ratingsclient_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/ratingsclient"
)

// ─── helpers ──────────────────────────────────────────────────────────────

// envelopeJSON wraps body in the BFF's {"data": ...} envelope. All BFF
// responses use this shape; the client unwraps once before parsing.
func envelopeJSON(body string) string {
	return `{"data":` + body + `}`
}

// sampleBody returns a draft-ratings response with one rated and one
// unrated card. Lets tests assert both HasGIHWR true and false paths.
func sampleBody(set, format string) string {
	return fmt.Sprintf(`{
		"set_code": %q,
		"draft_format": %q,
		"card_ratings": [
			{"arena_id": 100, "name": "Lightning Bolt", "gihwr": 58.4},
			{"arena_id": 200, "name": "Mountain"}
		],
		"color_ratings": []
	}`, set, format)
}

// fakeBFF is a one-stop HTTP server that records every request,
// optionally adds an artificial delay, and returns whatever the
// per-test handler returns. The Calls counter doubles as the
// singleflight + retry assertion mechanism.
type fakeBFF struct {
	*httptest.Server
	calls   atomic.Int64
	delay   time.Duration
	handler http.HandlerFunc
}

func newFakeBFF(handler http.HandlerFunc) *fakeBFF {
	f := &fakeBFF{handler: handler}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if f.delay > 0 {
			time.Sleep(f.delay)
		}
		f.handler(w, r)
	}))
	return f
}

// newClient builds a client wired to the fakeBFF with a deterministic
// clock the caller can advance. The clock starts at a fixed UTC moment
// so tests can compute TTL boundaries explicitly.
func newClient(bff *fakeBFF, ttl time.Duration) (*ratingsclient.Client, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)}
	c := ratingsclient.New(ratingsclient.Config{
		BFFURL: bff.URL,
		Token:  "test-bearer",
		TTL:    ttl,
		Clock:  clock.Now,
	})
	return c, clock
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

// ─── happy path / cache hit ───────────────────────────────────────────────

func TestGIHWR_FetchesAndCaches(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-bearer" {
			t.Errorf("Authorization = %q, want Bearer test-bearer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)
	if err := c.Warm(context.Background(), "BLB", "PremierDraft"); err != nil {
		t.Fatal(err)
	}

	v, ok := c.GIHWR("100", "PremierDraft")
	if !ok || v != 58.4 {
		t.Errorf("GIHWR(100) = (%v, %v), want (58.4, true)", v, ok)
	}
	if name := c.CardName("100"); name != "Lightning Bolt" {
		t.Errorf("CardName(100) = %q", name)
	}

	// Second call should not hit BFF.
	c.GIHWR("100", "PremierDraft")
	c.CardName("200")
	if got := bff.calls.Load(); got != 1 {
		t.Errorf("BFF calls = %d, want 1 (cache should serve subsequent)", got)
	}

	s := c.Stats()
	if s.Hit < 1 || s.Fetch != 1 {
		t.Errorf("Stats = %+v, want at least one Hit and exactly one Fetch", s)
	}
}

func TestGIHWR_HasGIHWRFalseWhenRatingAbsent(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)
	_ = c.Warm(context.Background(), "BLB", "PremierDraft")

	v, ok := c.GIHWR("200", "PremierDraft") // arena 200 has no gihwr
	if ok || v != 0 {
		t.Errorf("GIHWR(200) = (%v, %v), want (0, false)", v, ok)
	}
}

// ─── TTL ──────────────────────────────────────────────────────────────────

func TestCache_TTLExpiryTriggersRefetch(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, clock := newClient(bff, 1*time.Hour)
	_ = c.Warm(context.Background(), "BLB", "PremierDraft")
	if bff.calls.Load() != 1 {
		t.Fatalf("calls after first warm = %d, want 1", bff.calls.Load())
	}

	// Advance past TTL — next call should refetch.
	clock.Advance(2 * time.Hour)
	_ = c.Warm(context.Background(), "BLB", "PremierDraft")
	if got := bff.calls.Load(); got != 2 {
		t.Errorf("calls after TTL expiry = %d, want 2", got)
	}
}

// ─── 404 caches empty ─────────────────────────────────────────────────────

func TestFetch_404CachesEmptyEntry(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)
	if err := c.Warm(context.Background(), "BLB", "PremierDraft"); err != nil {
		t.Fatalf("404 should not propagate as error, got %v", err)
	}

	// Second call: cache hit (empty), no new BFF request.
	_, _ = c.GIHWR("100", "PremierDraft")
	if got := bff.calls.Load(); got != 1 {
		t.Errorf("BFF calls = %d, want 1 (404 should cache empty, not hammer)", got)
	}
}

// ─── 5xx retry ────────────────────────────────────────────────────────────

func TestFetch_5xxRetryThenSuccess(t *testing.T) {
	attempts := 0
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	// Tighten the retry windows so the test doesn't actually wait 250ms.
	// We accept the default backoff here — the retry loop is fast enough
	// for one retry to land in <500ms.
	c, _ := newClient(bff, 24*time.Hour)

	v, ok := c.GIHWR("100", "PremierDraft")
	// Initial fetch loads cache; GIHWR("100") without prior Warm uses
	// MRU which is empty before the fetch — but the lookup falls
	// through to fetchFor via Warm-equivalent path. We test via Warm
	// instead so the GIHWR resolves deterministically.
	_ = v
	_ = ok

	if err := c.Warm(context.Background(), "BLB", "PremierDraft"); err != nil {
		t.Fatalf("retry should have succeeded, got %v", err)
	}
	v, ok = c.GIHWR("100", "PremierDraft")
	if !ok || v != 58.4 {
		t.Errorf("GIHWR(100) after retry = (%v, %v), want (58.4, true)", v, ok)
	}
}

func TestFetch_5xxExhaustsRetries(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)

	// Direct Warm call exposes the error for assertion.
	err := c.Warm(context.Background(), "BLB", "PremierDraft")
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	// GIHWR call should degrade silently.
	v, ok := c.GIHWR("100", "PremierDraft")
	if ok || v != 0 {
		t.Errorf("GIHWR after fetch failure = (%v, %v), want (0, false)", v, ok)
	}

	s := c.Stats()
	if s.FetchError == 0 {
		t.Errorf("FetchError counter not bumped: %+v", s)
	}
}

// ─── singleflight ─────────────────────────────────────────────────────────

func TestFetch_SingleflightCoalescesConcurrentCalls(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	bff.delay = 50 * time.Millisecond // hold the request open long enough to coalesce
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)

	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_ = c.Warm(context.Background(), "BLB", "PremierDraft")
		}()
	}
	wg.Wait()

	if got := bff.calls.Load(); got != 1 {
		t.Errorf("BFF calls = %d, want 1 (singleflight should coalesce %d concurrent calls)", got, N)
	}
}

// ─── token rotation ───────────────────────────────────────────────────────

func TestSetToken_RotatesBearerForSubsequentRequests(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	bff := newFakeBFF(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Authorization"))
		mu.Unlock()
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, clock := newClient(bff, 1*time.Hour)
	_ = c.Warm(context.Background(), "BLB", "PremierDraft")

	c.SetToken("rotated-bearer")
	clock.Advance(2 * time.Hour) // force refetch
	_ = c.Warm(context.Background(), "BLB", "PremierDraft")

	if len(seen) != 2 {
		t.Fatalf("seen %d requests, want 2", len(seen))
	}
	if seen[0] != "Bearer test-bearer" {
		t.Errorf("first request Authorization = %q", seen[0])
	}
	if seen[1] != "Bearer rotated-bearer" {
		t.Errorf("second request Authorization = %q (token rotation not applied)", seen[1])
	}
}

// ─── context cancellation ────────────────────────────────────────────────

func TestFetch_ContextCancellationStopsInflight(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := c.Warm(ctx, "BLB", "PremierDraft")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if ctx.Err() == nil {
		t.Error("context should be cancelled")
	}
}

// ─── degraded header ──────────────────────────────────────────────────────

func TestFetch_DegradedHeaderTaggedAndCounted(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Cache-Degraded", "true")
		w.Header().Set("X-Cache-Age-Hours", "32")
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)
	if err := c.Warm(context.Background(), "BLB", "PremierDraft"); err != nil {
		t.Fatal(err)
	}

	// Data still served — stale 17Lands is better than nothing.
	v, ok := c.GIHWR("100", "PremierDraft")
	if !ok || v != 58.4 {
		t.Errorf("degraded fetch should still return data: (%v, %v)", v, ok)
	}
	s := c.Stats()
	if s.Degraded != 1 {
		t.Errorf("Degraded counter = %d, want 1", s.Degraded)
	}
}

// ─── stats counters ───────────────────────────────────────────────────────

func TestStats_CountersTrackHitMissFetch(t *testing.T) {
	bff := newFakeBFF(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("BLB", "PremierDraft"))))
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)
	_ = c.Warm(context.Background(), "BLB", "PremierDraft")

	c.GIHWR("100", "PremierDraft") // hit
	c.GIHWR("200", "PremierDraft") // miss (no gihwr)
	c.GIHWR("999", "PremierDraft") // miss (unknown card)

	s := c.Stats()
	if s.Hit != 1 {
		t.Errorf("Hit = %d, want 1", s.Hit)
	}
	if s.Miss != 2 {
		t.Errorf("Miss = %d, want 2", s.Miss)
	}
	if s.Fetch != 1 {
		t.Errorf("Fetch = %d, want 1", s.Fetch)
	}
	if s.FetchError != 0 {
		t.Errorf("FetchError = %d, want 0", s.FetchError)
	}
}

// ─── URL escaping ─────────────────────────────────────────────────────────

func TestFetch_EscapesPathSegments(t *testing.T) {
	var escaped string
	bff := newFakeBFF(func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath reports the wire path with reserved characters
		// still percent-encoded; r.URL.Path decodes them. We want the
		// wire form so we can verify the client escaped at all.
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(envelopeJSON(sampleBody("WEIRD/SET", "Premier Draft"))))
	})
	defer bff.Close()

	c, _ := newClient(bff, 24*time.Hour)
	_ = c.Warm(context.Background(), "WEIRD/SET", "Premier Draft")

	if escaped != "/api/v1/draft-ratings/WEIRD%2FSET/Premier%20Draft" {
		t.Errorf("escaped path = %q, want %q", escaped, "/api/v1/draft-ratings/WEIRD%2FSET/Premier%20Draft")
	}
}
