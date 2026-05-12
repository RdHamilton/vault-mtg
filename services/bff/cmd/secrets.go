package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// secretCreds holds the credentials returned by Secrets Manager.
// Matches the JSON shape of RDS-managed secrets
// ({"username":"...","password":"..."}).
type secretCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// credsFetcher abstracts the Secrets Manager call so tests can inject a stub.
type credsFetcher func(ctx context.Context, arn string) (secretCreds, error)

// fetchCredsFromAWS is the production credsFetcher.
func fetchCredsFromAWS(ctx context.Context, arn string) (secretCreds, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return secretCreds{}, fmt.Errorf("load AWS config: %w", err)
	}
	client := secretsmanager.NewFromConfig(cfg)
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &arn,
	})
	if err != nil {
		return secretCreds{}, fmt.Errorf("GetSecretValue: %w", err)
	}
	var creds secretCreds
	if err := json.Unmarshal([]byte(*out.SecretString), &creds); err != nil {
		return secretCreds{}, fmt.Errorf("parse secret JSON: %w", err)
	}
	if creds.Username == "" || creds.Password == "" {
		return secretCreds{}, fmt.Errorf("secret missing username or password")
	}
	return creds, nil
}

// resolveDBURL returns the DATABASE_URL to use.
// If secretARN is empty it returns rawURL unchanged.
// Otherwise it fetches credentials from Secrets Manager and splices them
// into rawURL, replacing any existing userinfo so stale passwords in env
// files are never used after an RDS rotation.
func resolveDBURL(ctx context.Context, fetch credsFetcher, secretARN, rawURL string) (string, error) {
	if secretARN == "" {
		return rawURL, nil
	}
	creds, err := fetch(ctx, secretARN)
	if err != nil {
		return "", fmt.Errorf("fetch DB credentials: %w", err)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	u.User = url.UserPassword(creds.Username, creds.Password)
	return u.String(), nil
}
