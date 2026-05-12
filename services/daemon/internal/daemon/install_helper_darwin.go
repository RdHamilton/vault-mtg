//go:build darwin

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runHelperInstaller invokes install-helper.sh with administrator privileges
// via osascript, prompting the user for their admin password.
//
// The script and plist are staged to /tmp first because macOS TCC prevents
// the root shell spawned by `do shell script ... with administrator privileges`
// from reading files inside ~/Documents (or any user-protected path).
func runHelperInstaller(helperBinary, scriptDir string) error {
	stagingDir, err := os.MkdirTemp("", "vaultmtg-install-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	// Stage the helper binary too — the root shell can't cp from ~/Documents.
	stagedBinary := filepath.Join(stagingDir, "collection-helper")
	if err := stageFile(helperBinary, stagedBinary); err != nil {
		return fmt.Errorf("stage collection-helper: %w", err)
	}
	if err := os.Chmod(stagedBinary, 0o755); err != nil {
		return fmt.Errorf("chmod collection-helper: %w", err)
	}

	for _, name := range []string{"install-helper.sh", "com.vaultmtg.collection-helper.plist"} {
		if err := stageFile(filepath.Join(scriptDir, name), filepath.Join(stagingDir, name)); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
	}
	if err := os.Chmod(filepath.Join(stagingDir, "install-helper.sh"), 0o755); err != nil {
		return fmt.Errorf("chmod install script: %w", err)
	}

	script := buildOsaScript(stagedBinary, stagingDir)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w — %s", err, string(out))
	}
	return nil
}

func stageFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// buildOsaScript constructs the AppleScript string for installing the helper.
// Both paths are shell-quoted so spaces and special characters are safe.
func buildOsaScript(helperBinary, scriptDir string) string {
	scriptPath := filepath.Join(scriptDir, "install-helper.sh")
	cmd := shellQuote(scriptPath) + " " + shellQuote(helperBinary)
	// Escape double-quotes so the AppleScript string delimiter is not broken.
	cmd = strings.ReplaceAll(cmd, `"`, `\"`)
	return fmt.Sprintf(`do shell script "%s" with administrator privileges`, cmd)
}

// shellQuote wraps s in single-quotes and escapes any embedded single-quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
