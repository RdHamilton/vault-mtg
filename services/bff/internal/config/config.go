// Package config reads and validates BFF runtime configuration from environment
// variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultStalenessThresholdHours is used when
	// DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS is not set.  Two times the daily
	// Sync cadence; allows one missed run without degraded-mode signals.
	DefaultStalenessThresholdHours = 48
)

// defaultAllowedOrigins is used when ALLOWED_ORIGINS is not set.
// Preserves pre-ADR-006 behaviour for local development.
var defaultAllowedOrigins = []string{"http://localhost:*", "http://127.0.0.1:*"}

// Config holds typed runtime configuration for the BFF service.
type Config struct {
	// Env is the runtime environment.  Sourced from MTGA_ENV (default "development").
	// Recognised values: "production", "development".
	// When Env is "production", DATABASE_URL must be set or Load returns an error.
	Env string

	// DatabaseURL is the PostgreSQL connection string.
	// Sourced from DATABASE_URL.
	DatabaseURL string

	// DraftRatingsStalenessThresholdHours is the maximum age (in hours) of the
	// draft ratings cache before the handler sets X-Cache-Degraded: true.
	// Sourced from DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS (default 48).
	DraftRatingsStalenessThresholdHours int

	// DraftRatingsBypassFreshnessCheck disables the staleness threshold check
	// entirely when true.  Intended as an operational escape hatch during Sync
	// maintenance windows.
	// Sourced from DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK (default false).
	DraftRatingsBypassFreshnessCheck bool

	// AllowedOrigins is the list of HTTP origins that the CORS middleware will
	// accept on cross-origin requests from browser clients.
	//
	// Sourced from ALLOWED_ORIGINS as a comma-separated list
	// (e.g. "https://mtga-companion.vercel.app,https://*.vercel.app").
	// Defaults to localhost-only values when the variable is not set so that
	// local development requires no environment configuration.
	AllowedOrigins []string

	// DaemonLatestVersion is the semver of the most recently published daemon
	// binary.  Sourced from BFF_DAEMON_LATEST_VERSION.
	// Defaults to "0.1.0" when unset so the endpoint always returns a non-empty
	// response in development without failing startup.
	DaemonLatestVersion string

	// DaemonReleasedAt is the RFC3339 timestamp when DaemonLatestVersion was
	// published.  Sourced from BFF_DAEMON_RELEASED_AT.  Empty string when unset.
	DaemonReleasedAt string

	// ClerkSecretKey is the Clerk backend API secret key used to verify Clerk
	// session JWTs on protected browser-facing routes.
	//
	// Sourced from CLERK_SECRET_KEY.  When MTGA_ENV=production this value must
	// be non-empty or Load returns an error — without it the ClerkAuth
	// middleware cannot be constructed and protected routes would be
	// inaccessible.
	//
	// The actual secret value must be stored in AWS SSM Parameter Store and
	// injected as an environment variable at deploy time.  See
	// infrastructure/ssm/parameters.md for the parameter path.
	ClerkSecretKey string
}

// Load reads configuration from environment variables, applies defaults, and
// returns a validated Config.  An error is returned if any value is invalid.
//
// Production mode (MTGA_ENV=production) requires DATABASE_URL to be set.
// Omitting DATABASE_URL in production is a fatal misconfiguration because the
// API key auth middleware cannot be constructed without a database, which would
// leave the SSE endpoint and any other guarded route unprotected.
func Load() (*Config, error) {
	env := os.Getenv("MTGA_ENV")
	if env == "" {
		env = "development"
	}

	dbURL := os.Getenv("DATABASE_URL")

	if env == "production" && dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be set when MTGA_ENV=production")
	}

	clerkSecretKey := strings.TrimSpace(os.Getenv("CLERK_SECRET_KEY"))

	if env == "production" && clerkSecretKey == "" {
		return nil, fmt.Errorf("CLERK_SECRET_KEY must be set when MTGA_ENV=production")
	}

	allowedOrigins := defaultAllowedOrigins
	if raw := os.Getenv("ALLOWED_ORIGINS"); raw != "" {
		parts := strings.Split(raw, ",")
		origins := make([]string, 0, len(parts))
		for _, p := range parts {
			if o := strings.TrimSpace(p); o != "" {
				origins = append(origins, o)
			}
		}
		if len(origins) > 0 {
			allowedOrigins = origins
		}
	}

	daemonLatestVersion := os.Getenv("BFF_DAEMON_LATEST_VERSION")
	if daemonLatestVersion == "" {
		daemonLatestVersion = "0.1.0"
	}

	cfg := &Config{
		Env:                                 env,
		DatabaseURL:                         dbURL,
		DraftRatingsStalenessThresholdHours: DefaultStalenessThresholdHours,
		DraftRatingsBypassFreshnessCheck:    false,
		AllowedOrigins:                      allowedOrigins,
		DaemonLatestVersion:                 daemonLatestVersion,
		DaemonReleasedAt:                    os.Getenv("BFF_DAEMON_RELEASED_AT"),
		ClerkSecretKey:                      clerkSecretKey,
	}

	if raw := os.Getenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS: %w", err)
		}

		if v <= 0 {
			return nil, fmt.Errorf("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS must be > 0, got %d", v)
		}

		cfg.DraftRatingsStalenessThresholdHours = v
	}

	if raw := os.Getenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK: %w", err)
		}

		cfg.DraftRatingsBypassFreshnessCheck = v
	}

	return cfg, nil
}
