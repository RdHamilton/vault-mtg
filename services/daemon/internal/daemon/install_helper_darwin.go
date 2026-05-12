//go:build darwin

package daemon

import (
	"fmt"
	"os/exec"
)

// runHelperInstaller invokes install-helper.sh with administrator privileges
// via osascript, prompting the user for their admin password.
func runHelperInstaller(helperBinary, scriptDir string) error {
	script := fmt.Sprintf(
		`do shell script "%s/install-helper.sh %s" with administrator privileges`,
		scriptDir, helperBinary,
	)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w — %s", err, string(out))
	}
	return nil
}
