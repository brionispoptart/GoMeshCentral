# GoMeshCentral Windows Agent - Professional Installation Guide

## Overview
GoMeshCentral Windows Agent can be deployed as:
1. **EXE Installer** (Quick, automatic enrollment) - Currently used
2. **MSI Installer** (Enterprise, Add/Remove Programs support) - Recommended
3. **Service Direct** (Manual installation by IT)

## Current Status

### ✅ Working Features
- Service-based deployment with auto-start
- Automatic enrollment via token
- WebSocket connection to server
- Remote command dispatch
- Heartbeat reporting

### 🔄 In Development
- **Tray Icon** (System tray UI) - Service sessions can't show tray icons; solution uses separate user-mode app
- **MSI Package** (Add/Remove Programs) - Requires WiX Toolset
- **Uninstall Shortcut** (Add/Remove Programs) - Added to MSI

---

## Part 1: Building the MSI Installer (for Administrators)

### Prerequisites
- WiX Toolset v4.x or v3.x
  - **Download:** https://wixtoolset.org/
  - **For v4 (Recommended):** Install `wix` command-line tool
  - **For v3:** Install candle & light tools

### Steps to Build MSI

```powershell
# 1. Ensure you have Go 1.26+ installed
go version

# 2. Run the MSI build script
cd packaging\windows
.\build-msi.ps1

# Output: dist\GoMeshCentralAgent.msi
```

The MSI will be created in `dist/GoMeshCentralAgent.msi`.

### Features Included in MSI
- ✅ Automatic service installation
- ✅ Program Files installation
- ✅ Registry entries for Add/Remove Programs
- ✅ uninstall.bat for command-line removal
- ✅ Enrollment token support (passed as MSI property)

---

## Part 2: Deploying MSI to Server

```bash
# Copy MSI to server dist directory
scp dist/GoMeshCentralAgent.msi user@server:~/gomeshcentral/dist/

# Server will serve it at: http://server/api/download/agent/windows-msi
```

---

## Part 3: Installation Methods

### Method 1: PowerShell (Recommended for End Users)

```powershell
# One-liner with auto-enrollment
powershell -NoProfile -ExecutionPolicy Bypass -Command `
  "Invoke-WebRequest -Uri 'http://server/api/download/install.ps1' -OutFile 'install.ps1' -UseBasicParsing; .\install.ps1 -Server 'server:8080' -EnrollToken 'TOKEN'"

# Flow:
# 1. Script attempts to download MSI first
# 2. Falls back to EXE if MSI unavailable
# 3. Auto-elevates to Administrator
# 4. Installs service
# 5. Auto-enrolls with token
```

### Method 2: Direct MSI (for IT Deployment)

```powershell
# Silent install with token
msiexec.exe /i GoMeshCentralAgent.msi /qn SERVER=server:8080 ENROLL_TOKEN=TOKEN123 /norestart

# Silent install without token (manual enrollment later)
msiexec.exe /i GoMeshCentralAgent.msi /qn SERVER=server:8080 /norestart
```

### Method 3: EXE Fallback (if MSI unavailable)

```powershell
# If server is not configured with MSI, falls back to EXE
.\install-exe.ps1 -Server 'server:8080' -EnrollToken 'TOKEN'
```

---

## Part 4: Uninstallation Methods

### Option 1: Add/Remove Programs (GUI)
1. Open Settings → Apps → Apps & features
2. Search for "GoMeshCentral Agent"
3. Click → Uninstall

### Option 2: Command Line

```powershell
# Via MSI (if installed via MSI)
msiexec.exe /x GoMeshCentralAgent.msi /qn /norestart

# Via batch script
C:\Program Files\GoMeshCentral\uninstall.bat

# Via PowerShell (manual)
Stop-Service GoMeshCentralAgent -Force -ErrorAction SilentlyContinue
sc.exe delete GoMeshCentralAgent
Remove-Item "C:\Program Files\GoMeshCentral" -Recurse -Force
Remove-Item "C:\ProgramData\GoMeshCentral" -Recurse -Force
```

---

## Part 5: Tray Icon / Monitoring (Coming Soon)

**Current Status:** Service runs headless (no tray icon)

**Planned Solution:** Separate user-mode tray application
```powershell
# Planned: Users can optionally run
C:\Program Files\GoMeshCentral\tray-ui.exe

# This will display:
# - Service status
# - Connected/Disconnected state
# - Server address
# - Unattended access toggle
# - Shutdown option
```

This allows administrators to deploy headless services while users can monitor status when logged in.

---

## Part 6: Server Configuration

### Endpoints Available
- `GET /api/download/install.ps1` - PowerShell installer (with MSI fallback)
- `GET /api/download/install-exe.ps1` - Direct EXE installer
- `GET /api/download/agent/windows-amd64` - Agent executable
- `GET /api/download/agent/windows-msi` - MSI installer (404 if not available)
- `GET /api/download/agent/linux-amd64` - Linux agent

### Server Properties
Set these when building MSI or running installer:
- `SERVER` - Server hostname:port (required)
- `ENROLL_TOKEN` - One-time enrollment token (optional)
- `HEARTBEAT_SEC` - Heartbeat interval (default: 10)
- `REPORT_SEC` - Report interval (default: 60)

---

## Troubleshooting

### Service Won't Start
```powershell
# Check service status
Get-Service GoMeshCentralAgent

# View logs
Get-Content "C:\ProgramData\GoMeshCentral\install.log" -Tail 10

# Check Windows Event Viewer
Get-EventLog Application -Source "*GoMeshCentral*" -Newest 5
```

### Agent Not Connecting
```powershell
# Verify state file exists
Test-Path "C:\ProgramData\GoMeshCentral\agent-state.json"

# View enrollment status
Get-Content "C:\ProgramData\GoMeshCentral\agent-state.json" | ConvertFrom-Json
```

### Uninstall Not Working
```powershell
# Force stop service
Stop-Service GoMeshCentralAgent -Force -ErrorAction SilentlyContinue

# Delete service registry
sc.exe delete GoMeshCentralAgent

# Remove folders
Remove-Item "C:\Program Files\GoMeshCentral" -Recurse -Force
Remove-Item "C:\ProgramData\GoMeshCentral" -Recurse -Force
```

---

## Support
For issues, contact: admin@gomeshcentral.servr.tech
