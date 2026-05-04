// Package config loads daemon configuration from a local file and environment variables.
// The cloud API URL is never hardcoded — it must be supplied via config file or environment.
package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Config holds all daemon configuration.
type Config struct {
	// filePath is the path used to load this config; used by Save to write-back.
	// Not exported and not serialised.
	filePath string

	// CloudAPIURL is the base URL of the Backend for Frontend service.
	// Required. Never hardcoded. Read from config file or MTGA_DAEMON_CLOUD_API_URL env var.
	CloudAPIURL string `json:"cloud_api_url"`

	// APIKey is the user API key used only for daemon registration.
	// After a successful registration the returned DaemonJWT is used for all ingest calls.
	// Read from config file or MTGA_DAEMON_API_KEY env var.
	APIKey string `json:"api_key"`

	// UserID is the BFF user ID associated with this daemon install.
	// Required when using JWT registration. Read from config file.
	UserID int `json:"user_id,omitempty"`

	// DaemonJWT is the JWT issued by the BFF on registration.
	// Persisted after the first successful registration; refreshed when near-expiry.
	DaemonJWT string `json:"daemon_jwt,omitempty"`

	// DaemonID is the UUID assigned by the BFF on registration.
	// Persisted after the first successful registration.
	DaemonID string `json:"daemon_id,omitempty"`

	// SyncEnabled controls whether events are dispatched to the cloud API.
	// Default: true.
	SyncEnabled bool `json:"sync_enabled"`

	// LogPath is the path to the MTGA Player.log file.
	// Optional: auto-detected from the platform if empty.
	LogPath string `json:"log_path"`

	// PollInterval is how often the poller checks for new log entries.
	// Default: 2 seconds.
	PollInterval time.Duration `json:"poll_interval"`

	// UseFSNotify enables fsnotify-based file watching instead of pure polling.
	// Default: true.
	UseFSNotify bool `json:"use_fs_notify"`

	// AccountID is the MTGA account ID used to tag events sent to BFF.
	AccountID string `json:"account_id"`

	// IngestPath is the BFF endpoint path for event ingestion.
	// Default: /v1/ingest/events.
	IngestPath string `json:"ingest_path"`

	// LogArchiveDir is the directory where log snapshots are stored.
	// Default: ~/.mtga-daemon/archives
	LogArchiveDir string `json:"log_archive_dir"`

	// LogArchiveMaxAge is how long to retain log snapshots before pruning.
	// Default: 7 days.
	LogArchiveMaxAge time.Duration `json:"log_archive_max_age"`

	// LogPreserveOnStart controls whether a snapshot of Player.log is taken
	// each time the daemon starts, before the poller begins reading.
	// Default: true.
	LogPreserveOnStart bool `json:"log_preserve_on_start"`
}

// Load reads daemon configuration. Sources in priority order:
//  1. JSON config file at path (if non-empty)
//  2. Environment variables
//  3. Defaults
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		if err := loadFile(cfg, path); err != nil {
			return nil, fmt.Errorf("load config file %q: %w", path, err)
		}
		cfg.filePath = path
	}

	applyEnv(cfg)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Save writes the current config back to the file it was loaded from.
// Returns an error if no file path is known (config was loaded from env only).
func (c *Config) Save() error {
	if c.filePath == "" {
		return fmt.Errorf("config: no file path to save to (config was not loaded from a file)")
	}
	return saveToFile(c, c.filePath)
}

// SaveTo writes the current config to path, replacing the file, and records
// path so future Save() calls reuse it.
func (c *Config) SaveTo(path string) error {
	if err := saveToFile(c, path); err != nil {
		return err
	}
	c.filePath = path
	return nil
}

// FilePath returns the path the config was loaded from (empty if env-only).
func (c *Config) FilePath() string {
	return c.filePath
}

// JWTNeedsRefresh reports whether the daemon should obtain a fresh JWT.
// Returns true when DaemonJWT is empty or expires within the next 24 hours.
// The expiry is read from the exp claim without verifying the signature.
func (c *Config) JWTNeedsRefresh() bool {
	if c.DaemonJWT == "" {
		return true
	}
	exp, err := jwtExpiry(c.DaemonJWT)
	if err != nil {
		// Malformed token — treat as expired so we re-register.
		log.Printf("[config] warn: could not parse daemon_jwt expiry: %v; will re-register", err)
		return true
	}
	return time.Until(exp) < 24*time.Hour
}

func defaults() *Config {
	archiveDir := defaultArchiveDir()
	return &Config{
		SyncEnabled:        true,
		PollInterval:       2 * time.Second,
		UseFSNotify:        true,
		IngestPath:         "/v1/ingest/events",
		LogPreserveOnStart: true,
		LogArchiveMaxAge:   7 * 24 * time.Hour,
		LogArchiveDir:      archiveDir,
	}
}

func defaultArchiveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir() + "/mtga-daemon/archives"
	}
	return home + "/.mtga-daemon/archives"
}

func loadFile(cfg *Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func saveToFile(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	// Write atomically: temp file in the same directory, then rename.
	dir := dirOf(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".daemon-cfg-*.tmp")
	if err != nil {
		return fmt.Errorf("config: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, werr := tmp.Write(data); werr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: write temp file: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: close temp file: %w", cerr)
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: rename to %q: %w", path, rerr)
	}
	return nil
}

// dirOf returns the directory portion of path, defaulting to "." for bare filenames.
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// jwtExpiry decodes the exp claim from a JWT without verifying the signature.
func jwtExpiry(token string) (time.Time, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("unmarshal claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("exp claim missing or zero")
	}
	return time.Unix(claims.Exp, 0), nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("MTGA_DAEMON_CLOUD_API_URL"); v != "" {
		cfg.CloudAPIURL = v
	}
	if v := os.Getenv("MTGA_DAEMON_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("MTGA_DAEMON_LOG_PATH"); v != "" {
		cfg.LogPath = v
	}
	if v := os.Getenv("MTGA_DAEMON_ACCOUNT_ID"); v != "" {
		cfg.AccountID = v
	}
	if v := os.Getenv("MTGA_DAEMON_LOG_ARCHIVE_DIR"); v != "" {
		cfg.LogArchiveDir = v
	}
}

func (c *Config) validate() error {
	if c.CloudAPIURL == "" {
		return fmt.Errorf("cloud_api_url is required (set MTGA_DAEMON_CLOUD_API_URL or provide config file)")
	}
	if c.SyncEnabled && c.APIKey == "" && c.DaemonJWT == "" {
		log.Printf("[config] warning: sync_enabled is true but neither api_key nor daemon_jwt is set; events will be sent without authentication")
	}
	return nil
}
