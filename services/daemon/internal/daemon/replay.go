// Log replay implementation wired into the daemon service.
//
// Replay reads every known Player.log location from the beginning, parses
// each log entry through the same classifier + dispatcher pipeline used during
// live monitoring, and emits replay:started / replay:progress /
// replay:completed / replay:error events so the frontend's useLogReplay hook
// can drive the Settings > Data Recovery progress UI.
//
// The replay is intentionally single-threaded per session (the dispatcher
// serial send order matters for the BFF projection worker).

package daemon

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/dispatch"
	"github.com/ramonehamilton/mtga-daemon/internal/logreader"
)

// ReplayProgressPayload is the payload sent with replay:progress events.
// Field names match gui.LogReplayProgress in frontend/src/types/models.ts.
type ReplayProgressPayload struct {
	TotalFiles       int     `json:"totalFiles"`
	ProcessedFiles   int     `json:"processedFiles"`
	CurrentFile      string  `json:"currentFile"`
	TotalEntries     int     `json:"totalEntries"`
	ProcessedEntries int     `json:"processedEntries"`
	PercentComplete  float64 `json:"percentComplete"`
	MatchesImported  int     `json:"matchesImported"`
	DecksImported    int     `json:"decksImported"`
	QuestsImported   int     `json:"questsImported"`
	DraftsImported   int     `json:"draftsImported"`
	Duration         float64 `json:"duration"`
	Error            string  `json:"error,omitempty"`
}

// Replay implements localapi.ReplayFunc.  It reads the Player.log file from
// the beginning, dispatches each recognised entry through the BFF, and emits
// progress events.
//
// clearDataFirst is accepted for forward-compatibility (the BFF's clear path
// will be added in a follow-up PR once the data-management surface is stable).
// In this version clearDataFirst is logged but does not affect log processing.
//
// This method must not be called directly from tests — use the localapi handler
// integration tests in localapi/replay_test.go instead.
func (s *Service) Replay(ctx context.Context, clearDataFirst bool) {
	start := time.Now()
	accountID := s.cfg.AccountID
	sessionID := s.sessionID

	sendEvent := func(eventType string, payload any) {
		evt, err := dispatch.BuildEvent(eventType, accountID, sessionID, payload)
		if err != nil {
			log.Printf("[replay] warn: build %s event: %v", eventType, err)
			return
		}
		sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if sendErr := s.dispatcher.Send(sendCtx, evt); sendErr != nil {
			log.Printf("[replay] warn: dispatch %s: %v", eventType, sendErr)
		}
	}

	// Locate log files to replay.
	logPath := s.cfg.LogPath
	if logPath == "" {
		var err error
		logPath, err = logreader.DefaultLogPath()
		if err != nil {
			log.Printf("[replay] cannot determine log path: %v", err)
			sendEvent("replay:error", ReplayProgressPayload{
				Error:    "cannot determine log path: " + err.Error(),
				Duration: time.Since(start).Seconds(),
			})
			return
		}
	}

	if clearDataFirst {
		log.Printf("[replay] clearDataFirst=true (BFF data-management clear path not yet implemented — skipping)")
	}

	// Announce start.
	sendEvent("replay:started", ReplayProgressPayload{
		TotalFiles:      1,
		ProcessedFiles:  0,
		CurrentFile:     logPath,
		PercentComplete: 0,
	})

	// Read all entries from the beginning of the log.
	reader, err := logreader.NewReader(logPath)
	if err != nil {
		log.Printf("[replay] open log: %v", err)
		sendEvent("replay:error", ReplayProgressPayload{
			Error:    "failed to open log file: " + err.Error(),
			Duration: time.Since(start).Seconds(),
		})
		return
	}
	defer func() { _ = reader.Close() }()

	progress := ReplayProgressPayload{
		TotalFiles:  1,
		CurrentFile: logPath,
	}

	var processedEntries, matchesImported, decksImported, questsImported, draftsImported int
	lastProgressEmit := time.Now()

	for {
		if ctx.Err() != nil {
			log.Printf("[replay] context cancelled — stopping replay")
			return
		}

		entry, err := reader.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[replay] read error: %v", err)
			break
		}

		processedEntries++

		// Re-use the live-monitoring classifier and dispatcher.
		if err := s.handleEntry(ctx, entry); err != nil {
			log.Printf("[replay] handleEntry: %v", err)
		}

		// Count imports by event type for progress reporting.
		if entry.IsJSON {
			switch classifyEntry(entry) {
			case "match.completed":
				matchesImported++
			case "deck.updated":
				decksImported++
			case "quest.progress", "quest.completed":
				questsImported++
			case "draft.pack", "draft.pick":
				draftsImported++
			}
		}

		// Emit progress at most once per second to avoid flooding SSE.
		if time.Since(lastProgressEmit) >= time.Second {
			lastProgressEmit = time.Now()
			progress.ProcessedEntries = processedEntries
			progress.MatchesImported = matchesImported
			progress.DecksImported = decksImported
			progress.QuestsImported = questsImported
			progress.DraftsImported = draftsImported
			progress.Duration = time.Since(start).Seconds()
			sendEvent("replay:progress", progress)
		}
	}

	// Final progress + completed event.
	progress.ProcessedFiles = 1
	progress.ProcessedEntries = processedEntries
	progress.PercentComplete = 100
	progress.MatchesImported = matchesImported
	progress.DecksImported = decksImported
	progress.QuestsImported = questsImported
	progress.DraftsImported = draftsImported
	progress.Duration = time.Since(start).Seconds()

	sendEvent("replay:completed", progress)
	log.Printf("[replay] complete: %d entries processed in %.2fs (matches=%d decks=%d quests=%d drafts=%d)",
		processedEntries, time.Since(start).Seconds(),
		matchesImported, decksImported, questsImported, draftsImported)
}
