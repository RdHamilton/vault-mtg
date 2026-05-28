package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/collectionclient"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
)

// TrayHooks lets the daemon update the tray icon and receive user actions.
// All fields are optional. Nil channels are never selected; nil funcs are no-ops.
type TrayHooks struct {
	// SyncNow is signalled by the tray when the user clicks "Sync Now".
	SyncNow <-chan struct{}
	// GrantAccess is signalled by the tray when the user clicks "Grant Access".
	GrantAccess <-chan struct{}
	// TryAgain is signalled by the tray when the user clicks "Try Again"
	// (keychain retry). Used by retryKeychain to skip the backoff timer.
	TryAgain <-chan struct{}
	// RetrySetup is signalled by the tray when the user clicks "Retry Setup…".
	// The onReady handler opens https://vaultmtg.app/setup in the browser and
	// re-runs the PKCE flow. Buffered cap=1; a second click before the first
	// completes is dropped.
	RetrySetup <-chan struct{}
	// SetHelperInstalled updates the tray menu to show Sync Now (true) or
	// Grant Access (false).
	SetHelperInstalled func(bool)
	// SetLastSync updates the "Collection: synced <time>" menu label.
	SetLastSync func(time.Time)
	// SetKeychainError shows or hides the keychain error state (StatusKeychainError
	// + "Try Again" menu item) in the tray. Call with true when retrying; false when resolved.
	SetKeychainError func(bool)
	// SetSetupRequired shows or hides the StatusSetupRequired state + "Retry Setup…"
	// menu item in the tray. Call with true when PKCE auth fails in onReady; false
	// once setup completes successfully.
	SetSetupRequired func(bool)
	// SetReauthRequired notifies the tray that the daemon received a 401 in
	// keychain mode and user re-authentication is needed. The reason string is
	// a human-readable hint shown in the tray tooltip or menu item.
	SetReauthRequired func(reason string)
}

// WithTray attaches tray integration to the service.
func (s *Service) WithTray(hooks TrayHooks) {
	s.trayHooks = hooks
}

// checkHelperOnStartup probes the helper socket and updates the tray.
func (s *Service) checkHelperOnStartup(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	c := collectionclient.New()
	installed := c.IsHelperRunning()
	if s.trayHooks.SetHelperInstalled != nil {
		s.trayHooks.SetHelperInstalled(installed)
	}
	if installed {
		log.Printf("[daemon] collection helper is running")
	} else {
		log.Printf("[daemon] collection helper not found — tray will prompt for install")
	}
}

// performCollectionSync finds the MTGA process, scans its memory via the
// privileged helper, and dispatches a collection.updated event to the BFF.
func (s *Service) performCollectionSync(ctx context.Context) {
	if s.cfg.AccountID == "" {
		log.Printf("[daemon] collection sync skipped: not authenticated")
		return
	}

	pid, err := findMTGAPID()
	if err != nil {
		log.Printf("[daemon] collection sync: %v", err)
		return
	}

	log.Printf("[daemon] collection sync: scanning MTGA PID %d", pid)

	c := collectionclient.New()
	resp, err := c.Scan(pid)
	if err != nil {
		log.Printf("[daemon] collection sync error: %v", err)
		return
	}

	log.Printf("[daemon] collection sync: %d unique cards", len(resp.Cards))

	cards := make([]contract.CollectionCard, 0, len(resp.Cards))
	for grpID, qty := range resp.Cards {
		cards = append(cards, contract.CollectionCard{
			ArenaID: grpID,
			Count:   qty,
		})
	}

	payload := contract.CollectionUpdatedPayload{
		Cards:   cards,
		IsDelta: false,
	}

	evt, err := dispatch.BuildEvent("collection.updated", s.cfg.AccountID, s.sessionID, payload)
	if err != nil {
		log.Printf("[daemon] collection sync: build event: %v", err)
		return
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.dispatcher.Send(dispatchCtx, evt); err != nil {
		log.Printf("[daemon] collection sync: dispatch: %v", err)
		return
	}

	log.Printf("[daemon] collection sync: dispatched %d cards (captured_at=%s)",
		len(cards), resp.CapturedAt.Format(time.RFC3339))

	if s.trayHooks.SetLastSync != nil {
		s.trayHooks.SetLastSync(resp.CapturedAt)
	}
}

// installCollectionHelper runs the privileged helper installer. On macOS this
// uses osascript to prompt for an admin password then run the install script.
// After install it re-probes the socket and updates the tray.
func (s *Service) installCollectionHelper() {
	if runtime.GOOS != "darwin" {
		log.Printf("[daemon] collection helper install not supported on %s", runtime.GOOS)
		return
	}

	helperBinary, scriptDir, err := locateHelperFiles()
	if err != nil {
		log.Printf("[daemon] cannot locate helper files: %v", err)
		return
	}

	log.Printf("[daemon] installing collection helper from %s", helperBinary)
	if err := runHelperInstaller(helperBinary, scriptDir); err != nil {
		log.Printf("[daemon] helper install failed: %v", err)
		return
	}

	// Poll until the helper socket is accepting connections (launchd startup
	// can take several seconds). Give up after 15s and let the next periodic
	// check update the tray when the socket eventually comes up.
	c := collectionclient.New()
	installed := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if c.IsHelperRunning() {
			installed = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if s.trayHooks.SetHelperInstalled != nil {
		s.trayHooks.SetHelperInstalled(installed)
	}
	if installed {
		log.Printf("[daemon] collection helper installed and running")
	} else {
		log.Printf("[daemon] collection helper installed but socket not yet up — will retry")
	}
}

// locateHelperFiles returns the path to the collection-helper binary and the
// directory containing the install script.
//
// In production both files live alongside the daemon binary.
// In development, set MTGA_COLLECTION_HELPER_DIR to the
// services/collection-agent-helper directory so GoLand / go run can find them.
func locateHelperFiles() (helperBinary, scriptDir string, err error) {
	dir := os.Getenv("MTGA_COLLECTION_HELPER_DIR")
	if dir == "" {
		exe, exeErr := os.Executable()
		if exeErr != nil {
			return "", "", exeErr
		}
		dir = filepath.Dir(exe)
	}
	helperBinary = filepath.Join(dir, "collection-helper")
	scriptDir = filepath.Join(dir, "install")
	if _, statErr := os.Stat(helperBinary); statErr != nil {
		return "", "", fmt.Errorf("collection-helper binary not found in %s (set MTGA_COLLECTION_HELPER_DIR to override): %w", dir, statErr)
	}
	if _, statErr := os.Stat(scriptDir); statErr != nil {
		return "", "", fmt.Errorf("install directory not found in %s: %w", dir, statErr)
	}
	return helperBinary, scriptDir, nil
}
