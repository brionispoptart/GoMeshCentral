# PowerShell Installer - Testing Guide

## Quick Test Procedure

### Step 1: Start the Server
```powershell
cd C:\Users\Brion Lund\Documents\GoMeshCentral
go build -o dist/server-windows.exe ./cmd/server
cd dist
.\server-windows.exe
```

### Step 2: Test PowerShell Installer
Open a new PowerShell window (Run as Administrator):

```powershell
# Set your server address (if running locally, use localhost:8080)
$server = "localhost:8080"

# Download the installer
Invoke-WebRequest -Uri "http://$server/api/download/install.ps1" -OutFile install.ps1

# Run the installer
.\install.ps1 -Server $server

# Monitor installation:
Get-Service GoMeshCentralAgent | Select-Object Status, StartType

# Check agent binary
Get-Item "C:\Program Files\GoMeshCentral\agent.exe"

# Check manifest location
Get-Item "C:\ProgramData\GoMeshCentral" -ErrorAction SilentlyContinue
```

### Step 3: Verify Service
```powershell
# Service details
Get-Service -Name GoMeshCentralAgent | Format-List *

# Service startup type should be "Automatic"
# Status should be "Running"

# Try stopping and starting
Stop-Service -Name GoMeshCentralAgent
Start-Service -Name GoMeshCentralAgent
```

### Step 4: Check Device in Dashboard
- Open http://localhost:8080 in browser
- Navigate to Devices tab
- New device should appear within 30 seconds
- Look for "Download Agent" button at top right

### Step 5: Test Verbose Installer (for diagnostics)
```powershell
# Download verbose installer for detailed logging
Invoke-WebRequest -Uri "http://$server/api/download/install-verbose.ps1" -OutFile install-verbose.ps1

# Run with full diagnostics
.\install-verbose.ps1 -Server $server

# Check logs at:
# $env:TEMP\GoMeshCentral-Install.log
# $env:TEMP\GoMeshCentralAgent-MSI.log (from previous runs, should not be created now)
```

## What Should NOT Happen
- ❌ No MSI download attempt
- ❌ No msiexec.exe process
- ❌ No "windows-msi" HTTP requests
- ❌ No installation prompts or wizards
- ❌ No uninstall.bat references

## What SHOULD Happen
- ✅ Binary downloaded from `/api/download/agent/windows-amd64`
- ✅ Service registered with `New-Service` cmdlet
- ✅ Service status shown as "Automatic" startup type
- ✅ Agent binary at `C:\Program Files\GoMeshCentral\agent.exe`
- ✅ Data directory at `C:\ProgramData\GoMeshCentral\`
- ✅ Service running within seconds
- ✅ Device appears in Devices tab

## Troubleshooting

### Service Won't Start
```powershell
# Check service binary path
(Get-Service -Name GoMeshCentralAgent | Select-Object -ExpandProperty Description)

# Check if binary exists
Test-Path "C:\Program Files\GoMeshCentral\agent.exe"

# Try starting manually
Start-Service -Name GoMeshCentralAgent -Verbose

# Check system event log for errors
Get-EventLog -LogName System -Newest 20 | Where-Object Source -match GoMesh
```

### Binary Not Found
```powershell
# Check download location
Get-Item "$env:TEMP\agent-download.exe" -ErrorAction SilentlyContinue

# Check server response
Invoke-WebRequest -Uri "http://localhost:8080/api/download/agent/windows-amd64" -OutFile test.exe
ls -la test.exe
```

### Installation Command Fails
```powershell
# Test server connectivity
Test-NetConnection -ComputerName localhost -Port 8080

# Test download directly
(Invoke-WebRequest -Uri "http://localhost:8080/api/download/install.ps1" -UseBasicParsing).StatusCode

# Check PowerShell version (must be 5.1+)
$PSVersionTable.PSVersion
```

## Linux Testing (No Changes)
```bash
# Download and run Linux installer
curl -sSL http://your-server:8080/api/download/install.sh | bash -s -- -server your-server:8080

# Verify agent running
systemctl status gomesh-agent
ps aux | grep gomesh-agent

# Check device in dashboard
# Should appear in Devices tab
```

## Cleanup (if needed)
```powershell
# Stop service
Stop-Service -Name GoMeshCentralAgent -Force

# Uninstall service
Remove-Service -Name GoMeshCentralAgent

# Remove binaries
Remove-Item "C:\Program Files\GoMeshCentral" -Recurse -Force

# Remove data directory
Remove-Item "C:\ProgramData\GoMeshCentral" -Recurse -Force

# Clean temp files
Remove-Item "$env:TEMP\agent-*" -Force -ErrorAction SilentlyContinue
```

## Expected Timeline
1. Download & execute installer: 5 seconds
2. Service registration: 1 second
3. Service start: 2 seconds
4. Agent connection to server: 10 seconds
5. Device visible in dashboard: 30 seconds
**Total time: ~50 seconds**

## Success Criteria
- [ ] PowerShell installer completes without errors
- [ ] Service registered and running
- [ ] Agent binary in correct location
- [ ] Device appears in Devices tab
- [ ] No MSI-related steps or downloads
- [ ] Installation time < 1 minute
- [ ] No elevation prompts after initial admin check
