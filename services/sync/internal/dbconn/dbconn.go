// Package dbconn provides helpers for connecting to RDS PostgreSQL
// using a password supplied at runtime via the Lambda environment.
//
// The sync Lambda authenticates to RDS with a static password fetched
// from SSM Parameter Store at stack-update time (resolved into the
// Lambda's DB_PASSWORD environment variable). The mtga_sync DB role
// has no other privileges than the application-level grants required
// to write card_ratings / cards / draft_* — there is no human
// path through this credential and no PII flows through it.
//
// (PR #2650 left as a forensic record of the IAM-auth attempt; see
// ADR-035 for the architectural decision context.)
package dbconn

import "fmt"

// Config holds the RDS connection parameters read from environment variables.
// All fields are required.
type Config struct {
	Host     string // DB_HOST
	Port     string // DB_PORT (default "5432")
	DBName   string // DB_NAME
	User     string // DB_USER
	Password string // DB_PASSWORD
}

// Validate returns an error if any required field is empty.
func (c Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if c.DBName == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if c.User == "" {
		return fmt.Errorf("DB_USER is required")
	}

	if c.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}

	return nil
}

// port returns the configured port or the PostgreSQL default.
func (c Config) port() string {
	if c.Port == "" {
		return "5432"
	}

	return c.Port
}

// BuildPasswordDSN assembles a PostgreSQL DSN that authenticates to RDS
// using a static password. The returned DSN is suitable for use with
// pgxpool.New.
//
// The DSN uses libpq key/value form rather than a URL because key/value
// is forgiving about special characters in the password (no URL-encoding
// required); pgx accepts both forms.
func BuildPasswordDSN(cfg Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		cfg.Host, cfg.port(), cfg.User, cfg.Password, cfg.DBName,
	), nil
}
