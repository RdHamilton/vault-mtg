package localapi

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/sentryhook"
)

// diagnosticsResponse is the JSON body returned by GET /api/v1/system/diagnostics.
// It is designed to be copy-pasted verbatim into a Discord support channel by
// the SPA's "Copy diagnostics" button (issue #1832), so every field must be
// human-readable and contain zero secrets.
type diagnosticsResponse struct {
	DaemonVersion string   `json:"daemon_version"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	UptimeSeconds int64    `json:"uptime_seconds"`
	StartedAt     string   `json:"started_at"`
	CloudAPIURL   string   `json:"cloud_api_url"`
	SessionID     string   `json:"session_id,omitempty"`
	LogPath       string   `json:"log_path"`
	LogTail       []string `json:"log_tail"`
	// LogTailError is set when the daemon log file could not be read. The body
	// still returns 200 with an empty LogTail so the SPA can render the rest
	// of the diagnostics blob — partial information beats a hard failure.
	LogTailError string `json:"log_tail_error,omitempty"`
}

// diagnosticsLogTailLines is the maximum number of trailing log lines returned
// in the response. Matches the ticket's "last 200 lines" requirement.
const diagnosticsLogTailLines = 200

// diagnosticsMaxBytes caps how much of the daemon log file is read. Defends
// against pathologically large log files (rotated bombshells, attached
// debug-mode dumps) — 1 MiB is comfortably more than 200 lines of structured
// log output but bounded enough to keep the request fast.
const diagnosticsMaxBytes = 1 << 20

// handleSystemDiagnostics returns the support-bundle JSON: daemon version, OS,
// uptime, cloud API URL, and the last 200 lines of the daemon log file with
// all secrets scrubbed via sentryhook.Scrub.
//
// Authentication: the localapi server binds 127.0.0.1 only (see server.go);
// the loopback bind IS the auth boundary for this endpoint, identical to
// every other /api/v1/system/* handler in this package. No bearer token is
// required because the surface is unreachable from any non-local origin.
func (s *Server) handleSystemDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	st := s.snapshot()

	logPath := DefaultDaemonLogPath()
	tail, tailErr := readLastLines(logPath, diagnosticsLogTailLines, diagnosticsMaxBytes)
	// Defensively scrub each line before returning. The daemon should not be
	// writing secrets to its log file, but a future contributor adding a
	// `log.Printf("apiKey=%s", ...)` would slip a credential into the support
	// bundle — the scrubber is the second line of defence.
	for i, line := range tail {
		tail[i] = sentryhook.Scrub(line)
	}

	resp := diagnosticsResponse{
		DaemonVersion: st.Version,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		UptimeSeconds: int64(time.Since(st.StartedAt).Seconds()),
		StartedAt:     st.StartedAt.UTC().Format(time.RFC3339),
		CloudAPIURL:   st.CloudAPIURL,
		SessionID:     st.SessionID,
		LogPath:       logPath,
		LogTail:       tail,
	}
	if tailErr != nil {
		resp.LogTailError = tailErr.Error()
	}
	writeJSON(w, r, http.StatusOK, resp)
}

// DefaultDaemonLogPath returns the platform-conventional path where the daemon
// writes its log file. Exported so the daemon's logging-init code can read the
// same value the diagnostics endpoint reports.
//
// The path is heuristic: the daemon does not currently configure a log
// destination explicitly — it logs to stderr, which launchd / NSSM redirects
// to a file. The paths returned here match the redirection targets configured
// in services/daemon/install/macos/pkg/postinstall and the Windows installer.
func DefaultDaemonLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "vaultmtg-daemon.log")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "vaultmtg", "daemon.log")
		}
		return filepath.Join(home, "vaultmtg", "daemon.log")
	default:
		return filepath.Join(home, ".vaultmtg", "daemon.log")
	}
}

// readLastLines returns the last n lines from path, scanning at most maxBytes
// from the end of the file. The implementation is a bounded streaming read:
// it Seeks to max(0, size-maxBytes), scans line-by-line into a ring buffer,
// then returns the buffer contents in original order.
//
// Returns (nil, error) when the file cannot be opened (e.g. missing on a
// fresh install). Callers should still render the diagnostics response with
// the error attached so the operator sees "no log file yet" rather than a
// hard 500.
func readLastLines(path string, n, maxBytes int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var start int64
	if info.Size() > int64(maxBytes) {
		start = info.Size() - int64(maxBytes)
	}
	if _, err := f.Seek(start, 0); err != nil {
		return nil, err
	}

	// Ring buffer of the last n lines.
	ring := make([]string, 0, n)
	scanner := bufio.NewScanner(f)
	// Allow up to 1 MiB per line so an exceptionally long log line (e.g. a
	// dumped JSON payload) does not silently terminate the scan.
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	skipFirst := start > 0 // first partial line is discarded when we mid-seeked
	for scanner.Scan() {
		if skipFirst {
			skipFirst = false
			continue
		}
		if len(ring) < n {
			ring = append(ring, scanner.Text())
		} else {
			// Shift left, append at end. Cheap because n is small (200).
			copy(ring, ring[1:])
			ring[n-1] = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		return ring, err
	}
	return ring, nil
}
