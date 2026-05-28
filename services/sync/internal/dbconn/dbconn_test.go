package dbconn_test

import (
	"strings"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/dbconn"
)

func validConfig() dbconn.Config {
	return dbconn.Config{
		Host:     "mydb.us-east-1.rds.amazonaws.com",
		Port:     "5432",
		DBName:   "mtga",
		User:     "mtga_sync",
		Password: "fake-password",
	}
}

func TestBuildPasswordDSN_Success(t *testing.T) {
	cfg := validConfig()

	dsn, err := dbconn.BuildPasswordDSN(cfg)
	if err != nil {
		t.Fatalf("BuildPasswordDSN: %v", err)
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

	if !strings.Contains(dsn, "password=fake-password") {
		t.Errorf("DSN missing password: %s", dsn)
	}

	if !strings.Contains(dsn, "dbname=mtga") {
		t.Errorf("DSN missing dbname: %s", dsn)
	}

	if !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("DSN missing sslmode=require: %s", dsn)
	}
}

func TestBuildPasswordDSN_DefaultPort(t *testing.T) {
	cfg := validConfig()
	cfg.Port = "" // omit port — should default to 5432

	dsn, err := dbconn.BuildPasswordDSN(cfg)
	if err != nil {
		t.Fatalf("BuildPasswordDSN: %v", err)
	}

	if !strings.Contains(dsn, "port=5432") {
		t.Errorf("expected default port 5432 in DSN: %s", dsn)
	}
}

func TestBuildPasswordDSN_MissingHost(t *testing.T) {
	cfg := validConfig()
	cfg.Host = ""

	_, err := dbconn.BuildPasswordDSN(cfg)
	if err == nil {
		t.Fatal("expected error when Host is empty")
	}

	if !strings.Contains(err.Error(), "DB_HOST") {
		t.Errorf("expected error to mention DB_HOST, got: %v", err)
	}
}

func TestBuildPasswordDSN_MissingDBName(t *testing.T) {
	cfg := validConfig()
	cfg.DBName = ""

	_, err := dbconn.BuildPasswordDSN(cfg)
	if err == nil {
		t.Fatal("expected error when DBName is empty")
	}

	if !strings.Contains(err.Error(), "DB_NAME") {
		t.Errorf("expected error to mention DB_NAME, got: %v", err)
	}
}

func TestBuildPasswordDSN_MissingUser(t *testing.T) {
	cfg := validConfig()
	cfg.User = ""

	_, err := dbconn.BuildPasswordDSN(cfg)
	if err == nil {
		t.Fatal("expected error when User is empty")
	}

	if !strings.Contains(err.Error(), "DB_USER") {
		t.Errorf("expected error to mention DB_USER, got: %v", err)
	}
}

func TestBuildPasswordDSN_MissingPassword(t *testing.T) {
	cfg := validConfig()
	cfg.Password = ""

	_, err := dbconn.BuildPasswordDSN(cfg)
	if err == nil {
		t.Fatal("expected error when Password is empty")
	}

	if !strings.Contains(err.Error(), "DB_PASSWORD") {
		t.Errorf("expected error to mention DB_PASSWORD, got: %v", err)
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
		{"empty password", func(c *dbconn.Config) { c.Password = "" }, "DB_PASSWORD"},
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
