package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

const testSecret = "test-secret-value-for-unit-tests"

// daemonOKHandler is a sentinel handler that writes the user_id from context.
var daemonOKHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.DaemonUserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "%d", uid)
})

func issueToken(t *testing.T, secret string, userID int64, daemonID string, exp time.Duration) string {
	t.Helper()
	now := time.Now().UTC()
	claims := middleware.DaemonClaims{
		UserID:   userID,
		DaemonID: daemonID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(exp)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestDaemonJWTAuth_ValidToken(t *testing.T) {
	token := issueToken(t, testSecret, 7, "daemon-uuid-abc", 30*24*time.Hour)

	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "7" {
		t.Errorf("expected user_id=7 in body, got %q", rr.Body.String())
	}
}

func TestDaemonJWTAuth_NoHeader(t *testing.T) {
	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestDaemonJWTAuth_WrongScheme(t *testing.T) {
	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestDaemonJWTAuth_WrongSecret(t *testing.T) {
	token := issueToken(t, "other-secret", 7, "daemon-uuid-abc", 30*24*time.Hour)

	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestDaemonJWTAuth_ExpiredToken(t *testing.T) {
	token := issueToken(t, testSecret, 7, "daemon-uuid-abc", -time.Hour)

	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rr.Code)
	}
}

func TestDaemonJWTAuth_MalformedToken(t *testing.T) {
	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.jwt")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for malformed token, got %d", rr.Code)
	}
}

func TestDaemonJWTAuth_WrongAlgorithm(t *testing.T) {
	// Sign with RS256 is not practical without a key pair; instead forge a header
	// claiming HS512 which DaemonJWTAuth must reject.
	now := time.Now().UTC()
	claims := middleware.DaemonClaims{
		UserID:   1,
		DaemonID: "d",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	// Use HS512 which is HMAC but not HS256 — our middleware restricts to HS256.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, _ := tok.SignedString([]byte(testSecret))

	handler := middleware.DaemonJWTAuth(testSecret)(daemonOKHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for HS512 token, got %d", rr.Code)
	}
}

func TestDaemonUserIDFromContext_NotPresent(t *testing.T) {
	uid, ok := middleware.DaemonUserIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	if ok || uid != 0 {
		t.Errorf("expected (0, false), got (%d, %v)", uid, ok)
	}
}

func TestIssueDaemonJWT_ValidRoundTrip(t *testing.T) {
	tokenStr, err := middleware.IssueDaemonJWT(testSecret, 42, "my-daemon-id")
	if err != nil {
		t.Fatalf("IssueDaemonJWT: %v", err)
	}

	// Use the middleware to validate the token we just issued.
	var capturedUID int64
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUID, _ = middleware.DaemonUserIDFromContext(r.Context())
	})

	handler := middleware.DaemonJWTAuth(testSecret)(capture)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if capturedUID != 42 {
		t.Errorf("expected user_id=42, got %d", capturedUID)
	}
}
