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
	t.Setenv("DAEMON_JWT_SECRET", "test-secret")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=production and DATABASE_URL is unset")
	}
}

// TestLoad_Env_Production_WithDatabaseURL_OK verifies that production mode
// succeeds when DATABASE_URL and DAEMON_JWT_SECRET are present.
func TestLoad_Env_Production_WithDatabaseURL_OK(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("DAEMON_JWT_SECRET", "test-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("production mode with required vars should not error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DatabaseURL 'postgres://localhost/test', got %q", cfg.DatabaseURL)
	}
}

// TestLoad_Env_Production_NoDaemonJWTSecret_Error verifies that starting in
// production mode without DAEMON_JWT_SECRET returns an error.  See #1169 —
// before this guard a missing secret silently disabled daemon ingest in prod.
func TestLoad_Env_Production_NoDaemonJWTSecret_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("DAEMON_JWT_SECRET", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=production and DAEMON_JWT_SECRET is unset")
	}
}

// TestLoad_Env_Production_WhitespaceDaemonJWTSecret_Error verifies a
// whitespace-only DAEMON_JWT_SECRET is rejected in production (treated as empty).
func TestLoad_Env_Production_WhitespaceDaemonJWTSecret_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("DAEMON_JWT_SECRET", "   ")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DAEMON_JWT_SECRET is only whitespace in production")
	}
}

// TestLoad_DaemonJWTSecret_StoredInConfig verifies the DAEMON_JWT_SECRET env
// var is surfaced as Config.DaemonJWTSecret for callers (with whitespace
// trimmed).
func TestLoad_DaemonJWTSecret_StoredInConfig(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DAEMON_JWT_SECRET", "  super-secret-jwt-key  ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DaemonJWTSecret != "super-secret-jwt-key" {
		t.Errorf("expected trimmed DaemonJWTSecret 'super-secret-jwt-key', got %q", cfg.DaemonJWTSecret)
	}
}

// TestLoad_Env_Development_NoDaemonJWTSecret_OK verifies development mode
// does not require DAEMON_JWT_SECRET (preserves local-dev ergonomics).
func TestLoad_Env_Development_NoDaemonJWTSecret_OK(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DAEMON_JWT_SECRET", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("development mode without DAEMON_JWT_SECRET should not error: %v", err)
	}

	if cfg.DaemonJWTSecret != "" {
		t.Errorf("expected empty DaemonJWTSecret in development, got %q", cfg.DaemonJWTSecret)
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

// TestLoad_AllowedOrigins_DefaultWhenUnset verifies that when ALLOWED_ORIGINS is
// not set the config falls back to the localhost-only default values (ADR-006).
func TestLoad_AllowedOrigins_DefaultWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ALLOWED_ORIGINS", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.AllowedOrigins) == 0 {
		t.Fatal("expected non-empty default AllowedOrigins, got empty slice")
	}

	// Defaults must include localhost — exact values checked here so any
	// unintentional change to the default is caught.
	found := false
	for _, o := range cfg.AllowedOrigins {
		if o == "http://localhost:*" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'http://localhost:*' in default AllowedOrigins, got %v", cfg.AllowedOrigins)
	}
}

// TestLoad_AllowedOrigins_ParsesCommaSeparated verifies that a comma-separated
// ALLOWED_ORIGINS value is split into individual origins (ADR-006).
func TestLoad_AllowedOrigins_ParsesCommaSeparated(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ALLOWED_ORIGINS", "https://mtga-companion.vercel.app,https://*.vercel.app")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 allowed origins, got %d: %v", len(cfg.AllowedOrigins), cfg.AllowedOrigins)
	}

	if cfg.AllowedOrigins[0] != "https://mtga-companion.vercel.app" {
		t.Errorf("unexpected first origin: %q", cfg.AllowedOrigins[0])
	}

	if cfg.AllowedOrigins[1] != "https://*.vercel.app" {
		t.Errorf("unexpected second origin: %q", cfg.AllowedOrigins[1])
	}
}

// TestLoad_AllowedOrigins_TrimsWhitespace verifies that leading/trailing
// whitespace around comma-separated values is stripped.
func TestLoad_AllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ALLOWED_ORIGINS", " https://mtga-companion.vercel.app , https://*.vercel.app ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 origins after whitespace trim, got %d: %v", len(cfg.AllowedOrigins), cfg.AllowedOrigins)
	}

	for _, o := range cfg.AllowedOrigins {
		if o != cfg.AllowedOrigins[0] && o != cfg.AllowedOrigins[1] {
			t.Errorf("unexpected origin after trim: %q", o)
		}
		if len(o) > 0 && (o[0] == ' ' || o[len(o)-1] == ' ') {
			t.Errorf("origin has leading/trailing whitespace: %q", o)
		}
	}
}
