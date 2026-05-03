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
)

// Service is the top-level daemon service.
type Service struct {
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	poller     *logreader.Poller
	sessionID  string
}

// New creates a Service from cfg.
func New(cfg *config.Config) *Service {
	d := dispatch.New(cfg.CloudAPIURL, cfg.IngestPath, cfg.APIKey)
	return &Service{
		cfg:        cfg,
		dispatcher: d,
		sessionID:  fmt.Sprintf("live-%s", uuid.New().String()),
	}
}

// Run starts the daemon, blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
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

	evt, err := dispatch.BuildEvent(eventType, s.cfg.AccountID, s.sessionID, entry.JSON)
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
