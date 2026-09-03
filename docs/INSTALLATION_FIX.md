# GoMeshCentral Agent MSI Installation Fix

## Problem
The agent MSI was attempting to auto-start the service immediately upon installation, before the manifest file was in place. This caused the service startup to fail with "Service failed to start" error.

## Solution
Modified the MSI installation process to:

### 1. MSI Service Configuration Changes
**File**: `packaging/windows/GoMeshCentralAgent.wxs`

Changed ServiceInstall behavior:
- `Start="auto"` → `Start="demand"` - Service won't auto-start at boot
- Removed immediate service start during installation

Changed ServiceControl behavior:
- `Start="install"` → `Start="none"` - Don't attempt to start service during MSI
- `Wait="yes"` → `Wait="no"` - Don't wait for service start

**Result**: Service is registered but not started during MSI installation.

### 2. PowerShell Installer Updates
**File**: `packaging/windows/install.ps1`

Added post-installation steps:
1. Download manifest before running MSI
2. Run MSI (service installed but not started)
3. Place manifest in `C:\ProgramData\GoMeshCentral\manifest.json`
4. Start the service after manifest is ready

```powershell
Start-Service -Name "GoMeshCentralAgent" -ErrorAction Stop
```

## Installation Flow

### Old (Broken) Flow
```
1. MSI installation starts
2. ServiceInstall configured with Start="auto"
3. Service registered AND immediately started
4. Service startup fails: manifest not yet in place
```

### New (Fixed) Flow
```
1. PowerShell installer downloads manifest
2. MSI installation starts
3. Service registered with Start="demand"
4. Service NOT started during MSI
5. Manifest placed in ProgramData directory
6. PowerShell starts service (now has manifest)
7. Service starts successfully
```

## Testing

### Clean Installation Test

1. **Uninstall any existing version**:
   ```powershell
   # As Administrator
   Get-Service GoMeshCentralAgent | Stop-Service -Force -ErrorAction SilentlyContinue
   Invoke-CimMethod -ClassName Win32_Product -MethodName Uninstall -Filter "Name='GoMeshCentral Agent'" -ErrorAction SilentlyContinue
   # Or use Add/Remove Programs
   ```

2. **Run the updated install script**:
   ```powershell
   powershell.exe -ExecutionPolicy Bypass -File install.ps1 -Server "your-server:8080"
   ```

3. **Verify installation**:
   ```powershell
   # Check service exists
   Get-Service GoMeshCentralAgent
   
   # Check manifest exists
   Get-Content "$env:ProgramData\GoMeshCentral\manifest.json" | ConvertFrom-Json
   
   # Check service status
   Get-Service GoMeshCentralAgent | Select Status, StartType
   
   # View service startup logs
   Get-Content "$env:ProgramData\GoMeshCentral\agent-startup.log" -Tail 50
   ```

### Troubleshooting

**Service still fails to start**:
1. Check manifest exists: `Test-Path "$env:ProgramData\GoMeshCentral\manifest.json"`
2. Check manifest is valid JSON: `Get-Content "$env:ProgramData\GoMeshCentral\manifest.json" | ConvertFrom-Json`
3. Check agent binary exists: `Test-Path "$env:ProgramFiles\GoMeshCentral\agent.exe"`
4. Check service configuration: `sc.exe query GoMeshCentralAgent`
5. Check service startup type is "demand": `Get-Service GoMeshCentralAgent | Select StartType`

**Manually start service**:
```powershell
Start-Service -Name "GoMeshCentralAgent" -Verbose
```

**View service logs**:
```powershell
# Windows Event Viewer
eventvwr.msc
# Look for: Windows Logs → System → GoMeshCentralAgent

# Or read agent log if available
Get-Content "$env:ProgramData\GoMeshCentral\*.log" -Tail 20 -ErrorAction SilentlyContinue
```

## Auto-Start After First Run

Once the agent has successfully connected and received credentials, you can enable auto-start:

```powershell
# As Administrator
Set-Service -Name "GoMeshCentralAgent" -StartupType Automatic
Start-Service -Name "GoMeshCentralAgent"
```

Or via the dashboard, once agent is connected, an admin can configure it to auto-start on next boot.

## Files Modified

- `packaging/windows/GoMeshCentralAgent.wxs` - Service start behavior
- `packaging/windows/install.ps1` - Post-install manifest placement + service start
- `docs/INSTALLATION_FIX.md` - This file

## Deployment Notes

1. **Service StartType is "demand"** - Admins must manually start on first boot or enable auto-start
2. **Manifest is required** - Service won't start without manifest (handles gracefully)
3. **Backward compatible** - Existing installations unaffected; only affects new installs
4. **Works with hardware identity** - Once service starts and agent runs, it auto-registers using hardware fingerprints

