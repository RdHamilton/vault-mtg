package ratingsclient

// Stats is a snapshot of operational counters. Each field counts a
// distinct lifecycle event so ops can see the daemon's hit rate +
// fetch error rate at a glance.
//
// Values are absolute counts since process start. Consumers compute
// rates by sampling Stats() at two points in time.
type Stats struct {
	// Hit — GIHWR returned a real rating (cache hit + card present
	// + HasGIHWR true).
	Hit uint64
	// Miss — GIHWR returned (0, false). Includes "card not in pack
	// ratings", "no rating on file for this card", and "no cache entry
	// yet because the fetch failed."
	Miss uint64
	// Fetch — number of outbound HTTP requests to the BFF. One per
	// (set, format) per TTL window when the cache works correctly.
	Fetch uint64
	// FetchError — fetches that exhausted retries. Each such event
	// also bumps Miss for the GIHWR call that triggered it.
	FetchError uint64
	// Degraded — fetches that succeeded with the BFF's
	// X-Cache-Degraded: true header. The BFF's own 17Lands cache was
	// stale; we served the data anyway.
	Degraded uint64
}

// statKey is an internal enum naming each counter so statsAdd can
// dispatch without a string→pointer map.
type statKey int

const (
	statHit statKey = iota
	statMiss
	statFetch
	statFetchError
)

// statsAdd bumps the named counter by delta. Always called under or
// without the cache lock — uses the mutex itself to serialise writes.
// Degraded gets bumped inline by storeEntry under the existing lock.
func (c *Client) statsAdd(k statKey, delta uint64) {
	c.mu.Lock()
	switch k {
	case statHit:
		c.stats.Hit += delta
	case statMiss:
		c.stats.Miss += delta
	case statFetch:
		c.stats.Fetch += delta
	case statFetchError:
		c.stats.FetchError += delta
	}
	c.mu.Unlock()
}
