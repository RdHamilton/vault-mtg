package config_test

import (
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Provide required production vars; all other flags at defaults.
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "sk_test_dummy")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Env != "production" {
		t.Errorf("expected default env 'production', got %q", cfg.Env)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DatabaseURL 'postgres://localhost/test', got %q", cfg.DatabaseURL)
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
	t.Setenv("CLERK_SECRET_KEY", "sk_test_dummy")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=production and DATABASE_URL is unset")
	}
}

// TestLoad_Env_Production_WithDatabaseURL_OK verifies that production mode
// succeeds when DATABASE_URL and CLERK_SECRET_KEY are present.
func TestLoad_Env_Production_WithDatabaseURL_OK(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "sk_test_dummy")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("production mode with required vars should not error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DatabaseURL 'postgres://localhost/test', got %q", cfg.DatabaseURL)
	}
}

// TestLoad_Env_Production_NoClerkSecretKey_Error verifies that starting in
// production mode without CLERK_SECRET_KEY returns an error.  Without it the
// BFF cannot construct the Clerk auth middleware and protected routes would be
// inaccessible.
func TestLoad_Env_Production_NoClerkSecretKey_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=production and CLERK_SECRET_KEY is unset")
	}
}

// TestLoad_Env_Production_WhitespaceClerkSecretKey_Error verifies a
// whitespace-only CLERK_SECRET_KEY is rejected in production.
func TestLoad_Env_Production_WhitespaceClerkSecretKey_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "   ")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when CLERK_SECRET_KEY is only whitespace in production")
	}
}

// TestLoad_ClerkSecretKey_StoredInConfig verifies the CLERK_SECRET_KEY env var
// is surfaced as Config.ClerkSecretKey for callers (with whitespace trimmed).
func TestLoad_ClerkSecretKey_StoredInConfig(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("CLERK_SECRET_KEY", "  sk_test_abc123  ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ClerkSecretKey != "sk_test_abc123" {
		t.Errorf("expected ClerkSecretKey 'sk_test_abc123', got %q", cfg.ClerkSecretKey)
	}
}

// TestLoad_ClerkSecretKey_EmptyInDevelopment verifies that an unset
// CLERK_SECRET_KEY is allowed in development mode.
func TestLoad_ClerkSecretKey_EmptyInDevelopment(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("CLERK_SECRET_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("development mode with empty CLERK_SECRET_KEY should not error: %v", err)
	}

	if cfg.ClerkSecretKey != "" {
		t.Errorf("expected empty ClerkSecretKey, got %q", cfg.ClerkSecretKey)
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
	t.Setenv("MTGA_ENV", "development")
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
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "0")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for threshold = 0, got nil")
	}
}

func TestLoad_StalenessThreshold_NegativeIsInvalid(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "-1")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for negative threshold, got nil")
	}
}

func TestLoad_StalenessThreshold_NonIntegerIsInvalid(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DRAFT_RATINGS_STALENESS_THRESHOLD_HOURS", "abc")
	t.Setenv("DRAFT_RATINGS_BYPASS_FRESHNESS_CHECK", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for non-integer threshold, got nil")
	}
}

func TestLoad_BypassFreshnessCheck_True(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
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
	t.Setenv("MTGA_ENV", "development")
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
	t.Setenv("MTGA_ENV", "development")
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
	t.Setenv("MTGA_ENV", "development")
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
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ALLOWED_ORIGINS", "https://app.vaultmtg.app,https://*.vaultmtg.app")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 allowed origins, got %d: %v", len(cfg.AllowedOrigins), cfg.AllowedOrigins)
	}

	if cfg.AllowedOrigins[0] != "https://app.vaultmtg.app" {
		t.Errorf("unexpected first origin: %q", cfg.AllowedOrigins[0])
	}

	if cfg.AllowedOrigins[1] != "https://*.vaultmtg.app" {
		t.Errorf("unexpected second origin: %q", cfg.AllowedOrigins[1])
	}
}

// TestLoad_DaemonLatestVersion_DefaultWhenUnset verifies that when
// BFF_DAEMON_LATEST_VERSION is not set the config defaults to "0.1.0" so that
// GET /api/v1/daemon/version always returns a non-empty response in development.
func TestLoad_DaemonLatestVersion_DefaultWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BFF_DAEMON_LATEST_VERSION", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DaemonLatestVersion != "0.1.0" {
		t.Errorf("expected default DaemonLatestVersion '0.1.0', got %q", cfg.DaemonLatestVersion)
	}
}

// TestLoad_DaemonLatestVersion_FromEnv verifies that BFF_DAEMON_LATEST_VERSION
// is surfaced as Config.DaemonLatestVersion.
func TestLoad_DaemonLatestVersion_FromEnv(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BFF_DAEMON_LATEST_VERSION", "0.5.2")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DaemonLatestVersion != "0.5.2" {
		t.Errorf("expected DaemonLatestVersion '0.5.2', got %q", cfg.DaemonLatestVersion)
	}
}

// TestLoad_DaemonReleasedAt_EmptyWhenUnset verifies that when
// BFF_DAEMON_RELEASED_AT is not set Config.DaemonReleasedAt is empty string.
func TestLoad_DaemonReleasedAt_EmptyWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BFF_DAEMON_RELEASED_AT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DaemonReleasedAt != "" {
		t.Errorf("expected empty DaemonReleasedAt, got %q", cfg.DaemonReleasedAt)
	}
}

// TestLoad_DaemonReleasedAt_FromEnv verifies that BFF_DAEMON_RELEASED_AT is
// surfaced as Config.DaemonReleasedAt.
func TestLoad_DaemonReleasedAt_FromEnv(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BFF_DAEMON_RELEASED_AT", "2026-05-01T12:00:00Z")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DaemonReleasedAt != "2026-05-01T12:00:00Z" {
		t.Errorf("expected DaemonReleasedAt '2026-05-01T12:00:00Z', got %q", cfg.DaemonReleasedAt)
	}
}

// TestLoad_SentryDSN_EmptyWhenUnset verifies that when SENTRY_DSN is not set
// Config.SentryDSN is an empty string (Sentry disabled).
func TestLoad_SentryDSN_EmptyWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SENTRY_DSN", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SentryDSN != "" {
		t.Errorf("expected empty SentryDSN when SENTRY_DSN unset, got %q", cfg.SentryDSN)
	}
}

// TestLoad_SentryDSN_FromEnv verifies that SENTRY_DSN is surfaced as
// Config.SentryDSN for callers (with leading/trailing whitespace trimmed).
func TestLoad_SentryDSN_FromEnv(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SENTRY_DSN", "  https://key@o0.ingest.sentry.io/0  ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SentryDSN != "https://key@o0.ingest.sentry.io/0" {
		t.Errorf("expected trimmed SentryDSN, got %q", cfg.SentryDSN)
	}
}

// TestLoad_AllowedOrigins_TrimsWhitespace verifies that leading/trailing
// whitespace around comma-separated values is stripped.
func TestLoad_AllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ALLOWED_ORIGINS", " https://app.vaultmtg.app , https://*.vaultmtg.app ")

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

// TestLoad_Env_DefaultIsProduction verifies that when MTGA_ENV is not set the
// default environment is "production" (fail-fast: requires DATABASE_URL and
// CLERK_SECRET_KEY).
func TestLoad_Env_DefaultIsProduction(t *testing.T) {
	t.Setenv("MTGA_ENV", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "sk_test_dummy")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Env != "production" {
		t.Errorf("expected default env 'production', got %q", cfg.Env)
	}
}

// TestLoad_Env_Staging verifies that MTGA_ENV=staging is returned as-is.
func TestLoad_Env_Staging(t *testing.T) {
	t.Setenv("MTGA_ENV", "staging")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "sk_test_dummy")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Env != "staging" {
		t.Errorf("expected env 'staging', got %q", cfg.Env)
	}
}

// TestLoad_Env_Staging_NoDatabaseURL_Error verifies staging mode requires
// DATABASE_URL (same fail-fast rule as production).
func TestLoad_Env_Staging_NoDatabaseURL_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "staging")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("CLERK_SECRET_KEY", "sk_test_dummy")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=staging and DATABASE_URL is unset")
	}
}

// TestLoad_Env_Staging_NoClerkSecretKey_Error verifies staging mode requires
// CLERK_SECRET_KEY (same fail-fast rule as production).
func TestLoad_Env_Staging_NoClerkSecretKey_Error(t *testing.T) {
	t.Setenv("MTGA_ENV", "staging")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CLERK_SECRET_KEY", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MTGA_ENV=staging and CLERK_SECRET_KEY is unset")
	}
}

// TestLoad_PostHogAPIKey_EmptyWhenUnset verifies that when POSTHOG_API_KEY is
// not set Config.PostHogAPIKey is an empty string (PostHog disabled).
func TestLoad_PostHogAPIKey_EmptyWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTHOG_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PostHogAPIKey != "" {
		t.Errorf("expected empty PostHogAPIKey when POSTHOG_API_KEY unset, got %q", cfg.PostHogAPIKey)
	}
}

// TestLoad_PostHogAPIKey_FromEnv verifies that POSTHOG_API_KEY is surfaced as
// Config.PostHogAPIKey with leading/trailing whitespace trimmed.
func TestLoad_PostHogAPIKey_FromEnv(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTHOG_API_KEY", "  phc_testkey123  ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PostHogAPIKey != "phc_testkey123" {
		t.Errorf("expected trimmed PostHogAPIKey 'phc_testkey123', got %q", cfg.PostHogAPIKey)
	}
}

// TestLoad_PostHogHost_DefaultWhenUnset verifies that when POSTHOG_HOST is not
// set Config.PostHogHost falls back to the canonical US ingest URL so the SDK
// always has a non-empty endpoint without requiring explicit configuration.
func TestLoad_PostHogHost_DefaultWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTHOG_HOST", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = "https://us.i.posthog.com"
	if cfg.PostHogHost != want {
		t.Errorf("expected default PostHogHost %q, got %q", want, cfg.PostHogHost)
	}
}

// TestLoad_PostHogHost_FromEnv verifies that POSTHOG_HOST is surfaced as
// Config.PostHogHost with leading/trailing whitespace trimmed, overriding the
// default.
func TestLoad_PostHogHost_FromEnv(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTHOG_HOST", "  https://eu.i.posthog.com  ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = "https://eu.i.posthog.com"
	if cfg.PostHogHost != want {
		t.Errorf("expected PostHogHost %q, got %q", want, cfg.PostHogHost)
	}
}

// TestLoad_GitCommit_EmptyWhenUnset verifies that when GIT_COMMIT is not set
// Config.GitCommit is an empty string (Sentry Release omitted).
func TestLoad_GitCommit_EmptyWhenUnset(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GIT_COMMIT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitCommit != "" {
		t.Errorf("expected empty GitCommit when GIT_COMMIT unset, got %q", cfg.GitCommit)
	}
}

// TestLoad_GitCommit_FromEnv verifies that GIT_COMMIT is surfaced as
// Config.GitCommit with leading/trailing whitespace trimmed.
func TestLoad_GitCommit_FromEnv(t *testing.T) {
	t.Setenv("MTGA_ENV", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GIT_COMMIT", "  abc123def456  ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitCommit != "abc123def456" {
		t.Errorf("expected trimmed GitCommit 'abc123def456', got %q", cfg.GitCommit)
	}
}
