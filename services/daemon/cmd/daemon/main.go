// Command mtga-daemon watches MTGA Player.log and forwards events to the BFF.
// Configuration is loaded from a JSON file (default: %APPDATA%\vaultmtg\daemon.json
// on Windows; ~/.vaultmtg/daemon.json on macOS/Linux) and can be overridden with
// environment variables. The cloud API URL is never hardcoded.
//
// Environment variables (ADR-022 Phase 2 dual-read shim: VAULTMTG_DAEMON_* wins
// when both are set; MTGA_DAEMON_* is the legacy fallback for existing service installs):
//
//	VAULTMTG_DAEMON_CLOUD_API_URL  Base URL of the cloud API / BFF (required if not in config file)
//	MTGA_DAEMON_CLOUD_API_URL      Legacy alias (fallback)
//	VAULTMTG_DAEMON_API_KEY        Bearer token for BFF authentication (legacy plaintext — migrated to keychain)
//	MTGA_DAEMON_API_KEY            Legacy alias (fallback)
//	VAULTMTG_DAEMON_LOG_PATH       Override MTGA log file path (auto-detected by default)
//	MTGA_DAEMON_LOG_PATH           Legacy alias (fallback)
//	VAULTMTG_DAEMON_ACCOUNT_ID     MTGA account ID to tag events
//	MTGA_DAEMON_ACCOUNT_ID         Legacy alias (fallback)
//	VAULTMTG_DAEMON_HEADLESS       Set to "1" to skip browser open and print the auth URL instead
//	MTGA_DAEMON_HEADLESS           Legacy alias (fallback)
//	MTGA_COLLECTION_HELPER_DIR     Directory containing collection-helper binary and install/ subdir (dev override)
//	CLERK_PUBLISHABLE_KEY          Clerk publishable key (pk_live_* / pk_test_*) used for PKCE OAuth
//	CLERK_FRONTEND_API             Clerk frontend API base URL (e.g. https://accounts.clerk.dev)
//
// Flags:
//
//	-config <path>   Path to JSON config file
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/daemon"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/keychain"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/migrate"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/pkce"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/sentryhook"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/tray"

	"github.com/getsentry/sentry-go"
)

// Version is the build-time version string injected via -ldflags -X main.Version=<ver>.
// Defaults to "dev" for local builds.
var Version = "dev"

// DefaultCloudAPIURL is the build-time default for cloud_api_url, injected via
// -ldflags -X main.DefaultCloudAPIURL=<url>. The release workflow picks the value:
//
//	stable tags (daemon/v0.3.1) -> https://api.vaultmtg.app/api/v1
//	pre-release tags (-rc/-alpha/-beta/-pre) -> https://staging-api.vaultmtg.app/api/v1
//
// Local builds (`go run`, `go build` without -ldflags) get the localhost default
// so a developer running the daemon directly from source talks to a local BFF,
// not production. Issue #2560.
var DefaultCloudAPIURL = "http://localhost:8080/api/v1"

// DefaultSentryDSN is the build-time Sentry DSN, injected via
// -ldflags -X main.DefaultSentryDSN=<dsn>. The release workflow picks the value
// from secrets.SENTRY_DSN_DAEMON_PRODUCTION / SENTRY_DSN_DAEMON_STAGING based
// on the tag (mirrors DefaultCloudAPIURL). Empty value disables Sentry — all
// SDK calls become safe no-ops (used by `go run`, local `go build`, and any
// snapshot build). The DSN itself is never logged. Issue #1832.
var DefaultSentryDSN = ""

func main() {
	defaultCfgPath := defaultConfigPath()
	cfgPath := flag.String("config", defaultCfgPath, "path to JSON config file")
	flag.Parse()

	// ── Step 0: config-dir migration (ADR-022 Phase 2) ─────────────────────────
	// Copy old brand directories to the new VaultMTG-namespaced paths so users
	// retain their configuration after upgrading the daemon binary.
	// This is a copy-not-move: the old directories are retained for downgrade safety.
	// The migration is idempotent and a no-op on fresh installs.
	runConfigDirMigration()

	// ── Step 1: first-run config detection ─────────────────────────────────────
	// If daemon.json is missing, write a stub with cloud_api_url (if supplied via
	// env) and open the setup URL in the browser (or print it on headless).
	// The PKCE flow is then initiated so the user authenticates before the daemon starts.
	cfgExists := fileExists(*cfgPath)
	if !cfgExists {
		handleMissingConfig(*cfgPath)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		// Config file may be a stub with no cloud_api_url — tolerate this
		// if we are about to run PKCE and will write the real config afterward.
		// For now exit on hard errors.
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	log.Printf("[mtga-daemon] version=%s default_cloud_api_url=%s", Version, DefaultCloudAPIURL)

	// ── Step 1b: Sentry init (#1832) ───────────────────────────────────────────
	// Boot before any goroutine starts so panics in setup steps are captured.
	// When DefaultSentryDSN is empty (local build, snapshot, dev), Init returns
	// sentryhook.ErrDisabled and SDK calls become no-ops — safe to leave the
	// downstream code unconditional.
	if err := sentryhook.Init(DefaultSentryDSN, Version, cfg.CloudAPIURL); err != nil {
		if errors.Is(err, sentryhook.ErrDisabled) {
			log.Printf("[mtga-daemon] Sentry disabled (no DSN baked in — local or snapshot build)")
		} else {
			log.Printf("[mtga-daemon] warn: sentry init failed: %v", err)
		}
	} else {
		log.Printf("[mtga-daemon] Sentry initialised (release=%s)", Version)
	}
	// Flush on graceful exit so in-flight events do not drop. Mirrors the BFF
	// pattern (services/bff/cmd/main.go). 2s timeout matches sentryhook.FlushTimeout.
	defer sentryhook.Flush()
	// Top-level panic safety net: any panic that escapes the goroutines below
	// is captured and re-raised so the launchd / NSSM restart loop still kicks in.
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentryhook.Flush()
			panic(r)
		}
	}()

	// ── Step 2: keychain migration (legacy plaintext api_key → OS keychain) ────
	if err := migrateLegacyAPIKey(cfg); err != nil {
		log.Printf("[mtga-daemon] warn: keychain migration failed: %v", err)
	}

	// ── Step 3: PKCE auth flow if no valid credentials ─────────────────────────
	// Runs when: no keychain sentinel, no plaintext key, no daemon JWT, OR when
	// the keychain sentinel is set but the OS keychain entry is missing/empty
	// (reinstall scenario — OS keychain was wiped after a full reinstall).
	if cfg.NeedsFirstRunAuth(keychain.Get) && cfg.CloudAPIURL != "" {
		log.Printf("[mtga-daemon] first-run: no API key detected — starting PKCE auth flow")
		if err := runPKCEAuth(cfg, *cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "auth error: %v\n", err)
			// Signal launchd that this exit is intentional so KeepAlive=true in
			// the plist does not respawn the process and trigger PKCE again every
			// ~10 seconds (ThrottleInterval). os.Exit bypasses defers, so we call
			// stopLaunchAgent explicitly here before exiting.
			stopLaunchAgent()
			os.Exit(1)
		}
	}

	// Attach the cached account_id as Sentry user context on every boot. On
	// the first run this is a no-op (cfg.AccountID is empty until PKCE runs);
	// runPKCEAuth also calls SetUser after registration. On subsequent runs
	// this is the only call site that fires. Issue #1832.
	sentryhook.SetUser(cfg.AccountID)

	ctx, cancel := context.WithCancel(context.Background())

	svc := daemon.New(cfg)
	svc.WithVersion(Version)
	log.Printf("[mtga-daemon] starting, cloud_api=%s", cfg.CloudAPIURL)

	// systray.Run must own the main OS thread (macOS Cocoa requirement).
	// onReady starts the daemon service in a goroutine; onQuit cancels the context.
	app := tray.New("https://app.vaultmtg.app", pkce.OpenBrowser, func() {
		// Tell launchd the stop was intentional so it does not immediately
		// respawn the process per KeepAlive=true in the plist. On non-macOS
		// platforms stopLaunchAgent is a no-op.
		stopLaunchAgent()
		cancel()
	})

	svc.WithTray(daemon.TrayHooks{
		SyncNow:            app.SyncNow,
		GrantAccess:        app.GrantAccess,
		TryAgain:           app.TryAgain,
		SetHelperInstalled: app.SetHelperInstalled,
		SetLastSync:        app.SetLastSync,
		SetKeychainError:   app.SetKeychainError,
	})

	// Handle OS signals: forward SIGTERM/SIGINT to systray so onQuit fires cleanly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		app.Quit()
	}()

	// headless is true when the daemon is running without a display / tray
	// (e.g. CI, server, or a user-invoked terminal session with VAULTMTG_DAEMON_HEADLESS=1).
	// Evaluated once here so the Run error handler can branch without re-reading the env.
	headless := config.EnvWithFallback("VAULTMTG_DAEMON_HEADLESS", "MTGA_DAEMON_HEADLESS") == "1"

	app.Run(func() {
		app.SetStatus(tray.StatusConnected)
		go func() {
			if err := svc.Run(ctx); err != nil {
				if headless {
					// REV-2: headless path — log the canonical FATAL line and exit
					// non-zero so the supervisor (launchd / systemd) respawns.
					// NeedsFirstRunAuth will trigger PKCE on the next boot.
					log.Println("[daemon] FATAL: keychain unavailable after retries — exiting")
					os.Exit(1)
				}
				log.Printf("[mtga-daemon] fatal: %v", err)
				app.Quit()
			}
		}()
	})
}

// handleMissingConfig is called when no daemon.json exists (first install).
// It prints (or opens) the setup URL so the user knows where to go.
// A stub config is NOT written here — the full config is written after PKCE completes.
func handleMissingConfig(cfgPath string) {
	setupURL := "https://vaultmtg.app/setup"
	headless := config.EnvWithFallback("VAULTMTG_DAEMON_HEADLESS", "MTGA_DAEMON_HEADLESS") == "1"

	if headless {
		fmt.Printf("[mtga-daemon] First run: open %s to complete setup.\n", setupURL)
	} else {
		fmt.Printf("[mtga-daemon] First run: opening %s in your browser...\n", setupURL)
		if err := pkce.OpenBrowser(setupURL); err != nil {
			log.Printf("[mtga-daemon] warn: could not open browser: %v", err)
			fmt.Printf("[mtga-daemon] Please open: %s\n", setupURL)
		}
	}

	// Write a minimal stub so config.Load succeeds even without env vars.
	// The PKCE flow will fill in the real values.
	//
	// Resolution order for the stub cloud_api_url:
	//   1. VAULTMTG_DAEMON_CLOUD_API_URL env var (set by the postinstall plist on
	//      packaged installs).
	//   2. MTGA_DAEMON_CLOUD_API_URL env var (legacy fallback per ADR-022 Phase 2
	//      dual-read shim).
	//   3. main.DefaultCloudAPIURL — injected via -ldflags at build time. Stable
	//      releases get production; pre-release tags get staging; raw `go build`
	//      gets http://localhost:8080/api/v1 (Issue #2560).
	cloudAPIURL := config.EnvWithFallback("VAULTMTG_DAEMON_CLOUD_API_URL", "MTGA_DAEMON_CLOUD_API_URL")
	if cloudAPIURL == "" {
		cloudAPIURL = DefaultCloudAPIURL
	}

	stub := map[string]interface{}{
		"cloud_api_url": cloudAPIURL,
	}
	data, _ := json.MarshalIndent(stub, "", "  ")
	dir := filepath.Dir(cfgPath)
	if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
		log.Printf("[mtga-daemon] warn: could not create config dir %q: %v", dir, mkdirErr)
		return
	}
	if writeErr := os.WriteFile(cfgPath, data, 0o600); writeErr != nil {
		log.Printf("[mtga-daemon] warn: could not write stub config to %q: %v", cfgPath, writeErr)
	}
}

// migrateLegacyAPIKey detects a plaintext api_key in the config file and migrates
// it to the OS keychain, rewriting daemon.json with keychain:true.
// This is a one-time, transparent upgrade per ADR-020 §Migration path.
func migrateLegacyAPIKey(cfg *config.Config) error {
	if cfg.Keychain || cfg.APIKey == "" || cfg.FilePath() == "" {
		return nil // nothing to migrate
	}

	log.Printf("[mtga-daemon] migrating plaintext api_key to OS keychain")

	if err := keychain.Set(cfg.APIKey); err != nil {
		return fmt.Errorf("write to keychain: %w", err)
	}

	cfg.APIKey = ""
	cfg.Keychain = true

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config after keychain migration: %w", err)
	}

	log.Printf("[mtga-daemon] api_key migrated to OS keychain; daemon.json updated")
	return nil
}

// runPKCEAuth executes the PKCE browser-redirect flow and:
//  1. Obtains a Clerk session JWT.
//  2. Calls POST /v1/daemon/register on the BFF to mint an API key.
//  3. On fresh registration (201 Created + non-empty api_key): stores the key
//     in the OS keychain and writes daemon.json with keychain:true.
//  4. On already-registered (200 OK + empty api_key): verifies the existing
//     keychain entry is still present and writes daemon.json without touching
//     the keychain.
//  5. On already-registered + keychain miss: calls DELETE /api/v1/daemons/{device_id}
//     to revoke the stale row, then re-registers with an empty device_id so the
//     BFF mints a fresh identity (ADR-034 §3, ADR-036 I-3). One attempt only;
//     if recovery fails, returns StatusSetupRequired and exits so launchd respawns.
func runPKCEAuth(cfg *config.Config, cfgPath string) error {
	clerkFrontendAPI := os.Getenv("CLERK_FRONTEND_API")
	clientID := os.Getenv("CLERK_OAUTH_CLIENT_ID")
	if clerkFrontendAPI == "" || clientID == "" {
		return fmt.Errorf("CLERK_FRONTEND_API and CLERK_OAUTH_CLIENT_ID must be set for PKCE auth")
	}

	headless := config.EnvWithFallback("VAULTMTG_DAEMON_HEADLESS", "MTGA_DAEMON_HEADLESS") == "1"

	tokenEndpoint := strings.TrimRight(clerkFrontendAPI, "/") + "/oauth/token"

	pkceCfg := pkce.Config{
		ClerkFrontendAPI: clerkFrontendAPI,
		ClientID:         clientID,
		TokenEndpoint:    tokenEndpoint,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Printf("[mtga-daemon] PKCE: opening browser for Clerk authentication")
	tok, err := pkce.Run(ctx, pkceCfg, headless)
	if err != nil {
		return fmt.Errorf("pkce flow: %w", err)
	}

	log.Printf("[mtga-daemon] PKCE: auth code received; registering with BFF")

	// Per ADR-028: the BFF is the source of truth for device_id.
	// Pass cfg.DaemonID as-is — empty on first install, cached value on
	// subsequent runs. The BFF mints a fresh UUIDv4 when it receives empty
	// and echoes the authoritative value back in the response.
	apiKey, accountID, serverDeviceID, alreadyRegistered, err := registerWithBFF(ctx, cfg.CloudAPIURL, tok.AccessToken, cfg.DaemonID, runtime.GOOS, Version)
	if err != nil {
		return fmt.Errorf("BFF registration: %w", err)
	}

	if alreadyRegistered {
		// BFF returned HTTP 200 + empty api_key: the device was already registered.
		// The API key is still in the OS keychain from the original install — do not
		// overwrite it. Just verify it is still there (OS keychain could have been
		// wiped by an OS reinstall even though daemon.json survived).
		log.Printf("[mtga-daemon] device already registered; using existing keychain key")

		existing, kcErr := keychain.Get()
		if kcErr == nil && existing != "" {
			// Keychain entry is intact. Write/refresh daemon.json with the account_id
			// and the BFF-authoritative device_id (ADR-028: daemon always persists the
			// server-echoed value, even when it matches the cached value — idempotent).
			cfg.Keychain = true
			cfg.APIKey = ""
			cfg.AccountID = accountID
			cfg.DaemonID = serverDeviceID

			if err := cfg.SaveTo(cfgPath); err != nil {
				return fmt.Errorf("write daemon.json: %w", err)
			}

			// Attach hashed account_id as Sentry user context (#1832).
			sentryhook.SetUser(accountID)
			log.Printf("[mtga-daemon] already-registered device — daemon.json refreshed, keychain untouched")
			return nil
		}

		// Keychain entry is missing (OS keychain wiped after reinstall).
		// Recovery path (ADR-034 §3, ADR-036 I-3):
		//   1. Revoke the stale BFF row via DELETE /api/v1/daemons/{device_id}.
		//   2. Re-register with an empty device_id — BFF mints a fresh identity.
		// One attempt only. Failure exits with StatusSetupRequired so launchd respawns.
		log.Printf("[mtga-daemon] keychain entry missing for registered device %s; attempting recovery", serverDeviceID)

		if delErr := revokeFromBFF(ctx, cfg.CloudAPIURL, tok.AccessToken, serverDeviceID); delErr != nil {
			log.Printf("[mtga-daemon] recovery: DELETE /api/v1/daemons/%s failed: %v; entering setup-required state", serverDeviceID, delErr)
			return fmt.Errorf("re-register recovery: revoke stale device: %w", delErr)
		}
		log.Printf("[mtga-daemon] recovery: stale device %s revoked; re-registering with empty device_id", serverDeviceID)

		// Clear the stale device_id so registerWithBFF sends "" and the BFF mints fresh.
		cfg.DaemonID = ""
		newAPIKey, newAccountID, newDeviceID, _, regErr := registerWithBFF(ctx, cfg.CloudAPIURL, tok.AccessToken, "", runtime.GOOS, Version)
		if regErr != nil {
			log.Printf("[mtga-daemon] recovery: re-registration failed: %v; entering setup-required state", regErr)
			return fmt.Errorf("re-register recovery: re-registration failed: %w", regErr)
		}

		log.Printf("[mtga-daemon] recovery: re-registered as device %s (account %s)", newDeviceID, newAccountID)
		if err := keychain.Set(newAPIKey); err != nil {
			return fmt.Errorf("re-register recovery: store new API key in keychain: %w", err)
		}

		cfg.Keychain = true
		cfg.APIKey = ""
		cfg.AccountID = newAccountID
		cfg.DaemonID = newDeviceID

		if err := cfg.SaveTo(cfgPath); err != nil {
			return fmt.Errorf("re-register recovery: write daemon.json: %w", err)
		}

		// Attach hashed account_id as Sentry user context (#1832).
		sentryhook.SetUser(newAccountID)
		log.Printf("[mtga-daemon] recovery complete — new device_id=%s written to daemon.json", newDeviceID)
		return nil
	}

	// Fresh registration (201 Created + non-empty api_key).
	log.Printf("[mtga-daemon] BFF registered (account_id=%s); storing key in OS keychain", accountID)
	if err := keychain.Set(apiKey); err != nil {
		return fmt.Errorf("store API key in keychain: %w", err)
	}

	// Write daemon.json with keychain:true, account_id, and the server-issued
	// device_id per ADR-028 §"Implementation Notes" item 2.
	cfg.Keychain = true
	cfg.APIKey = ""
	cfg.AccountID = accountID
	cfg.DaemonID = serverDeviceID

	if err := cfg.SaveTo(cfgPath); err != nil {
		return fmt.Errorf("write daemon.json: %w", err)
	}

	// Attach the (hashed) account_id as Sentry user context so events from
	// post-auth code paths are searchable per user without storing PII.
	// Mirrors the BFF pattern (hashAccountID in posthog.go). The daemon does
	// not see the raw Clerk user_id; account_id is the stable identifier the
	// daemon does see. Issue #1832.
	sentryhook.SetUser(accountID)

	log.Printf("[mtga-daemon] first-run auth complete — daemon.json written, key in OS keychain")
	return nil
}

// registerWithBFF calls POST /daemon/register (relative to the configured
// cloud_api_url, which already includes the /api/v1 prefix) with the Clerk JWT
// and returns the minted API key, account_id, the server-authoritative device_id,
// and whether the device was already registered.
//
// alreadyRegistered is true when the BFF returns HTTP 200 with an empty
// api_key field, meaning the device_id is already known to the BFF and the
// caller should reuse the existing OS keychain entry rather than storing a
// new key. On a fresh registration the BFF returns HTTP 201 with a non-empty
// api_key.
//
// deviceID may be empty on first install — the BFF will mint a fresh UUIDv4
// per ADR-028 and echo it back in the response. The returned serverDeviceID
// must be persisted to cfg.DaemonID by the caller before cfg.SaveTo.
//
// platform is runtime.GOOS, and daemonVer is the build-time version string —
// both are required by the BFF handler.
func registerWithBFF(ctx context.Context, bffBaseURL, clerkJWT, deviceID, platform, daemonVer string) (apiKey, accountID, serverDeviceID string, alreadyRegistered bool, err error) {
	url := strings.TrimRight(bffBaseURL, "/") + "/daemon/register"

	body, err := json.Marshal(map[string]string{
		"device_id":  deviceID,
		"platform":   platform,
		"daemon_ver": daemonVer,
	})
	if err != nil {
		return "", "", "", false, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", "", false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+clerkJWT)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", false, fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", false, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", "", false, fmt.Errorf("BFF returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		APIKey    string `json:"api_key"`
		AccountID string `json:"account_id"`
		DeviceID  string `json:"device_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", "", false, fmt.Errorf("decode response: %w", err)
	}

	// HTTP 200 + empty api_key means the BFF already has this device_id on file.
	// Signal the caller to reuse the existing OS keychain entry rather than
	// treating this as an error — previously this caused os.Exit(1) and a
	// launchd respawn loop every 10 s (Issue #2169).
	if resp.StatusCode == http.StatusOK && result.APIKey == "" {
		return "", result.AccountID, result.DeviceID, true, nil
	}

	return result.APIKey, result.AccountID, result.DeviceID, false, nil
}

// revokeFromBFF calls DELETE /api/v1/daemons/{deviceID} on the BFF using the
// supplied Clerk JWT as the bearer token. Returns nil on 204, an error on any
// other status or transport failure. Used by the keychain-miss recovery path in
// runPKCEAuth (ADR-034 §3).
func revokeFromBFF(ctx context.Context, bffBaseURL, clerkJWT, deviceID string) error {
	url := strings.TrimRight(bffBaseURL, "/") + "/daemons/" + deviceID

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+clerkJWT)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("BFF returned %d: %s", resp.StatusCode, string(body))
}

// fileExists returns true when path exists and is readable.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// defaultConfigPath returns the platform-appropriate default config path:
//   - Windows: %APPDATA%\vaultmtg\daemon.json
//   - macOS/Linux: ~/.vaultmtg/daemon.json
//
// The -config flag overrides this; Task Scheduler on Windows always passes
// -config explicitly, so the default is only used when running the binary
// directly without that flag.
func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "vaultmtg", "daemon.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "daemon.json"
	}
	return filepath.Join(home, ".vaultmtg", "daemon.json")
}

// runConfigDirMigration copies old brand-namespaced config directories to the
// new VaultMTG-namespaced paths on daemon startup (ADR-022 Phase 2).
//
// Old directories migrated:
//   - ~/.mtga-companion  (or %APPDATA%\mtga-companion on Windows) → new config root
//   - ~/.mtga-daemon     (or %APPDATA%\mtga-daemon on Windows)    → new config root
//
// Each migration is a copy-not-move: the old directories are retained so that
// users who downgrade the daemon binary still work. Deletion of the old
// directories is deferred to Phase 6, gated on uptake telemetry.
func runConfigDirMigration() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[mtga-daemon] warn: config-dir migration skipped: could not resolve home dir: %v", err)
		return
	}

	var oldDirs []string
	var newDir string

	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			log.Printf("[mtga-daemon] warn: config-dir migration skipped: APPDATA not set")
			return
		}
		oldDirs = []string{
			filepath.Join(appdata, "mtga-companion"),
			filepath.Join(appdata, "mtga-daemon"),
		}
		newDir = filepath.Join(appdata, "vaultmtg")
	} else {
		oldDirs = []string{
			filepath.Join(home, ".mtga-companion"),
			filepath.Join(home, ".mtga-daemon"),
		}
		newDir = filepath.Join(home, ".vaultmtg")
	}

	for _, oldDir := range oldDirs {
		if err := migrate.MigrateConfigDir(oldDir, newDir); err != nil {
			log.Printf("[mtga-daemon] warn: config-dir migration %q → %q failed: %v", oldDir, newDir, err)
		}
	}
}
