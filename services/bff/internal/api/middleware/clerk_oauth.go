// Package middleware — Clerk OAuth access-token validator.
//
// RequireClerkAuth verifies Clerk SESSION JWTs (issued by the Clerk JS SDK to
// browser clients). The daemon's PKCE flow returns a different token type:
// a Clerk OAuth ACCESS TOKEN (jti prefix "oat_") signed by the same JWKS but
// without the session claims Clerk's session middleware requires.
//
// This file adds a parallel middleware that validates OAuth access tokens by
// calling Clerk's /oauth/userinfo introspection endpoint. On success it
// attaches a synthetic SessionClaims{Subject: user_id} to the request context
// so downstream handlers can keep using ClerkUserIDFromContext unchanged.

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/observability"
	"github.com/clerk/clerk-sdk-go/v2"
)

// oauthUserInfoTimeout is the HTTP timeout for the /oauth/userinfo call.
const oauthUserInfoTimeout = 10 * time.Second

// userInfoResponse is the subset of Clerk's /oauth/userinfo response we need.
// Clerk follows the OIDC UserInfo spec — "sub" is the user identifier.
type userInfoResponse struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
}

// RequireClerkOAuthToken returns middleware that validates a Clerk OAuth
// access token via the /oauth/userinfo endpoint at clerkFrontendAPI.
//
// On success a synthetic clerk.SessionClaims{Subject: <user_id>} is attached
// to the context so ClerkUserIDFromContext continues to work.
//
// clerkFrontendAPI is the Clerk Frontend API host (e.g. https://clerk.vaultmtg.app).
func RequireClerkOAuthToken(clerkFrontendAPI string) func(http.Handler) http.Handler {
	base := strings.TrimRight(clerkFrontendAPI, "/")
	userInfoURL := base + "/oauth/userinfo"
	client := &http.Client{Timeout: oauthUserInfoTimeout}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			sub, err := fetchUserInfo(r.Context(), client, userInfoURL, token)
			if err != nil || sub == "" {
				writeUnauthorized(w)
				return
			}

			claims := &clerk.SessionClaims{}
			claims.Subject = sub
			ctx := clerk.ContextWithSessionClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// fetchUserInfo calls Clerk's /oauth/userinfo with the access token and returns
// the verified user_id from the response's "sub" field.
func fetchUserInfo(ctx context.Context, client *http.Client, url, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("userinfo http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read userinfo body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		outErr := fmt.Errorf("outbound clerk-oauth: status %d", resp.StatusCode)
		observability.ReportError(ctx, outErr, map[string]string{"component": "outbound", "target": "clerk-oauth"})
		return "", fmt.Errorf("userinfo status %d: %s", resp.StatusCode, string(body))
	}

	var info userInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("decode userinfo: %w", err)
	}
	return info.Sub, nil
}
