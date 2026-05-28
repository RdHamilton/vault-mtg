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
	// Env is the runtime environment.  Sourced from MTGA_ENV (default "production").
	// Recognised values: "production", "staging", "development".
	// When Env is "production" or "staging", DATABASE_URL and CLERK_SECRET_KEY
	// must be set or Load returns an error.
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
	// (e.g. "https://app.vaultmtg.app,https://*.vaultmtg.app").
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

	// ClerkFrontendAPI is the Clerk Frontend API base URL (e.g.
	// https://clerk.vaultmtg.app) used by the OAuth-token middleware to call
	// /oauth/userinfo when validating PKCE access tokens from the daemon.
	//
	// Sourced from CLERK_FRONTEND_API.  May be empty in development; when empty
	// the OAuth-token middleware is not constructed and routes protected by it
	// are not mounted.
	ClerkFrontendAPI string

	// SentryDSN is the Sentry Data Source Name used to initialise the
	// sentry-go SDK at BFF startup.
	//
	// Sourced from SENTRY_DSN.  The actual value is stored in AWS SSM
	// Parameter Store at /vaultmtg/prod/sentry-bff-dsn and injected as an
	// environment variable at deploy time.
	//
	// When empty (e.g. local development without a Sentry account), Sentry
	// initialisation is skipped and a warning is logged.  This value must
	// NEVER be logged or included in any error response body.
	SentryDSN string

	// PostHogAPIKey is the PostHog server-side API key used to emit
	// server-side analytics events from the BFF.
	//
	// Sourced from POSTHOG_API_KEY.  The actual value is stored in AWS SSM
	// Parameter Store at /vaultmtg/prod/posthog-api-key and injected as an
	// environment variable at deploy time.
	//
	// When empty (e.g. local development), PostHog is disabled and a no-op
	// client is used.  This value must NEVER be logged or included in any
	// error response body.
	PostHogAPIKey string

	// GitCommit is the Git SHA of the deployed revision.  Sourced from the
	// GIT_COMMIT environment variable, which the deploy pipeline writes into
	// the BFF env file alongside the other provisioned secrets.
	//
	// When non-empty it is passed to the Sentry SDK as the Release field so
	// that error events are correlated to a specific build in the Sentry UI.
	// When empty (e.g. local development or a pre-#2363 deploy) Sentry
	// initialises without a Release tag — this is safe and expected.
	GitCommit string

	// BFFAdminToken is the static high-entropy Bearer token that protects the
	// admin fleet-health endpoint (GET /api/v1/admin/daemons/fleet-health).
	//
	// Sourced from BFF_ADMIN_TOKEN (set by ec2-bootstrap.sh from SSM
	// /vaultmtg/app/production/bff-admin-token, SecureString).
	//
	// When empty, the admin endpoint is mounted but the AdminTokenAuth
	// middleware rejects ALL requests — this is the safe default for local
	// development. The value must NEVER be logged or included in any error
	// response body.
	BFFAdminToken string

	// MailchimpAPIKey is the Mailchimp Marketing API key (format: <key>-<dc>)
	// used by the waitlist handler to subscribe new emails.
	//
	// Sourced from MAILCHIMP_API_KEY (set by ec2-bootstrap.sh from SSM
	// /vaultmtg/prod/mailchimp-api-key — Ray will provision via ticket #122).
	//
	// When empty, the waitlist handler still persists DB rows but skips the
	// Mailchimp API call. The value must NEVER be logged or included in any
	// error response body.
	MailchimpAPIKey string

	// MailchimpListID is the Mailchimp audience list ID to subscribe members to.
	//
	// Sourced from MAILCHIMP_LIST_ID (set by ec2-bootstrap.sh from SSM
	// /vaultmtg/prod/mailchimp-list-id — Ray will provision via ticket #122).
	//
	// When empty, the Mailchimp client is not constructed (same effect as an
	// empty MailchimpAPIKey).
	MailchimpListID string
}

// Load reads configuration from environment variables, applies defaults, and
// returns a validated Config.  An error is returned if any value is invalid.
//
// Production and staging modes (MTGA_ENV=production or MTGA_ENV=staging) require
// DATABASE_URL and CLERK_SECRET_KEY to be set.  Omitting DATABASE_URL is a fatal
// misconfiguration because the API key auth middleware cannot be constructed
// without a database, which would leave the SSE endpoint and any other guarded
// route unprotected.
func Load() (*Config, error) {
	env := os.Getenv("MTGA_ENV")
	if env == "" {
		env = "production"
	}

	dbURL := os.Getenv("DATABASE_URL")

	if (env == "production" || env == "staging") && dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be set when MTGA_ENV=%s", env)
	}

	clerkSecretKey := strings.TrimSpace(os.Getenv("CLERK_SECRET_KEY"))

	if (env == "production" || env == "staging") && clerkSecretKey == "" {
		return nil, fmt.Errorf("CLERK_SECRET_KEY must be set when MTGA_ENV=%s", env)
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
		ClerkFrontendAPI:                    strings.TrimSpace(os.Getenv("CLERK_FRONTEND_API")),
		SentryDSN:                           strings.TrimSpace(os.Getenv("SENTRY_DSN")),
		PostHogAPIKey:                       strings.TrimSpace(os.Getenv("POSTHOG_API_KEY")),
		GitCommit:                           strings.TrimSpace(os.Getenv("GIT_COMMIT")),
		BFFAdminToken:                       strings.TrimSpace(os.Getenv("BFF_ADMIN_TOKEN")),
		MailchimpAPIKey:                     strings.TrimSpace(os.Getenv("MAILCHIMP_API_KEY")),
		MailchimpListID:                     strings.TrimSpace(os.Getenv("MAILCHIMP_LIST_ID")),
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
