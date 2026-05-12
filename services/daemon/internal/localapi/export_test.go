package localapi

// SetShutdownExitForTest swaps the os.Exit hook used by
// handleSystemUninstall with the provided function and returns a
// restore func the caller should defer. Production code never calls
// this; it lives in an _test.go file so the override is invisible to
// non-test builds.
func SetShutdownExitForTest(fn func(int)) func() {
	prev := shutdownExit
	shutdownExit = fn
	return func() { shutdownExit = prev }
}
