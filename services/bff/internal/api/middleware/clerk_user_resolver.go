package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// clerkUserUpsertRepo is the minimal interface ClerkUserResolver needs.
// *repository.UserRepository satisfies it.
type clerkUserUpsertRepo interface {
	UpsertByClerkUserID(ctx context.Context, clerkUserID string) (*repository.User, error)
}

// ClerkUserResolver returns middleware that bridges the Clerk string user ID
// (placed on context by RequireClerkAuth) to the DB int64 user ID that
// UserIDFromContext returns to handlers.
//
// It calls repo.UpsertByClerkUserID for JIT provisioning: if the Clerk user
// has never signed in before, a new users row is inserted automatically.
//
// Errors:
//   - 500 on unexpected DB errors.
//   - Passes to next handler on success with the int64 user ID on context.
func ClerkUserResolver(repo clerkUserUpsertRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clerkUserID, ok := ClerkUserIDFromContext(r)
			if !ok || clerkUserID == "" {
				// RequireClerkAuth must run before this middleware.
				log.Printf("[clerk_user_resolver] missing Clerk user ID — check middleware chain order")
				writeInternalError(w)

				return
			}

			user, err := repo.UpsertByClerkUserID(r.Context(), clerkUserID)
			if err != nil {
				log.Printf("[clerk_user_resolver] UpsertByClerkUserID(%q): %v", clerkUserID, err)
				writeInternalError(w)

				return
			}

			ctx := WithUserID(r.Context(), user.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeInternalError writes a 500 JSON response.
func writeInternalError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`{"error":"internal server error"}`))
}
