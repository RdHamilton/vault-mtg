package config_test

import (
	"testing"

	"github.com/ramonehamilton/mtga-bff/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DraftRatingsStalenessThresholdHours != config.DefaultStalenessThresholdHours {
		t.Errorf("expected default threshold %d, got %d",
			config.DefaultStalenessThresholdHours, cfg.DraftRatingsStalenessThresholdHours)
	}

	if cfg.DraftRatingsBypassFreshnessCheck {
		t.Error("expected bypass to default to false")
	}
}

func TestLoad_StalenessThreshold_ValidPositive(t *testing.T) {
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
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "0")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for threshold = 0, got nil")
	}
}

func TestLoad_StalenessThreshold_NegativeIsInvalid(t *testing.T) {
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "-1")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for negative threshold, got nil")
	}
}

func TestLoad_StalenessThreshold_NonIntegerIsInvalid(t *testing.T) {
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "abc")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for non-integer threshold, got nil")
	}
}

func TestLoad_BypassFreshnessCheck_True(t *testing.T) {
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
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "yes")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid bypass value, got nil")
	}
}
