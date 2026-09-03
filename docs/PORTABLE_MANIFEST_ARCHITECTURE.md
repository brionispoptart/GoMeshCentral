# Portable Manifest Architecture for Agent Deployment

## Overview

The agent deployment now uses a **manifest-based configuration system** that enables truly portable binaries. Instead of compiling agents with deployment-specific endpoints, the server generates a JSON manifest file containing all deployment configuration, and the agent reads this manifest at runtime.

## Architecture

### Three-Part Flow

```
1. Server Setup Phase
   └─ Server endpoint configured in dashboard
   └─ Branding configured in dashboard
   └─ Organization settings stored

2. Installer Download Phase  
   └─ Admin/user initiates agent download
   └─ Server generates manifest.json with org settings
   └─ Installer downloads manifest alongside binary

3. Agent Runtime Phase
   └─ Agent starts up
   └─ Loads manifest from well-known path
   └─ Uses manifest server endpoint to connect
   └─ Performs auto-registration via hardware identity
```

## Manifest Structure

The manifest is JSON containing deployment-specific configuration:

```json
{
  "serverEndpoint": "gomeshcentral.example.com:8080",
  "orgId": "org-12345",
  "bootstrapToken": "token-xyz",
  "companyName": "ACME IT Services",
  "logoDataUrl": "data:image/png;base64,..."
}
```

### Fields

- **serverEndpoint** (required): The server endpoint the agent connects to (e.g., `server.example.com:8080`)
- **orgId**: Organization ID for branding context
- **bootstrapToken**: Optional one-time credential for initial registration
- **companyName**: Organization name for UI branding
- **logoDataUrl**: Base64-encoded logo for tray UI

## Server-Side Implementation

### Manifest Generation Endpoints

**`GET /api/download/agent/manifest`** - Authenticated
- Requires user to be logged in
- Returns manifest for the user's organization
- Includes branding and server endpoint

**`POST /api/download/agent/manifest-installer`** - For installer workflows
- Called by installers after user selects configuration
- Returns manifest with bootstrap credentials
- Used during MSI/script-based deployment

### Implementation Details

Located in `internal/httpapi/manifest.go`:

```go
type deploymentManifest struct {
    ServerEndpoint string `json:"serverEndpoint"`
    OrgID          string `json:"orgId,omitempty"`
    BootstrapToken string `json:"bootstrapToken,omitempty"`
    CompanyName    string `json:"companyName,omitempty"`
    LogoDataURL    string `json:"logoDataUrl,omitempty"`
}

func (s *Server) handleAgentManifest(w http.ResponseWriter, r *http.Request)
func (s *Server) handleDownloadAgentManifestForInstaller(w http.ResponseWriter, r *http.Request)
```

## Agent-Side Implementation

### Manifest Loading

Located in `cmd/agent/manifest.go`:

Agent loads manifest from well-known paths:
- **Windows**: `C:\ProgramData\GoMeshCentral\manifest.json`
- **Linux**: `/var/lib/gomeshcentral/manifest.json`

```go
type deploymentManifest struct {
    ServerEndpoint string `json:"serverEndpoint"`
    OrgID          string `json:"orgId,omitempty"`
    BootstrapToken string `json:"bootstrapToken,omitempty"`
    CompanyName    string `json:"companyName,omitempty"`
    LogoDataURL    string `json:"logoDataUrl,omitempty"`
}

func loadManifest() (deploymentManifest, error)
```

### Startup Integration

Modified `startAgent()` in `cmd/agent/main.go` to:

1. Load manifest (if present)
2. Merge with command-line flags (flags take precedence)
3. Use manifest values for connection

```go
// Load deployment manifest if present
manifest, err := loadManifest()

// Merge manifest with command-line flags
if server == "" && manifest.ServerEndpoint != "" {
    server = manifest.ServerEndpoint
}
if enrollToken == "" && manifest.BootstrapToken != "" {
    enrollToken = manifest.BootstrapToken
}
```

## Installer Integration

### Windows PowerShell Installer

**Modified**: `packaging/windows/install.ps1`

Steps:
1. Download manifest from `http://$Server/api/download/agent/manifest-installer`
2. Download MSI
3. Run MSI installation
4. Place manifest at `C:\ProgramData\GoMeshCentral\manifest.json`

```powershell
$ManifestUrl = "http://$Server/api/download/agent/manifest-installer"
Invoke-WebRequest -Uri $ManifestUrl -OutFile $ManifestPath
# ... run MSI ...
Copy-Item $ManifestPath "$DataDir\manifest.json"
```

### Linux Shell Installer

**Modified**: `packaging/linux/install.sh`

Steps:
1. Download manifest from `http://$SERVER/api/download/agent/manifest-installer`
2. Download agent binary
3. Install systemd service
4. Place manifest at `/var/lib/gomeshcentral/manifest.json`

```bash
curl -sSL "http://$SERVER/api/download/agent/manifest-installer" \
     -o "$STATE_DIR/manifest.json"
```

## Deployment Flow

### For New Deployments

1. **Server Admin** configures:
   - Server endpoint in dashboard
   - Branding (company name, logo)
   - Organization settings

2. **IT Technician** initiates download:
   - Requests Windows MSI or Linux installer
   - Server generates deployment-specific manifest
   - Downloads installer + manifest together

3. **On Target Machine**:
   - Runs installer (MSI or shell script)
   - Installer places manifest in data directory
   - Agent service starts
   - Agent reads manifest and connects to configured server
   - Auto-registers using hardware identity consensus (2-of-3 signals)

### Portability Benefits

- **Single Binary**: Same agent.exe/agent works for all deployments
- **Server-Controlled**: All deployment config changes via dashboard
- **No Recompilation**: New deployments don't require Go rebuild
- **Cross-Platform**: Linux server can generate Windows MSI without C/C++ toolchain
- **Distributed**: Third-party IT shops can run their own server instance
- **Branding**: Each deployment gets custom branding (company name, logo)

## Configuration Precedence

When the agent starts, settings are applied in this order (first match wins):

1. **Manifest file** - If manifest.json exists with values
2. **Command-line flags** - If provided at service installation
3. **Defaults** - Hardcoded fallbacks (localhost:8080)

This allows:
- Default behavior from manifest
- Override with service flags if needed
- Manual specification without manifest

## Hardware Identity Auto-Registration

Once the agent connects with the correct endpoint (from manifest), it automatically registers using the 2-of-3 hardware identity consensus system:

1. Agent collects hardware identifiers (Windows MachineGuid + BIOS serial, Linux machine-id + product UUID + board serial)
2. Hashes identifiers (SHA-256)
3. POSTs to `/api/agents/register` with all three hashes
4. Server matches against existing devices
5. If ≥2 hashes match existing device: device updated and refreshed
6. If <2 hashes match: new device created
7. Server issues agent key credential
8. Agent persists credential locally for future connections

**Result**: Agent connects without requiring enrollment token, purely based on stable hardware identifiers.

## Security Considerations

- **Manifest in Plaintext**: Contains server endpoint (public info) but not secrets
- **Bootstrap Token**: Optional, can be included for additional first-run security
- **Hardware Identity**: SHA-256 hashes prevent accidental serial number leaks
- **Agent Key**: Bcrypt-hashed in database, only raw key sent once to agent
- **Manifest Access**: Authenticated endpoint available only to logged-in admins

## Future Enhancements

- [ ] Manifest versioning for backward compatibility
- [ ] Signed manifests for security-critical deployments
- [ ] Manifest refresh intervals (agent periodically re-checks for config changes)
- [ ] Per-device manifest overrides in dashboard
- [ ] Manifest template system for white-label deployments
- [ ] Manifest encryption for sensitive deployments

## Troubleshooting

### Agent Won't Connect

1. Verify manifest exists at expected path:
   - Windows: `C:\ProgramData\GoMeshCentral\manifest.json`
   - Linux: `/var/lib/gomeshcentral/manifest.json`

2. Check manifest content:
   ```bash
   # Windows PowerShell
   Get-Content "$env:ProgramData\GoMeshCentral\manifest.json" | ConvertFrom-Json
   
   # Linux
   cat /var/lib/gomeshcentral/manifest.json | jq .
   ```

3. Verify server endpoint in manifest matches running server

4. Check agent logs for manifest loading errors

### Manifest Not Downloaded

If installer fails to download manifest:

1. Verify server endpoint is correct
2. Check network connectivity to server
3. Verify server is running `/api/download/agent/manifest-installer` endpoint
4. Agent falls back to command-line flags if manifest absent

## Files Modified

- `cmd/agent/manifest.go` - Manifest loading (new)
- `cmd/agent/main.go` - startAgent manifest integration
- `internal/httpapi/manifest.go` - Server manifest handlers (new)
- `internal/httpapi/server.go` - Route registration
- `packaging/windows/install.ps1` - Manifest download
- `packaging/linux/install.sh` - Manifest download

