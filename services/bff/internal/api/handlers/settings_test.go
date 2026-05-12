package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type settingsAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (s *settingsAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return s.accountID, s.found, s.err
}

type stubSettingsReader struct {
	listResult     map[string]json.RawMessage
	listErr        error
	getValue       json.RawMessage
	getPresent     bool
	getErr         error
	upsertErr      error
	upsertedKey    string
	upsertedValue  json.RawMessage
	upsertManyErr  error
	upsertedValues map[string]json.RawMessage
}

func (s *stubSettingsReader) ListByAccount(_ context.Context, _ int64) (map[string]json.RawMessage, error) {
	return s.listResult, s.listErr
}

func (s *stubSettingsReader) Get(_ context.Context, _ int64, _ string) (json.RawMessage, bool, error) {
	return s.getValue, s.getPresent, s.getErr
}

func (s *stubSettingsReader) Upsert(_ context.Context, _ int64, key string, value json.RawMessage) error {
	s.upsertedKey = key
	s.upsertedValue = value
	return s.upsertErr
}

func (s *stubSettingsReader) UpsertMany(_ context.Context, _ int64, values map[string]json.RawMessage) error {
	s.upsertedValues = values
	return s.upsertManyErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedSettingsRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeSettingsEnvelope(t *testing.T, body []byte, into any) {
	t.Helper()
	wrapper := struct {
		Data json.RawMessage `json:"data"`
	}{}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatalf("envelope decode: %v body=%s", err, string(body))
	}
	if err := json.Unmarshal(wrapper.Data, into); err != nil {
		t.Fatalf("payload decode: %v data=%s", err, string(wrapper.Data))
	}
}

func chiSettingsContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newSettingsHandler(s *stubSettingsReader) *handlers.SettingsHandler {
	return handlers.NewSettingsHandler(s, &settingsAccountLookup{accountID: 7, found: true})
}

// ─── GET /settings ──────────────────────────────────────────────────────────

func TestSettingsGetAll_HappyPath(t *testing.T) {
	reader := &stubSettingsReader{listResult: map[string]json.RawMessage{
		"theme":       json.RawMessage(`"dark"`),
		"autoRefresh": json.RawMessage(`true`),
		"metaWeight":  json.RawMessage(`0.6`),
	}}
	h := newSettingsHandler(reader)
	req := authedSettingsRequest(t, http.MethodGet, "/api/v1/settings", nil, 168)
	rr := httptest.NewRecorder()
	h.GetSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeSettingsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["theme"] != "dark" || resp["autoRefresh"] != true {
		t.Errorf("payload: %v", resp)
	}
}

func TestSettingsGetAll_AccountNotFoundReturnsEmpty(t *testing.T) {
	h := handlers.NewSettingsHandler(&stubSettingsReader{}, &settingsAccountLookup{found: false})
	req := authedSettingsRequest(t, http.MethodGet, "/api/v1/settings", nil, 168)
	rr := httptest.NewRecorder()
	h.GetSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeSettingsEnvelope(t, rr.Body.Bytes(), &resp)
	if len(resp) != 0 {
		t.Errorf("expected empty map, got %v", resp)
	}
}

func TestSettingsGetAll_RepoError(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{listErr: errors.New("boom")})
	req := authedSettingsRequest(t, http.MethodGet, "/api/v1/settings", nil, 168)
	rr := httptest.NewRecorder()
	h.GetSettings(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── PUT /settings ──────────────────────────────────────────────────────────

func TestSettingsUpdateAll_HappyPath(t *testing.T) {
	reader := &stubSettingsReader{}
	h := newSettingsHandler(reader)
	body, _ := json.Marshal(map[string]any{
		"theme":       "light",
		"autoRefresh": false,
		"metaWeight":  0.4,
	})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings", body, 168)
	rr := httptest.NewRecorder()
	h.UpdateSettings(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if len(reader.upsertedValues) != 3 {
		t.Fatalf("expected 3 keys, got %v", reader.upsertedValues)
	}
	if string(reader.upsertedValues["theme"]) != `"light"` {
		t.Errorf("theme value: %s", string(reader.upsertedValues["theme"]))
	}
}

func TestSettingsUpdateAll_RejectsEmptyBody(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings", []byte{}, 168)
	rr := httptest.NewRecorder()
	h.UpdateSettings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestSettingsUpdateAll_RejectsNonObject(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings", []byte(`["not","an","object"]`), 168)
	rr := httptest.NewRecorder()
	h.UpdateSettings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestSettingsUpdateAll_AccountNotFound(t *testing.T) {
	h := handlers.NewSettingsHandler(&stubSettingsReader{}, &settingsAccountLookup{found: false})
	body, _ := json.Marshal(map[string]any{"theme": "dark"})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings", body, 168)
	rr := httptest.NewRecorder()
	h.UpdateSettings(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestSettingsUpdateAll_RepoError(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{upsertManyErr: errors.New("boom")})
	body, _ := json.Marshal(map[string]any{"theme": "dark"})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings", body, 168)
	rr := httptest.NewRecorder()
	h.UpdateSettings(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── GET /settings/{key} ────────────────────────────────────────────────────

func TestSettingsGetOne_HappyPath(t *testing.T) {
	reader := &stubSettingsReader{getValue: json.RawMessage(`"dark"`), getPresent: true}
	h := newSettingsHandler(reader)
	req := authedSettingsRequest(t, http.MethodGet, "/api/v1/settings/theme", nil, 168)
	req = chiSettingsContext(req, "key", "theme")
	rr := httptest.NewRecorder()
	h.GetSetting(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeSettingsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["value"] != "dark" {
		t.Errorf("value: %v", resp)
	}
}

func TestSettingsGetOne_NotFound(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{getPresent: false})
	req := authedSettingsRequest(t, http.MethodGet, "/api/v1/settings/theme", nil, 168)
	req = chiSettingsContext(req, "key", "theme")
	rr := httptest.NewRecorder()
	h.GetSetting(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestSettingsGetOne_RequiresKey(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{})
	req := authedSettingsRequest(t, http.MethodGet, "/api/v1/settings/", nil, 168)
	req = chiSettingsContext(req, "key", "")
	rr := httptest.NewRecorder()
	h.GetSetting(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── PUT /settings/{key} ────────────────────────────────────────────────────

func TestSettingsUpdateOne_HappyPath(t *testing.T) {
	reader := &stubSettingsReader{}
	h := newSettingsHandler(reader)
	body, _ := json.Marshal(map[string]any{"value": "light"})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings/theme", body, 168)
	req = chiSettingsContext(req, "key", "theme")
	rr := httptest.NewRecorder()
	h.UpdateSetting(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.upsertedKey != "theme" {
		t.Errorf("key: %s", reader.upsertedKey)
	}
	if string(reader.upsertedValue) != `"light"` {
		t.Errorf("value: %s", string(reader.upsertedValue))
	}
}

func TestSettingsUpdateOne_AcceptsComplexValue(t *testing.T) {
	reader := &stubSettingsReader{}
	h := newSettingsHandler(reader)
	body, _ := json.Marshal(map[string]any{"value": map[string]any{"primary": "blue", "weight": 0.6}})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings/colorPalette", body, 168)
	req = chiSettingsContext(req, "key", "colorPalette")
	rr := httptest.NewRecorder()
	h.UpdateSetting(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.upsertedKey != "colorPalette" {
		t.Errorf("key: %s", reader.upsertedKey)
	}
	var decoded map[string]any
	if err := json.Unmarshal(reader.upsertedValue, &decoded); err != nil {
		t.Fatalf("decode persisted value: %v", err)
	}
	if decoded["primary"] != "blue" {
		t.Errorf("decoded: %v", decoded)
	}
}

func TestSettingsUpdateOne_RejectsMissingValue(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{})
	body, _ := json.Marshal(map[string]any{})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings/theme", body, 168)
	req = chiSettingsContext(req, "key", "theme")
	rr := httptest.NewRecorder()
	h.UpdateSetting(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestSettingsUpdateOne_RepoError(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{upsertErr: errors.New("boom")})
	body, _ := json.Marshal(map[string]any{"value": "dark"})
	req := authedSettingsRequest(t, http.MethodPut, "/api/v1/settings/theme", body, 168)
	req = chiSettingsContext(req, "key", "theme")
	rr := httptest.NewRecorder()
	h.UpdateSetting(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestSettingsUnauthorized(t *testing.T) {
	h := newSettingsHandler(&stubSettingsReader{})
	// No userID in context.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rr := httptest.NewRecorder()
	h.GetSettings(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}
