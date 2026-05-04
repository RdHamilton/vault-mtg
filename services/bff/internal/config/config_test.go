package config_test

import (
	"testing"

	"github.com/ramonehamilton/mtga-bff/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Env != "development" {
		t.Errorf("expected default env 'development', got %q", cfg.Env)
	}

	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty DatabaseURL, got %q", cfg.DatabaseURL)
	}

	if cfg.DraftRatingsStalenessThresholdHours != config.DefaultStalenessThresholdHours {
		t.Errorf("expected default threshold %d, got %d",
			config.DefaultStalenessThresholdHours, cfg.DraftRatingsStalenessThresholdHours)
	}

	if cfg.DraftRatingsBypassFreshnessCheck {
		t.Error("expected bypass to default to false")
	}
}

// TestLoad_Env_Development_NoDatabaseURL_OK verifies that development mode
// does not require DATABASE_URL.
func TestLoad_Env_Development_NoDatabaseURL_OK(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("development mode without DATABASE_URL should not error: %v", err)
	}

	if cfg.Env != "development" {
		t.Errorf("expected env 'development', got %q", cfg.Env)
	}
}

// TestLoad_Env_Production_NoDatabaseURL_Error verifies that starting in
// production mode without DATABASE_URL returns an error (fail-fast guard).
func TestLoad_Env_Production_NoDatabaseURL_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=production and DATABASE_URL is unset")
	}
}

// TestLoad_Env_Production_WithDatabaseURL_OK verifies that production mode
// succeeds when DATABASE_URL is present.
func TestLoad_Env_Production_WithDatabaseURL_OK(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("production mode with DATABASE_URL should not error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DatabaseURL 'postgres://localhost/test', got %q", cfg.DatabaseURL)
	}
}

// TestLoad_DatabaseURL_StoredInConfig verifies the DATABASE_URL env var is
// surfaced as Config.DatabaseURL for callers.
func TestLoad_DatabaseURL_StoredInConfig(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://user:pass@host/db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://user:pass@host/db" {
		t.Errorf("expected DatabaseURL 'postgres://user:pass@host/db', got %q", cfg.DatabaseURL)
	}
}

func TestLoad_StalenessThreshold_ValidPositive(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "72")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DraftRatingsStalenessThresholdHours != 72 {
		t.Errorf("expected 72, got %d", cfg.DraftRatingsStalenessThresholdHours)
	}
}

func TestLoad_StalenessThreshold_ZeroIsInvalid(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "0")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for threshold = 0, got nil")
	}
}

func TestLoad_StalenessThreshold_NegativeIsInvalid(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "-1")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for negative threshold, got nil")
	}
}

func TestLoad_StalenessThreshold_NonIntegerIsInvalid(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "abc")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for non-integer threshold, got nil")
	}
}

func TestLoad_BypassFreshnessCheck_True(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.DraftRatingsBypassFreshnessCheck {
		t.Error("expected bypass to be true")
	}
}

func TestLoad_BypassFreshnessCheck_False(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DraftRatingsBypassFreshnessCheck {
		t.Error("expected bypass to be false")
	}
}

func TestLoad_BypassFreshnessCheck_InvalidIsError(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "yes")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid bypass value, got nil")
	}
}
