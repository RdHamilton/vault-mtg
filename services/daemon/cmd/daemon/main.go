// Command mtga-daemon watches MTGA Player.log and forwards events to the BFF.
// Configuration is loaded from a JSON file (default: %APPDATA%\mtga-companion\daemon.json
// on Windows; ~/.mtga-companion/daemon.json on macOS/Linux) and can be overridden with
// environment variables. The cloud API URL is never hardcoded.
//
// Environment variables:
//
//	MTGA_DAEMON_CLOUD_API_URL     Base URL of the cloud API / BFF (required if not in config file)
//	MTGA_DAEMON_API_KEY           Bearer token for BFF authentication (legacy plaintext — migrated to keychain)
//	MTGA_DAEMON_LOG_PATH          Override MTGA log file path (auto-detected by default)
//	MTGA_DAEMON_ACCOUNT_ID        MTGA account ID to tag events
//	MTGA_DAEMON_HEADLESS          Set to "1" to skip browser open and print the auth URL instead
//	CLERK_PUBLISHABLE_KEY         Clerk publishable key (pk_live_* / pk_test_*) used for PKCE OAuth
//	CLERK_FRONTEND_API            Clerk frontend API base URL (e.g. https://accounts.clerk.dev)
//
// Flags:
//
//	-config <path>   Path to JSON config file
package main

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/google/uuid"
	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/ramonehamilton/mtga-daemon/internal/daemon"
	"github.com/ramonehamilton/mtga-daemon/internal/keychain"
	"github.com/ramonehamilton/mtga-daemon/internal/pkce"
)

// Version is the build-time version string injected via -ldflags -X main.Version=<ver>.
// Defaults to "dev" for local builds.
var Version = "dev"

func main() {
	defaultCfgPath := defaultConfigPath()
	cfgPath := flag.String("config", defaultCfgPath, "path to JSON config file")
	flag.Parse()

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

	log.Printf("[mtga-daemon] version=%s", Version)

	// ── Step 2: keychain migration (legacy plaintext api_key → OS keychain) ────
	if err := migrateLegacyAPIKey(cfg); err != nil {
		log.Printf("[mtga-daemon] warn: keychain migration failed: %v", err)
	}

	// ── Step 3: PKCE auth flow if no valid credentials ─────────────────────────
	// Runs when: no keychain sentinel, no plaintext key, no daemon JWT.
	if cfg.NeedsFirstRunAuth() && cfg.CloudAPIURL != "" {
		log.Printf("[mtga-daemon] first-run: no API key detected — starting PKCE auth flow")
		if err := runPKCEAuth(cfg, *cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "auth error: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	svc := daemon.New(cfg)
	svc.WithVersion(Version)
	log.Printf("[mtga-daemon] starting, cloud_api=%s", cfg.CloudAPIURL)

	if err := svc.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// handleMissingConfig is called when no daemon.json exists (first install).
// It prints (or opens) the setup URL so the user knows where to go.
// A stub config is NOT written here — the full config is written after PKCE completes.
func handleMissingConfig(cfgPath string) {
	setupURL := "https://vaultmtg.app/setup"
	headless := os.Getenv("MTGA_DAEMON_HEADLESS") == "1"

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
	cloudAPIURL := os.Getenv("MTGA_DAEMON_CLOUD_API_URL")
	if cloudAPIURL == "" {
		cloudAPIURL = "https://api.vaultmtg.app"
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
//  3. Stores the API key in the OS keychain.
//  4. Writes daemon.json with keychain:true.
func runPKCEAuth(cfg *config.Config, cfgPath string) error {
	clerkFrontendAPI := os.Getenv("CLERK_FRONTEND_API")
	clientID := os.Getenv("CLERK_OAUTH_CLIENT_ID")
	if clerkFrontendAPI == "" || clientID == "" {
		return fmt.Errorf("CLERK_FRONTEND_API and CLERK_OAUTH_CLIENT_ID must be set for PKCE auth")
	}

	headless := os.Getenv("MTGA_DAEMON_HEADLESS") == "1"

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

	// device_id is a stable per-installation UUID. Generate one on first run
	// and persist it via daemon.json so re-registrations reuse the same id.
	if cfg.DaemonID == "" {
		cfg.DaemonID = uuid.NewString()
	}

	apiKey, accountID, err := registerWithBFF(ctx, cfg.CloudAPIURL, tok.AccessToken, cfg.DaemonID, runtime.GOOS, Version)
	if err != nil {
		return fmt.Errorf("BFF registration: %w", err)
	}

	log.Printf("[mtga-daemon] BFF registered (account_id=%s); storing key in OS keychain", accountID)
	if err := keychain.Set(apiKey); err != nil {
		return fmt.Errorf("store API key in keychain: %w", err)
	}

	// Write daemon.json with keychain:true and account_id.
	cfg.Keychain = true
	cfg.APIKey = ""
	cfg.AccountID = accountID

	if err := cfg.SaveTo(cfgPath); err != nil {
		return fmt.Errorf("write daemon.json: %w", err)
	}

	log.Printf("[mtga-daemon] first-run auth complete — daemon.json written, key in OS keychain")
	return nil
}

// registerWithBFF calls POST /daemon/register (relative to the configured
// cloud_api_url, which already includes the /api/v1 prefix) with the Clerk JWT
// and returns the minted API key and account_id.
//
// deviceID is a per-installation UUID, platform is runtime.GOOS, and daemonVer
// is the build-time version string — all three are required by the BFF handler.
func registerWithBFF(ctx context.Context, bffBaseURL, clerkJWT, deviceID, platform, daemonVer string) (apiKey, accountID string, err error) {
	url := strings.TrimRight(bffBaseURL, "/") + "/daemon/register"

	body, err := json.Marshal(map[string]string{
		"device_id":  deviceID,
		"platform":   platform,
		"daemon_ver": daemonVer,
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+clerkJWT)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("BFF returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		APIKey    string `json:"api_key"`
		AccountID string `json:"account_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	if result.APIKey == "" {
		return "", "", fmt.Errorf("BFF returned empty api_key (reinstall? use existing key from keychain)")
	}

	return result.APIKey, result.AccountID, nil
}

// fileExists returns true when path exists and is readable.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// defaultConfigPath returns the platform-appropriate default config path:
//   - Windows: %APPDATA%\mtga-companion\daemon.json
//   - macOS/Linux: ~/.mtga-companion/daemon.json
//
// The -config flag overrides this; Task Scheduler on Windows always passes
// -config explicitly, so the default is only used when running the binary
// directly without that flag.
func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "mtga-companion", "daemon.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "daemon.json"
	}
	return filepath.Join(home, ".mtga-companion", "daemon.json")
}
