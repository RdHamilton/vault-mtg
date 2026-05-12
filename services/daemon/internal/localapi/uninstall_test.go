package localapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/localapi"
)

// stubUninstaller records what was passed to Run and returns the
// configured response. Lets tests assert the handler's behaviour
// without touching the host machine's launchctl / task scheduler.
type stubUninstaller struct {
	calls     atomic.Int32
	lastPurge atomic.Bool
	msg       string
	err       error
}

func (s *stubUninstaller) Run(purge bool) (string, error) {
	s.calls.Add(1)
	s.lastPurge.Store(purge)
	return s.msg, s.err
}

// newTestServer starts a localapi server on an OS-assigned port with the
// uninstaller stub injected. Caller is responsible for calling Stop.
func newTestServer(t *testing.T, stub *stubUninstaller) *localapi.Server {
	t.Helper()
	srv := localapi.New(0, localapi.State{
		Version:   "test",
		StartedAt: time.Now(),
	})
	srv.SetUninstaller(stub)
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return srv
}

func TestSystemUninstall_HappyPath_DefaultsToPurgeFalse(t *testing.T) {
	stub := &stubUninstaller{msg: "Daemon stopped. Drag VaultMTG to Trash."}
	// Block the post-response exit so the goroutine inside the handler
	// can't actually call os.Exit in tests.
	restore := localapi.SetShutdownExitForTest(func(_ int) {})
	t.Cleanup(restore)

	srv := newTestServer(t, stub)

	resp, err := http.Post("http://"+srv.Addr()+"/api/v1/system/uninstall", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "scheduled" {
		t.Errorf("status = %q, want scheduled", body.Status)
	}
	if body.Message != stub.msg {
		t.Errorf("message = %q, want %q", body.Message, stub.msg)
	}
	if stub.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", stub.calls.Load())
	}
	if stub.lastPurge.Load() {
		t.Errorf("purge = true, want false (default)")
	}
}

func TestSystemUninstall_PassesPurgeTrue(t *testing.T) {
	stub := &stubUninstaller{msg: "Daemon stopped and config wiped."}
	restore := localapi.SetShutdownExitForTest(func(_ int) {})
	t.Cleanup(restore)

	srv := newTestServer(t, stub)

	resp, err := http.Post(
		"http://"+srv.Addr()+"/api/v1/system/uninstall?purge=true",
		"application/json", nil,
	)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if !stub.lastPurge.Load() {
		t.Errorf("purge = false, want true")
	}
}

func TestSystemUninstall_PropagatesError(t *testing.T) {
	stub := &stubUninstaller{err: errors.New("boom")}
	restore := localapi.SetShutdownExitForTest(func(_ int) {})
	t.Cleanup(restore)

	srv := newTestServer(t, stub)

	resp, err := http.Post("http://"+srv.Addr()+"/api/v1/system/uninstall", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: %d, want 500", resp.StatusCode)
	}
	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Status != "error" {
		t.Errorf("status = %q, want error", body.Status)
	}
	if body.Message != "boom" {
		t.Errorf("message = %q, want %q", body.Message, "boom")
	}
}

func TestSystemUninstall_RejectsNonPOST(t *testing.T) {
	stub := &stubUninstaller{msg: "ok"}
	restore := localapi.SetShutdownExitForTest(func(_ int) {})
	t.Cleanup(restore)

	srv := newTestServer(t, stub)

	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/system/uninstall")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != "POST" {
		t.Errorf("Allow header = %q, want POST", got)
	}
	if stub.calls.Load() != 0 {
		t.Errorf("uninstaller called %d times for GET", stub.calls.Load())
	}
}
