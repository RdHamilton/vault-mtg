package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

const registerSecret = "register-test-secret"

// reqWithUserID builds a POST request with the given user ID injected into
// context via the middleware helper (simulates APIKeyAuth having run).
func reqWithUserID(userID int64) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/daemon/register", nil)
	return r.WithContext(middleware.WithUserID(r.Context(), userID))
}

func TestDaemonRegister_Success(t *testing.T) {
	h := handlers.NewDaemonRegisterHandler(registerSecret)

	req := reqWithUserID(5)
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Token    string `json:"token"`
		DaemonID string `json:"daemon_id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.DaemonID == "" {
		t.Error("expected non-empty daemon_id")
	}

	// Validate the token contains the correct user_id from context, not body.
	var claims middleware.DaemonClaims
	tok, err := jwt.ParseWithClaims(resp.Token, &claims, func(t *jwt.Token) (any, error) {
		return []byte(registerSecret), nil
	})
	if err != nil || !tok.Valid {
		t.Fatalf("token invalid: %v", err)
	}
	if claims.UserID != 5 {
		t.Errorf("expected user_id=5, got %d", claims.UserID)
	}
	if claims.DaemonID != resp.DaemonID {
		t.Errorf("daemon_id mismatch: claims=%q body=%q", claims.DaemonID, resp.DaemonID)
	}
}

// TestDaemonRegister_MissingContext verifies that a request without an
// authenticated user ID (i.e. APIKeyAuth middleware was skipped) is rejected.
func TestDaemonRegister_MissingContext(t *testing.T) {
	h := handlers.NewDaemonRegisterHandler(registerSecret)

	// No user_id in context — simulates unauthenticated call.
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/register", nil)
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestDaemonRegister_BodyUserIDIgnored verifies that a body containing a
// different user_id does NOT affect the JWT — the context value wins.
func TestDaemonRegister_BodyUserIDIgnored(t *testing.T) {
	h := handlers.NewDaemonRegisterHandler(registerSecret)

	// Context says user 7; body says user 999.
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/register",
		strings.NewReader(`{"user_id":999}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserID(req.Context(), 7))
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var resp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var claims middleware.DaemonClaims
	if _, err := jwt.ParseWithClaims(resp.Token, &claims, func(t *jwt.Token) (any, error) {
		return []byte(registerSecret), nil
	}); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != 7 {
		t.Errorf("expected user_id=7 from context, got %d", claims.UserID)
	}
}

func TestDaemonRegister_EmptySecret(t *testing.T) {
	h := handlers.NewDaemonRegisterHandler("")

	req := reqWithUserID(1)
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for missing secret, got %d", rr.Code)
	}
}

func TestDaemonRegister_TokenIsValidForIngestMiddleware(t *testing.T) {
	h := handlers.NewDaemonRegisterHandler(registerSecret)

	req := reqWithUserID(99)
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", rr.Code)
	}

	var resp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Run the issued token through DaemonJWTAuth middleware.
	var capturedUID int64
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUID, _ = middleware.DaemonUserIDFromContext(r.Context())
	})

	mwHandler := middleware.DaemonJWTAuth(registerSecret)(capture)
	ingestReq := httptest.NewRequest(http.MethodPost, "/v1/ingest/events", nil)
	ingestReq.Header.Set("Authorization", "Bearer "+resp.Token)
	ingestRR := httptest.NewRecorder()

	mwHandler.ServeHTTP(ingestRR, ingestReq)

	if ingestRR.Code != http.StatusOK {
		t.Fatalf("ingest middleware: expected 200, got %d", ingestRR.Code)
	}
	if capturedUID != 99 {
		t.Errorf("expected user_id=99, got %d", capturedUID)
	}
}
