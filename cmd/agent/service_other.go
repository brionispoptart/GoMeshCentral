//go:build !windows && !linux

package main

import "errors"

var errServiceUnsupported = errors.New("agent OS service management is only supported on Windows")

func runningAsWindowsService() bool { return false }

func runWindowsServiceAgent(func(stop chan struct{}, status func(string)) error) {}

func installWindowsService(string, string, string, int, int, int, string) (string, error) {
	return "", errServiceUnsupported
}

func uninstallWindowsService() error { return errServiceUnsupported }

func windowsServiceArgs(string, string, string, int, int, int) []string { return nil }

func newUnattendedControls(string, string, string, int, int, int) unattendedControls {
	return unattendedControls{}
}

func agentServiceActive() bool { return false }
