// Package config reads and validates BFF runtime configuration from environment
// variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// DefaultStalenessThresholdHours is used when
	// DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS is not set.  Two times the daily
	// Sync cadence; allows one missed run without degraded-mode signals.
	DefaultStalenessThresholdHours = 48
)

// Config holds typed runtime configuration for the BFF service.
type Config struct {
	// DraftRatingsStalenessThresholdHours is the maximum age (in hours) of the
	// draft ratings cache before the handler sets X-Cache-Degraded: true.
	// Sourced from DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS (default 48).
	DraftRatingsStalenessThresholdHours int

	// DraftRatingsBypassFreshnessCheck disables the staleness threshold check
	// entirely when true.  Intended as an operational escape hatch during Sync
	// maintenance windows.
	// Sourced from DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK (default false).
	DraftRatingsBypassFreshnessCheck bool
}

// Load reads configuration from environment variables, applies defaults, and
// returns a validated Config.  An error is returned if any value is invalid.
func Load() (*Config, error) {
	cfg := &Config{
		DraftRatingsStalenessThresholdHours: DefaultStalenessThresholdHours,
		DraftRatingsBypassFreshnessCheck:    false,
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
