package localapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/localapi"
)

func TestHealthReturnsState(t *testing.T) {
	started := time.Date(2026, 5, 11, 21, 0, 0, 0, time.UTC)
	srv := localapi.New(0, localapi.State{
		Version:   "0.3.1-rc17",
		SessionID: "live-test-session",
		StartedAt: started,
		AccountID: "user_abc",
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	resp, err := http.Get("http://" + srv.Addr() + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("content-type: got %q, want application/json", got)
	}

	var body struct {
		Status    string `json:"status"`
		Version   string `json:"version"`
		SessionID string `json:"session_id"`
		StartedAt string `json:"started_at"`
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("status: got %q, want ok", body.Status)
	}
	if body.Version != "0.3.1-rc17" {
		t.Errorf("version: got %q", body.Version)
	}
	if body.SessionID != "live-test-session" {
		t.Errorf("session_id: got %q", body.SessionID)
	}
	if body.AccountID != "user_abc" {
		t.Errorf("account_id: got %q", body.AccountID)
	}
	if body.StartedAt != "2026-05-11T21:00:00Z" {
		t.Errorf("started_at: got %q", body.StartedAt)
	}
}

func TestCORSPreflightAllowsKnownOrigin(t *testing.T) {
	srv := localapi.New(0, localapi.State{Version: "test"})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	req, _ := http.NewRequest(http.MethodOptions, "http://"+srv.Addr()+"/health", nil)
	req.Header.Set("Origin", "https://stg-app.vaultmtg.app")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("preflight status: got %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://stg-app.vaultmtg.app" {
		t.Errorf("Allow-Origin: got %q", got)
	}
}

func TestHealthRejectsPost(t *testing.T) {
	srv := localapi.New(0, localapi.State{Version: "test"})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	resp, err := http.Post("http://"+srv.Addr()+"/health", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", resp.StatusCode)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	srv := localapi.New(0, localapi.State{Version: "test"})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("first Stop: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("second Stop: %v", err)
	}
}

func TestStopBeforeStartIsNoop(t *testing.T) {
	srv := localapi.New(0, localapi.State{Version: "test"})
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop before Start: %v", err)
	}
}

// TestHealthAuthStatusField verifies that auth_status is always present in the
// /health JSON response and reflects the value set on State.
func TestHealthAuthStatusField(t *testing.T) {
	cases := []struct {
		name       string
		authStatus string
	}{
		{"authenticated", localapi.AuthStatusAuthenticated},
		{"setup_required", localapi.AuthStatusSetupRequired},
		{"keychain_error", localapi.AuthStatusKeychainError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := localapi.New(0, localapi.State{
				Version:    "test",
				AuthStatus: tc.authStatus,
			})
			if err := srv.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer func() { _ = srv.Stop() }()

			resp, err := http.Get("http://" + srv.Addr() + "/health")
			if err != nil {
				t.Fatalf("GET /health: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status: got %d, want 200", resp.StatusCode)
			}

			var body struct {
				AuthStatus string `json:"auth_status"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.AuthStatus != tc.authStatus {
				t.Errorf("auth_status: got %q, want %q", body.AuthStatus, tc.authStatus)
			}
		})
	}
}

// TestHealthAuthStatusAlwaysPresent verifies that auth_status is always
// serialized (no omitempty), so an empty string is visible as a derivation bug
// rather than silently absent.
func TestHealthAuthStatusAlwaysPresent(t *testing.T) {
	srv := localapi.New(0, localapi.State{Version: "test"}) // AuthStatus intentionally empty
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	resp, err := http.Get("http://" + srv.Addr() + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["auth_status"]; !ok {
		t.Error("auth_status key must always be present in /health response (no omitempty)")
	}
}
