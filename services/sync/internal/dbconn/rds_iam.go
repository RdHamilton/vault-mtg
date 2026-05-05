// Package dbconn provides helpers for connecting to RDS PostgreSQL
// using AWS IAM authentication tokens.
//
// ADR-003 mandates IAM auth for the sync Lambda: no static password is stored.
// The Lambda execution role must have rds-db:connect permission for the
// mtga_sync DB user.  IAM tokens expire after 15 minutes; refreshing once
// per Lambda invocation is sufficient because Lambda invocations are short-lived.
package dbconn

import (
	"context"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
)

// Config holds the RDS connection parameters read from environment variables.
// All fields are required when using IAM auth.
type Config struct {
	Host   string // DB_HOST
	Port   string // DB_PORT (default "5432")
	DBName string // DB_NAME
	User   string // DB_USER
	Region string // AWS_REGION
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

	if c.Region == "" {
		return fmt.Errorf("AWS_REGION is required")
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

// endpoint returns "host:port" as expected by BuildAuthToken.
func (c Config) endpoint() string {
	return fmt.Sprintf("%s:%s", c.Host, c.port())
}

// TokenProvider is the function signature used to generate an IAM auth token.
// Abstracted for testing.
type TokenProvider func(ctx context.Context, endpoint, region, dbUser string, creds aws.CredentialsProvider, optFns ...func(*auth.BuildAuthTokenOptions)) (string, error)

// BuildDSN generates a PostgreSQL DSN that uses an RDS IAM auth token as the
// password.  The token is fetched fresh on every call (tokens expire after
// 15 minutes; calling once per Lambda invocation is safe).
//
// The returned DSN is suitable for use with pgxpool.New.
func BuildDSN(ctx context.Context, cfg Config, creds aws.CredentialsProvider, tokenFn TokenProvider) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}

	token, err := tokenFn(ctx, cfg.endpoint(), cfg.Region, cfg.User, creds)
	if err != nil {
		return "", fmt.Errorf("generate RDS IAM auth token: %w", err)
	}

	// url.QueryEscape encodes the token safely for use as a password in the DSN.
	// IAM tokens contain "/" and "=" characters that would break a raw DSN.
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		cfg.Host, cfg.port(), cfg.User, url.QueryEscape(token), cfg.DBName,
	)

	return dsn, nil
}
