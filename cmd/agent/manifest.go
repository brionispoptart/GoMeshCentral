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

// loadManifest reads a deployment manifest from a well-known path.
// On Windows: C:\ProgramData\GoMeshCentral\manifest.json
// On Linux: /var/lib/gomeshcentral/manifest.json
// Returns empty manifest if file does not exist (no error).
func loadManifest() (deploymentManifest, error) {
	var manifestPath string
	switch runtime.GOOS {
	case "windows":
		manifestPath = filepath.Join(windowsDataDir(), "manifest.json")
	case "linux":
		manifestPath = linuxDataDir + "/manifest.json"
	default:
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
