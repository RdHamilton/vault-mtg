package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// stubKeyLister implements activeKeyLister for tests.
type stubKeyLister struct {
	keys        []repository.APIKey
	listErr     error
	updateErr   error
	updateCalls []int64
}

func (s *stubKeyLister) ListAllActive(_ context.Context) ([]repository.APIKey, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}

	return s.keys, nil
}

func (s *stubKeyLister) UpdateLastUsedAt(_ context.Context, id int64) error {
	s.updateCalls = append(s.updateCalls, id)

	return s.updateErr
}

// hashKey returns a bcrypt hash of key or fails the test.
func hashKey(t *testing.T, key string) string {
	t.Helper()

	h, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	return string(h)
}

func makeKey(t *testing.T, id, userID int64, plaintext string) repository.APIKey {
	t.Helper()

	now := time.Now()

	return repository.APIKey{
		ID:        id,
		UserID:    userID,
		KeyHash:   hashKey(t, plaintext),
		CreatedAt: now,
		Revoked:   false,
	}
}

// okHandler is a sentinel 200 OK handler to confirm the middleware passed through.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%d", uid)
})

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	const plaintext = "deadbeefcafebabedeadbeefcafebabe01234567890abcdef01234567890abcd"

	stub := &stubKeyLister{
		keys: []repository.APIKey{makeKey(t, 1, 99, plaintext)},
	}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if rr.Body.String() != "99" {
		t.Errorf("expected user_id 99 in body, got %q", rr.Body.String())
	}
}

func TestAPIKeyAuth_NoAuthHeader(t *testing.T) {
	stub := &stubKeyLister{}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAPIKeyAuth_WrongScheme(t *testing.T) {
	stub := &stubKeyLister{}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAPIKeyAuth_WrongKey(t *testing.T) {
	stub := &stubKeyLister{
		keys: []repository.APIKey{makeKey(t, 1, 1, "correct-key-value-here-1234567890abcdef")},
	}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key-value-here-1234567890abcdef")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAPIKeyAuth_RevokedKey_NotInList(t *testing.T) {
	// Revoked keys are filtered by ListAllActive, so the stub returns empty.
	stub := &stubKeyLister{keys: nil}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer some-revoked-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAPIKeyAuth_DBError(t *testing.T) {
	stub := &stubKeyLister{listErr: fmt.Errorf("connection refused")}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIKeyAuth_EmptyBearerToken(t *testing.T) {
	stub := &stubKeyLister{}

	handler := middleware.APIKeyAuth(stub)(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestUserIDFromContext_NotPresent(t *testing.T) {
	uid, ok := middleware.UserIDFromContext(context.Background())
	if ok || uid != 0 {
		t.Errorf("expected (0, false), got (%d, %v)", uid, ok)
	}
}
