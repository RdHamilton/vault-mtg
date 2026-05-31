//go:build integration && windows

// Package nsis_test is an integration-only, Windows-only lifecycle test for the
// NSIS daemon installer.  It runs inside the existing install-lifecycle-windows-nsis
// CI job (daemon-install-lifecycle.yml) after the NSIS installer has already been
// executed by the workflow's PowerShell steps.
//
// What this test asserts (Group A — stub BFF daemon_event, ticket #41):
//  1. Kills any pre-existing vaultmtg-daemon.exe process (idempotent).
//  2. Overwrites %APPDATA%\vaultmtg\daemon.json with a test config that points
//     at a stub BFF HTTP server and uses a fake far-future JWT to bypass PKCE.
//  3. Starts the daemon binary directly (not via Task Scheduler).
//  4. Waits for the daemon /health endpoint to return 200.
//  5. Writes a synthetic draft.pack log entry to trigger an immediate dispatch.
//  6. Asserts at least one contract.DaemonEvent with Type "draft.pack" and
//     AccountID "lifecycle-test-001" arrives at the stub BFF POST /ingest/events.
//  7. Terminates the daemon process cleanly.
//
// Auth strategy: same as TestDaemonBinarySmoke — pre-seed daemon_jwt (fake far-future
// token), keychain: false, sync_enabled: false, account_id: lifecycle-test-001.
// This bypasses PKCE entirely.  CLERK_* env vars are zeroed in the workflow step
// to guard against any code-path change that might re-enable the PKCE flow.
//
// The test expects the workflow to have:
//   - Run the NSIS installer (binary at %LOCALAPPDATA%\VaultMTG\vaultmtg-daemon.exe)
//   - Set DAEMON_BINARY_PATH to the cross-compiled binary path (for reference only;
//     the installed binary at %LOCALAPPDATA%\VaultMTG is used directly)
//   - Set MTGA_DAEMON_HEADLESS=1, CLERK_PUBLISHABLE_KEY="", CLERK_FRONTEND_API="",
//     CLERK_OAUTH_CLIENT_ID=""
//   - Created a stub Player.log at the path returned by logreader.DefaultLogPath()
package nsis_test

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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a mutex-protected bytes.Buffer safe for concurrent writes (from
// the exec goroutine) and reads (from the test goroutine).
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

// installedBinaryPath returns the path where the NSIS installer places the
// daemon binary.  Per installer.nsi: InstallDir = $LOCALAPPDATA\VaultMTG.
func installedBinaryPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "VaultMTG", "vaultmtg-daemon.exe")
}

// daemonConfigPath returns the path where the NSIS installer writes daemon.json.
// Per installer.nsi: $APPDATA\vaultmtg\daemon.json.
func daemonConfigPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "vaultmtg", "daemon.json")
}

// killExistingDaemon forcibly terminates any running vaultmtg-daemon.exe process.
// This is idempotent — it succeeds whether or not the process is running.
func killExistingDaemon(t *testing.T) {
	t.Helper()
	out, _ := exec.Command("taskkill", "/F", "/IM", "vaultmtg-daemon.exe").CombinedOutput()
	t.Logf("taskkill output: %s", strings.TrimSpace(string(out)))
}

// b64URLEncodeJSON encodes v as compact JSON and then base64url-encodes it without
// padding.  This mirrors the helper in smoke_test.go — copied here because that
// file is in an internal package that is not importable from an external test package.
func b64URLEncodeJSON(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)

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

// writeDaemonConfig overwrites daemon.json at cfgPath with a test config that:
//   - Points at the stub BFF (bffURL).
//   - Uses a fake far-future daemon_jwt to bypass PKCE.
//   - Sets keychain=false, sync_enabled=false, account_id=lifecycle-test-001.
//   - Directs the log poller at logPath.
func writeDaemonConfig(t *testing.T, cfgPath, bffURL, logPath string) {
	t.Helper()

	farFuture := time.Now().Add(365 * 24 * time.Hour).Unix()
	header := "eyJhbGciOiJIUzI1NiJ9" // {"alg":"HS256"}
	claims := b64URLEncodeJSON(t, map[string]int64{"exp": farFuture})
	fakeJWT := fmt.Sprintf("%s.%s.lifecycle-sig", header, claims)

	cfg := map[string]interface{}{
		"cloud_api_url":         bffURL,
		"keychain":              false,
		"api_key":               "",
		"daemon_jwt":            fakeJWT,
		"sync_enabled":          false, // skip registration
		"account_id":            "lifecycle-test-001",
		"log_path":              logPath,
		"ingest_path":           "/ingest/events",
		"use_fs_notify":         false,
		"log_preserve_on_start": false,
		"poll_interval":         200000000, // 200 ms in nanoseconds
		"disable_update_check":  true,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(cfgPath, data, 0o600))
	t.Logf("wrote daemon config at %s:\n%s", cfgPath, string(data))
}

// waitForDaemonHealth polls 127.0.0.1:9001/health until 200 or timeout.
func waitForDaemonHealth(t *testing.T, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:9001/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("daemon /health returned 200 after %s", timeout-time.Until(deadline))
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("daemon /health did not return 200 within %s", timeout)
}

// TestWindowsNSISDaemonEvent is the Group A assertion for ticket #41.
// It proves that the daemon binary installed by the NSIS installer correctly
// dispatches a contract.DaemonEvent to a stub BFF.
//
// Pre-conditions (satisfied by preceding CI steps):
//   - NSIS installer has run; binary is at %LOCALAPPDATA%\VaultMTG\vaultmtg-daemon.exe.
//   - %APPDATA%\vaultmtg\daemon.json exists (written by installer or pre-staged step).
//   - A stub Player.log exists at the path expected by the daemon's log poller.
func TestWindowsNSISDaemonEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping NSIS lifecycle test in -short mode")
	}

	binaryPath := installedBinaryPath()
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("installed daemon binary not found at %s — NSIS installer must have run before this test: %v", binaryPath, err)
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
				// Unmarshal failure — still accept; log and record raw.
				t.Logf("stub BFF: unmarshal error for body %q: %v", string(body), err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			bffMu.Lock()
			received = append(received, evt)
			bffMu.Unlock()
			w.WriteHeader(http.StatusAccepted)

		case strings.HasPrefix(r.URL.Path, "/daemon/version"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"latest":"0.0.1","released_at":"2026-01-01T00:00:00Z","download_url":"https://example.com"}`)

		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()
	t.Logf("stub BFF listening at %s", srv.URL)

	// ── Kill any existing daemon instance ─────────────────────────────────────

	killExistingDaemon(t)
	time.Sleep(1 * time.Second) // brief settle after kill

	// ── Write test daemon config ──────────────────────────────────────────────
	// The NSIS installer already ran and placed (or pre-staged) daemon.json.
	// We overwrite it with a config pointing at the stub BFF.

	cfgPath := daemonConfigPath()
	// Ensure config directory exists (installer creates it, but be defensive).
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))

	// The workflow pre-stages a stub Player.log at the MTGA default location.
	// Use that path so the daemon's log poller can open it immediately.
	logPath := filepath.Join(os.Getenv("USERPROFILE"),
		"AppData", "LocalLow", "Wizards Of The Coast", "MTGA", "Player.log")
	writeDaemonConfig(t, cfgPath, srv.URL, logPath)

	// ── Start installed daemon directly ───────────────────────────────────────

	cmd := exec.Command(binaryPath, "-config", cfgPath)
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

	require.NoError(t, cmd.Start(), "failed to start installed daemon binary")

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
		if t.Failed() {
			t.Logf("daemon output:\n%s", daemonLogs.String())
		}
	})

	// ── Wait for daemon health ────────────────────────────────────────────────

	waitForDaemonHealth(t, 30*time.Second)

	// ── Append synthetic draft.pack log line ──────────────────────────────────
	// This matches the structure used in TestDaemonBinarySmoke and fires within
	// ~1–2s, well ahead of the 30-second heartbeat ticker.

	logF, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err, "failed to open stub Player.log for append")
	draftPackLine := `{"CurrentModule":"BotDraft","Payload":"{\"EventName\":\"QuickDraft_SOS_20260526\",\"PackNumber\":0,\"PickNumber\":0,\"DraftPack\":[\"11001\",\"22002\",\"33003\"]}"}` + "\n"
	_, err = logF.WriteString(draftPackLine)
	require.NoError(t, err)
	require.NoError(t, logF.Close())
	t.Logf("appended draft.pack log line to %s", logPath)

	// ── Wait for event dispatch ───────────────────────────────────────────────

	var draftPackEvent *contract.DaemonEvent
	deadline := time.Now().Add(30 * time.Second)
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

	// ── Assert event fields ───────────────────────────────────────────────────

	assert.Equal(t, "draft.pack", draftPackEvent.Type, "event type must be draft.pack")
	assert.Equal(t, "lifecycle-test-001", draftPackEvent.AccountID,
		"account_id must match lifecycle-test-001 from daemon config")
	assert.NotEmpty(t, draftPackEvent.Payload, "draft.pack event payload must be non-empty")

	t.Logf("PASS: stub BFF received draft.pack event for account %s", draftPackEvent.AccountID)

	// ── Terminate daemon ──────────────────────────────────────────────────────
	// On Windows there is no SIGTERM.  Process.Kill() is the clean shutdown path
	// for the test runner; the daemon handles this via os.Interrupt / signal.NotifyContext.

	require.NoError(t, cmd.Process.Kill(), "failed to terminate daemon process")
	exitDone := make(chan error, 1)
	go func() { exitDone <- cmd.Wait() }()

	select {
	case exitErr := <-exitDone:
		// Non-zero exit is expected after Kill on Windows.
		t.Logf("daemon exited: %v", exitErr)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit within 10s after Kill")
	}
}
