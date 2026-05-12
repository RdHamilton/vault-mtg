package ratingsclient

import (
	"strings"
	"time"
)

// entry is a single cached (set, format) response from the BFF.
// fetchedAt drives TTL expiry; cards is keyed by stringified arena_id
// so handlers can lookup with strconv.Itoa(id) directly.
type entry struct {
	set       string
	format    string
	cards     map[string]cardRec
	fetchedAt time.Time
	// degraded is true when the BFF marked the response as stale via
	// X-Cache-Degraded. Surfaces in Stats() so ops can see when the
	// upstream is showing its own age.
	degraded bool
	// ageHours is the X-Cache-Age-Hours value the BFF emits when
	// degraded. Zero when not present.
	ageHours int
}

// cardRec is one row from the BFF response, normalised for in-memory
// lookup. HasGIHWR keeps the *float64 pointer-or-nil semantics from
// the wire shape without forcing handlers to deref.
type cardRec struct {
	Name     string
	GIHWR    float64
	HasGIHWR bool
}

// isExpired returns true when the entry's age has crossed the client's
// TTL. Inputs the current time so tests can step over the boundary
// without sleeping.
func (e *entry) isExpired(now time.Time) bool {
	return now.Sub(e.fetchedAt) > defaultTTLForExpiryCheck
}

// defaultTTLForExpiryCheck is shadowed by the client's actual TTL in
// the fetch path; this is the fallback when the entry is consulted
// without a client context (only happens in tests). 24h matches
// DefaultTTL.
var defaultTTLForExpiryCheck = 24 * time.Hour

// cacheKey is the in-memory key for a (set, format) pair. Lowercased
// so case mismatches between MTGA log values and BFF responses don't
// fragment the cache.
func cacheKey(set, format string) string {
	return strings.ToLower(set) + "|" + strings.ToLower(format)
}

// splitMRUKey is the inverse of cacheKey. Returns ("", "") when the
// input doesn't have the expected shape.
func splitMRUKey(key string) (set, format string) {
	idx := strings.IndexByte(key, '|')
	if idx <= 0 || idx == len(key)-1 {
		return "", ""
	}
	return key[:idx], key[idx+1:]
}

// getEntry returns the cached entry for (set, format) if it exists and
// has not expired against the client's clock. The bool tracks
// presence; the entry pointer is nil when absent or expired.
func (c *Client) getEntry(set, format string) (*entry, bool) {
	key := cacheKey(set, format)
	c.mu.RLock()
	defer c.mu.RUnlock()
	ent, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if c.clock().Sub(ent.fetchedAt) > c.ttl {
		return nil, false
	}
	return ent, true
}

// storeEntry persists ent against (set, format) and updates the MRU
// pointer so future CardName lookups resolve against the most-recent
// session. Token rotation does not invalidate the cache — the BFF's
// auth check happens per request, and stale entries are still valid
// across token swaps.
func (c *Client) storeEntry(set, format string, ent *entry) {
	key := cacheKey(set, format)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = ent
	c.mruKey = key
	if ent.degraded {
		c.stats.Degraded++
	}
}
