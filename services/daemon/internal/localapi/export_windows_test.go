//go:build windows

package localapi

// SetSchtasksRunForTest replaces the schtasks execution hook with the
// provided stub and returns a restore func the caller should defer.
// This lets Windows-only tests verify the task-management logic without
// touching the host machine's Task Scheduler.
func SetSchtasksRunForTest(fn func([]string) error) func() {
	prev := schtasksRun
	schtasksRun = fn
	return func() { schtasksRun = prev }
}

// RunPlatformUninstall exposes runPlatformUninstall for Windows tests.
func RunPlatformUninstall(purge bool) (string, error) {
	return runPlatformUninstall(purge)
}
