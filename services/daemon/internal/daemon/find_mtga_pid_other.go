//go:build !darwin

package daemon

import "fmt"

func findMTGAPID() (int, error) {
	return 0, fmt.Errorf("MTGA PID detection not implemented on this platform")
}
