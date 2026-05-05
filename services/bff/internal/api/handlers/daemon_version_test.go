package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/config"
)

func newVersionConfig(version, releasedAt string) *config.Config {
	return &config.Config{
		DaemonLatestVersion: version,
		DaemonReleasedAt:    releasedAt,
	}
}

// TestGetDaemonVersion_HappyPath verifies the response body, status code, and
// Cache-Control header when the config has a non-empty version.
func TestGetDaemonVersion_HappyPath(t *testing.T) {
	cfg := newVersionConfig("0.3.0", "2026-05-01T12:00:00Z")
	h := handlers.NewDaemonVersionHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemon/version", nil)
	rec := httptest.NewRecorder()

	h.GetDaemonVersion(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", res.StatusCode, http.StatusOK)
	}

	cc := res.Header.Get("Cache-Control")
	if cc != "public, max-age=300" {
		t.Errorf("Cache-Control: got %q, want %q", cc, "public, max-age=300")
	}

	ct := res.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var resp contract.DaemonVersionResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.Latest != "0.3.0" {
		t.Errorf("latest: got %q, want %q", resp.Latest, "0.3.0")
	}

	if resp.ReleasedAt != "2026-05-01T12:00:00Z" {
		t.Errorf("released_at: got %q, want %q", resp.ReleasedAt, "2026-05-01T12:00:00Z")
	}

	wantURL := "https://github.com/RdHamilton/MTGA-Companion/releases/tag/daemon/v0.3.0"
	if resp.DownloadURL != wantURL {
		t.Errorf("download_url: got %q, want %q", resp.DownloadURL, wantURL)
	}
}

// TestGetDaemonVersion_MissingVersion verifies that when DaemonLatestVersion is
// empty (e.g. misconfigured but not fatal), the handler still returns 200 with
// an empty string for latest — callers treat "" as "unknown".
func TestGetDaemonVersion_MissingVersion(t *testing.T) {
	cfg := newVersionConfig("", "")
	h := handlers.NewDaemonVersionHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemon/version", nil)
	rec := httptest.NewRecorder()

	h.GetDaemonVersion(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", res.StatusCode, http.StatusOK)
	}

	var resp contract.DaemonVersionResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.Latest != "" {
		t.Errorf("latest: got %q, want empty string", resp.Latest)
	}

	// Cache-Control must still be set even for an empty-config response.
	cc := res.Header.Get("Cache-Control")
	if cc != "public, max-age=300" {
		t.Errorf("Cache-Control: got %q, want %q", cc, "public, max-age=300")
	}
}

// TestGetDaemonVersion_DefaultDownloadURL verifies that the download URL is
// constructed correctly from the version string.
func TestGetDaemonVersion_DefaultDownloadURL(t *testing.T) {
	cfg := newVersionConfig("0.1.0", "")
	h := handlers.NewDaemonVersionHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemon/version", nil)
	rec := httptest.NewRecorder()

	h.GetDaemonVersion(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	var resp contract.DaemonVersionResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	want := "https://github.com/RdHamilton/MTGA-Companion/releases/tag/daemon/v0.1.0"
	if resp.DownloadURL != want {
		t.Errorf("download_url: got %q, want %q", resp.DownloadURL, want)
	}
}
