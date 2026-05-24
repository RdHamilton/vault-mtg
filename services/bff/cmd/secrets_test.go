package main

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubFetcher(creds secretCreds, err error) credsFetcher {
	return func(_ context.Context, _ string) (secretCreds, error) {
		return creds, err
	}
}

func TestResolveDBURL_NoARN(t *testing.T) {
	raw := "postgresql://user:pass@host:5432/db"
	got, err := resolveDBURL(context.Background(), nil, "", raw)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

func TestResolveDBURL_SplicesCredentials(t *testing.T) {
	raw := "postgresql://olduser:oldpass@host:5432/db?sslmode=require"
	creds := secretCreds{Username: "mtga_admin", Password: "newpass"}
	got, err := resolveDBURL(context.Background(), stubFetcher(creds, nil), "arn:fake", raw)
	require.NoError(t, err)
	assert.Equal(t, "postgresql://mtga_admin:newpass@host:5432/db?sslmode=require", got)
}

func TestResolveDBURL_NoExistingCredentials(t *testing.T) {
	raw := "postgresql://host:5432/db"
	creds := secretCreds{Username: "mtga_admin", Password: "secret"}
	got, err := resolveDBURL(context.Background(), stubFetcher(creds, nil), "arn:fake", raw)
	require.NoError(t, err)
	assert.Equal(t, "postgresql://mtga_admin:secret@host:5432/db", got)
}

func TestResolveDBURL_FetchError(t *testing.T) {
	_, err := resolveDBURL(context.Background(), stubFetcher(secretCreds{}, errors.New("access denied")), "arn:fake", "postgresql://host/db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestResolveDBURL_BadRawURL(t *testing.T) {
	creds := secretCreds{Username: "u", Password: "p"}
	_, err := resolveDBURL(context.Background(), stubFetcher(creds, nil), "arn:fake", "://bad url")
	require.Error(t, err)
}

func TestResolveDBURL_SpecialCharsInPassword(t *testing.T) {
	raw := "postgresql://host:5432/db"
	creds := secretCreds{Username: "admin", Password: "p@$$w0rd!#%"}
	got, err := resolveDBURL(context.Background(), stubFetcher(creds, nil), "arn:fake", raw)
	require.NoError(t, err)
	// url.UserPassword percent-encodes special chars; the URL must round-trip.
	u, parseErr := url.Parse(got)
	require.NoError(t, parseErr)
	gotPass, _ := u.User.Password()
	assert.Equal(t, creds.Password, gotPass)
}

// shouldResolveFromSM gates the runtime Secrets Manager call.
// The default (toggle unset/false) is OFF — the BFF reads DATABASE_URL
// inline and never constructs an SM client. Opting in requires BOTH
// BFF_DB_RESOLVE_FROM_SM=true AND a non-empty DB_SECRET_ARN.
func TestShouldResolveFromSM(t *testing.T) {
	cases := []struct {
		name   string
		toggle string
		arn    string
		want   bool
	}{
		{"toggle unset, arn unset", "", "", false},
		{"toggle unset, arn set (default off)", "", "arn:aws:secretsmanager:...:secret:foo", false},
		{"toggle false, arn set", "false", "arn:aws:secretsmanager:...:secret:foo", false},
		{"toggle TRUE (wrong case), arn set", "TRUE", "arn:aws:secretsmanager:...:secret:foo", false},
		{"toggle true, arn unset", "true", "", false},
		{"toggle true, arn set (opt-in path)", "true", "arn:aws:secretsmanager:...:secret:foo", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldResolveFromSM(tc.toggle, tc.arn)
			assert.Equal(t, tc.want, got)
		})
	}
}
