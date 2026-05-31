package dbconn

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"valid", Config{Host: "h", DBName: "d", User: "u", Password: "p"}, ""},
		{"no host", Config{DBName: "d", User: "u", Password: "p"}, "DB_HOST"},
		{"no dbname", Config{Host: "h", User: "u", Password: "p"}, "DB_NAME"},
		{"no user", Config{Host: "h", DBName: "d", Password: "p"}, "DB_USER"},
		{"no password", Config{Host: "h", DBName: "d", User: "u"}, "password"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestBuildPasswordDSN(t *testing.T) {
	dsn, err := BuildPasswordDSN(Config{Host: "rds.example", DBName: "mtga", User: "mtga_sync", Password: "p@ss word"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"host=rds.example", "port=5432", "user=mtga_sync", "password=p@ss word", "dbname=mtga", "sslmode=require"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN missing %q: %s", want, dsn)
		}
	}
}

func TestBuildPasswordDSN_CustomPort(t *testing.T) {
	dsn, err := BuildPasswordDSN(Config{Host: "h", DBName: "d", User: "u", Password: "p", Port: "6543"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(dsn, "port=6543") {
		t.Errorf("expected custom port, got %s", dsn)
	}
}

// fakeSSM implements ssmGetter.
type fakeSSM struct {
	value        *string
	err          error
	gotName      string
	gotDecrypted bool
}

func (f *fakeSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if in.Name != nil {
		f.gotName = *in.Name
	}
	if in.WithDecryption != nil {
		f.gotDecrypted = *in.WithDecryption
	}
	if f.err != nil {
		return nil, f.err
	}
	return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: f.value}}, nil
}

func TestFetchSecureString_Success(t *testing.T) {
	fake := &fakeSSM{value: aws.String("s3cr3t")}
	got, err := FetchSecureString(context.Background(), fake, "/vaultmtg/app/production/sync-db-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("value: got %q", got)
	}
	if fake.gotName != "/vaultmtg/app/production/sync-db-password" {
		t.Errorf("name: got %q", fake.gotName)
	}
	if !fake.gotDecrypted {
		t.Error("WithDecryption must be true")
	}
}

func TestFetchSecureString_EmptyPath(t *testing.T) {
	if _, err := FetchSecureString(context.Background(), &fakeSSM{}, ""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestFetchSecureString_APIError(t *testing.T) {
	fake := &fakeSSM{err: errors.New("access denied")}
	if _, err := FetchSecureString(context.Background(), fake, "/p"); err == nil {
		t.Fatal("expected error from API failure")
	}
}

func TestFetchSecureString_NoValue(t *testing.T) {
	fake := &fakeSSM{value: nil}
	if _, err := FetchSecureString(context.Background(), fake, "/p"); err == nil {
		t.Fatal("expected error when parameter has no value")
	}
}
