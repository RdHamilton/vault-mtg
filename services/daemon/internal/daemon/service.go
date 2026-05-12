// Package daemon provides the standalone daemon service.
// The daemon reads MTGA Player.log, classifies events, and POSTs them
// to the BFF via contract.DaemonEvent. It never connects to a database.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/google/uuid"
	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/ramonehamilton/mtga-daemon/internal/dispatch"
	"github.com/ramonehamilton/mtga-daemon/internal/draftstate"
	"github.com/ramonehamilton/mtga-daemon/internal/gre"
	"github.com/ramonehamilton/mtga-daemon/internal/keychain"
	"github.com/ramonehamilton/mtga-daemon/internal/localapi"
	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
	"github.com/ramonehamilton/mtga-daemon/internal/ratingsclient"
	"github.com/ramonehamilton/mtga-daemon/internal/registrar"
	"github.com/ramonehamilton/mtga-daemon/internal/updatecheck"
)

// jwtRefreshInterval is how often the run loop checks whether the JWT needs
// refreshing during an active session. It is a variable so tests can shorten it.
var jwtRefreshInterval = time.Hour

// updateCheckInterval is how often the run loop checks for a newer daemon version.
// It is a variable so tests can shorten it.
var updateCheckInterval = 24 * time.Hour

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
	switch {
	case cfg.Keychain:
		key, err := keychain.Get()
		if err != nil {
			log.Printf("[daemon] warn: keychain.Get failed: %v — dispatcher will start with no bearer", err)
		}
		token = key
	case cfg.DaemonJWT != "":
		token = cfg.DaemonJWT
	default:
		token = cfg.APIKey
	}
	d := dispatch.New(cfg.CloudAPIURL, cfg.IngestPath, token)
	sessionID := fmt.Sprintf("live-%s", uuid.New().String())

	svc := &Service{
		cfg:        cfg,
		dispatcher: d,
		sessionID:  sessionID,
		regClient:  registrar.NewClient(cfg.CloudAPIURL),
		version:    "dev",
		draftState: draftstate.New(),
		ratings: ratingsclient.New(ratingsclient.Config{
			BFFURL: cfg.CloudAPIURL,
			Token:  token,
		}),
	}
	// Wire the legacy refresher only when NOT in keychain mode. With keychain
	// the api_key does not expire, and the legacy /api/daemon/register endpoint
	// is not served by the BFF — calling it on every 401 would just spam 404s.
	if !cfg.Keychain {
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

	return s.dispatcher.Send(dispatchCtx, evt)
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
	localAPI := localapi.New(localapi.DefaultPort, localapi.State{
		Version:      s.version,
		SessionID:    s.sessionID,
		StartedAt:    time.Now().UTC(),
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
	if err := localAPI.Start(); err != nil {
		log.Printf("[daemon] warn: local API server did not start: %v", err)
	}
	defer func() { _ = localAPI.Stop() }()

	log.Printf("[daemon] started (session=%s cloud_api=%s)", s.sessionID, s.cfg.CloudAPIURL)

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
			payload = entry.JSON
		} else {
			payload = p
		}
	case "quest.progress":
		p, err := logreader.ParseQuestProgressEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse quest progress: %v", err)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "quest.completed":
		p, err := logreader.ParseQuestCompletedEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse quest completed: %v", err)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "collection.updated":
		p, err := logreader.ParseCollectionEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse collection: %v", err)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "deck.updated":
		p, err := logreader.ParseDeckEntry(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse deck: %v", err)
			payload = entry.JSON
		} else {
			payload = p
		}
	case "match.completed":
		// Pass empty playerUserID — the daemon config does not store the MTGA
		// userId, so opponent identification falls back to the first non-empty
		// playerName in reservedPlayers.
		p, err := logreader.ParseMatchCompletedEntry(entry, "")
		if err != nil {
			log.Printf("[daemon] warn: parse match completed: %v", err)
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

	return s.dispatcher.Send(dispatchCtx, evt)
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
