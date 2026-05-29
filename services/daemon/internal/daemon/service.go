// Package daemon provides the standalone daemon service.
// The daemon reads MTGA Player.log, classifies events, and POSTs them
// to the BFF via contract.DaemonEvent. It never connects to a database.
package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/draftstate"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/gre"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/keychain"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/localapi"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/pkce"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/ratingsclient"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/registrar"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/updatecheck"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
)

// jwtRefreshInterval is how often the run loop checks whether the JWT needs
// refreshing during an active session. It is a variable so tests can shorten it.
var jwtRefreshInterval = time.Hour

// updateCheckInterval is how often the run loop checks for a newer daemon version.
// It is a variable so tests can shorten it.
var updateCheckInterval = 24 * time.Hour

// heartbeatInterval is how often the run loop sends a daemon.heartbeat event to
// the BFF so the health check has a liveness signal even when MTGA is idle.
// It is a variable so tests can shorten it.
var heartbeatInterval = 30 * time.Second

// helperCheckInterval is how often the run loop probes the collection helper
// socket to keep the tray state in sync (e.g. if the user installs or stops
// the helper outside of the Grant Access flow).
var helperCheckInterval = 30 * time.Second

// Service is the top-level daemon service.
type Service struct {
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	poller     *logreader.Poller
	sessionID  string
	regClient  *registrar.Client
	version    string // build-time version; "dev" skips update checks
	greManager *gre.Manager
	// draftState caches the live draft session(s) the daemon has seen
	// so localapi handlers can answer /drafts/{id}/current-pack and
	// friends without re-reading the log. Populated as draft.pack /
	// draft.pick events flow through handleEntry.
	draftState *draftstate.Store
	// ratings is the daemon's read-through cache for the BFF's
	// /api/v1/draft-ratings/{set}/{format} endpoint. Satisfies both
	// pkg/draftalgo.CardLookup and .RatingsLookup so the localapi
	// draft handlers can grade picks against real 17Lands data.
	// Wired into localAPI via SetDraftLookups; token kept in sync
	// with the dispatcher via SetToken on each JWT rotation.
	ratings *ratingsclient.Client
	// mtgaUserID is the local player's MTGA Arena account ID (e.g.
	// "WTC_12345678") extracted from the authenticateResponse log entry.
	// It is used to identify the local player's team in match results so
	// win/loss can be derived.  Empty until a player.authenticated event
	// has been processed in this daemon session.
	mtgaUserID string
	// trayHooks connects the tray icon to the daemon event loop.
	// All fields are optional — nil channels block forever in select (safe no-op).
	trayHooks TrayHooks
	// keychainErrMu guards keychainErr so the goroutine spawned by
	// keychainRefresherAdapter (AC-3, #2135) can write to it safely while the
	// heartbeat ticker reads it on the main run-loop goroutine.
	keychainErrMu sync.Mutex
	// keychainErr is set in New() if keychain.Get() fails at startup.
	// Cleared on retry success inside retryKeychain. When non-nil, Run()
	// calls retryKeychain before starting the event loop.
	// Must be accessed under keychainErrMu after New() returns (the goroutine
	// spawned by keychainRefresherAdapter writes from a different goroutine).
	keychainErr error
	// keychainGet is the function used to read the API key from the OS keychain.
	// Defaults to keychain.Get; overridden in tests for deterministic behaviour.
	keychainGet func() (string, error)
	// eventBuffer is the bounded ring buffer wired into the dispatcher so that
	// events are not silently lost when the BFF is transiently unreachable.
	// Capacity is 1000 (hard-coded for v0.3.3; configurable knob deferred to
	// v0.4.0). Dropped() is sampled on every SetState update and surfaced on
	// /api/v1/system/health as metrics.dispatchDropped.
	eventBuffer *dispatch.RingBuffer

	// driftMu guards the three parse-failure tracking fields below.
	// recordParseFailure acquires the lock on every typed-parse error;
	// snapshotAndResetDrift acquires it once per heartbeat tick to read
	// and zero the state atomically.
	driftMu sync.Mutex
	// parseFailureCount counts typed-parse errors since the last heartbeat.
	parseFailureCount uint32
	// sampleLineHash is the SHA-256 hex[:16] of the most recently failing
	// raw log line. Overwritten on each failure; never the raw line itself.
	sampleLineHash string
	// failedEventTypes accumulates the distinct event-type strings for which
	// at least one parse error occurred since the last heartbeat reset.
	failedEventTypes map[string]struct{}

	// authPaused is true when the daemon has reached the max PKCE attempt cap
	// (#2133 consent loop guard). It is set via WithAuthPaused (called from
	// main.go after loading daemon-state.json) and guards computeAuthStatus.
	// Only cleared when the user explicitly triggers a successful auth or
	// clicks "Retry Setup" (RC3: no timer-based reset). Read-only after Run()
	// starts; never written from goroutines spawned inside Run(). Protected
	// by atomic.Bool to allow safe concurrent reads from the heartbeat ticker.
	authPaused atomic.Bool

	// reauthFunc is the in-process PKCE re-auth callback set via WithReauthFunc.
	// When non-nil and a 401 is received in keychain mode, keychainRefresherAdapter
	// calls this function to trigger an in-process PKCE re-auth flow rather than
	// immediately surfacing ErrReauthRequired to the tray. The function is
	// responsible for updating the dispatcher token on success (via SetToken).
	// Set once before Run() via WithReauthFunc; never mutated after that.
	reauthFunc func(ctx context.Context) error

	// reauthInProgress is a concurrency gate: only ONE PKCE attempt runs at a
	// time, even if multiple 401 responses arrive concurrently (e.g. events
	// processed in rapid succession). The second caller sees reauthInProgress=true
	// and returns ErrReauthRequired immediately without triggering a second PKCE
	// flow — the first flow will update the token for both.
	//
	// CONCURRENCY PRIMITIVE ONLY. Must NEVER appear in computeAuthStatus,
	// any localapi.State field, or any /health response — per Ray Q2 (#2135).
	// Reading or setting this field from application state logic is a bug.
	reauthInProgress atomic.Bool

	// bffMu guards the two BFF-failure tracking fields below.
	// recordBFFFailure and clearBFFFailureCounter acquire this lock.
	bffMu sync.Mutex
	// consecutiveBFFFailures counts how many consecutive SendOrBuffer calls
	// have ended in terminal failure (all retries exhausted). Reset to 0 on
	// the next successful dispatch. Included in the heartbeat payload so the
	// BFF can emit daemon.dispatch_degraded when the count exceeds the threshold.
	consecutiveBFFFailures uint32
	// lastBFFStatusCode is the HTTP status code from the most recent terminal
	// BFF failure. 0 for transport-level failures.
	lastBFFStatusCode int
}

// New creates a Service from cfg.
func New(cfg *config.Config) *Service {
	// Resolve the dispatcher bearer token in this priority order:
	//   1. cfg.Keychain == true → load api_key from the OS keychain (PKCE path).
	//   2. cfg.DaemonJWT (legacy HMAC daemon-JWT path).
	//   3. cfg.APIKey plaintext (pre-keychain-migration legacy path).
	// The PKCE path is the only one that works against the current BFF; the
	// legacy registrar refresher is no longer wired because the BFF no longer
	// mounts /api/daemon/register (see ADR-009 / #1315).
	token := ""
	var keychainErr error
	switch {
	case cfg.Keychain:
		key, err := keychain.Get()
		if err != nil {
			keychainErr = err
			log.Printf("[daemon] warn: keychain.Get failed: %v — will retry on startup", err)
		}
		token = key
	case cfg.DaemonJWT != "":
		token = cfg.DaemonJWT
	default:
		token = cfg.APIKey
	}
	// Bounded ring buffer: capacity 1000, hard-coded for v0.3.3.
	// Configurable knob deferred to v0.4.0 (see #2557 follow-on).
	buf := dispatch.NewRingBuffer(1000)
	d := dispatch.New(cfg.CloudAPIURL, cfg.IngestPath, token).WithBuffer(buf)
	sessionID := fmt.Sprintf("live-%s", uuid.New().String())

	svc := &Service{
		cfg:         cfg,
		dispatcher:  d,
		eventBuffer: buf,
		sessionID:   sessionID,
		regClient:   registrar.NewClient(cfg.CloudAPIURL),
		version:     "dev",
		draftState:  draftstate.New(),
		keychainErr: keychainErr,
		keychainGet: keychain.Get,
		ratings: ratingsclient.New(ratingsclient.Config{
			BFFURL: cfg.CloudAPIURL,
			Token:  token,
		}),
	}
	// Wire the appropriate Refresher based on the auth mode:
	// - Keychain mode: KeychainReauthRequired fires the tray hook and returns
	//   ErrReauthRequired, which breaks the retry loop immediately.
	// - Legacy mode: Refresh calls the registration endpoint to obtain a new JWT.
	if cfg.Keychain {
		d.WithRefresher(svc.keychainRefresherAdapter())
	} else {
		d.WithRefresher(svc)
	}

	// Build the GRE session manager.  The flush func emits a match.game_ended
	// DaemonEvent carrying the accumulated GRE entries as the raw payload.
	svc.greManager = gre.NewManager(gre.ManagerConfig{
		FlushThreshold: cfg.GRESessionFlushThreshold,
		StaleMinutes:   cfg.GRESessionStaleMinutes,
		Flush:          svc.flushGREBuffer,
	})

	// Wire the BFF-failure callback so terminal dispatch failures are counted,
	// and the success callback so the counter resets on the next confirmed send.
	// The counter is included in the next heartbeat payload so the BFF can emit
	// daemon.dispatch_degraded to PostHog when the count exceeds the threshold.
	// onBFFFailure fires only on "all retries exhausted" — NOT on context
	// cancellation. onBFFSuccess fires only on an actual HTTP 2xx delivery.
	d.WithOnBFFFailure(svc.recordBFFFailure).WithOnBFFSuccess(svc.clearBFFFailureCounter)

	return svc
}

// computeAuthStatus derives the auth_status string from config, the current
// keychain error sentinel, and the auth-paused flag (#2133). It is a pure
// function (no receiver) so it can be tested independently.
//
// Precedence rules (highest priority first):
//  1. authPaused == true → auth_paused, regardless of any other state.
//     auth_paused OUTRANKS keychain_error (RC5, #2133).
//  2. keychainErr != nil → keychain_error, regardless of Keychain or AccountID.
//  3. cfg.AccountID == "" OR cfg.Keychain == false → setup_required.
//  4. cfg.Keychain == true AND cfg.AccountID != "" AND keychainErr == nil → authenticated.
//
// NOTE: s.keychainErr is the single source of truth for the keychain-error
// state. Do NOT introduce a parallel boolean; when #2136 lands its graceful-
// degradation state machine it must continue to set/clear s.keychainErr so this
// derivation picks up the transition automatically on the next heartbeat tick.
func computeAuthStatus(cfg *config.Config, keychainErr error, authPaused bool) string {
	if authPaused {
		return localapi.AuthStatusAuthPaused
	}
	if keychainErr != nil {
		return localapi.AuthStatusKeychainError
	}
	if !cfg.Keychain || cfg.AccountID == "" {
		return localapi.AuthStatusSetupRequired
	}
	return localapi.AuthStatusAuthenticated
}

// flushGREBuffer is the FlushFunc wired into the GRE session manager.
// It builds a GamePlayPayload from the accumulated entries and dispatches it
// to the BFF as a "match.game_ended" DaemonEvent with partial=true.
func (s *Service) flushGREBuffer(ctx context.Context, sessionID string, entries []json.RawMessage, partial bool) error {
	payload := contract.GamePlayPayload{
		Partial:     partial,
		LifeChanges: []contract.LifeChangeEntry{},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("flushGREBuffer: marshal payload: %w", err)
	}

	evt := contract.DaemonEvent{
		Type:       "match.game_ended",
		AccountID:  s.cfg.AccountID,
		SessionID:  s.sessionID,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(raw),
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return s.dispatcher.SendOrBuffer(dispatchCtx, evt)
}

// WithVersion sets the build-time version string used for update checks.
// Call this after New() before Run(). Defaults to "dev" if never called.
func (s *Service) WithVersion(v string) {
	if v != "" {
		s.version = v
	}
}

// setKeychainErr sets s.keychainErr under the mutex. Use this wherever
// keychainErr is written after New() returns (i.e., in any goroutine).
func (s *Service) setKeychainErr(err error) {
	s.keychainErrMu.Lock()
	s.keychainErr = err
	s.keychainErrMu.Unlock()
}

// getKeychainErr returns s.keychainErr under the mutex. Use this wherever
// keychainErr is read concurrently with keychainRefresherAdapter goroutines.
func (s *Service) getKeychainErr() error {
	s.keychainErrMu.Lock()
	defer s.keychainErrMu.Unlock()
	return s.keychainErr
}

// WithReauthFunc wires an in-process PKCE re-auth callback into the service.
// Call this after New() and before Run(). When set, a BFF 401 in keychain mode
// triggers an in-process PKCE re-auth via fn rather than immediately surfacing
// ErrReauthRequired to the tray. The daemon stays running throughout (Ray Q1).
//
// fn must update the dispatcher token on success (via s.dispatcher.SetToken).
// On failure fn should return a non-nil error; ErrReauthFailed will be set on
// s.keychainErr so computeAuthStatus routes to "keychain_error" at the next
// heartbeat tick. The keychain is NOT cleared on failure (Ray Q5).
func (s *Service) WithReauthFunc(fn func(ctx context.Context) error) {
	s.reauthFunc = fn
}

// WithAuthPaused sets the auth-paused flag from daemon-state.json before
// Run() starts (#2133 consent loop guard). When true, the daemon does not
// open a browser or attempt PKCE on startup; computeAuthStatus returns
// AuthStatusAuthPaused until the user explicitly clicks "Retry Setup".
//
// Call this after New() and before Run(), from main.go after loading
// daemon-state.json (RC2: state file is read BEFORE NeedsFirstRunAuth).
func (s *Service) WithAuthPaused(paused bool) {
	s.authPaused.Store(paused)
}

// ClearAuthPaused clears the auth-paused flag. Called after a successful
// PKCE completion or after the user explicitly clicks "Retry Setup" (RC3).
// Callers are responsible for persisting the cleared state to daemon-state.json.
func (s *Service) ClearAuthPaused() {
	s.authPaused.Store(false)
}

// Refresh implements dispatch.Refresher. It is called by the dispatcher when
// the BFF returns 401 so a new JWT can be obtained before the next retry.
func (s *Service) Refresh(ctx context.Context) (string, error) {
	return s.register(ctx)
}

// KeychainReauthRequired is called when the BFF returns 401 in keychain mode.
// It fires the tray hook (if wired) so the UI can prompt the user to
// re-authenticate, then returns dispatch.ErrReauthRequired to signal the
// dispatcher to break its retry loop immediately.
func (s *Service) KeychainReauthRequired(reason string) error {
	if s.trayHooks.SetReauthRequired != nil {
		s.trayHooks.SetReauthRequired(reason)
	}
	return dispatch.ErrReauthRequired
}

// keychainRefresherAdapter wraps the keychain 401 recovery as a dispatch.Refresher.
//
// When s.reauthFunc is set (wired via WithReauthFunc from cmd/daemon/main.go),
// it attempts an in-process PKCE re-auth:
//   - The reauthInProgress gate (atomic.Bool) ensures only one PKCE attempt runs
//     at a time; a concurrent caller returns ErrReauthRequired immediately so the
//     first PKCE flow can complete and update the token for both.
//   - A Sentry breadcrumb is added on trigger and outcome (zero-PII payload).
//   - On success: s.keychainErr is cleared so the next heartbeat reports "authenticated".
//   - On failure: s.keychainErr is set to ErrReauthFailed so computeAuthStatus
//     routes to "keychain_error". The keychain is NOT cleared (Ray Q5).
//
// When s.reauthFunc is nil (no WithReauthFunc call), the original behavior is
// preserved: fire the tray hook and return ErrReauthRequired immediately.
func (s *Service) keychainRefresherAdapter() dispatch.Refresher {
	return refresherFunc(func(ctx context.Context) (string, error) {
		if s.reauthFunc == nil {
			// No in-process reauth wired — fall back to tray-hook only.
			return "", s.KeychainReauthRequired("BFF returned 401")
		}

		// Concurrency gate: if a PKCE attempt is already in flight, let it
		// complete and return ErrReauthRequired so this caller waits for the
		// next dispatcher retry cycle with the refreshed token.
		if !s.reauthInProgress.CompareAndSwap(false, true) {
			log.Printf("[daemon] reauth: PKCE already in progress — skipping duplicate attempt")
			return "", dispatch.ErrReauthRequired
		}

		// Run the PKCE flow in a goroutine so the dispatcher's Refresh call
		// returns promptly. ErrReauthRequired breaks the current retry loop;
		// the next inbound event will retry with the fresh token if PKCE succeeds.
		//
		// context.Background() is intentional here: ctx is the dispatcher's
		// 5-second dispatch timeout, which fires long before a user can complete
		// browser-based PKCE auth (typically 10–30s of real interaction). Using
		// ctx would cause guaranteed context.DeadlineExceeded, set ErrReauthFailed
		// permanently, and leave the daemon stuck in keychain_error forever.
		// The PKCE flow manages its own internal deadline; reauthInProgress
		// prevents concurrent goroutines so there is no goroutine-leak risk.
		// (Sarah S-07 P1 fix — #2135)
		go func() {
			defer s.reauthInProgress.Store(false)

			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "reauth",
				Message:  "reactive 401 re-auth triggered",
				Level:    sentry.LevelInfo,
			})

			log.Printf("[daemon] reauth: starting in-process PKCE re-auth (BFF returned 401)")

			if err := s.reauthFunc(context.Background()); err != nil {
				log.Printf("[daemon] reauth: PKCE re-auth failed: %v", err)
				// Emit daemon.auth_failed with the classified reason code so
				// operators can distinguish user-cancellation from wall-clock
				// timeout in PostHog. Fire-and-forget in a goroutine so the
				// reauthInProgress goroutine is not blocked by the 5-second
				// dispatch timeout. Matches the existing bff_rejected pattern
				// at service.go:1166.
				go s.dispatchAuthFailed(context.Background(), classifyPKCEError(err))
				// Set sentinel so computeAuthStatus routes to "keychain_error"
				// at the next heartbeat tick. Do NOT clear the keychain (Ray Q5).
				s.setKeychainErr(ErrReauthFailed)

				sentry.AddBreadcrumb(&sentry.Breadcrumb{
					Category: "reauth",
					Message:  "reactive 401 re-auth failed",
					Level:    sentry.LevelError,
				})
				return
			}

			log.Printf("[daemon] reauth: in-process PKCE re-auth succeeded")
			// Read the fresh token from keychain and wire it into the dispatcher
			// so subsequent events use the new API key immediately.
			// reauthFunc is responsible for storing the new key in the OS keychain
			// before returning nil; we read it back here to keep token management
			// in one place (the keychain is the source of truth in keychain mode).
			if freshKey, kcErr := s.keychainGet(); kcErr == nil && freshKey != "" {
				s.dispatcher.SetToken(freshKey)
				if s.ratings != nil {
					s.ratings.SetToken(freshKey)
				}
			} else {
				log.Printf("[daemon] reauth: warn: could not read fresh key from keychain after reauth: %v", kcErr)
			}
			// Clear keychainErr so computeAuthStatus reports "authenticated"
			// at the next heartbeat tick.
			s.setKeychainErr(nil)

			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "reauth",
				Message:  "reactive 401 re-auth succeeded",
				Level:    sentry.LevelInfo,
			})
		}()

		return "", dispatch.ErrReauthRequired
	})
}

// refresherFunc is a function type that adapts a plain func to dispatch.Refresher.
type refresherFunc func(ctx context.Context) (string, error)

func (f refresherFunc) Refresh(ctx context.Context) (string, error) {
	return f(ctx)
}

// register calls the BFF registration endpoint and persists the resulting JWT.
func (s *Service) register(ctx context.Context) (string, error) {
	resp, err := s.regClient.Register(ctx, s.cfg.APIKey, s.cfg.UserID)
	if err != nil {
		return "", fmt.Errorf("daemon: registration failed: %w", err)
	}

	s.cfg.DaemonJWT = resp.Token
	s.cfg.DaemonID = resp.DaemonID
	s.dispatcher.SetToken(resp.Token)
	// Keep the ratings client's bearer in sync with the dispatcher so
	// the next /api/v1/draft-ratings fetch (after TTL expiry) uses the
	// fresh token. Cache contents stay valid across the swap — the
	// BFF's auth check happens per-request.
	if s.ratings != nil {
		s.ratings.SetToken(resp.Token)
	}

	if saveErr := s.cfg.Save(); saveErr != nil {
		log.Printf("[daemon] warn: could not persist JWT to config file: %v", saveErr)
	}

	log.Printf("[daemon] registered successfully (daemon_id=%s)", resp.DaemonID)
	return resp.Token, nil
}

// ErrSetupRequired is returned by retryKeychain when s.keychainErr is
// keychain.ErrNotFound. ErrNotFound is permanent (the api key was never stored
// or the keychain was wiped), so retries are pointless. The caller (Run /
// main.go) must exit immediately; launchd respawn + NeedsFirstRunAuth handles
// the PKCE re-auth on the next boot.
//
// Distinct from the generic "retries exhausted" error so callers can branch on
// error type rather than string comparison.
var ErrSetupRequired = errors.New("keychain: api key not found — setup required")

// ErrReauthFailed is set on s.keychainErr when a PKCE re-auth callback
// (WithReauthFunc) returns an error. It signals computeAuthStatus to route to
// "keychain_error" at the next heartbeat tick, making the failure visible via
// the /health endpoint. The TryAgain tray channel (#2136) can clear this state.
//
// The keychain is NOT cleared on PKCE failure — per Ray's Q5 answer (#2135).
var ErrReauthFailed = errors.New("reauth: PKCE flow failed")

// keychainMaxRetries is the number of keychain retry attempts before the daemon
// gives up and exits. Exposed as a var so tests can override it.
var keychainMaxRetries = 3

// keychainRetryBase is the base backoff duration for keychain retries. The
// actual wait for attempt N is keychainRetryBase * N (2s, 4s, 8s). Exposed as
// a var so tests can use shorter durations.
//
// NOTE: The ticket AC specified 500ms/1s/2s backoff. Per Ray's plan-review
// (#2136#issuecomment-4566034474), the existing 2s/4s/8s linear schedule is
// correct — the AC was written before retryKeychain existed; code wins.
var keychainRetryBase = 2 * time.Second

// retryKeychain retries keychain.Get with linear backoff (2s/4s/8s), surfacing
// the error state in the tray. Returns nil on success, ErrSetupRequired if the
// error is permanent (ErrNotFound), or a generic error after all retries are
// exhausted or the context is cancelled.
//
// REV-1: ErrNotFound short-circuit is the FIRST statement — before any tray
// state change. This ensures computeAuthStatus returns "setup_required" (not
// "keychain_error") on the next heartbeat tick.
func (s *Service) retryKeychain(ctx context.Context) error {
	// ── REV-1: ErrNotFound short-circuit ──────────────────────────────────────
	// ErrNotFound is permanent (key never stored / keychain wiped). Retrying
	// would loop forever. Clear keychainErr so computeAuthStatus routes to
	// "setup_required" rather than "keychain_error", then return the sentinel
	// without touching tray state. Launchd respawn + NeedsFirstRunAuth handles
	// PKCE re-auth on the next boot.
	if errors.Is(s.getKeychainErr(), keychain.ErrNotFound) {
		s.setKeychainErr(nil)
		return ErrSetupRequired
	}

	// ── Transient error: surface tray state and retry ─────────────────────────
	if s.trayHooks.SetKeychainError != nil {
		s.trayHooks.SetKeychainError(true)
	}
	defer func() {
		if s.trayHooks.SetKeychainError != nil {
			s.trayHooks.SetKeychainError(false)
		}
	}()

	for attempt := 1; attempt <= keychainMaxRetries; attempt++ {
		backoff := keychainRetryBase * time.Duration(attempt)
		log.Printf("[daemon] keychain retry %d/%d in %s", attempt, keychainMaxRetries, backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.trayHooks.TryAgain:
			// User clicked Try Again — retry immediately, skipping backoff.
			log.Printf("[daemon] keychain retry %d/%d triggered by user", attempt, keychainMaxRetries)
		case <-time.After(backoff):
			// Automatic retry after backoff.
		}

		key, err := s.keychainGet()
		if err == nil && key != "" {
			log.Printf("[daemon] keychain retry %d/%d succeeded", attempt, keychainMaxRetries)
			s.setKeychainErr(nil)
			s.dispatcher.SetToken(key)
			if s.ratings != nil {
				s.ratings.SetToken(key)
			}
			return nil
		}
		log.Printf("[daemon] keychain retry %d/%d failed: %v", attempt, keychainMaxRetries, err)
	}

	// Dispatch daemon.keychain_error only when AccountID is non-empty (post-auth
	// case B per Ray's OQ-1 verdict). Pre-auth keychain failures are unobservable
	// via the BFF emission boundary — the event would have no api_key and never
	// reach the BFF. The correct signal for the pre-auth case is heartbeat-absence.
	if s.cfg.AccountID != "" {
		go s.dispatchKeychainError(ctx, "os_error")
	}

	return fmt.Errorf("keychain unavailable after %d retries", keychainMaxRetries)
}

// runUpdateCheck calls updatecheck.Check and swallows any panics. Errors are
// already swallowed inside the updatecheck package itself; this wrapper ensures
// the version check can never affect service health.
func (s *Service) runUpdateCheck(ctx context.Context) {
	if s.cfg.DisableUpdateCheck {
		return
	}
	updatecheck.Check(ctx, s.cfg.CloudAPIURL, s.version)
}

// Run starts the daemon, blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	// Phase 0: if the keychain was unavailable at startup, retry before
	// starting the event loop. Returns an error if all retries fail —
	// the caller (main.go) will quit cleanly.
	if s.getKeychainErr() != nil {
		if err := s.retryKeychain(ctx); err != nil {
			return fmt.Errorf("keychain unavailable after retries: %w", err)
		}
	}

	// Phase 1: ensure we have a valid JWT before starting event dispatch.
	// Skipped when cfg.Keychain is true — the PKCE flow's api_key does not
	// expire and the legacy /api/daemon/register endpoint is not mounted.
	if !s.cfg.Keychain && s.cfg.SyncEnabled && s.cfg.JWTNeedsRefresh() && s.cfg.APIKey != "" {
		if _, err := s.register(ctx); err != nil {
			// Non-fatal: log and continue; the dispatcher will retry on 401.
			log.Printf("[daemon] warn: startup registration failed: %v", err)
		}
	}

	// Run update check once on startup (non-blocking).
	go s.runUpdateCheck(ctx)

	logPath := s.cfg.LogPath
	if logPath == "" {
		detected, err := logreader.DefaultLogPath()
		if err != nil {
			return fmt.Errorf("detect log path: %w", err)
		}
		logPath = detected
		log.Printf("[daemon] auto-detected log path: %s", logPath)
	}

	if s.cfg.LogPreserveOnStart {
		dst, err := logreader.Snapshot(logPath, s.cfg.LogArchiveDir)
		if err != nil {
			log.Printf("[daemon] warn: log snapshot failed: %v", err)
		} else if dst != "" {
			log.Printf("[daemon] log snapshot saved: %s", dst)
		}
		if err := logreader.PruneSnapshots(s.cfg.LogArchiveDir, s.cfg.LogArchiveMaxAge); err != nil {
			log.Printf("[daemon] warn: prune snapshots failed: %v", err)
		}
	}

	pollerCfg := logreader.DefaultPollerConfig(logPath)
	pollerCfg.Interval = s.cfg.PollInterval
	pollerCfg.UseFileEvents = s.cfg.UseFSNotify
	pollerCfg.ReadFromStart = true

	poller, err := logreader.NewPoller(pollerCfg)
	if err != nil {
		return fmt.Errorf("create poller: %w", err)
	}
	s.poller = poller

	updates := poller.Start()
	errs := poller.Errors()

	// Phase 0 of the daemon-local-API plan: serve a /health endpoint on
	// 127.0.0.1:9001 so the SPA's "daemon connected" indicator can detect
	// this process. Non-fatal — if the port is busy (e.g. a previous daemon
	// instance is still draining), the daemon continues with dispatch only.
	startedAt := time.Now().UTC()
	localAPI := localapi.New(localapi.DefaultPort, localapi.State{
		Version:      s.version,
		SessionID:    s.sessionID,
		StartedAt:    startedAt,
		AccountID:    s.cfg.AccountID,
		CloudAPIURL:  s.cfg.CloudAPIURL,
		BFFReachable: true, // optimistic — flips when a dispatch fails
		AuthStatus:   computeAuthStatus(s.cfg, s.getKeychainErr(), s.authPaused.Load()),
	})
	// Hand the localapi server a read view of the live draft state so
	// /api/v1/drafts/{id}/current-pack, /grade-pick, and /win-probability
	// can answer from in-memory data without a separate parse pass.
	localAPI.SetDraftStore(s.draftState)
	// Wire the ratings client as both CardLookup and RatingsLookup —
	// it satisfies both interfaces. Without this, grade-pick returns
	// "N/A" and win-probability falls back to the neutral baseline.
	localAPI.SetDraftLookups(s.ratings, s.ratings)
	// Wire the replay trigger so POST /api/v1/replay can start a
	// historical log replay that emits replay:* events via the BFF.
	localAPI.SetReplayTrigger(s.Replay)
	// Use the daemon lifecycle context so replay goroutines are cancelled
	// when the daemon stops rather than on HTTP request completion.
	localAPI.WithContext(ctx)
	if err := localAPI.Start(); err != nil {
		log.Printf("[daemon] warn: local API server did not start: %v", err)
	}
	defer func() { _ = localAPI.Stop() }()

	log.Printf("[daemon] started (session=%s cloud_api=%s)", s.sessionID, s.cfg.CloudAPIURL)

	// Check if the privileged collection helper is already installed.
	go s.checkHelperOnStartup(ctx)

	// Start the GRE stale-buffer sweep goroutine.
	go s.greManager.RunSweep(ctx)

	// Periodic JWT refresh: check every jwtRefreshInterval whether the stored
	// token is within the refresh window and re-register if so. This ensures
	// mid-session expiry is handled without requiring a daemon restart.
	jwtTicker := time.NewTicker(jwtRefreshInterval)
	defer jwtTicker.Stop()

	// Periodic version check: every 24 hours, check for a newer daemon release.
	updateTicker := time.NewTicker(updateCheckInterval)
	defer updateTicker.Stop()

	// Periodic liveness heartbeat: every 30 seconds, send a daemon.heartbeat
	// event so the BFF health check has a signal even when MTGA is idle.
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	// Periodic helper status check: probe the collection helper socket so the
	// tray reflects current state if the helper is installed or stopped outside
	// of the Grant Access flow.
	helperCheckTicker := time.NewTicker(helperCheckInterval)
	defer helperCheckTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			poller.Stop()
			// Flush all non-empty GRE session buffers before exit.
			flushCtx, flushCancel := context.WithTimeout(context.Background(), 10*time.Second)
			s.greManager.FlushAll(flushCtx)
			flushCancel()
			log.Printf("[daemon] stopped")
			return nil

		case <-jwtTicker.C:
			// Skip in keychain (PKCE) mode — api_key does not expire.
			if !s.cfg.Keychain && s.cfg.SyncEnabled && s.cfg.JWTNeedsRefresh() && s.cfg.APIKey != "" {
				log.Printf("[daemon] JWT within refresh window — re-registering")
				if _, err := s.register(ctx); err != nil {
					log.Printf("[daemon] warn: periodic JWT refresh failed: %v", err)
				}
			}

		case <-updateTicker.C:
			// Run non-blocking; errors are swallowed inside runUpdateCheck.
			go s.runUpdateCheck(ctx)

		case <-heartbeatTicker.C:
			// Refresh the localapi state snapshot so /health reflects the
			// current dispatch_dropped counter. The heartbeat tick is the
			// natural update point — 30-second staleness is acceptable.
			now := time.Now().UTC()
			localAPI.SetState(localapi.State{
				Version:         s.version,
				SessionID:       s.sessionID,
				StartedAt:       startedAt,
				AccountID:       s.cfg.AccountID,
				CloudAPIURL:     s.cfg.CloudAPIURL,
				BFFReachable:    true,
				DispatchDropped: s.eventBuffer.Dropped(),
				LastDispatchAt:  &now,
				AuthStatus:      computeAuthStatus(s.cfg, s.getKeychainErr(), s.authPaused.Load()),
			})

			// Skip when AccountID is not yet set (daemon not authenticated).
			if s.cfg.AccountID == "" {
				continue
			}
			// Snapshot and reset the parse-failure counter so each heartbeat
			// window carries an independent, non-overlapping slice of counts.
			driftCount, driftHash, driftTypes := s.snapshotAndResetDrift()
			// Snapshot BFF failure counter under lock; do not reset it here —
			// the daemon resets it on the next successful SendOrBuffer, not per
			// heartbeat. The BFF decides whether to emit dispatch_degraded.
			s.bffMu.Lock()
			bffFailCount := s.consecutiveBFFFailures
			bffStatusCode := s.lastBFFStatusCode
			s.bffMu.Unlock()
			hbPayload := heartbeatPayload{
				ParseFailureCount:      driftCount,
				SampleLineHash:         driftHash,
				FailedEventTypes:       driftTypes,
				ConsecutiveBFFFailures: bffFailCount,
				LastBFFStatusCode:      bffStatusCode,
			}
			evt, err := dispatch.BuildEvent("daemon.heartbeat", s.cfg.AccountID, s.sessionID, hbPayload)
			if err != nil {
				log.Printf("[daemon] warn: build heartbeat event: %v", err)
				continue
			}
			dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if sendErr := s.dispatcher.SendOrBuffer(dispatchCtx, evt); sendErr != nil {
				log.Printf("[daemon] warn: heartbeat dispatch: %v", sendErr)
			}
			cancel()

		case <-helperCheckTicker.C:
			go s.checkHelperOnStartup(ctx)

		case <-s.trayHooks.SyncNow:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[daemon] panic in performCollectionSync: %v", r)
						// Capture for Sentry. Calls on a nil client are safe
						// no-ops when sentry.Init was not called (#1832).
						sentry.CurrentHub().Recover(r)
					}
				}()
				s.performCollectionSync(ctx)
			}()

		case <-s.trayHooks.GrantAccess:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[daemon] panic in installCollectionHelper: %v", r)
						sentry.CurrentHub().Recover(r)
					}
				}()
				s.installCollectionHelper()
			}()

		case err, ok := <-errs:
			if !ok {
				return nil
			}
			log.Printf("[daemon] poller error: %v", err)

		case entry, ok := <-updates:
			if !ok {
				return nil
			}
			if err := s.handleEntry(ctx, entry); err != nil {
				log.Printf("[daemon] handle entry: %v", err)
			}
		}
	}
}

// heartbeatPayload is the JSON body of a daemon.heartbeat event.
// When parse failures have occurred since the last heartbeat, ParseFailureCount
// is non-zero and the drift fields are populated; the BFF inspects these to
// emit a daemon.log_format_drift PostHog event (per ADR-027 §OQ-5, #2569).
// ConsecutiveBFFFailures is the number of consecutive SendOrBuffer terminal
// failures since the last success; the BFF emits daemon.dispatch_degraded when
// this counter is >= dispatchDegradedThreshold (#2139).
type heartbeatPayload struct {
	// From #2569: parse-failure drift detection.
	ParseFailureCount uint32   `json:"parse_failure_count"`
	SampleLineHash    string   `json:"sample_line_hash,omitempty"`
	FailedEventTypes  []string `json:"failed_event_types,omitempty"`
	// From #2139: BFF dispatch degradation signal.
	ConsecutiveBFFFailures uint32 `json:"consecutive_bff_failures,omitempty"`
	LastBFFStatusCode      int    `json:"last_bff_status_code,omitempty"`
}

// recordParseFailure increments the per-heartbeat parse-failure counter,
// overwrites sampleLineHash with a SHA-256 hex[:16] of rawLine, and adds
// eventType to the failedEventTypes set. The raw line is never stored.
// This method is safe to call concurrently; driftMu is acquired internally.
func (s *Service) recordParseFailure(eventType, rawLine string) {
	sum := sha256.Sum256([]byte(rawLine))
	hash := fmt.Sprintf("%x", sum)[:16]

	s.driftMu.Lock()
	s.parseFailureCount++
	s.sampleLineHash = hash
	if s.failedEventTypes == nil {
		s.failedEventTypes = make(map[string]struct{})
	}
	s.failedEventTypes[eventType] = struct{}{}
	s.driftMu.Unlock()
}

// snapshotAndResetDrift reads the three drift fields under the lock, zeroes
// them, and returns copies to the caller. Called once per heartbeat tick so
// each heartbeat window carries an independent, non-overlapping slice of counts.
func (s *Service) snapshotAndResetDrift() (count uint32, hash string, types []string) {
	s.driftMu.Lock()
	count = s.parseFailureCount
	hash = s.sampleLineHash
	for et := range s.failedEventTypes {
		types = append(types, et)
	}
	s.parseFailureCount = 0
	s.sampleLineHash = ""
	s.failedEventTypes = nil
	s.driftMu.Unlock()

	sort.Strings(types)
	return count, hash, types
}

// sentryDispatchDegradedThreshold is the consecutive-failure count at which
// recordBFFFailure emits a Sentry warning event. Mirrors the BFF-side
// dispatchDegradedThreshold (services/bff/internal/api/handlers/ingest.go) so
// the two systems agree on what "degraded" means. Held as a separate constant
// to avoid a daemon→bff package import.
const sentryDispatchDegradedThreshold = uint32(3)

// recordBFFFailure increments the consecutive-BFF-failure counter and records
// the last status code. Called by the onBFFFailure callback wired into the
// Dispatcher in New(). Safe to call concurrently; bffMu is held internally.
//
// On the transition into a multi-failure streak (count == sentryDispatchDegradedThreshold),
// emit a Sentry message so degraded-BFF episodes surface in the crash
// aggregator. We emit at the threshold rather than on every failure so a brief
// network blip doesn't spam Sentry — only sustained degradation reaches the
// alert surface. #1832.
func (s *Service) recordBFFFailure(statusCode int) {
	s.bffMu.Lock()
	s.consecutiveBFFFailures++
	count := s.consecutiveBFFFailures
	s.lastBFFStatusCode = statusCode
	s.bffMu.Unlock()

	if count == sentryDispatchDegradedThreshold {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("event", "daemon.dispatch_degraded")
			scope.SetTag("bff_status_code", fmt.Sprintf("%d", statusCode))
			scope.SetLevel(sentry.LevelWarning)
			sentry.CaptureMessage(fmt.Sprintf(
				"daemon.dispatch_degraded count=%d status=%d", count, statusCode,
			))
		})
	}
}

// clearBFFFailureCounter resets the consecutive-failure counter and status code
// to zero. Called after a successful SendOrBuffer. Safe to call concurrently.
func (s *Service) clearBFFFailureCounter() {
	s.bffMu.Lock()
	s.consecutiveBFFFailures = 0
	s.lastBFFStatusCode = 0
	s.bffMu.Unlock()
}

// authFailedPayload is the JSON body of a daemon.auth_failed dispatch event.
// reason is one of: "bff_rejected", "pkce_timeout", "pkce_cancelled",
// "pkce_token_exchange_failed". The latter is emitted when the Clerk token
// endpoint rejects the authorization code (e.g. HTTP 4xx "invalid_grant") and
// was added in #2172 as the third code in the cb4a4c15 [#88] taxonomy.
// BFFStatusCode is populated only when reason is "bff_rejected"; it carries the
// raw HTTP status (401, 403, etc.) for operator routing on the dashboard.
type authFailedPayload struct {
	Reason        string `json:"reason"`
	BFFStatusCode int    `json:"bff_status_code,omitempty"`
	Platform      string `json:"platform"`
	DaemonVersion string `json:"daemon_version"`
}

// keychainErrorPayload is the JSON body of a daemon.keychain_error dispatch event.
// ErrorType is one of: "not_found", "os_error".
type keychainErrorPayload struct {
	ErrorType     string `json:"error_type"`
	Platform      string `json:"platform"`
	DaemonVersion string `json:"daemon_version"`
}

// dispatchAuthFailed sends a daemon.auth_failed event to the BFF via a
// transient dispatcher that has NO refresher set. This is intentional:
// dispatching telemetry about an auth failure must not itself trigger the
// auth-failure tray hook again (which would happen if we used s.dispatcher in
// keychain mode and the BFF returned 401 for the telemetry event). A no-refresher
// dispatcher will retry up to 3 times and buffer on exhaustion — correct
// behaviour for a telemetry event that the BFF may briefly be unable to accept.
// reason must be one of: "bff_rejected", "pkce_timeout", "pkce_cancelled",
// "pkce_token_exchange_failed" (added #2172). For "bff_rejected", lastBFFStatusCode
// is read under the lock and included as
// bff_status_code. This is best-effort — errors are logged and swallowed.
func (s *Service) dispatchAuthFailed(ctx context.Context, reason string) {
	s.bffMu.Lock()
	statusCode := s.lastBFFStatusCode
	s.bffMu.Unlock()

	p := authFailedPayload{
		Reason:        reason,
		Platform:      runtime.GOOS,
		DaemonVersion: s.version,
	}
	if reason == "bff_rejected" {
		p.BFFStatusCode = statusCode
	}

	// Capture the failure as a Sentry exception so beta-time auth regressions
	// surface in the crash aggregator alongside any related panics. The
	// PostHog event (dispatched below) covers user-impact analytics; Sentry
	// covers exception-side root-cause investigation. #1832.
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("event", "daemon.auth_failed")
		scope.SetTag("reason", reason)
		if reason == "bff_rejected" {
			scope.SetTag("bff_status_code", fmt.Sprintf("%d", statusCode))
		}
		sentry.CaptureMessage(fmt.Sprintf("daemon.auth_failed reason=%s", reason))
	})

	evt, err := dispatch.BuildEvent("daemon.auth_failed", s.cfg.AccountID, s.sessionID, p)
	if err != nil {
		log.Printf("[daemon] warn: build auth_failed event: %v", err)
		return
	}
	// Use a transient dispatcher without a refresher so that if the BFF
	// returns 401 for this telemetry event, we retry silently and buffer —
	// without re-triggering the keychain reauth tray hook a second time.
	// Token() returns the primary dispatcher's current bearer token.
	d := dispatch.New(s.cfg.CloudAPIURL, s.cfg.IngestPath, s.dispatcher.Token()).
		WithBuffer(s.eventBuffer)
	dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.SendOrBuffer(dispatchCtx, evt); err != nil {
		log.Printf("[daemon] warn: dispatch auth_failed event: %v", err)
	}
}

// classifyPKCEError maps a PKCE error to the appropriate daemon.auth_failed
// reason code. Precedence (highest first):
//
//  1. context.Canceled (bare or wrapped) — user dismissed the browser window
//     → "pkce_cancelled".
//  2. pkce.ErrTokenExchange (wrapped via %w in pkce.Run) — Clerk token endpoint
//     rejected the authorization code (e.g. HTTP 4xx "invalid_grant") →
//     "pkce_token_exchange_failed". Detected via errors.Is; never strings.Contains.
//  3. All other errors — wall-clock timeout, port-bind failure, etc. →
//     "pkce_timeout" (safe default).
//
// Commit cb4a4c15 [#88] established the two-code taxonomy (pkce_cancelled,
// pkce_timeout). This function extends it with pkce_token_exchange_failed (#2172).
func classifyPKCEError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "pkce_cancelled"
	}
	if errors.Is(err, pkce.ErrTokenExchange) {
		return "pkce_token_exchange_failed"
	}
	return "pkce_timeout"
}

// dispatchKeychainError sends a daemon.keychain_error event to the BFF via a
// transient no-refresher dispatcher (same pattern as dispatchAuthFailed).
// errorType must be one of: "not_found", "os_error".
// This is best-effort — errors are logged and swallowed.
func (s *Service) dispatchKeychainError(ctx context.Context, errorType string) {
	p := keychainErrorPayload{
		ErrorType:     errorType,
		Platform:      runtime.GOOS,
		DaemonVersion: s.version,
	}
	// Capture for Sentry alongside the PostHog telemetry event. #1832.
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("event", "daemon.keychain_error")
		scope.SetTag("error_type", errorType)
		sentry.CaptureMessage(fmt.Sprintf("daemon.keychain_error type=%s", errorType))
	})

	evt, err := dispatch.BuildEvent("daemon.keychain_error", s.cfg.AccountID, s.sessionID, p)
	if err != nil {
		log.Printf("[daemon] warn: build keychain_error event: %v", err)
		return
	}
	d := dispatch.New(s.cfg.CloudAPIURL, s.cfg.IngestPath, s.dispatcher.Token()).
		WithBuffer(s.eventBuffer)
	dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.SendOrBuffer(dispatchCtx, evt); err != nil {
		log.Printf("[daemon] warn: dispatch keychain_error event: %v", err)
	}
}

// handleEntry classifies a log entry and dispatches it to the BFF.
func (s *Service) handleEntry(ctx context.Context, entry *logreader.LogEntry) error {
	if entry == nil || !entry.IsJSON {
		return nil
	}

	eventType := classifyEntry(entry)
	if eventType == "" {
		// Not a tracked event type
		return nil
	}

	// For known event types, use typed payloads so the BFF receives validated,
	// well-typed JSON rather than the raw map[string]interface{} from the log.
	var payload interface{}
	switch eventType {
	case "draft.pack":
		p, err := logreader.ParseDraftPack(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse draft pack: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
			// Mirror the typed payload into in-memory draftstate so
			// localapi handlers can serve current-pack / grade-pick /
			// win-probability without re-parsing the log.
			if s.draftState != nil {
				s.draftState.HandlePack(p)
			}
		}
	case "draft.pick":
		p, err := logreader.ParseDraftPick(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse draft pick: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
			if s.draftState != nil {
				s.draftState.HandlePick(p)
			}
		}
	case "inventory.updated":
		p, err := logreader.ParseInventoryEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse inventory: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "quest.progress":
		p, err := logreader.ParseQuestProgressEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse quest progress: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "quest.completed":
		p, err := logreader.ParseQuestCompletedEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse quest completed: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "collection.updated":
		p, err := logreader.ParseCollectionEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse collection: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "deck.updated":
		p, err := logreader.ParseDeckEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse deck: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "player.authenticated":
		// Cache the local player's MTGA Arena account ID so subsequent
		// match.completed events can determine win/loss from reservedPlayers.
		if resp, ok := entry.JSON["authenticateResponse"].(map[string]interface{}); ok {
			if uid, ok := resp["accountId"].(string); ok && uid != "" {
				s.mtgaUserID = uid
				log.Printf("[daemon] cached MTGA user ID from authenticateResponse")
			}
		}
		payload = entry.JSON

	case "match.completed":
		p, err := logreader.ParseMatchCompletedEntry(entry, s.mtgaUserID)
		if err != nil {
			log.Printf("[daemon] warn: parse match completed: %v", err)
			s.recordParseFailure(eventType, entry.Raw)
			payload = entry.JSON
		} else {
			payload = p
		}
	default:
		payload = entry.JSON
	}

	evt, err := dispatch.BuildEvent(eventType, s.cfg.AccountID, s.sessionID, payload)
	if err != nil {
		return fmt.Errorf("build event: %w", err)
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.dispatcher.SendOrBuffer(dispatchCtx, evt); err != nil {
		if errors.Is(err, dispatch.ErrReauthRequired) {
			// ErrReauthRequired: the BFF returned 401/403 in keychain mode.
			// Fire a dedicated daemon.auth_failed dispatch event in a goroutine
			// (fire-and-forget) so the BFF can emit to PostHog with low latency.
			// Runs in a goroutine so the run-loop entry is not blocked by
			// additional dispatch retries. Uses a transient no-refresher
			// dispatcher so the auth-failure tray hook is not re-triggered.
			go s.dispatchAuthFailed(context.Background(), "bff_rejected")
			return nil
		}
		return err
	}
	// NOTE: the BFF failure/success counter is managed entirely via the
	// onBFFFailure and onBFFSuccess callbacks wired to the Dispatcher in New().
	// Do NOT call clearBFFFailureCounter here — it is called by onBFFSuccess
	// inside SendOrBuffer on confirmed HTTP 2xx delivery.
	return nil
}

// classifyEntry maps a log entry to a semantic event type string.
// Returns "" if the entry is not a tracked event.
func classifyEntry(entry *logreader.LogEntry) string {
	// Draft pick
	if _, ok := entry.JSON["draftPack"]; ok {
		return "draft.pack"
	}
	if _, ok := entry.JSON["pickedCards"]; ok {
		return "draft.pick"
	}

	// Scene change (draft start/end)
	if toScene, ok := entry.JSON["toSceneName"].(string); ok {
		if toScene == "Draft" {
			return "draft.started"
		}
		if fromScene, ok2 := entry.JSON["fromSceneName"].(string); ok2 && fromScene == "Draft" {
			return "draft.ended"
		}
	}

	// Match events — prefer the matchGameRoomStateChangedEvent path (single
	// log line with full result data) over the legacy CurrentEventState path.
	if logreader.IsMatchCompletedEntry(entry) {
		return "match.completed"
	}
	if state, ok := entry.JSON["CurrentEventState"].(string); ok {
		switch state {
		case "MatchCompleted":
			return "match.completed"
		case "MatchInProgress":
			return "match.started"
		}
	}

	// Player authentication / profile
	if _, ok := entry.JSON["authenticateResponse"]; ok {
		return "player.authenticated"
	}

	// Rank update
	if _, ok := entry.JSON["rankClass"]; ok {
		return "player.rank_updated"
	}

	// Inventory update (Arena 2026.58+: wrapped under "InventoryInfo" key)
	if logreader.IsInventoryEntry(entry) {
		return "inventory.updated"
	}

	// Quest events — check completed before progress (more specific).
	if logreader.IsQuestCompletedEntry(entry) {
		return "quest.completed"
	}
	if logreader.IsQuestProgressEntry(entry) {
		return "quest.progress"
	}

	// Collection snapshot (PlayerInventoryGetPlayerCardsV3).
	if logreader.IsCollectionEntry(entry) {
		return "collection.updated"
	}

	// Deck update (DeckUpsertDeckV2).
	if logreader.IsDeckEntry(entry) {
		return "deck.updated"
	}

	return ""
}
