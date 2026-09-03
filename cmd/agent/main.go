package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"gomeshcentral/internal/hub"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type agentState struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	AgentKey string `json:"agentKey,omitempty"`
}

type enrollmentRequest struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId"`
	Name     string `json:"name,omitempty"`
}

type enrollmentResponse struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
}

type registrationRequest struct {
	Name          string `json:"name"`
	MachineIDHash string `json:"machineIdHash"`
	SystemIDHash  string `json:"systemIdHash"`
	BoardIDHash   string `json:"boardIdHash"`
}

type registrationResponse struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
}

type rotateCredentialRequest struct {
	DeviceID   string `json:"deviceId"`
	CurrentKey string `json:"currentAgentKey"`
}

type rotateCredentialResponse struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
}

type agentReportPayload struct {
	Hostname           string   `json:"hostname"`
	Username           string   `json:"username"`
	OS                 string   `json:"os"`
	Arch               string   `json:"arch"`
	CPUCount           int      `json:"cpuCount"`
	CPUUsagePercent    float64  `json:"cpuUsagePercent"`
	MemoryUsagePercent float64  `json:"memoryUsagePercent"`
	MemoryUsedBytes    uint64   `json:"memoryUsedBytes"`
	MemoryTotalBytes   uint64   `json:"memoryTotalBytes"`
	LocalIPs           []string `json:"localIps"`
	ExecutablePath     string   `json:"executablePath"`
	WorkingDir         string   `json:"workingDir"`
	ProcessID          int      `json:"processId"`
	AgentStartedAt     string   `json:"agentStartedAt"`
	AgentUptimeSeconds int64    `json:"agentUptimeSeconds"`
}

var errShutdown = errors.New("shutdown requested")

// errStandDown signals the interactive agent to drop its connection because the
// unattended Windows service is now active and owns this device's identity.
// Keeping both connected would collide on the same device_id.
var errStandDown = errors.New("standing down for unattended service")

// unattendedControls exposes the "Unattended Access" tray toggle to the UI layer.
// On unsupported platforms supported is false and the item is hidden.
type unattendedControls struct {
	supported bool
	installed func() bool
	toggle    func(enable bool) error
}

func main() {
	var (
		server        = flag.String("server", "localhost:8080", "server host:port")
		deviceID      = flag.String("id", "", "device id (optional, persisted if omitted)")
		name          = flag.String("name", "", "device name (optional, persisted if omitted)")
		enrollToken   = flag.String("enroll-token", "", "one-time enrollment token issued by the server")
		relaunchAdmin = flag.Bool("elevated-launch", false, "internal flag used after self-elevation on Windows")
		selfElevate   = flag.Bool("self-elevate", false, "relaunch elevated via UAC when not already an administrator (interactive runs); the installed service already runs elevated")
		installSvc    = flag.Bool("install-service", false, "install the agent as an always-on elevated OS service and exit (Windows)")
		uninstallSvc  = flag.Bool("uninstall-service", false, "uninstall the agent OS service and exit (Windows)")
		rotateNow     = flag.Bool("rotate-now", false, "rotate agent credential once on startup")
		rotateEvery   = flag.Int("rotate-every-minutes", 0, "automatically rotate agent credential at interval; 0 disables")
		heartbeatSec  = flag.Int("heartbeat-seconds", 10, "heartbeat interval in seconds")
		reportSec     = flag.Int("report-seconds", 60, "agent system report interval in seconds")
		statePath     = flag.String("state", "data/agent-state.json", "path to persisted agent state")
		trayIcon      = flag.String("tray-icon", "assets/icons/agent/agent.ico", "path to tray icon file (.ico on Windows)")
		trayUIOnly    = flag.Bool("tray-ui-only", false, "run only the tray UI (Windows); connects to the background service for status")
	)
	flag.Parse()

	if *trayUIOnly && runtime.GOOS == "windows" {
		runTrayUIOnly(*trayIcon)
		return
	}

	if *installSvc {
		if runtime.GOOS == "windows" {
			if err := ensureWindowsElevated(*relaunchAdmin); err != nil {
				log.Fatalf("elevate for install: %v", err)
			}
		}
		installedExe, err := installWindowsService(*server, *enrollToken, *statePath, *heartbeatSec, *reportSec, *rotateEvery, *trayIcon)
		if err != nil {
			log.Fatalf("install service: %v", err)
		}
		log.Printf("agent service installed to %s and started (auto-start at boot)", installedExe)
		return
	}
	if *uninstallSvc {
		if runtime.GOOS == "windows" {
			if err := ensureWindowsElevated(*relaunchAdmin); err != nil {
				log.Fatalf("elevate for uninstall: %v", err)
			}
		}
		if err := uninstallWindowsService(); err != nil {
			log.Fatalf("uninstall service: %v", err)
		}
		log.Printf("agent service uninstalled")
		return
	}

	if runtime.GOOS == "windows" {
		if runningAsWindowsService() {
			runWindowsServiceAgent(func(stop chan struct{}, status func(string)) error {
				return startAgent(*server, *enrollToken, *statePath, *deviceID, *name, *heartbeatSec, *reportSec, *rotateEvery, *rotateNow, stop, status, nil)
			})
			return
		}
		if *selfElevate {
			if err := ensureWindowsElevated(*relaunchAdmin); err != nil {
				log.Fatalf("elevate agent: %v", err)
			}
		}
	}

	stop := make(chan struct{})
	var stopOnce sync.Once
	requestStop := func() {
		stopOnce.Do(func() { close(stop) })
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		requestStop()
		requestUIQuit()
	}()

	iconBytes, err := os.ReadFile(*trayIcon)
	if err != nil {
		log.Printf("tray icon not loaded (%s): %v", *trayIcon, err)
	}

	statusText := make(chan string, 8)
	agentErr := make(chan error, 1)
	go func() {
		agentErr <- startAgent(*server, *enrollToken, *statePath, *deviceID, *name, *heartbeatSec, *reportSec, *rotateEvery, *rotateNow, stop, func(s string) {
			select {
			case statusText <- s:
			default:
			}
		}, agentServiceActive)
	}()

	go func() {
		err := <-agentErr
		if err != nil && !errors.Is(err, errShutdown) {
			log.Printf("agent failed: %v", err)
		}
		requestStop()
		requestUIQuit()
	}()

	runAgentUI(iconBytes, statusText, requestStop, stop, newUnattendedControls(*server, *statePath, *enrollToken, *heartbeatSec, *reportSec, *rotateEvery))
}

// runTrayUIOnly runs only the tray UI without the agent core. Used when launched
// by the Windows service at user logon. The UI shows status from the running service.
func runTrayUIOnly(trayIconPath string) {
	iconBytes, err := os.ReadFile(trayIconPath)
	if err != nil {
		log.Printf("tray icon not loaded (%s): %v", trayIconPath, err)
	}

	statusText := make(chan string, 8)
	stop := make(chan struct{})
	var stopOnce sync.Once
	requestStop := func() {
		stopOnce.Do(func() { close(stop) })
	}

	// Send an initial status message
	statusText <- "Service running"

	// Simple UI-only mode - just show the tray icon with a status message
	// The unattended controls will still work to toggle the service
	runAgentUI(iconBytes, statusText, requestStop, stop, unattendedControls{
		supported: true,
		installed: func() bool {
			// Check if the service is installed
			return isWindowsServiceInstalled()
		},
		toggle: func(enable bool) error {
			// This will handle toggling the service via the existing mechanism
			return spawnServiceToggle(enable, "localhost:8080", "", "", 10, 60, 0)
		},
	})
}

// startAgent loads (or initializes) the persisted identity and runs the agent
// connection loop until stop is closed. It is shared by the interactive runtime
// and the installed Windows service runtime.
func startAgent(server, enrollToken, statePath, deviceID, name string, heartbeatSec, reportSec, rotateEvery int, rotateNow bool, stop chan struct{}, status func(string), standDown func() bool) error {
	agentStartedAt := time.Now().UTC()

	// Load deployment manifest if present
	manifest, err := loadManifest()
	if err != nil {
		log.Printf("warning: failed to load manifest: %v", err)
	}

	// Merge manifest with command-line flags: command-line takes precedence
	if server == "" && manifest.ServerEndpoint != "" {
		server = manifest.ServerEndpoint
		log.Printf("using server endpoint from manifest: %s", server)
	}
	if enrollToken == "" && manifest.BootstrapToken != "" {
		enrollToken = manifest.BootstrapToken
		log.Printf("using bootstrap token from manifest")
	}
	if server == "" {
		server = "localhost:8080"
	}

	state, err := loadOrInitializeState(statePath, deviceID, name)
	if err != nil {
		return err
	}
	identity := collectHardwareIdentity()
	if state.AgentKey == "" && identity.count() < 2 {
		return errors.New("unable to collect two stable hardware identifiers; enroll with an enrollment token")
	}
	log.Printf("agent identity loaded: id=%s name=%s", state.DeviceID, state.Name)

	rotationInterval := time.Duration(rotateEvery) * time.Minute
	reportInterval := time.Duration(reportSec) * time.Second
	if reportInterval <= 0 {
		reportInterval = 60 * time.Second
	}
	return runAgentLoop(server, enrollToken, statePath, state, identity, time.Duration(heartbeatSec)*time.Second, reportInterval, rotationInterval, rotateNow, agentStartedAt, stop, status, standDown)
}

func runAgentLoop(server, enrollToken, statePath string, state agentState, identity hardwareIdentity, heartbeat, reportInterval, rotationInterval time.Duration, rotateNow bool, agentStartedAt time.Time, stop <-chan struct{}, status func(string), standDown func() bool) error {
	backoff := 2 * time.Second
	pendingRotateNow := rotateNow
	for {
		select {
		case <-stop:
			status("Stopped")
			return errShutdown
		default:
		}

		// When the unattended service is installed it owns this device's identity.
		// The interactive agent stands down (stays UI-only) to avoid a device_id
		// collision, and reclaims the connection once the service is removed.
		if standDown != nil && standDown() {
			status("Unattended service active")
			sleepWithJitter(3*time.Second, stop)
			continue
		}

		if state.AgentKey == "" {
			if identity.count() >= 2 {
				registeredState, err := registerAgent(server, state, identity)
				if err != nil {
					status("Registration Failed")
					log.Printf("automatic registration failed: %v", err)
					sleepWithJitter(backoff, stop)
					backoff = nextBackoff(backoff)
					continue
				}
				state = registeredState
				if err := saveState(statePath, state); err != nil {
					log.Printf("save state failed: %v", err)
				}
				log.Printf("agent automatically registered with server-issued credential")
			} else if enrollToken != "" {
				enrolledState, err := enrollAgent(server, enrollToken, state)
				if err != nil {
					status("Enrollment Failed")
					log.Printf("enrollment failed: %v", err)
					sleepWithJitter(backoff, stop)
					backoff = nextBackoff(backoff)
					continue
				}
				state = enrolledState
				if err := saveState(statePath, state); err != nil {
					log.Printf("save state failed: %v", err)
				}
				log.Printf("agent enrolled with server-issued credential")
			} else {
				status("Needs Identity")
				log.Printf("no agent credential and insufficient hardware identity")
				sleepWithJitter(backoff, stop)
				backoff = nextBackoff(backoff)
				continue
			}
		}

		if state.AgentKey != "" && pendingRotateNow {
			rotated, err := rotateAgentCredential(server, state)
			if err != nil {
				status("Rotate Failed")
				log.Printf("credential rotation failed: %v", err)
				sleepWithJitter(backoff, stop)
				backoff = nextBackoff(backoff)
				continue
			}
			state.AgentKey = rotated.AgentKey
			if err := saveState(statePath, state); err != nil {
				status("State Save Failed")
				log.Printf("save state failed after rotation: %v", err)
				sleepWithJitter(backoff, stop)
				backoff = nextBackoff(backoff)
				continue
			}
			pendingRotateNow = false
			status("Credential Rotated")
			log.Printf("agent credential rotated on startup")
		}

		u := url.URL{Scheme: "ws", Host: server, Path: "/ws/agent"}
		q := u.Query()
		q.Set("agent_key", state.AgentKey)
		q.Set("device_id", state.DeviceID)
		u.RawQuery = q.Encode()

		conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			if response != nil && response.StatusCode == http.StatusUnauthorized && identity.count() >= 2 {
				state.AgentKey = ""
				if saveErr := saveState(statePath, state); saveErr != nil {
					log.Printf("clear rejected credential failed: %v", saveErr)
				}
				status("Re-registering")
				log.Printf("server rejected agent credential; re-registering hardware identity")
				continue
			}
			status("Reconnecting")
			log.Printf("dial failed: %v", err)
			sleepWithJitter(backoff, stop)
			backoff = nextBackoff(backoff)
			continue
		}

		status("Connected")
		log.Printf("connected to %s", u.String())
		backoff = 2 * time.Second
		err = runSession(conn, server, statePath, &state, heartbeat, reportInterval, rotationInterval, agentStartedAt, stop, status, standDown)
		_ = conn.Close()
		if errors.Is(err, errShutdown) {
			status("Stopped")
			log.Printf("agent stopped")
			return nil
		}
		if errors.Is(err, errStandDown) {
			status("Unattended service active")
			log.Printf("standing down: unattended service now owns this device")
			continue
		}
		if err != nil {
			status("Disconnected")
			log.Printf("session ended: %v", err)
		}
		sleepWithJitter(backoff, stop)
		backoff = nextBackoff(backoff)
	}
}

func runSession(conn *websocket.Conn, server, statePath string, state *agentState, heartbeat, reportInterval, rotationInterval time.Duration, agentStartedAt time.Time, stop <-chan struct{}, status func(string), standDown func() bool) error {
	var writeMu sync.Mutex
	writeEnvelope := func(msg hub.AgentEnvelope) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(msg)
	}

	terminal := newTerminalManager(writeEnvelope)
	defer terminal.CloseAll()

	fileTransfer := newFileTransferManager(writeEnvelope)

	readErr := make(chan error, 1)
	go func() {
		defer close(readErr)
		for {
			var msg hub.AgentEnvelope
			if err := conn.ReadJSON(&msg); err != nil {
				readErr <- err
				return
			}
			switch msg.Type {
			case "command":
				log.Printf("received command: %s", msg.Command)
				go func(command string) {
					result := executeAgentCommand(command)
					err := writeEnvelope(hub.AgentEnvelope{
						Type:     "command_result",
						DeviceID: state.DeviceID,
						Command:  command,
						Data:     result.Output,
						ExitCode: result.ExitCode,
						Error:    result.Error,
					})
					if err != nil {
						log.Printf("failed to send command_result: %v", err)
					}
				}(msg.Command)
			case "terminal_open", "terminal_data", "terminal_resize", "terminal_close":
				terminal.Handle(msg)
			case "file_list", "file_download_start":
				// One-shot, independent of other sessions: safe to run off the
				// read loop so a big listing/download doesn't stall other traffic.
				go fileTransfer.Handle(msg)
			case "file_upload_start", "file_upload_chunk", "file_upload_complete":
				// Must stay in read-loop order: chunks for the same upload session
				// have to be written to disk in the order they arrive.
				fileTransfer.Handle(msg)
			}
		}
	}()

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	reportTicker := time.NewTicker(reportInterval)
	defer reportTicker.Stop()

	var rotationTicker *time.Ticker
	if rotationInterval > 0 {
		rotationTicker = time.NewTicker(rotationInterval)
		defer rotationTicker.Stop()
	}

	standDownTicker := time.NewTicker(3 * time.Second)
	defer standDownTicker.Stop()

	for {
		select {
		case <-reportTicker.C:
			report := collectAgentReport(agentStartedAt)
			reportPayload, err := json.Marshal(report)
			if err != nil {
				log.Printf("failed to marshal report: %v", err)
				continue
			}
			err = writeEnvelope(hub.AgentEnvelope{Type: "report", DeviceID: state.DeviceID, Name: state.Name, Report: reportPayload})
			if err != nil {
				return err
			}
			log.Printf("report sent for %s", state.DeviceID)
		case <-ticker.C:
			err := writeEnvelope(hub.AgentEnvelope{Type: "heartbeat", DeviceID: state.DeviceID, Name: state.Name})
			if err != nil {
				return err
			}
			log.Printf("heartbeat sent for %s", state.DeviceID)
		case <-rotationTickChannel(rotationTicker):
			rotated, err := rotateAgentCredential(server, *state)
			if err != nil {
				status("Rotate Failed")
				log.Printf("periodic credential rotation failed: %v", err)
				continue
			}
			state.AgentKey = rotated.AgentKey
			if err := saveState(statePath, *state); err != nil {
				status("State Save Failed")
				log.Printf("save state failed after periodic rotation: %v", err)
				continue
			}
			status("Credential Rotated")
			log.Printf("agent credential rotated successfully")
		case err := <-readErr:
			if err == nil {
				return errors.New("connection closed")
			}
			return err
		case <-standDownTicker.C:
			if standDown != nil && standDown() {
				return errStandDown
			}
		case <-stop:
			return errShutdown
		}
	}
}

func loadOrInitializeState(path, requestedID, requestedName string) (agentState, error) {
	state := agentState{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &state); err != nil {
			return agentState{}, err
		}
	}

	if requestedID != "" {
		state.DeviceID = requestedID
	}
	if requestedName != "" {
		state.Name = requestedName
	}

	if state.DeviceID == "" {
		id, err := randomDeviceID()
		if err != nil {
			return agentState{}, err
		}
		state.DeviceID = id
	}
	if state.Name == "" {
		hostname, err := os.Hostname()
		if err != nil || hostname == "" {
			hostname = "Endpoint"
		}
		state.Name = hostname
	}

	if err := saveState(path, state); err != nil {
		return agentState{}, err
	}

	return state, nil
}

func randomDeviceID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "agent-" + hex.EncodeToString(b), nil
}

func sleepWithJitter(base time.Duration, stop <-chan struct{}) {
	maxJitterMs := int64(math.Max(float64(base/time.Millisecond)/4, 250))
	r, err := rand.Int(rand.Reader, big.NewInt(maxJitterMs+1))
	jitter := time.Duration(0)
	if err == nil {
		jitter = time.Duration(r.Int64()) * time.Millisecond
	}
	wait := base + jitter
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
	case <-stop:
	}
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > time.Minute {
		return time.Minute
	}
	return next
}

func rotationTickChannel(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func saveState(path string, state agentState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}
func registerAgent(server string, state agentState, identity hardwareIdentity) (agentState, error) {
	payload, err := json.Marshal(registrationRequest{Name: state.Name, MachineIDHash: identity.MachineID, SystemIDHash: identity.SystemID, BoardIDHash: identity.BoardID})
	if err != nil {
		return agentState{}, err
	}
	resp, err := http.Post("http://"+server+"/api/agents/register", "application/json", bytes.NewReader(payload))
	if err != nil {
		return agentState{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return agentState{}, fmt.Errorf("registration rejected: %s", strings.TrimSpace(string(body)))
	}
	var out registrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return agentState{}, err
	}
	if out.DeviceID == "" || out.AgentKey == "" {
		return agentState{}, errors.New("invalid registration response")
	}
	state.DeviceID, state.AgentKey = out.DeviceID, out.AgentKey
	return state, nil
}

func enrollAgent(server, enrollToken string, state agentState) (agentState, error) {
	reqBody := enrollmentRequest{
		Token:    enrollToken,
		DeviceID: state.DeviceID,
		Name:     state.Name,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return agentState{}, err
	}

	url := enrollmentURL(server)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return agentState{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return agentState{}, errors.New("enrollment rejected")
	}

	var out enrollmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return agentState{}, err
	}
	if out.DeviceID == "" || out.AgentKey == "" {
		return agentState{}, errors.New("invalid enrollment response")
	}

	state.DeviceID = out.DeviceID
	state.AgentKey = out.AgentKey
	return state, nil
}

func enrollmentURL(server string) string {
	if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
		return strings.TrimRight(server, "/") + "/api/agents/enroll"
	}
	if strings.HasPrefix(server, "ws://") {
		return "http://" + strings.TrimPrefix(server, "ws://") + "/api/agents/enroll"
	}
	if strings.HasPrefix(server, "wss://") {
		return "https://" + strings.TrimPrefix(server, "wss://") + "/api/agents/enroll"
	}
	return "http://" + strings.TrimRight(server, "/") + "/api/agents/enroll"
}

func rotateCredentialURL(server string) string {
	if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
		return strings.TrimRight(server, "/") + "/api/agents/rotate-key"
	}
	if strings.HasPrefix(server, "ws://") {
		return "http://" + strings.TrimPrefix(server, "ws://") + "/api/agents/rotate-key"
	}
	if strings.HasPrefix(server, "wss://") {
		return "https://" + strings.TrimPrefix(server, "wss://") + "/api/agents/rotate-key"
	}
	return "http://" + strings.TrimRight(server, "/") + "/api/agents/rotate-key"
}

func rotateAgentCredential(server string, state agentState) (rotateCredentialResponse, error) {
	reqBody := rotateCredentialRequest{
		DeviceID:   state.DeviceID,
		CurrentKey: state.AgentKey,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return rotateCredentialResponse{}, err
	}

	resp, err := http.Post(rotateCredentialURL(server), "application/json", bytes.NewReader(payload))
	if err != nil {
		return rotateCredentialResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return rotateCredentialResponse{}, fmt.Errorf("rotation rejected: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var out rotateCredentialResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return rotateCredentialResponse{}, err
	}
	if out.DeviceID == "" || out.AgentKey == "" {
		return rotateCredentialResponse{}, errors.New("invalid rotation response")
	}
	if out.DeviceID != state.DeviceID {
		return rotateCredentialResponse{}, errors.New("rotation response device mismatch")
	}

	return out, nil
}

func collectAgentReport(agentStartedAt time.Time) agentReportPayload {
	hostname, _ := os.Hostname()
	currentUser := ""
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}
	execPath, _ := os.Executable()
	workingDir, _ := os.Getwd()
	localIPs := collectLocalIPs()
	cpuUsage := collectCPUUsagePercent()
	memPercent, memUsed, memTotal := collectMemoryUsage()

	return agentReportPayload{
		Hostname:           hostname,
		Username:           currentUser,
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		CPUCount:           runtime.NumCPU(),
		CPUUsagePercent:    cpuUsage,
		MemoryUsagePercent: memPercent,
		MemoryUsedBytes:    memUsed,
		MemoryTotalBytes:   memTotal,
		LocalIPs:           localIPs,
		ExecutablePath:     execPath,
		WorkingDir:         workingDir,
		ProcessID:          os.Getpid(),
		AgentStartedAt:     agentStartedAt.UTC().Format(time.RFC3339Nano),
		AgentUptimeSeconds: int64(time.Since(agentStartedAt).Seconds()),
	}
}

func collectCPUUsagePercent() float64 {
	values, err := cpu.Percent(200*time.Millisecond, false)
	if err != nil || len(values) == 0 {
		return 0
	}
	return values[0]
}

func collectMemoryUsage() (float64, uint64, uint64) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0
	}
	return vm.UsedPercent, vm.Used, vm.Total
}

func collectLocalIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}
	set := map[string]struct{}{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			set[ip.String()] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for ip := range set {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out
}
