package dbconn_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/ramonehamilton/mtga-sync/internal/dbconn"
)

// stubTokenProvider is a test double for dbconn.TokenProvider.
type stubTokenProvider struct {
	token string
	err   error
	calls int

	capturedEndpoint string
	capturedRegion   string
	capturedUser     string
}

func (s *stubTokenProvider) Provide(
	_ context.Context, endpoint, region, dbUser string,
	_ aws.CredentialsProvider,
	_ ...func(*auth.BuildAuthTokenOptions),
) (string, error) {
	s.calls++
	s.capturedEndpoint = endpoint
	s.capturedRegion = region
	s.capturedUser = dbUser

	return s.token, s.err
}

func validConfig() dbconn.Config {
	return dbconn.Config{
		Host:   "mydb.us-east-1.rds.amazonaws.com",
		Port:   "5432",
		DBName: "mtga",
		User:   "mtga_sync",
		Region: "us-east-1",
	}
}

func TestBuildDSN_Success(t *testing.T) {
	stub := &stubTokenProvider{token: "fake-iam-token"}
	cfg := validConfig()

	dsn, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err != nil {
		t.Fatalf("BuildDSN: %v", err)
	}

	if !strings.Contains(dsn, "host=mydb.us-east-1.rds.amazonaws.com") {
		t.Errorf("DSN missing host: %s", dsn)
	}

	if !strings.Contains(dsn, "port=5432") {
		t.Errorf("DSN missing port: %s", dsn)
	}

	if !strings.Contains(dsn, "user=mtga_sync") {
		t.Errorf("DSN missing user: %s", dsn)
	}

	if !strings.Contains(dsn, "dbname=mtga") {
		t.Errorf("DSN missing dbname: %s", dsn)
	}

	if !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("DSN missing sslmode=require: %s", dsn)
	}

	if stub.calls != 1 {
		t.Errorf("expected 1 token call, got %d", stub.calls)
	}
}

func TestBuildDSN_PassesCorrectEndpointAndRegion(t *testing.T) {
	stub := &stubTokenProvider{token: "tok"}
	cfg := validConfig()

	_, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err != nil {
		t.Fatalf("BuildDSN: %v", err)
	}

	if stub.capturedEndpoint != "mydb.us-east-1.rds.amazonaws.com:5432" {
		t.Errorf("endpoint: got %q, want %q", stub.capturedEndpoint, "mydb.us-east-1.rds.amazonaws.com:5432")
	}

	if stub.capturedRegion != "us-east-1" {
		t.Errorf("region: got %q, want %q", stub.capturedRegion, "us-east-1")
	}

	if stub.capturedUser != "mtga_sync" {
		t.Errorf("user: got %q, want %q", stub.capturedUser, "mtga_sync")
	}
}

func TestBuildDSN_DefaultPort(t *testing.T) {
	stub := &stubTokenProvider{token: "tok"}
	cfg := validConfig()
	cfg.Port = "" // omit port — should default to 5432

	dsn, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err != nil {
		t.Fatalf("BuildDSN: %v", err)
	}

	if !strings.Contains(dsn, "port=5432") {
		t.Errorf("expected default port 5432 in DSN: %s", dsn)
	}

	if stub.capturedEndpoint != "mydb.us-east-1.rds.amazonaws.com:5432" {
		t.Errorf("endpoint: got %q, want port 5432 default", stub.capturedEndpoint)
	}
}

func TestBuildDSN_TokenError(t *testing.T) {
	tokenErr := errors.New("iam token generation failed")
	stub := &stubTokenProvider{err: tokenErr}

	_, err := dbconn.BuildDSN(context.Background(), validConfig(), nil, stub.Provide)
	if err == nil {
		t.Fatal("expected error from BuildDSN when token fails, got nil")
	}

	if !errors.Is(err, tokenErr) {
		t.Errorf("expected wrapped tokenErr, got: %v", err)
	}
}

func TestBuildDSN_MissingHost(t *testing.T) {
	cfg := validConfig()
	cfg.Host = ""

	stub := &stubTokenProvider{token: "tok"}

	_, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err == nil {
		t.Fatal("expected error when Host is empty")
	}

	if !strings.Contains(err.Error(), "DB_HOST") {
		t.Errorf("expected error to mention DB_HOST, got: %v", err)
	}

	// Token should not have been called.
	if stub.calls != 0 {
		t.Errorf("expected 0 token calls, got %d", stub.calls)
	}
}

func TestBuildDSN_MissingDBName(t *testing.T) {
	cfg := validConfig()
	cfg.DBName = ""

	stub := &stubTokenProvider{token: "tok"}

	_, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err == nil {
		t.Fatal("expected error when DBName is empty")
	}

	if !strings.Contains(err.Error(), "DB_NAME") {
		t.Errorf("expected error to mention DB_NAME, got: %v", err)
	}
}

func TestBuildDSN_MissingUser(t *testing.T) {
	cfg := validConfig()
	cfg.User = ""

	stub := &stubTokenProvider{token: "tok"}

	_, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err == nil {
		t.Fatal("expected error when User is empty")
	}

	if !strings.Contains(err.Error(), "DB_USER") {
		t.Errorf("expected error to mention DB_USER, got: %v", err)
	}
}

func TestBuildDSN_MissingRegion(t *testing.T) {
	cfg := validConfig()
	cfg.Region = ""

	stub := &stubTokenProvider{token: "tok"}

	_, err := dbconn.BuildDSN(context.Background(), cfg, nil, stub.Provide)
	if err == nil {
		t.Fatal("expected error when Region is empty")
	}

	if !strings.Contains(err.Error(), "AWS_REGION") {
		t.Errorf("expected error to mention AWS_REGION, got: %v", err)
	}
}

func TestBuildDSN_TokenWithSpecialCharsIsEncoded(t *testing.T) {
	// IAM tokens contain "/" and "=" which must be URL-encoded for the DSN
	// to parse correctly.
	rawToken := "abc/def==ghi+jkl"
	stub := &stubTokenProvider{token: rawToken}

	dsn, err := dbconn.BuildDSN(context.Background(), validConfig(), nil, stub.Provide)
	if err != nil {
		t.Fatalf("BuildDSN: %v", err)
	}

	// The raw token must NOT appear verbatim in the DSN.
	if strings.Contains(dsn, rawToken) {
		t.Errorf("DSN contains unencoded token; special chars must be escaped: %s", dsn)
	}
}

func TestConfig_Validate_AllFieldsRequired(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *dbconn.Config)
		wantMsg string
	}{
		{"empty host", func(c *dbconn.Config) { c.Host = "" }, "DB_HOST"},
		{"empty dbname", func(c *dbconn.Config) { c.DBName = "" }, "DB_NAME"},
		{"empty user", func(c *dbconn.Config) { c.User = "" }, "DB_USER"},
		{"empty region", func(c *dbconn.Config) { c.Region = "" }, "AWS_REGION"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}

			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantMsg)
			}
		})
	}
}
