//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	windowsServiceName    = "GoMeshCentralAgent"
	windowsServiceDisplay = "GoMeshCentral Agent"
	windowsServiceDesc    = "GoMeshCentral remote management agent (persistent, elevated background service)."

	// Industry-standard install layout: the binary lives under Program Files and
	// mutable state under ProgramData, both in a vendor-named subfolder.
	windowsInstallFolder = "GoMeshCentral"
	windowsAgentExeName  = "agent.exe"
	windowsAgentIconName = "agent.ico"
	windowsStateFileName = "agent-state.json"
)

func windowsProgramFilesDir() string {
	if p := os.Getenv("ProgramFiles"); p != "" {
		return p
	}
	return `C:\Program Files`
}

func windowsProgramDataDir() string {
	if p := os.Getenv("ProgramData"); p != "" {
		return p
	}
	return `C:\ProgramData`
}

func windowsInstallDir() string { return filepath.Join(windowsProgramFilesDir(), windowsInstallFolder) }

func windowsDataDir() string { return filepath.Join(windowsProgramDataDir(), windowsInstallFolder) }

func sameWindowsPath(a, b string) bool {
	ca, err1 := filepath.Abs(a)
	cb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(ca), filepath.Clean(cb))
}

func copyWindowsFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeWindowsInstallLog appends install progress to ProgramData so failures are
// diagnosable even when the elevated installer runs in a hidden window.
func writeWindowsInstallLog(dataDir, message string) {
	_ = os.MkdirAll(dataDir, 0o755)
	f, err := os.OpenFile(filepath.Join(dataDir, "install.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().Format(time.RFC3339) + " " + message + "\r\n")
}

// windowsAgentService adapts the headless agent runtime to the Windows Service
// Control Manager (SCM) lifecycle.
type windowsAgentService struct {
	runAgent func(stop chan struct{}, status func(string)) error
}

func (w *windowsAgentService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.runAgent(stop, func(string) {}); err != nil && !errors.Is(err, errShutdown) {
			log.Printf("service agent runtime ended: %v", err)
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				close(stop)
				<-done
				return false, 0
			}
		case <-done:
			return false, 0
		}
	}
}

// runningAsWindowsService reports whether the process was launched by the SCM.
func runningAsWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}

// runWindowsServiceAgent blocks running the agent under SCM control.
func runWindowsServiceAgent(runAgent func(stop chan struct{}, status func(string)) error) {
	if err := svc.Run(windowsServiceName, &windowsAgentService{runAgent: runAgent}); err != nil {
		log.Fatalf("windows service run failed: %v", err)
	}
}

// windowsServiceArgs builds the command-line arguments baked into the installed
// service so the SCM-launched process reconnects with the same configuration.
func windowsServiceArgs(server, statePath, enrollToken string, heartbeatSec, reportSec, rotateEvery int) []string {
	absState := statePath
	if p, err := filepath.Abs(statePath); err == nil {
		absState = p
	}
	args := []string{
		"-server", server,
		"-state", absState,
		"-heartbeat-seconds", strconv.Itoa(heartbeatSec),
		"-report-seconds", strconv.Itoa(reportSec),
	}
	if enrollToken != "" {
		args = append(args, "-enroll-token", enrollToken)
	}
	if rotateEvery > 0 {
		args = append(args, "-rotate-every-minutes", strconv.Itoa(rotateEvery))
	}
	return args
}

func installWindowsService(server, enrollToken, sourceStatePath string, heartbeatSec, reportSec, rotateEvery int, trayIconSrc string) (installedExe string, err error) {
	installDir := windowsInstallDir()
	dataDir := windowsDataDir()
	exeDest := filepath.Join(installDir, windowsAgentExeName)
	stateDest := filepath.Join(dataDir, windowsStateFileName)

	defer func() {
		if err != nil {
			writeWindowsInstallLog(dataDir, "install failed: "+err.Error())
			return
		}
		writeWindowsInstallLog(dataDir, "installed exe="+exeDest+" state="+stateDest)
	}()

	srcExe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	srcExe, err = filepath.Abs(srcExe)
	if err != nil {
		return "", fmt.Errorf("resolve absolute executable path: %w", err)
	}

	if err = os.MkdirAll(installDir, 0o755); err != nil {
		return "", fmt.Errorf("create install directory %q: %w", installDir, err)
	}
	if err = os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create data directory %q: %w", dataDir, err)
	}

	// Copy the agent binary into Program Files unless it is already running there.
	if !sameWindowsPath(srcExe, exeDest) {
		if err = copyWindowsFile(srcExe, exeDest); err != nil {
			return "", fmt.Errorf("copy agent binary to %q: %w", exeDest, err)
		}
	}

	// Keep the install directory self-contained for a future user-session tray
	// helper (a LocalSystem service cannot draw a tray icon itself; see below).
	if trayIconSrc != "" {
		if _, statErr := os.Stat(trayIconSrc); statErr == nil {
			if copyErr := copyWindowsFile(trayIconSrc, filepath.Join(installDir, windowsAgentIconName)); copyErr != nil {
				log.Printf("warning: could not copy tray icon: %v", copyErr)
			}
		}
	}

	// Preserve an existing agent identity if one is present at the source state
	// path so the installed service reconnects as the same device instead of
	// enrolling anew. A fresh install without prior state relies on -enroll-token.
	if sourceStatePath != "" && !sameWindowsPath(sourceStatePath, stateDest) {
		if _, statErr := os.Stat(sourceStatePath); statErr == nil {
			if _, destErr := os.Stat(stateDest); errors.Is(destErr, os.ErrNotExist) {
				if copyErr := copyWindowsFile(sourceStatePath, stateDest); copyErr != nil {
					log.Printf("warning: could not seed agent state: %v", copyErr)
				}
			}
		}
	}

	args := windowsServiceArgs(server, stateDest, enrollToken, heartbeatSec, reportSec, rotateEvery)

	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("connect service manager (run from an elevated shell): %w", err)
	}
	defer m.Disconnect()

	if existing, oerr := m.OpenService(windowsServiceName); oerr == nil {
		existing.Close()
		return "", fmt.Errorf("service %q is already installed; uninstall it first", windowsServiceName)
	}

	config := mgr.Config{
		DisplayName: windowsServiceDisplay,
		Description: windowsServiceDesc,
		StartType:   mgr.StartAutomatic,
		// An empty ServiceStartName installs the service under LocalSystem,
		// which runs permanently elevated with full local privileges.
	}

	service, err := m.CreateService(windowsServiceName, exeDest, config, args...)
	if err != nil {
		return "", fmt.Errorf("create service: %w", err)
	}
	defer service.Close()

	recovery := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}
	if rerr := service.SetRecoveryActions(recovery, 86400); rerr != nil {
		log.Printf("warning: could not set service recovery actions: %v", rerr)
	}

	if err = service.Start(); err != nil {
		return "", fmt.Errorf("start service: %w", err)
	}

	registerWindowsARP(exeDest)
	return exeDest, nil
}

func uninstallWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager (run from an elevated shell): %w", err)
	}
	defer m.Disconnect()

	service, err := m.OpenService(windowsServiceName)
	if err != nil {
		return fmt.Errorf("service %q is not installed", windowsServiceName)
	}
	defer service.Close()

	if _, err := service.Control(svc.Stop); err != nil {
		log.Printf("warning: could not stop service before delete: %v", err)
	}
	if err := service.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}

	unregisterWindowsARP()
	return nil
}

func registerWindowsARP(exeDest string) {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+windowsServiceName, registry.ALL_ACCESS)
	if err != nil {
		log.Printf("warning: could not register Add/Remove Programs entry: %v", err)
		return
	}
	defer key.Close()

	_ = key.SetStringValue("DisplayName", windowsServiceDisplay)
	_ = key.SetStringValue("DisplayVersion", "1.0.0")
	_ = key.SetStringValue("Publisher", "GoMeshCentral")
	_ = key.SetStringValue("UninstallString", fmt.Sprintf(`"%s" -uninstall-service`, exeDest))
	_ = key.SetStringValue("QuietUninstallString", fmt.Sprintf(`"%s" -uninstall-service`, exeDest))
	_ = key.SetStringValue("InstallLocation", filepath.Dir(exeDest))
	_ = key.SetStringValue("DisplayIcon", exeDest)
	_ = key.SetDWordValue("NoModify", 1)
	_ = key.SetDWordValue("NoRepair", 1)
}

func unregisterWindowsARP() {
	_ = registry.DeleteKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+windowsServiceName)
}

// isWindowsServiceInstalled reports whether the unattended service exists. It
// uses a read-only SCM connection so it works from the non-elevated user-session
// agent (mgr.Connect requires admin and would always fail here).
func isWindowsServiceInstalled() bool {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false
	}
	defer windows.CloseServiceHandle(scm)

	namePtr, err := windows.UTF16PtrFromString(windowsServiceName)
	if err != nil {
		return false
	}
	handle, err := windows.OpenService(scm, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false
	}
	windows.CloseServiceHandle(handle)
	return true
}

// spawnServiceToggle relaunches this binary to install or uninstall the
// unattended service. The child self-elevates (UAC prompt) and performs the
// privileged work, so the caller stays non-elevated and non-blocking.
func spawnServiceToggle(enable bool, server, statePath, enrollToken string, heartbeatSec, reportSec, rotateEvery int) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	var args []string
	if enable {
		args = []string{
			"-install-service",
			"-server", server,
			"-state", statePath,
			"-heartbeat-seconds", strconv.Itoa(heartbeatSec),
			"-report-seconds", strconv.Itoa(reportSec),
		}
		if enrollToken != "" {
			args = append(args, "-enroll-token", enrollToken)
		}
		if rotateEvery > 0 {
			args = append(args, "-rotate-every-minutes", strconv.Itoa(rotateEvery))
		}
	} else {
		args = []string{"-uninstall-service"}
	}

	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch service toggle: %w", err)
	}
	return nil
}

// newUnattendedControls wires the tray "Unattended Access" toggle to the Windows
// service lifecycle.
func newUnattendedControls(server, statePath, enrollToken string, heartbeatSec, reportSec, rotateEvery int) unattendedControls {
	return unattendedControls{
		supported: true,
		installed: isWindowsServiceInstalled,
		toggle: func(enable bool) error {
			return spawnServiceToggle(enable, server, statePath, enrollToken, heartbeatSec, reportSec, rotateEvery)
		},
	}
}

// agentServiceActive reports whether the unattended service owns this device, so
// the interactive agent knows to stand down.
func agentServiceActive() bool { return isWindowsServiceInstalled() }
