package updatecheck_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ramonehamilton/mtga-daemon/internal/updatecheck"
)

// captureLog redirects log output to a buffer for the duration of f, then
// restores the original writer and returns the captured output.
func captureLog(f func()) string {
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(nil) // restore to os.Stderr default after test
	f()
	return buf.String()
}

func versionHandler(latest, downloadURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(updatecheck.VersionResponse{
			Latest:      latest,
			ReleasedAt:  "2026-05-01T12:00:00Z",
			DownloadURL: downloadURL,
		})
	})
}

func TestCheck_DevVersionSkipped(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	updatecheck.Check(context.Background(), srv.URL, "dev")

	if called {
		t.Error("expected server to not be called for 'dev' version")
	}
}

func TestCheck_EqualVersion_NoWarn(t *testing.T) {
	srv := httptest.NewServer(versionHandler("1.2.3", "https://example.com/releases/v1.2.3"))
	defer srv.Close()

	out := captureLog(func() {
		updatecheck.Check(context.Background(), srv.URL, "1.2.3")
	})

	if strings.Contains(out, "WARN") {
		t.Errorf("expected no WARN for equal version, got: %s", out)
	}
}

func TestCheck_NewerRemoteVersion_LogsWarn(t *testing.T) {
	const dlURL = "https://example.com/releases/v1.3.0"
	srv := httptest.NewServer(versionHandler("1.3.0", dlURL))
	defer srv.Close()

	out := captureLog(func() {
		updatecheck.Check(context.Background(), srv.URL, "1.2.0")
	})

	if !strings.Contains(out, "WARN") {
		t.Errorf("expected WARN log for newer version, got: %s", out)
	}
	if !strings.Contains(out, "1.3.0") {
		t.Errorf("expected new version in log, got: %s", out)
	}
	if !strings.Contains(out, dlURL) {
		t.Errorf("expected download URL in log, got: %s", out)
	}
}

func TestCheck_OlderRemoteVersion_NoWarn(t *testing.T) {
	srv := httptest.NewServer(versionHandler("1.1.0", "https://example.com/releases/v1.1.0"))
	defer srv.Close()

	out := captureLog(func() {
		updatecheck.Check(context.Background(), srv.URL, "1.2.0")
	})

	if strings.Contains(out, "WARN") {
		t.Errorf("expected no WARN when remote version is older, got: %s", out)
	}
}

func TestCheck_NetworkError_NoFatal(t *testing.T) {
	// Point at a server that is already closed so the request fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	// Must not panic or call t.Fatal; just log at INFO level.
	out := captureLog(func() {
		updatecheck.Check(context.Background(), srv.URL, "1.2.0")
	})

	if strings.Contains(out, "WARN") {
		t.Errorf("expected no WARN on network error, got: %s", out)
	}
	if !strings.Contains(out, "version check failed") {
		t.Errorf("expected info log about failure, got: %s", out)
	}
}

func TestCheck_Non200Response_Handled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	out := captureLog(func() {
		updatecheck.Check(context.Background(), srv.URL, "1.2.0")
	})

	if strings.Contains(out, "WARN") {
		t.Errorf("expected no WARN for non-200 response, got: %s", out)
	}
	if !strings.Contains(out, "503") {
		t.Errorf("expected status code in log, got: %s", out)
	}
}

func TestCheck_MalformedJSON_Handled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	out := captureLog(func() {
		updatecheck.Check(context.Background(), srv.URL, "1.2.0")
	})

	if strings.Contains(out, "WARN") {
		t.Errorf("expected no WARN for malformed JSON, got: %s", out)
	}
	if !strings.Contains(out, "failed to decode") {
		t.Errorf("expected decode error in log, got: %s", out)
	}
}

func TestCheck_UserAgentHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(updatecheck.VersionResponse{Latest: "1.2.0"})
	}))
	defer srv.Close()

	updatecheck.Check(context.Background(), srv.URL, "1.2.0")

	if gotUA != "mtga-daemon/1.2.0" {
		t.Errorf("expected User-Agent 'mtga-daemon/1.2.0', got %q", gotUA)
	}
}
