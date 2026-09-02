//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	linuxInstallDir  = "/opt/gomeshcentral"
	linuxAgentExe    = "/opt/gomeshcentral/gomesh-agent"
	linuxDataDir     = "/var/lib/gomeshcentral"
	linuxStateFile   = "/var/lib/gomeshcentral/agent-state.json"
	linuxServiceFile = "/etc/systemd/system/gomesh-agent.service"
)

var errServiceUnsupported = errors.New("agent OS service management is supported on Windows and Linux (systemd)")

func runningAsWindowsService() bool { return false }

func runWindowsServiceAgent(func(stop chan struct{}, status func(string)) error) {}

func installLinuxService(server, enrollToken, sourceStatePath string, heartbeatSec, reportSec, rotateEvery int) (string, error) {
	if os.Geteuid() != 0 {
		return "", errors.New("installing Linux systemd service requires root privileges (run with sudo)")
	}

	srcExe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	if err := os.MkdirAll(linuxInstallDir, 0755); err != nil {
		return "", fmt.Errorf("create install dir %s: %w", linuxInstallDir, err)
	}
	if err := os.MkdirAll(linuxDataDir, 0700); err != nil {
		return "", fmt.Errorf("create data dir %s: %w", linuxDataDir, err)
	}

	if srcExe != linuxAgentExe {
		input, err := os.ReadFile(srcExe)
		if err != nil {
			return "", fmt.Errorf("read source binary %s: %w", srcExe, err)
		}
		if err := os.WriteFile(linuxAgentExe, input, 0755); err != nil {
			return "", fmt.Errorf("write target binary %s: %w", linuxAgentExe, err)
		}
	}

	if sourceStatePath != "" && sourceStatePath != linuxStateFile {
		if _, err := os.Stat(sourceStatePath); err == nil {
			if _, destErr := os.Stat(linuxStateFile); errors.Is(destErr, os.ErrNotExist) {
				if stateData, rerr := os.ReadFile(sourceStatePath); rerr == nil {
					_ = os.WriteFile(linuxStateFile, stateData, 0600)
				}
			}
		}
	}

	args := []string{
		linuxAgentExe,
		"-server", server,
		"-state", linuxStateFile,
		"-heartbeat-seconds", strconv.Itoa(heartbeatSec),
		"-report-seconds", strconv.Itoa(reportSec),
	}
	if enrollToken != "" {
		args = append(args, "-enroll-token", enrollToken)
	}
	if rotateEvery > 0 {
		args = append(args, "-rotate-every-minutes", strconv.Itoa(rotateEvery))
	}

	execStart := strings.Join(args, " ")

	unitContent := fmt.Sprintf(`[Unit]
Description=GoMeshCentral Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=%s
Restart=always
RestartSec=5
UMask=0077

[Install]
WantedBy=multi-user.target
`, execStart)

	if err := os.WriteFile(linuxServiceFile, []byte(unitContent), 0644); err != nil {
		return "", fmt.Errorf("write systemd unit file %s: %w", linuxServiceFile, err)
	}

	_ = exec.Command("systemctl", "daemon-reload").Run()
	if err := exec.Command("systemctl", "enable", "--now", "gomesh-agent").Run(); err != nil {
		return "", fmt.Errorf("enable systemd service: %w", err)
	}

	return linuxAgentExe, nil
}

func uninstallLinuxService() error {
	if os.Geteuid() != 0 {
		return errors.New("uninstalling Linux systemd service requires root privileges (run with sudo)")
	}

	_ = exec.Command("systemctl", "disable", "--now", "gomesh-agent").Run()
	_ = os.Remove(linuxServiceFile)
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = os.Remove(linuxAgentExe)
	return nil
}

func isLinuxServiceInstalled() bool {
	_, err := os.Stat(linuxServiceFile)
	return err == nil
}

func installWindowsService(server, enrollToken, sourceStatePath string, heartbeatSec, reportSec, rotateEvery int, _ string) (string, error) {
	return installLinuxService(server, enrollToken, sourceStatePath, heartbeatSec, reportSec, rotateEvery)
}

func uninstallWindowsService() error {
	return uninstallLinuxService()
}

func windowsServiceArgs(string, string, string, int, int, int) []string { return nil }

func newUnattendedControls(string, string, string, int, int, int) unattendedControls {
	return unattendedControls{}
}

func agentServiceActive() bool { return isLinuxServiceInstalled() }
