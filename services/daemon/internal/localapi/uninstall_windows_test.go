//go:build windows

// NOTE: This file intentionally references the legacy "MTGA-Companion-Daemon"
// task name and "MTGA-Companion" config dir. These are the actual pre-rename
// install artifacts that the uninstall path must clean up; the tests assert
// that legacy-cleanup behavior. They are NOT stale repo references and must not
// be renamed. This file is excluded from the stale-repo-grep gate in
// .github/workflows/process-gates.yml.

package localapi_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/localapi"
)

// schtasksCall records a single invocation of schtasks.
type schtasksCall struct {
	args []string
}

// stubSchtasks records all calls and returns the configured error for a
// matching task name (or nil for all calls when taskErr is empty).
type stubSchtasks struct {
	calls    []schtasksCall
	taskErrs map[string]error // keyed on task name in args
}

func (s *stubSchtasks) Run(args []string) error {
	s.calls = append(s.calls, schtasksCall{args: args})
	for taskName, err := range s.taskErrs {
		for _, a := range args {
			if a == taskName {
				return err
			}
		}
	}
	return nil
}

// taskNames returns the set of unique /TN values seen across all calls.
func (s *stubSchtasks) taskNames() map[string]bool {
	names := make(map[string]bool)
	for _, c := range s.calls {
		for i, a := range c.args {
			if strings.EqualFold(a, "/TN") && i+1 < len(c.args) {
				names[c.args[i+1]] = true
			}
		}
	}
	return names
}

// ── TC1: both current and legacy tasks are targeted ──────────────────────────

// TestRunPlatformUninstall_BothTasksTargeted verifies that runPlatformUninstall
// issues schtasks /End and /Delete calls for both VaultMTG-Daemon AND the legacy
// MTGA-Companion-Daemon task, ensuring two daemons cannot run simultaneously
// after an upgrade.
func TestRunPlatformUninstall_BothTasksTargeted(t *testing.T) {
	stub := &stubSchtasks{}
	restore := localapi.SetSchtasksRunForTest(stub.Run)
	defer restore()

	_, err := localapi.RunPlatformUninstall(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := stub.taskNames()
	if !names["VaultMTG-Daemon"] {
		t.Error("expected VaultMTG-Daemon to be targeted by schtasks calls")
	}
	if !names["MTGA-Companion-Daemon"] {
		t.Error("expected MTGA-Companion-Daemon (legacy) to be targeted by schtasks calls")
	}
}

// ── TC2: /End and /Delete are called for each task ───────────────────────────

// TestRunPlatformUninstall_EndAndDeleteCalled verifies that both /End and
// /Delete operations are issued for each task name.
func TestRunPlatformUninstall_EndAndDeleteCalled(t *testing.T) {
	stub := &stubSchtasks{}
	restore := localapi.SetSchtasksRunForTest(stub.Run)
	defer restore()

	_, err := localapi.RunPlatformUninstall(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build a map of (operation, taskName) → count.
	opTaskCounts := make(map[string]int)
	for _, c := range stub.calls {
		var op, tn string
		for i, a := range c.args {
			switch strings.ToUpper(a) {
			case "/END", "/DELETE":
				op = strings.ToUpper(a)
			case "/TN":
				if i+1 < len(c.args) {
					tn = c.args[i+1]
				}
			}
		}
		if op != "" && tn != "" {
			opTaskCounts[fmt.Sprintf("%s:%s", op, tn)]++
		}
	}

	required := []string{
		"/END:VaultMTG-Daemon",
		"/DELETE:VaultMTG-Daemon",
		"/END:MTGA-Companion-Daemon",
		"/DELETE:MTGA-Companion-Daemon",
	}
	for _, key := range required {
		if opTaskCounts[key] == 0 {
			t.Errorf("missing expected schtasks call: %s", key)
		}
	}
}

// ── TC3: purge=false leaves config dir intact ────────────────────────────────

// TestRunPlatformUninstall_PurgeFalse verifies that purge=false does not remove
// the config directory.
func TestRunPlatformUninstall_PurgeFalse(t *testing.T) {
	stub := &stubSchtasks{}
	restore := localapi.SetSchtasksRunForTest(stub.Run)
	defer restore()

	// Create a temp config dir and point APPDATA at its parent.
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "vaultmtg")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	daemonJSON := filepath.Join(configDir, "daemon.json")
	if err := os.WriteFile(daemonJSON, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("APPDATA", tmp)

	_, err := localapi.RunPlatformUninstall(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Config dir must still exist.
	if _, statErr := os.Stat(configDir); os.IsNotExist(statErr) {
		t.Error("config dir was removed with purge=false")
	}
}

// ── TC4: purge=true removes the vaultmtg config dir ─────────────────────────

// TestRunPlatformUninstall_PurgeTrue verifies that purge=true removes
// %APPDATA%\vaultmtg (the new branded config dir).
func TestRunPlatformUninstall_PurgeTrue(t *testing.T) {
	stub := &stubSchtasks{}
	restore := localapi.SetSchtasksRunForTest(stub.Run)
	defer restore()

	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "vaultmtg")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Setenv("APPDATA", tmp)

	_, err := localapi.RunPlatformUninstall(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(configDir); !os.IsNotExist(statErr) {
		t.Error("expected config dir to be removed with purge=true")
	}
}

// ── TC5: purge=true no longer targets the old MTGA-Companion dir ────────────

// TestRunPlatformUninstall_PurgeTargetsNewDirOnly verifies that purge=true
// removes %APPDATA%\vaultmtg only — NOT the legacy %APPDATA%\MTGA-Companion
// dir. Downgrade safety: users who purge and re-install the old binary should
// still find their old config.
func TestRunPlatformUninstall_PurgeTargetsNewDirOnly(t *testing.T) {
	stub := &stubSchtasks{}
	restore := localapi.SetSchtasksRunForTest(stub.Run)
	defer restore()

	tmp := t.TempDir()
	vaultmtgDir := filepath.Join(tmp, "vaultmtg")
	legacyDir := filepath.Join(tmp, "MTGA-Companion")

	for _, d := range []string{vaultmtgDir, legacyDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	t.Setenv("APPDATA", tmp)

	_, err := localapi.RunPlatformUninstall(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vaultmtg must be gone.
	if _, statErr := os.Stat(vaultmtgDir); !os.IsNotExist(statErr) {
		t.Error("expected %APPDATA%\\vaultmtg to be removed")
	}
	// Legacy dir must still be present.
	if _, statErr := os.Stat(legacyDir); os.IsNotExist(statErr) {
		t.Error("legacy MTGA-Companion config dir must NOT be removed by purge")
	}
}

// ── TC6: schtasks failure propagates ─────────────────────────────────────────

// TestRunPlatformUninstall_SchtasksError verifies that a real schtasks failure
// (not a "task not found" idempotent case) surfaces as an error.
func TestRunPlatformUninstall_SchtasksError(t *testing.T) {
	boom := errors.New("permission denied")
	stub := &stubSchtasks{
		taskErrs: map[string]error{
			"VaultMTG-Daemon": boom,
		},
	}
	restore := localapi.SetSchtasksRunForTest(stub.Run)
	defer restore()

	_, err := localapi.RunPlatformUninstall(false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error chain: %v (wanted to contain %v)", err, boom)
	}
}
