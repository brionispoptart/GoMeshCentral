//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func ensureWindowsElevated(relaunchFlag bool) error {
	if isWindowsAdmin() {
		return nil
	}
	if relaunchFlag {
		return fmt.Errorf("agent failed to start with elevated privileges")
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	quotedFilePath := psQuote(executablePath)
	quotedWorkingDir := psQuote(workingDir)
	argumentList := psArgumentList(append(os.Args[1:], "-elevated-launch"))
	psScript := "Start-Process -Verb RunAs -WindowStyle Hidden -WorkingDirectory " + quotedWorkingDir + " -FilePath " + quotedFilePath + " -ArgumentList " + argumentList

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch elevated agent: %w", err)
	}
	os.Exit(0)
	return nil
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func psArgumentList(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, psQuote(arg))
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}
