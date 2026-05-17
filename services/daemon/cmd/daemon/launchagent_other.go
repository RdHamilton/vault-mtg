//go:build !darwin

package main

// stopLaunchAgent is a no-op on non-macOS platforms. The LaunchAgent mechanism
// is macOS-specific; Windows and Linux use their own service managers (Task
// Scheduler and systemd respectively) and do not need equivalent handling here.
func stopLaunchAgent() {}
