// Package daemon provides the standalone daemon service.
// The daemon reads MTGA Player.log, classifies events, and POSTs them
// to the BFF via contract.DaemonEvent. It never connects to a database.
package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/ramonehamilton/mtga-daemon/internal/dispatch"
	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
	"github.com/ramonehamilton/mtga-daemon/internal/registrar"
)

// Service is the top-level daemon service.
type Service struct {
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	poller     *logreader.Poller
	sessionID  string
	regClient  *registrar.Client
}

// New creates a Service from cfg.
func New(cfg *config.Config) *Service {
	// Use DaemonJWT as the bearer token if present; fall back to APIKey.
	token := cfg.DaemonJWT
	if token == "" {
		token = cfg.APIKey
	}
	d := dispatch.New(cfg.CloudAPIURL, cfg.IngestPath, token)
	svc := &Service{
		cfg:        cfg,
		dispatcher: d,
		sessionID:  fmt.Sprintf("live-%s", uuid.New().String()),
		regClient:  registrar.NewClient(cfg.CloudAPIURL),
	}
	// Wire the dispatcher's 401 refresher to the service.
	d.WithRefresher(svc)
	return svc
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

	if saveErr := s.cfg.Save(); saveErr != nil {
		log.Printf("[daemon] warn: could not persist JWT to config file: %v", saveErr)
	}

	log.Printf("[daemon] registered successfully (daemon_id=%s)", resp.DaemonID)
	return resp.Token, nil
}

// Run starts the daemon, blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	// Phase 1: ensure we have a valid JWT before starting event dispatch.
	if s.cfg.SyncEnabled && s.cfg.JWTNeedsRefresh() && s.cfg.APIKey != "" {
		if _, err := s.register(ctx); err != nil {
			// Non-fatal: log and continue; the dispatcher will retry on 401.
			log.Printf("[daemon] warn: startup registration failed: %v", err)
		}
	}

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

	log.Printf("[daemon] started (session=%s cloud_api=%s)", s.sessionID, s.cfg.CloudAPIURL)

	for {
		select {
		case <-ctx.Done():
			poller.Stop()
			log.Printf("[daemon] stopped")
			return nil

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

	// For draft events use typed payloads so the BFF receives validated,
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
		}
	case "draft.pick":
		p, err := logreader.ParseDraftPick(entry)
		if err != nil {
			log.Printf("[daemon] warn: parse draft pick: %v", err)
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

	// Match events
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

	return ""
}
