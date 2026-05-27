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
	"sort"
	"sync"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/draftstate"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/gre"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/keychain"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/localapi"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/ratingsclient"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/registrar"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/updatecheck"
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
	// keychainErr is set in New() if keychain.Get() fails at startup.
	// Cleared on retry success inside retryKeychain. When non-nil, Run()
	// calls retryKeychain before starting the event loop.
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

	return svc
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

// keychainRefresherAdapter wraps KeychainReauthRequired as a dispatch.Refresher.
// The Refresh signature requires (ctx, token) but keychain reauth does not need
// them — it unconditionally fires the tray hook and returns ErrReauthRequired.
func (s *Service) keychainRefresherAdapter() dispatch.Refresher {
	return refresherFunc(func(_ context.Context) (string, error) {
		return "", s.KeychainReauthRequired("BFF returned 401")
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

// keychainMaxRetries is the number of keychain retry attempts before the daemon
// gives up and exits. Exposed as a var so tests can override it.
var keychainMaxRetries = 3

// keychainRetryBase is the base backoff duration for keychain retries. The
// actual wait for attempt N is keychainRetryBase * N (2s, 4s, 8s). Exposed as
// a var so tests can use shorter durations.
var keychainRetryBase = 2 * time.Second

// retryKeychain retries keychain.Get with exponential backoff, surfacing the
// error state in the tray. Returns nil on success, an error after all retries
// are exhausted or the context is cancelled.
func (s *Service) retryKeychain(ctx context.Context) error {
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
			s.keychainErr = nil
			s.dispatcher.SetToken(key)
			if s.ratings != nil {
				s.ratings.SetToken(key)
			}
			return nil
		}
		log.Printf("[daemon] keychain retry %d/%d failed: %v", attempt, keychainMaxRetries, err)
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
	if s.keychainErr != nil {
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
			})

			// Skip when AccountID is not yet set (daemon not authenticated).
			if s.cfg.AccountID == "" {
				continue
			}
			// Snapshot and reset the parse-failure counter so each heartbeat
			// window carries an independent, non-overlapping slice of counts.
			driftCount, driftHash, driftTypes := s.snapshotAndResetDrift()
			hbPayload := heartbeatPayload{
				ParseFailureCount: driftCount,
				SampleLineHash:    driftHash,
				FailedEventTypes:  driftTypes,
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
					}
				}()
				s.performCollectionSync(ctx)
			}()

		case <-s.trayHooks.GrantAccess:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[daemon] panic in installCollectionHelper: %v", r)
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
type heartbeatPayload struct {
	ParseFailureCount uint32   `json:"parse_failure_count"`
	SampleLineHash    string   `json:"sample_line_hash,omitempty"`
	FailedEventTypes  []string `json:"failed_event_types,omitempty"`
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
			// Logged once by the dispatcher; suppress per-entry spam.
			return nil
		}
		return err
	}
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
