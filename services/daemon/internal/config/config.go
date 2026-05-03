// Package config loads daemon configuration from a local file and environment variables.
// The cloud API URL is never hardcoded — it must be supplied via config file or environment.
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Config holds all daemon configuration.
type Config struct {
	// CloudAPIURL is the base URL of the Backend for Frontend service.
	// Required. Never hardcoded. Read from config file or MTGA_DAEMON_CLOUD_API_URL env var.
	CloudAPIURL string `json:"cloud_api_url"`

	// APIKey is the bearer token used to authenticate with the BFF.
	// Read from config file or MTGA_DAEMON_API_KEY env var.
	APIKey string `json:"api_key"`

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
	}

	applyEnv(cfg)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
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
	if c.SyncEnabled && c.APIKey == "" {
		log.Printf("[config] warning: sync_enabled is true but api_key is not set; events will be sent without authentication")
	}
	return nil
}
