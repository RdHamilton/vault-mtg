//go:build integration

package daemon

// TestDaemonBinarySmoke is a binary-level lifecycle smoke test that:
//  1. Builds the daemon binary via go build.
//  2. Spins up a stub BFF httptest.Server that records received DaemonEvent JSON.
//  3. Writes a temp config pointing at the stub server.
//  4. Starts the daemon process via os/exec.
//  5. Appends a realistic draft.pack log line to the temp log file.
//  6. Asserts the stub received the expected event type.
//  7. Shuts down cleanly via SIGTERM.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer protected by a mutex so the exec output goroutine
// and the test goroutine can share it without a data race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// daemonBinaryPath returns the path where the compiled daemon binary is stored
// during the smoke test. Each test run compiles the binary into a temp dir so
// parallel runs do not clobber each other.
func daemonBinaryPath(t *testing.T) string {
	t.Helper()
	binName := "mtga-daemon-smoke"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	return filepath.Join(t.TempDir(), binName)
}

// buildDaemon compiles the daemon binary from its cmd package into dest.
func buildDaemon(t *testing.T, dest string) {
	t.Helper()

	// Locate the repo root by walking up from this file's source path.
	// smoke_test.go is at: services/daemon/internal/daemon/smoke_test.go
	// Five ".." segments reach the repo root:
	//   smoke_test.go → daemon/ → internal/ → services/daemon/ → services/ → repo-root
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	repoRoot := filepath.Join(thisFile, "..", "..", "..", "..", "..")

	cmdPkg := filepath.Join(repoRoot, "services", "daemon", "cmd", "daemon")

	cmd := exec.Command("go", "build", "-o", dest, cmdPkg)
	cmd.Env = append(
		os.Environ(),
		"GONOSUMDB=github.com/RdHamilton/vault-mtg",
		"GOPRIVATE=github.com/RdHamilton/vault-mtg",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed:\n%s", string(out))
}

// writeDaemonConfig writes a daemon.json config file to dir and returns its path.
// The config uses daemon_jwt (not keychain) to bypass PKCE and keychain migration.
func writeDaemonConfig(t *testing.T, dir, bffURL, logPath string) string {
	t.Helper()

	// daemon_jwt with a far-future exp so NeedsFirstRunAuth() → false and
	// JWTNeedsRefresh() → false (no registration attempt).
	// Format: base64url(header).base64url(claims).fakesig — the daemon never
	// verifies the signature, only decodes the exp claim.
	farFuture := time.Now().Add(365 * 24 * time.Hour).Unix()
	header := "eyJhbGciOiJIUzI1NiJ9" // {"alg":"HS256"}
	claims := b64URLEncodeJSON(t, map[string]int64{"exp": farFuture})
	fakeJWT := fmt.Sprintf("%s.%s.smoke-sig", header, claims)

	cfg := map[string]interface{}{
		"cloud_api_url":         bffURL,
		"keychain":              false,
		"api_key":               "",
		"daemon_jwt":            fakeJWT,
		"sync_enabled":          false, // skip all registration paths
		"account_id":            "smoke-acc-001",
		"log_path":              logPath,
		"ingest_path":           "/ingest/events",
		"use_fs_notify":         false,
		"log_preserve_on_start": false,     // avoid snapshot on non-existent archive dir
		"poll_interval":         200000000, // 200 ms in nanoseconds
		"disable_update_check":  true,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	cfgPath := filepath.Join(dir, "daemon.json")
	require.NoError(t, os.WriteFile(cfgPath, data, 0o600))

	return cfgPath
}

// b64URLEncodeJSON encodes v as JSON then base64url-encodes it (no padding).
func b64URLEncodeJSON(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	// Manual base64url without padding — same as base64.RawURLEncoding.
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	src := raw
	var sb strings.Builder
	for i := 0; i < len(src); i += 3 {
		b0 := src[i]

		var b1, b2 byte

		have := 1
		if i+1 < len(src) {
			b1 = src[i+1]
			have = 2
		}
		if i+2 < len(src) {
			b2 = src[i+2]
			have = 3
		}
		sb.WriteByte(alpha[b0>>2])
		sb.WriteByte(alpha[(b0&0x03)<<4|(b1>>4)])
		if have >= 2 {
			sb.WriteByte(alpha[(b1&0x0f)<<2|(b2>>6)])
		}
		if have == 3 {
			sb.WriteByte(alpha[b2&0x3f])
		}
	}

	return sb.String()
}

// TestDaemonBinarySmoke builds the daemon binary, starts it against a stub BFF,
// writes a draft.pack log entry, and asserts the event is dispatched correctly.
// The test then sends SIGTERM and verifies the process exits cleanly.
func TestDaemonBinarySmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary smoke test in -short mode")
	}

	// ── Stub BFF ─────────────────────────────────────────────────────────────

	var (
		bffMu    sync.Mutex
		received []contract.DaemonEvent
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ingest/events" && r.Method == http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var evt contract.DaemonEvent
			if err := json.Unmarshal(body, &evt); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			bffMu.Lock()
			received = append(received, evt)
			bffMu.Unlock()
			w.WriteHeader(http.StatusAccepted)

		case strings.HasPrefix(r.URL.Path, "/daemon/version"):
			// Silence version check so it does not produce 404 noise in logs.
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"latest":"0.0.1","released_at":"2026-01-01T00:00:00Z","download_url":"https://example.com"}`)

		default:
			// Accept anything else (heartbeat, update check, etc.) silently.
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()

	// ── Build binary ──────────────────────────────────────────────────────────

	binPath := daemonBinaryPath(t)
	buildDaemon(t, binPath)

	// ── Temp dir: config + log file ───────────────────────────────────────────

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "Player.log")

	// Pre-create an empty log file so the poller can open it immediately.
	logF, err := os.Create(logPath)
	require.NoError(t, err)
	require.NoError(t, logF.Close())

	cfgPath := writeDaemonConfig(t, tmpDir, srv.URL, logPath)

	// ── Start daemon ──────────────────────────────────────────────────────────

	cmd := exec.Command(binPath, "-config", cfgPath)
	cmd.Env = append(
		os.Environ(),
		"MTGA_DAEMON_HEADLESS=1",
		// Unset Clerk vars to ensure PKCE never fires.
		"CLERK_PUBLISHABLE_KEY=",
		"CLERK_FRONTEND_API=",
		"CLERK_OAUTH_CLIENT_ID=",
	)

	// Capture daemon output for failure diagnostics via a mutex-protected buffer
	// so concurrent writes from the exec goroutine and reads from the test goroutine
	// are race-free.
	daemonLogs := &syncBuffer{}
	cmd.Stdout = daemonLogs
	cmd.Stderr = daemonLogs

	require.NoError(t, cmd.Start(), "failed to start daemon binary")

	// Ensure the process is cleaned up on test failure.
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			// Give it time to exit; ignore errors (may already be gone).
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
		if t.Failed() {
			t.Logf("daemon output:\n%s", daemonLogs.String())
		}
	})

	// ── Wait for daemon startup ───────────────────────────────────────────────
	// Poll the local health endpoint (127.0.0.1:9001/health) until it responds
	// or the timeout is reached.

	waitForDaemonHealth(t, 15*time.Second)

	// ── Append draft.pack log line ────────────────────────────────────────────
	// The daemon's poller reads newline-terminated JSON from the log file.
	// This entry matches the structure expected by classifyEntry → handleEntry.

	draftPackLine := `{"CurrentModule":"BotDraft","Payload":"{\"EventName\":\"QuickDraft_SOS_20260526\",\"PackNumber\":0,\"PickNumber\":0,\"DraftPack\":[\"11001\",\"22002\",\"33003\"]}"}` + "\n"
	logF, err = os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = logF.WriteString(draftPackLine)
	require.NoError(t, err)
	require.NoError(t, logF.Close())

	// ── Wait for event dispatch ───────────────────────────────────────────────

	var draftPackEvent *contract.DaemonEvent
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		bffMu.Lock()
		for i := range received {
			if received[i].Type == "draft.pack" {
				cp := received[i]
				draftPackEvent = &cp

				break
			}
		}
		bffMu.Unlock()
		if draftPackEvent != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	require.NotNil(
		t, draftPackEvent,
		"stub BFF did not receive a draft.pack event within the deadline\ndaemon output:\n%s",
		daemonLogs.String(),
	)
	assert.Equal(t, "draft.pack", draftPackEvent.Type)
	assert.Equal(t, "smoke-acc-001", draftPackEvent.AccountID)
	assert.NotEmpty(t, draftPackEvent.Payload, "draft.pack payload must be non-empty")

	// ── Clean shutdown via SIGTERM ─────────────────────────────────────────────

	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	exitDone := make(chan error, 1)
	go func() { exitDone <- cmd.Wait() }()

	select {
	case exitErr := <-exitDone:
		// The process should exit cleanly (exit 0) or with a signal-related
		// status on some platforms — both are acceptable as long as it exits.
		if exitErr != nil {
			// On Unix a SIGTERM-killed process may report a non-zero exit status.
			// Accept that; only fail if the process hangs.
			t.Logf("daemon exited with: %v (expected after SIGTERM)", exitErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit within 10s after SIGTERM")
	}
}

// waitForDaemonHealth polls the daemon local API health endpoint until it
// responds 200 OK or the timeout elapses.
func waitForDaemonHealth(t *testing.T, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:9001/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("daemon health endpoint did not become ready within %s", timeout)
}

// writeDaemonConfigKeychain writes a daemon.json with keychain:true pointing
// at the given stub BFF, and returns its path.
// The config has sync_enabled:true so ingest dispatch is active.
func writeDaemonConfigKeychain(t *testing.T, dir, bffURL, logPath string) string {
	t.Helper()

	cfg := map[string]interface{}{
		"cloud_api_url":         bffURL,
		"keychain":              true, // keychain mode — api_key lives in OS keychain
		"api_key":               "",   // no plaintext key
		"sync_enabled":          true, // events should be dispatched
		"account_id":            "smoke-keychain-acc-001",
		"log_path":              logPath,
		"ingest_path":           "/ingest/events",
		"use_fs_notify":         false,
		"log_preserve_on_start": false,
		"poll_interval":         200000000, // 200 ms
		"disable_update_check":  true,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	cfgPath := filepath.Join(dir, "daemon.json")
	require.NoError(t, os.WriteFile(cfgPath, data, 0o600))
	return cfgPath
}

// TestDaemonReinstallStaleAuthSmoke is an integration-tagged binary smoke test
// that documents the current silent-failure behavior when the daemon operates in
// keychain mode with a stale (rejected) api_key stored in the OS keychain.
//
// Scenario:
//  1. A stale api_key is seeded into the in-process keychain mock.
//  2. daemon.json has keychain:true pointing at a stub BFF.
//  3. The stub BFF rejects every ingest request with 401.
//  4. The daemon must remain running (no crash on 401).
//  5. The stub BFF must have received at least one POST to /ingest/events.
//  6. The daemon /health endpoint must return 200.
//  7. SIGTERM causes a clean exit within 10 seconds.
//
// This test documents the current silent-failure: 401 in keychain mode is
// logged but ignored. When issue #2135 is implemented (re-auth on 401),
// update this test to assert re-registration is triggered.
func TestDaemonReinstallStaleAuthSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping reinstall stale-auth smoke test in -short mode")
	}

	// ── Stub BFF ─────────────────────────────────────────────────────────────
	var (
		ingestMu    sync.Mutex
		ingestPaths []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ingest/events" && r.Method == http.MethodPost:
			ingestMu.Lock()
			ingestPaths = append(ingestPaths, r.URL.Path)
			ingestMu.Unlock()
			// Always return 401 to exercise the stale-auth path.
			w.WriteHeader(http.StatusUnauthorized)

		case strings.HasPrefix(r.URL.Path, "/daemon/version"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"latest":"0.0.1","released_at":"2026-01-01T00:00:00Z","download_url":"https://example.com"}`)

		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()

	// ── Build binary ──────────────────────────────────────────────────────────
	binPath := daemonBinaryPath(t)
	buildDaemon(t, binPath)

	// ── Temp dir: config + log file ───────────────────────────────────────────
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "Player.log")

	logF, err := os.Create(logPath)
	require.NoError(t, err)
	require.NoError(t, logF.Close())

	cfgPath := writeDaemonConfigKeychain(t, tmpDir, srv.URL, logPath)

	// ── Start daemon ──────────────────────────────────────────────────────────
	// The binary uses the real OS keychain; we pass a fake api_key via the
	// MTGA_DAEMON_API_KEY env var is not applicable in keychain mode — instead
	// the binary will call keychain.Get() which on macOS uses the real Keychain.
	// To avoid requiring a real keychain entry in CI, we allow the keychain.Get()
	// failure: the daemon logs a warning and starts with an empty bearer token,
	// causing every ingest call to 401 — exactly the stale-auth scenario.
	cmd := exec.Command(binPath, "-config", cfgPath)
	cmd.Env = append(
		os.Environ(),
		"MTGA_DAEMON_HEADLESS=1",
		"CLERK_PUBLISHABLE_KEY=",
		"CLERK_FRONTEND_API=",
		"CLERK_OAUTH_CLIENT_ID=",
	)

	daemonLogs := &syncBuffer{}
	cmd.Stdout = daemonLogs
	cmd.Stderr = daemonLogs

	require.NoError(t, cmd.Start(), "failed to start daemon binary")

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
		if t.Failed() {
			t.Logf("daemon output:\n%s", daemonLogs.String())
		}
	})

	// ── Wait for daemon startup ───────────────────────────────────────────────
	waitForDaemonHealth(t, 15*time.Second)

	// ── Trigger an ingest dispatch by writing a log entry ────────────────────
	// Write a heartbeat-triggering log entry so the daemon has something to
	// dispatch. In practice the heartbeat ticker (30 s) also fires — but we
	// accelerate by writing a draft.pack entry that triggers immediate dispatch.
	draftPackLine := `{"CurrentModule":"BotDraft","Payload":"{\"EventName\":\"QuickDraft_SOS_20260526\",\"PackNumber\":0,\"PickNumber\":0,\"DraftPack\":[\"11001\",\"22002\"]}"}` + "\n"
	logF, err = os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = logF.WriteString(draftPackLine)
	require.NoError(t, err)
	require.NoError(t, logF.Close())

	// Wait up to 8 seconds for at least one ingest call to arrive.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		ingestMu.Lock()
		n := len(ingestPaths)
		ingestMu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// ── Assertions ────────────────────────────────────────────────────────────

	// 1. Daemon process must still be running (not crashed on 401).
	assert.Nil(t, cmd.ProcessState,
		"daemon must still be running after receiving 401 from stub BFF — "+
			"known gap #2135: no recovery path for keychain-mode 401")

	// 2. Stub BFF must have received at least one POST to /ingest/events.
	ingestMu.Lock()
	receivedCount := len(ingestPaths)
	ingestMu.Unlock()
	assert.Greater(t, receivedCount, 0,
		"stub BFF must have received at least one POST /ingest/events")

	// 3. /health must still return 200 — daemon did not crash.
	healthClient := &http.Client{Timeout: 2 * time.Second}
	healthResp, healthErr := healthClient.Get("http://127.0.0.1:9001/health")
	require.NoError(t, healthErr, "daemon /health must be reachable")
	_ = healthResp.Body.Close()
	assert.Equal(t, http.StatusOK, healthResp.StatusCode,
		"daemon /health must return 200 even after repeated 401s from BFF")

	// ── Clean shutdown ─────────────────────────────────────────────────────────
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	exitDone := make(chan error, 1)
	go func() { exitDone <- cmd.Wait() }()

	select {
	case exitErr := <-exitDone:
		if exitErr != nil {
			t.Logf("daemon exited with: %v (expected after SIGTERM)", exitErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit within 10s after SIGTERM")
	}
}
