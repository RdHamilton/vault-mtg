// Phase 2 PR #18 — daemon uninstall surface.
//
// POST /api/v1/system/uninstall lets the SPA trigger a clean uninstall
// without forcing users into Terminal / Add-Remove Programs. The handler
// schedules the actual uninstall in a background goroutine and returns
// immediately with a status payload so the HTTP response makes it back
// to the SPA before the daemon process exits.
//
// Why not just shell out to the bundled uninstall.sh / uninstall.ps1?
// Those scripts need sudo (macOS: `sudo rm /usr/local/bin/...`) or
// admin (Windows: system-scoped tasks) — neither works from a headless
// daemon process with no TTY. The scripts remain for manual terminal
// uninstalls; this endpoint does the same thing in-process, scoped to
// what a user-context daemon CAN do without elevation:
//
//   macOS:    launchctl unload + remove ~/Library/LaunchAgents/<plist>
//   Windows:  Stop-ScheduledTask + Unregister-ScheduledTask (user task)
//   Linux:    (no installer exists today — 400 with a clear message)
//
// The daemon binary itself stays on disk after this endpoint runs.
// The SPA surfaces a "drag VaultMTG to Trash" / "use Add/Remove Programs"
// hint in the response payload. That's the 5% of uninstall the daemon
// can't do without elevation.

package localapi

import (
	"net/http"
	"os"
	"time"
)

// Uninstaller is the minimal surface handleSystemUninstall depends on.
// Tests inject a stub; production wires defaultUninstaller, which
// dispatches to the platform-specific implementations below.
type Uninstaller interface {
	// Run performs the platform-appropriate uninstall steps. The string
	// is a user-facing residual-action message (e.g. "drag VaultMTG to
	// the Trash to remove the app bundle"). When purge is true, also
	// removes the daemon's config directory under ~/.config or %APPDATA%.
	Run(purge bool) (message string, err error)
}

type defaultUninstaller struct{}

func (defaultUninstaller) Run(purge bool) (string, error) {
	return runPlatformUninstall(purge)
}

// uninstallResponse is the JSON body returned by POST /api/v1/system/uninstall.
type uninstallResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// shutdownDelay is the grace period between responding to the HTTP
// request and exiting the daemon process. Gives the kernel time to
// flush the response before the listener closes.
const shutdownDelay = 200 * time.Millisecond

// shutdownExit is overridable in tests so they don't actually call
// os.Exit. Production points at os.Exit.
var shutdownExit = os.Exit

// handleSystemUninstall handles POST /api/v1/system/uninstall[?purge=true|false].
// Always POST — uninstall is a side-effect call.
//
// The handler runs the uninstall synchronously (so a failure still
// surfaces a useful error response) and only then schedules the
// daemon's own exit. Without the synchronous step the SPA would see a
// successful 200 even when launchctl bailed.
func (s *Server) handleSystemUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	purge := r.URL.Query().Get("purge") == "true"

	u := s.uninstaller
	if u == nil {
		u = defaultUninstaller{}
	}
	msg, err := u.Run(purge)
	if err != nil {
		writeJSON(w, r, http.StatusInternalServerError, uninstallResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, r, http.StatusOK, uninstallResponse{
		Status:  "scheduled",
		Message: msg,
	})

	// Flush + close the response, then exit shortly after. The
	// goroutine + sleep keep the response body from getting truncated
	// when the process dies.
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	// Capture the exit hook synchronously so the background goroutine
	// doesn't race with tests swapping it back to the original.
	exit := shutdownExit
	go func() {
		time.Sleep(shutdownDelay)
		exit(0)
	}()
}
