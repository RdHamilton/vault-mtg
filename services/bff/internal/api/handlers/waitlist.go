package handlers

import (
	"context"
	"crypto/md5" //nolint:gosec // Mailchimp subscriber hash is MD5 by Mailchimp API spec — not a security choice.
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	posthog "github.com/posthog/posthog-go"
)

// RC4 note: hashAccountID (posthog.go) uses SHA-256 for PostHog PII hashing.
// Mailchimp's subscriber hash is MD5(lowercase(email)) per the Mailchimp API
// spec (https://mailchimp.com/developer/marketing/api/list-members/). These
// are different algorithms for different purposes — MD5 is required here by
// the external API contract, not as a security primitive.

const (
	// waitlistRateLimitWindow is the sliding window for per-IP rate limiting.
	// Mirrors the daemon_register per-account window (1 hour).
	waitlistRateLimitWindow = time.Hour

	// waitlistRateLimitMax is the maximum POST /api/v1/waitlist calls allowed
	// per IP per waitlistRateLimitWindow (RC5).
	waitlistRateLimitMax = 5
)

// waitlistRateEntry tracks request timestamps for one IP address.
// It is separate from rateEntry (daemon_register.go) so it can apply the
// waitlist-specific window and max without touching the daemon rate-limit path.
type waitlistRateEntry struct {
	mu        sync.Mutex
	callTimes []time.Time
}

// allow returns true if the request is within the rate limit, false otherwise.
// Uses waitlistRateLimitWindow and waitlistRateLimitMax.
func (e *waitlistRateEntry) allow() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-waitlistRateLimitWindow)

	filtered := e.callTimes[:0]
	for _, t := range e.callTimes {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	e.callTimes = filtered

	if len(e.callTimes) >= waitlistRateLimitMax {
		return false
	}
	e.callTimes = append(e.callTimes, now)
	return true
}

// waitlistRepo is the subset of WaitlistRepository used by WaitlistHandler.
type waitlistRepo interface {
	InsertIfNew(ctx context.Context, email string, utmSource, utmMedium, utmCampaign *string, referrer *string) (id string, created bool, err error)
	UpdateMailchimpStatus(ctx context.Context, id, status string) error
}

// MailchimpClient is a mockable interface for the Mailchimp Marketing API.
type MailchimpClient interface {
	AddMember(ctx context.Context, email string) error
}

// WaitlistHandler handles POST /api/v1/waitlist.
//
// Public endpoint (no Clerk auth required). Rate limited at 5 req/hour per IP.
//
// Idempotent: a duplicate email returns 200 OK; a new email returns 201 Created.
// Response body is {"ok": true} in both cases.
//
// Mailchimp signup is best-effort and non-fatal: a Mailchimp 5xx results in the
// DB row retaining mailchimp_status='failed' and the handler still returning
// 201. A future reconciler (separate ticket) picks up failed rows.
//
// PostHog: fires funnel_waitlist_signup_completed on the new-email path only.
// Goroutine-dispatched so PostHog latency does not block the HTTP response.
type WaitlistHandler struct {
	repo      waitlistRepo
	mailchimp MailchimpClient
	postHog   PostHogClient
	rateMu    sync.Mutex
	rateByIP  map[string]*waitlistRateEntry
}

// NewWaitlistHandler returns a handler backed by repo. mc may be nil in tests
// or when MAILCHIMP_API_KEY is not configured; Mailchimp signup is skipped.
func NewWaitlistHandler(repo waitlistRepo, mc MailchimpClient) *WaitlistHandler {
	return &WaitlistHandler{
		repo:      repo,
		mailchimp: mc,
		postHog:   noopPostHogClient{},
		rateByIP:  make(map[string]*waitlistRateEntry),
	}
}

// WithPostHogClient wires a PostHog client for analytics events.
func (h *WaitlistHandler) WithPostHogClient(ph PostHogClient) *WaitlistHandler {
	h.postHog = ph
	return h
}

// waitlistRequest is the JSON body for POST /api/v1/waitlist.
type waitlistRequest struct {
	Email       string `json:"email"`
	UTMSource   string `json:"utm_source"`
	UTMMedium   string `json:"utm_medium"`
	UTMCampaign string `json:"utm_campaign"`
	Referrer    string `json:"referrer"`
}

// waitlistResponse is the JSON body returned by POST /api/v1/waitlist.
type waitlistResponse struct {
	OK bool `json:"ok"`
}

// Join handles POST /api/v1/waitlist.
func (h *WaitlistHandler) Join(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate limit: 5 req/hour. Uses the same rateEntry type as daemon_register.
	ip := realIP(r)
	if !h.rateAllow(ip) {
		writeJSONError(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	var req waitlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[waitlist] decode body: %v", err)
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		writeJSONError(w, "email is required", http.StatusBadRequest)
		return
	}

	nullableStr := func(s string) *string {
		v := strings.TrimSpace(s)
		if v == "" {
			return nil
		}
		return &v
	}

	utmSource := nullableStr(req.UTMSource)
	utmMedium := nullableStr(req.UTMMedium)
	utmCampaign := nullableStr(req.UTMCampaign)
	referrer := nullableStr(req.Referrer)

	// Insert or no-op. ON CONFLICT DO NOTHING RETURNING id gives us the
	// idempotency signal: no row returned → email already existed.
	id, created, err := h.repo.InsertIfNew(r.Context(), email, utmSource, utmMedium, utmCampaign, referrer)
	if err != nil {
		log.Printf("[waitlist] InsertIfNew email=%s: %v", email, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated

		// Best-effort Mailchimp signup. Non-fatal: on any error the row keeps
		// mailchimp_status='failed' and a future reconciler will retry.
		if h.mailchimp != nil {
			go func(rowID, addr string) {
				mcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				mcErr := h.mailchimp.AddMember(mcCtx, addr)
				if mcErr != nil {
					log.Printf("[waitlist] mailchimp AddMember email=%s: %v (non-fatal; reconciler will retry)", addr, mcErr)
					return
				}

				if dbErr := h.repo.UpdateMailchimpStatus(mcCtx, rowID, "subscribed"); dbErr != nil {
					log.Printf("[waitlist] UpdateMailchimpStatus id=%s: %v", rowID, dbErr)
				}
			}(id, email)
		}

		// Fire PostHog funnel_waitlist_signup_completed on the new-email path only.
		// Goroutine-dispatched: PostHog latency must not block the HTTP response.
		// distinct_id: SHA-256 hash of email — reuses hashAccountID for PII safety.
		go func(addr string, src, medium, campaign, ref *string) {
			props := posthog.NewProperties().
				Set("utm_source", strOrEmpty(src)).
				Set("utm_medium", strOrEmpty(medium)).
				Set("utm_campaign", strOrEmpty(campaign)).
				Set("referrer", strOrEmpty(ref))

			if err := h.postHog.Enqueue(posthog.Capture{
				DistinctId: hashAccountID(addr),
				Event:      "funnel_waitlist_signup_completed",
				Properties: props,
			}); err != nil {
				log.Printf("[waitlist] posthog enqueue: %v", err)
			}
		}(email, utmSource, utmMedium, utmCampaign, referrer)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(waitlistResponse{OK: true}); err != nil {
		log.Printf("[waitlist] encode: %v", err)
	}
}

// strOrEmpty returns the dereferenced string or "" when p is nil.
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// rateAllow checks and records a rate-limit call for ip.
func (h *WaitlistHandler) rateAllow(ip string) bool {
	h.rateMu.Lock()
	entry, ok := h.rateByIP[ip]
	if !ok {
		entry = &waitlistRateEntry{}
		h.rateByIP[ip] = entry
	}
	h.rateMu.Unlock()
	return entry.allow()
}

// realIP extracts the client IP from X-Forwarded-For or RemoteAddr.
// Uses the first value in X-Forwarded-For when set (nginx-proxied path).
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr (host:port format).
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// mailchimpSubscriberHash returns the MD5 hash of the lower-cased email address
// as required by the Mailchimp Marketing API for subscriber lookups and adds.
// MD5 is mandated by Mailchimp's API spec — this is not a security primitive.
func mailchimpSubscriberHash(email string) string {
	h := md5.Sum([]byte(strings.ToLower(email))) //nolint:gosec
	return fmt.Sprintf("%x", h)
}
