package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/getlantern/systray"
)

type AgentState struct {
	DeviceID string `json:"deviceId"`
	AgentKey string `json:"agentKey"`
	Name     string `json:"name"`
}

func main() {
	// Read agent state
	stateFile := filepath.Join(os.Getenv("PROGRAMDATA"), "GoMeshCentral", "agent-state.json")
	state := &AgentState{}
	if data, err := os.ReadFile(stateFile); err == nil {
		json.Unmarshal(data, state)
	}

	// Run tray UI
	go systray.Run(func() {
		setupUI(state)
	}, func() {})

	// Keep running
	select {}
}

func setupUI(state *AgentState) {
	// Load icon
	iconData := generateIcon()
	systray.SetIcon(iconData)
	systray.SetTitle("GoMeshCentral")
	systray.SetTooltip("GoMeshCentral Agent Monitor")

	// Status menu
	statusItem := systray.AddMenuItem(fmt.Sprintf("Status: Connected"), "Agent status")
	statusItem.Disable()

	if state.DeviceID != "" {
		nameItem := systray.AddMenuItem(fmt.Sprintf("Device: %s", state.Name), "Device name")
		nameItem.Disable()
	}

	systray.AddSeparator()

	// Dashboard
	dashboardItem := systray.AddMenuItem("Open Dashboard", "View web dashboard")
	go func() {
		for range dashboardItem.ClickedCh {
			exec.Command("explorer", "http://localhost:8080").Start()
		}
	}()

	systray.AddSeparator()

	// Uninstall
	uninstallItem := systray.AddMenuItem("Uninstall Agent", "Remove GoMeshCentral")
	go func() {
		for range uninstallItem.ClickedCh {
			if exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
				`Stop-Service GoMeshCentralAgent -Force -ErrorAction SilentlyContinue; `+
					`sc.exe delete GoMeshCentralAgent 2>$null; `+
					`Remove-Item 'C:\Program Files\GoMeshCentral' -Recurse -Force -ErrorAction SilentlyContinue; `+
					`Write-Host 'Agent uninstalled'`).Run() == nil {
				systray.Quit()
			}
		}
	}()

	// Exit
	exitItem := systray.AddMenuItem("Exit Monitor", "Close this window")
	go func() {
		<-exitItem.ClickedCh
		systray.Quit()
	}()

	// Update status periodically
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if isServiceRunning() {
				statusItem.SetTitle("Status: Connected")
			} else {
				statusItem.SetTitle("Status: Disconnected")
			}
		}
	}()
}

func isServiceRunning() bool {
	resp, err := http.Get("http://localhost:8080/api/devices")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func generateIcon() []byte {
	// Create a simple 16x16 cyan/blue icon
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			img.Set(x, y, color.RGBA{0, 120, 200, 255})
		}
	}
	buf := new(bytes.Buffer)
	png.Encode(buf, img)
	return buf.Bytes()
}
