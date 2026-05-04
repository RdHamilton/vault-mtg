package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
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

	// Assert ExpiresAt is within the expected 30-day window.
	if claims.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set")
	}
	now := time.Now().UTC()
	lo := now.Add(29 * 24 * time.Hour)
	hi := now.Add(31 * 24 * time.Hour)
	exp := claims.ExpiresAt.Time
	if exp.Before(lo) || exp.After(hi) {
		t.Errorf("ExpiresAt %v is not within [now+29d, now+31d]", exp)
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

// TestDaemonRegister_RouterIntegration mounts the real routes with real
// middleware and exercises the full auth chain end-to-end.
//
//  1. POST /api/daemon/register with a seeded user-ID context → expect 201 + token.
//  2. POST /v1/ingest/events with "Authorization: Bearer <token>" → expect 202 and
//     DaemonUserIDFromContext set to the registered user ID.
//  3. POST /api/daemon/register with no user-ID context → expect 401.
func TestDaemonRegister_RouterIntegration(t *testing.T) {
	const secret = "router-integration-secret"
	const wantUserID int64 = 55

	// --- build the router ------------------------------------------------
	r := chi.NewRouter()

	registerHandler := handlers.NewDaemonRegisterHandler(secret)
	r.Post("/api/daemon/register", registerHandler.Register)

	// Capture the DaemonUserID that the ingest handler sees so we can assert it.
	var capturedDaemonUID int64
	ingestInner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedDaemonUID, _ = middleware.DaemonUserIDFromContext(req.Context())
		w.WriteHeader(http.StatusAccepted)
	})
	r.With(middleware.DaemonJWTAuth(secret)).Post("/v1/ingest/events", ingestInner)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// --- 1. register with a valid user-ID context ------------------------
	regReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/daemon/register", nil)
	// Inject the user ID the same way APIKeyAuth middleware would.
	// We wrap the server handler for this one call so we can seed the context.
	var token string
	{
		rr := httptest.NewRecorder()
		fakeReq := httptest.NewRequest(http.MethodPost, "/api/daemon/register", nil)
		fakeReq = fakeReq.WithContext(middleware.WithUserID(fakeReq.Context(), wantUserID))
		registerHandler.Register(rr, fakeReq)

		if rr.Code != http.StatusCreated {
			t.Fatalf("register: expected 201, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("register decode: %v", err)
		}
		token = resp.Token
	}
	_ = regReq // suppress unused warning

	// --- 2. ingest with the issued token ----------------------------------
	eventBody, _ := json.Marshal(map[string]interface{}{
		"type":        "draft:pick",
		"account_id":  "acct_test",
		"session_id":  "sess_test",
		"occurred_at": time.Now().UTC(),
		"payload":     json.RawMessage(`{}`),
	})
	ingestReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ingest/events",
		bytes.NewReader(eventBody))
	ingestReq.Header.Set("Content-Type", "application/json")
	ingestReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(ingestReq)
	if err != nil {
		t.Fatalf("ingest request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("ingest: expected 202, got %d", resp.StatusCode)
	}
	if capturedDaemonUID != wantUserID {
		t.Errorf("DaemonUserIDFromContext=%d, want %d", capturedDaemonUID, wantUserID)
	}

	// --- 3. register with no user-ID context → 401 -----------------------
	{
		rr := httptest.NewRecorder()
		noCtxReq := httptest.NewRequest(http.MethodPost, "/api/daemon/register", nil)
		// No middleware.WithUserID — context is empty.
		registerHandler.Register(rr, noCtxReq)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("no-context register: expected 401, got %d", rr.Code)
		}
	}
}
