//go:build !windows

package main

// ensureWindowsElevated is a no-op on non-Windows platforms. Elevated,
// always-on execution on Linux is handled by installing the agent as a
// root-owned systemd unit (see docs/gomesh-agent.service).
func ensureWindowsElevated(bool) error { return nil }
