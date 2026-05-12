// Package ratingsclient is the daemon's read-through cache for the BFF's
// /api/v1/draft-ratings/{set}/{format} endpoint. It satisfies both
// pkg/draftalgo.CardLookup and pkg/draftalgo.RatingsLookup so the
// localapi draft handlers (current-pack, grade-pick, win-probability)
// can grade picks against real 17Lands data instead of the no-op stubs
// shipped in PR #17b.
//
// Scope (PR #20):
//
//   - Per-(set, format) cache with a 24h TTL — matches the BFF's own
//     staleness threshold.
//   - singleflight dedup so a pack of 14 concurrent grade-pick calls
//     against a cold cache fires exactly one HTTP request.
//   - Bounded retry on transient errors (network failures + 5xx) with
//     exponential backoff. 4xx is final; 404 caches an empty entry so
//     a set without 17Lands data yet doesn't get hammered.
//   - Context-cancellation everywhere — daemon shutdown stops in-flight
//     fetches cleanly.
//   - SetToken hook so the daemon's JWT rotation pipeline (see
//     services/daemon/internal/daemon/service.go's jwtTicker loop) can
//     swap the bearer without rebuilding the client.
//   - Stats() counters for hit / miss / fetch / fetch-error /
//     degraded — surfaces via the daemon's /api/v1/system/health when
//     wired in.
//   - X-Cache-Degraded header from the BFF logs a warning + bumps a
//     counter; data still served (better than nothing).
//   - URL-escapes every path segment — defensive for future format
//     names with reserved characters.
//   - Never serves data past TTL — once expired, refetch or return
//     "no data" so the SPA gets honest "N/A" grades, never stale
//     misleadingly-confident ones.
package ratingsclient

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// DefaultTTL is how long a fetched (set, format) entry is treated as
// fresh. Matches the BFF's DraftRatingsStalenessThresholdHours default
// so the daemon's cache rotates in step with the BFF's own.
const DefaultTTL = 24 * time.Hour

// DefaultTimeout caps a single HTTP request to the BFF. The retry loop
// in fetch.go may make up to maxRetries+1 attempts in total.
const DefaultTimeout = 10 * time.Second

// Client is the daemon-side ratings + card-name lookup. Safe for
// concurrent use by every localapi handler goroutine.
type Client struct {
	bffURL string
	ttl    time.Duration
	http   *http.Client
	sf     singleflight.Group

	mu     sync.RWMutex
	token  string            // bearer token; SetToken rotates it
	cache  map[string]*entry // key = set+"|"+format (lowercased)
	mruKey string            // most-recent (set,format) for CardName fallback
	stats  Stats             // per-counter values; copied via Stats() snapshot
	clock  func() time.Time  // overridable in tests
}

// Config holds the dependencies callers inject into the client.
type Config struct {
	// BFFURL is the cloud API base (e.g. https://api.vaultmtg.app).
	// Path "/api/v1/draft-ratings/..." is appended internally.
	BFFURL string
	// Token is the initial bearer token. May be empty; SetToken
	// rotates it later.
	Token string
	// TTL overrides DefaultTTL. Zero uses DefaultTTL.
	TTL time.Duration
	// HTTPClient overrides the default *http.Client. Useful for tests
	// that want to inject a custom transport.
	HTTPClient *http.Client
	// Clock overrides time.Now. Tests use this to step over TTL
	// boundaries without sleeping.
	Clock func() time.Time
}

// New constructs a Client from cfg. The cache is empty; the first
// GIHWR or CardName call against an unseen (set, format) triggers an
// HTTP fetch.
func New(cfg Config) *Client {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Client{
		bffURL: cfg.BFFURL,
		ttl:    ttl,
		http:   httpClient,
		token:  cfg.Token,
		cache:  map[string]*entry{},
		clock:  clock,
	}
}

// SetToken rotates the bearer token. Safe to call concurrently with
// active GIHWR / CardName lookups; the next outbound HTTP request will
// pick up the new value.
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
}

// GIHWR returns the 17Lands "games in hand win rate" for the card in
// the given format. Format and set are derived from the draft session
// the caller already knows (via draftstate). Returns (0, false) when
// no rating is on file or the BFF is unreachable — handlers degrade to
// "N/A" grade in that case.
//
// Implements draftalgo.RatingsLookup.
func (c *Client) GIHWR(id, format string) (float64, bool) {
	// MTGA log CourseName splits to (Format, SetCode) — but
	// pickquality + prediction call GIHWR with the format string only.
	// We pull the set out of the format-less argument by deferring to
	// the most-recently-touched (set, format) the client has fetched.
	// In practice handlers populate ratings via the per-session path
	// in fetchFor(set, format), which seeds the MRU before the first
	// GIHWR call against that session.
	set, fullFormat := splitMRUKey(c.mruByFormat(format))
	if set == "" {
		// Format-only lookup with no MRU match → can't resolve.
		c.statsAdd(statMiss, 1)
		return 0, false
	}
	return c.lookupGIHWR(set, fullFormat, id)
}

// CardName returns the card's printed name from the most-recently-
// touched (set, format) cache entry. Returns "" when the daemon has
// no fetched entry yet or the card isn't in it — handlers fall back
// to "Unknown Card" in that case.
//
// Implements draftalgo.CardLookup.
func (c *Client) CardName(id string) string {
	c.mu.RLock()
	key := c.mruKey
	c.mu.RUnlock()
	if key == "" {
		return ""
	}
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if !ok || entry.isExpired(c.clock()) {
		return ""
	}
	if rec, ok := entry.cards[id]; ok {
		return rec.Name
	}
	return ""
}

// Warm proactively fetches (set, format) so the next GIHWR / CardName
// call doesn't pay the 200ms cold-start cost. Optional — lazy fetch
// still works without it. Returns the fetch error (or nil) so callers
// can log a warning if BFF was unreachable; lookups will still degrade
// gracefully without it.
func (c *Client) Warm(ctx context.Context, set, format string) error {
	_, err := c.fetchFor(ctx, set, format)
	return err
}

// Stats returns a snapshot of operational counters. Safe to call
// concurrently; values are read under the cache lock.
func (c *Client) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// ─── internals ─────────────────────────────────────────────────────────────

// lookupGIHWR drives a GIHWR read against (set, format, id), fetching
// on cache miss. Returns (0, false) on any failure — never propagates
// errors to the algorithms.
func (c *Client) lookupGIHWR(set, format, id string) (float64, bool) {
	ent, err := c.fetchFor(context.Background(), set, format)
	if err != nil || ent == nil {
		return 0, false
	}
	rec, ok := ent.cards[id]
	if !ok {
		c.statsAdd(statMiss, 1)
		return 0, false
	}
	if !rec.HasGIHWR {
		c.statsAdd(statMiss, 1)
		return 0, false
	}
	c.statsAdd(statHit, 1)
	return rec.GIHWR, true
}

// mruByFormat returns the cache key for a (set, format) pair where
// `format` matches the supplied value. When format is empty or no
// matching entry is found, returns the most-recently-touched key
// regardless of format — a forgiving heuristic for the live-draft
// path where the SPA reliably sticks to one format at a time.
func (c *Client) mruByFormat(format string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if format == "" {
		return c.mruKey
	}
	// Walk all entries; pick the one matching format, falling back to
	// the MRU key when none match.
	for key, ent := range c.cache {
		if ent.format == format {
			return key
		}
	}
	return c.mruKey
}
