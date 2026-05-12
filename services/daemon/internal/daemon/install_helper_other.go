//go:build !darwin

package daemon

import "fmt"

func runHelperInstaller(helperBinary, scriptDir string) error {
	return fmt.Errorf("helper install not implemented on this platform")
}
