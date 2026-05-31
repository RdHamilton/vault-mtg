// Package dbconn provides helpers for connecting the meta-scrape Lambda to RDS
// PostgreSQL. Unlike the sync Lambda — whose password is injected into a
// plaintext env var via an SSM Type=String dynamic reference (the ADR-035
// trade-off) — this package fetches the password from an SSM SecureString
// parameter at runtime, decrypting it via kms:Decrypt. The decrypted credential
// never lands in the Lambda's environment, CloudFormation state, or the
// `get-function-configuration` output. See #341 and ADR-044.
package dbconn

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Config holds the RDS connection parameters. Password is supplied separately
// (fetched from SSM SecureString) rather than read from the environment.
type Config struct {
	Host     string // DB_HOST
	Port     string // DB_PORT (default "5432")
	DBName   string // DB_NAME
	User     string // DB_USER
	Password string // fetched from SSM SecureString at runtime
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
		return fmt.Errorf("DB password is required")
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

// BuildPasswordDSN assembles a libpq key/value PostgreSQL DSN. Key/value form is
// used (rather than a URL) because it is forgiving of special characters in the
// password without URL-encoding; pgx accepts both forms.
func BuildPasswordDSN(cfg Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		cfg.Host, cfg.port(), cfg.User, cfg.Password, cfg.DBName,
	), nil
}

// ssmGetter is the subset of the SSM client used to read a parameter. *ssm.Client
// satisfies it; tests inject a stub.
type ssmGetter interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// FetchSecureString reads and decrypts a SecureString SSM parameter. It returns
// an error when the path is empty or the parameter has no value. WithDecryption
// is always true, so the role must hold ssm:GetParameter on the param plus
// kms:Decrypt on its CMK.
func FetchSecureString(ctx context.Context, client ssmGetter, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("DB_PASSWORD_SSM_PATH is required")
	}
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(path),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("get SSM parameter %q: %w", path, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil || *out.Parameter.Value == "" {
		return "", fmt.Errorf("SSM parameter %q has no value", path)
	}
	return *out.Parameter.Value, nil
}
