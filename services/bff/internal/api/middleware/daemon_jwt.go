package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ctxKeyDaemonUserID is the context key for the user_id extracted from a daemon JWT.
const ctxKeyDaemonUserID ctxKey = "daemon_user_id"

// DaemonClaims are the JWT claims issued to registered daemons.
type DaemonClaims struct {
	UserID   int64  `json:"user_id"`
	DaemonID string `json:"daemon_id"`
	jwt.RegisteredClaims
}

// DaemonJWTAuth returns middleware that validates an "Authorization: Bearer <jwt>"
// signed with the provided HMAC-SHA256 secret.
//
// On success it stores the user_id from the token in the request context and
// calls the next handler. On failure it returns 401.
func DaemonJWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			claims, err := parseDaemonJWT(raw, secret)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyDaemonUserID, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DaemonUserIDFromContext retrieves the user_id stored by DaemonJWTAuth.
// Returns (0, false) when no daemon user ID is present.
func DaemonUserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKeyDaemonUserID).(int64)
	return v, ok
}

// WithDaemonUserID returns a copy of ctx with the given user ID stored as if
// DaemonJWTAuth middleware had validated a daemon JWT.  Intended for use in
// tests and middleware-chaining helpers.
func WithDaemonUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKeyDaemonUserID, userID)
}

// IssueDaemonJWT creates a signed JWT for the given userID and daemonID.
// The token is valid for 30 days from the time of issuance.
func IssueDaemonJWT(secret string, userID int64, daemonID string) (string, error) {
	now := time.Now().UTC()
	claims := DaemonClaims{
		UserID:   userID,
		DaemonID: daemonID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// parseDaemonJWT validates the token string and returns the claims on success.
func parseDaemonJWT(tokenStr, secret string) (*DaemonClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&DaemonClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*DaemonClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	if strings.TrimSpace(claims.DaemonID) == "" {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
