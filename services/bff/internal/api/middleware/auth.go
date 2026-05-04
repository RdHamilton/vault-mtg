// Package middleware provides HTTP middleware for the BFF service.
package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ctxKeyUserID is the context key used to store the authenticated user ID.
type ctxKey string

const ctxKeyUserID ctxKey = "user_id"

// activeKeyLister is the subset of APIKeyRepository used by the middleware.
type activeKeyLister interface {
	ListAllActive(ctx context.Context) ([]repository.APIKey, error)
	UpdateLastUsedAt(ctx context.Context, id int64) error
}

// APIKeyAuth returns middleware that validates an "Authorization: Bearer <key>"
// header against the stored bcrypt hashes in the database.
//
// On a valid key it sets the authenticated user_id on the request context and
// updates last_used_at.  On failure it returns 401.
//
// NOTE(v1): Authentication performs a full table scan of active api_keys followed
// by a bcrypt.CompareHashAndPassword for each row.  This is correct and safe for
// low daemon volume (single daemon per user) but will degrade under high key
// counts.  A prefix-index optimisation is tracked in ticket #1000.
func APIKeyAuth(repo activeKeyLister) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			keys, err := repo.ListAllActive(r.Context())
			if err != nil {
				log.Printf("[auth] ListAllActive: %v", err)
				writeUnauthorized(w)
				return
			}

			for _, k := range keys {
				if bcrypt.CompareHashAndPassword([]byte(k.KeyHash), []byte(token)) == nil {
					// Matched — update last_used_at in background to avoid
					// blocking the request on a non-critical write.
					go func(id int64) {
						if err := repo.UpdateLastUsedAt(context.Background(), id); err != nil {
							log.Printf("[auth] UpdateLastUsedAt id=%d: %v", id, err)
						}
					}(k.ID)

					ctx := context.WithValue(r.Context(), ctxKeyUserID, k.UserID)
					next.ServeHTTP(w, r.WithContext(ctx))

					return
				}
			}

			writeUnauthorized(w)
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID stored by APIKeyAuth.
// Returns (0, false) when no user ID is present on the context.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKeyUserID).(int64)

	return v, ok
}

// WithUserID returns a copy of ctx with the given user ID stored as if the
// APIKeyAuth middleware had authenticated the request.  Intended for use in
// tests and middleware-chaining helpers.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userID)
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimPrefix(header, prefix)
	if token == "" {
		return "", false
	}

	return token, true
}

// writeUnauthorized writes a 401 JSON response.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	body, _ := json.Marshal(map[string]string{"error": "unauthorized"})
	_, _ = w.Write(body)
}
