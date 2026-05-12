//go:build darwin

package daemon

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// findMTGAPID returns the PID of the running MTGA process, or an error if it
// is not found.
func findMTGAPID() (int, error) {
	out, err := exec.Command("pgrep", "-x", "-o", "MTGA").Output()
	if err != nil {
		return 0, fmt.Errorf("MTGA not running")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse MTGA pid: %w", err)
	}
	return pid, nil
}
