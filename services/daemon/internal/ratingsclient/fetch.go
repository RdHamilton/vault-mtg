package ratingsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxRetries — total request budget is maxRetries+1 (one initial + N
// retries). 3 retries gives ~5.25s worst-case (250ms + 1s + 4s)
// without dragging draft live state too far behind the user's pick.
const maxRetries = 3

// initialBackoff is the first sleep before retry; doubles on each
// subsequent attempt (250ms → 1s → 4s) capped at maxBackoff.
const (
	initialBackoff = 250 * time.Millisecond
	maxBackoff     = 4 * time.Second
)

// envelope mirrors the BFF's standard JSON wrapping (every BFF response
// is wrapped in {"data": ...}). We unmarshal once into envelope, then
// once again into draftRatingsBody.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// draftRatingsBody mirrors the relevant subset of the
// /api/v1/draft-ratings/{set}/{format} response. We ignore color
// ratings and per-card metrics we don't need (OHWR, ALSA, etc.) so
// the wire contract can grow without forcing a daemon redeploy.
type draftRatingsBody struct {
	SetCode     string           `json:"set_code"`
	DraftFormat string           `json:"draft_format"`
	CardRatings []cardRatingWire `json:"card_ratings"`
}

type cardRatingWire struct {
	ArenaID int      `json:"arena_id"`
	Name    string   `json:"name"`
	GIHWR   *float64 `json:"gihwr,omitempty"`
}

// fetchFor returns the cached entry for (set, format), fetching it
// when the cache is cold or the entry has expired. Concurrent callers
// for the same (set, format) coalesce via singleflight — the BFF gets
// exactly one request per key per cold-cache window.
//
// On total failure (BFF unreachable, all retries exhausted) returns
// (nil, error) so the immediate caller can choose to log; consumers
// up-stack (GIHWR, CardName) translate that into "no rating" without
// propagating the error to the SPA.
func (c *Client) fetchFor(ctx context.Context, set, format string) (*entry, error) {
	if set == "" || format == "" {
		return nil, fmt.Errorf("ratingsclient: set and format are required")
	}

	if ent, ok := c.getEntry(set, format); ok {
		// Cache hit — still bump MRU so CardName resolves against
		// this entry for subsequent lookups in the same session.
		c.touchMRU(set, format)
		return ent, nil
	}

	// singleflight key matches cacheKey so concurrent cold-cache calls
	// share the result. Once Do returns, the result is in the cache
	// for the duration of c.ttl.
	key := cacheKey(set, format)
	v, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Re-check the cache inside singleflight in case another
		// caller filled it while we were queued.
		if ent, ok := c.getEntry(set, format); ok {
			return ent, nil
		}
		c.statsAdd(statFetch, 1)
		ent, err := c.fetchWithRetry(ctx, set, format)
		if err != nil {
			c.statsAdd(statFetchError, 1)
			return nil, err
		}
		c.storeEntry(set, format, ent)
		return ent, nil
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("ratingsclient: nil entry returned from fetch")
	}
	return v.(*entry), nil
}

// touchMRU updates the most-recently-used cache key so subsequent
// CardName lookups resolve against this (set, format) pair.
func (c *Client) touchMRU(set, format string) {
	key := cacheKey(set, format)
	c.mu.Lock()
	c.mruKey = key
	c.mu.Unlock()
}

// fetchWithRetry performs the HTTP fetch with exponential backoff on
// transient failures (5xx + network errors). 4xx responses are final
// — 404 caches an empty entry so a set without 17Lands data yet
// doesn't get hammered on every pick.
func (c *Client) fetchWithRetry(ctx context.Context, set, format string) (*entry, error) {
	endpoint, err := c.endpointURL(set, format)
	if err != nil {
		return nil, err
	}

	backoff := initialBackoff
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		ent, retryable, err := c.fetchOnce(ctx, endpoint, set, format)
		if err == nil {
			return ent, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
		if attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 4
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return nil, fmt.Errorf("ratingsclient: %s exhausted retries: %w", endpoint, lastErr)
}

// fetchOnce performs a single HTTP fetch. The retryable bool tells the
// outer loop whether to back off and try again (true for network +
// 5xx) or surface the error immediately (false for 4xx that isn't 404
// + 401/403 which auth rotation won't fix mid-session).
//
// A 404 result is treated as success with an empty entry — that's the
// "no ratings on file for this set yet" case the BFF returns for new
// releases before the scrape pipeline has caught up.
func (c *Client) fetchOnce(ctx context.Context, endpoint, set, format string) (*entry, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		// Empty entry — set has no 17Lands data yet, not an error.
		return &entry{
			set:       set,
			format:    format,
			cards:     map[string]cardRec{},
			fetchedAt: c.clock(),
		}, false, nil
	case resp.StatusCode >= 500:
		// Drain body so connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, true, fmt.Errorf("BFF returned %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, false, fmt.Errorf("BFF returned %d (non-retryable)", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false, fmt.Errorf("decode envelope: %w", err)
	}
	var body draftRatingsBody
	if err := json.Unmarshal(env.Data, &body); err != nil {
		return nil, false, fmt.Errorf("decode draft-ratings body: %w", err)
	}

	ent := &entry{
		set:       set,
		format:    format,
		cards:     make(map[string]cardRec, len(body.CardRatings)),
		fetchedAt: c.clock(),
	}
	for _, r := range body.CardRatings {
		rec := cardRec{Name: r.Name}
		if r.GIHWR != nil {
			rec.GIHWR = *r.GIHWR
			rec.HasGIHWR = true
		}
		ent.cards[strconv.Itoa(r.ArenaID)] = rec
	}

	// X-Cache-Degraded from the BFF means the BFF's own 17Lands cache
	// is stale. Log and tag the entry so Stats() reflects it; still
	// serve the data.
	if strings.EqualFold(resp.Header.Get("X-Cache-Degraded"), "true") {
		ent.degraded = true
		if v, err := strconv.Atoi(resp.Header.Get("X-Cache-Age-Hours")); err == nil {
			ent.ageHours = v
		}
		log.Printf("[ratingsclient] BFF degraded for set=%s format=%s age=%dh — serving anyway", set, format, ent.ageHours)
	}

	return ent, false, nil
}

// endpointURL builds the BFF URL for (set, format), URL-escaping both
// segments defensively. Returns the full URL including scheme + host.
func (c *Client) endpointURL(set, format string) (string, error) {
	if c.bffURL == "" {
		return "", fmt.Errorf("ratingsclient: BFFURL not configured")
	}
	base := strings.TrimRight(c.bffURL, "/")
	return fmt.Sprintf(
		"%s/api/v1/draft-ratings/%s/%s",
		base,
		url.PathEscape(set),
		url.PathEscape(format),
	), nil
}
