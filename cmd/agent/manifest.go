package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

const (
	linuxDataDir = "/var/lib/gomeshcentral"
)

// deploymentManifest contains server-provided configuration that is portable
// across installations and generated per-deployment by the server.
type deploymentManifest struct {
	ServerEndpoint string `json:"serverEndpoint"`
	OrgID          string `json:"orgId,omitempty"`
	BootstrapToken string `json:"bootstrapToken,omitempty"`
	CompanyName    string `json:"companyName,omitempty"`
	LogoDataURL    string `json:"logoDataUrl,omitempty"`
}

// getManifestPath returns the path to the deployment manifest file
// based on the current platform.
func getManifestPath() string {
	switch runtime.GOOS {
	case "windows":
		// On Windows, use ProgramData\GoMeshCentral\manifest.json
		// We can't call windowsDataDir() here since it's Windows-only,
		// so we compute it directly
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "GoMeshCentral", "manifest.json")
	case "linux":
		return filepath.Join(linuxDataDir, "manifest.json")
	default:
		return "" // no manifest on unsupported platforms
	}
}

// loadManifest reads a deployment manifest from a well-known path.
// On Windows: C:\ProgramData\GoMeshCentral\manifest.json
// On Linux: /var/lib/gomeshcentral/manifest.json
// Returns empty manifest if file does not exist (no error).
func loadManifest() (deploymentManifest, error) {
	manifestPath := getManifestPath()
	if manifestPath == "" {
		return deploymentManifest{}, nil // no manifest on unsupported platforms
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return deploymentManifest{}, nil // manifest not yet provided
		}
		return deploymentManifest{}, err
	}

	var manifest deploymentManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return deploymentManifest{}, err
	}
	return manifest, nil
}
