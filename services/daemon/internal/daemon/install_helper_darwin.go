//go:build darwin

package daemon

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// runHelperInstaller invokes install-helper.sh with administrator privileges
// via osascript, prompting the user for their admin password.
func runHelperInstaller(helperBinary, scriptDir string) error {
	script := buildOsaScript(helperBinary, scriptDir)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w — %s", err, string(out))
	}
	return nil
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
