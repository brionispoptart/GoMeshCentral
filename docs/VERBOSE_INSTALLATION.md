# GoMeshCentral Agent - Verbose Installation Guide

## Quick Verbose Installation (Recommended for Troubleshooting)

**Open PowerShell as Administrator** and run:

```powershell
iwr -useb http://10.10.0.242:8080/api/download/install-verbose.ps1 | iex; Install-GoMeshAgent -Server 10.10.0.242:8080
```

Or paste this in PowerShell:

```powershell
(New-Object Net.WebClient).DownloadString('http://10.10.0.242:8080/api/download/install-verbose.ps1') | iex; Install-GoMeshAgent -Server 10.10.0.242:8080
```

## What the Verbose Installer Does

The verbose installer provides **detailed logging at every stage**:

1. ✅ **Parameter Validation** - Verifies required parameters
2. ✅ **Administrator Check** - Ensures elevation privileges
3. ✅ **System Information** - Logs OS version, PowerShell version, architecture
4. ✅ **Manifest Download** - Downloads deployment configuration (with detailed logging)
5. ✅ **MSI Download** - Downloads installer with size/hash verification
6. ✅ **Directory Preparation** - Creates all necessary folders
7. ✅ **MSI Execution** - Runs with full verbose logging (`/l*v` option)
8. ✅ **File Verification** - Checks that agent.exe was installed
9. ✅ **Manifest Installation** - Places manifest in correct location
10. ✅ **Service Registration** - Registers Windows service with correct arguments
11. ✅ **Service Startup** - Starts the service
12. ✅ **Diagnostics Collection** - Captures service status and Windows Event Viewer entries

## Log Files Created

After installation, three log files are created (in `C:\Temp` or `%TEMP%`):

### 1. **GoMeshCentral-Install.log** (Main Installation Log)
Contains all steps, errors, and diagnostics.

**What to look for:**
- Search for `[ERROR]` for any failures
- Check "STEP 8: Service registration" for service issues
- Check "STEP 10: Diagnostics" for Windows Event Viewer errors

### 2. **GoMeshCentralAgent-MSI.log** (MSI Installation Details)
Low-level Windows Installer log with all file operations.

**What to look for:**
- `Return value 3` = MSI failed
- `Error 1603` = Generic installation failure
- File path issues (e.g., wrong directory)
- Permissions errors

### 3. **GoMeshCentral-Diagnostics/** (Diagnostic Bundle)
Folder containing copies of the above logs for easy collection.

## Reading Log Files

### Quick Check - Is Installation Successful?
```powershell
# Check service status
Get-Service GoMeshCentralAgent

# Expected output: Running or Stopped (either is OK)
# Status : Running (or Stopped)
# Name   : GoMeshCentralAgent
```

### If Installation Failed - Collect Logs

**Copy the logs:**
```powershell
$logPath = "$env:TEMP\GoMeshCentral-Install.log"
$msiLog = "$env:TEMP\GoMeshCentralAgent-MSI.log"
$diagnostics = "$env:TEMP\GoMeshCentral-Diagnostics"

# Show main log
Get-Content $logPath | Where-Object { $_ -match "ERROR|WARN" }

# Show MSI log (last 50 lines)
Get-Content $msiLog -Tail 50
```

**Send these files to support:**
- `C:\Temp\GoMeshCentral-Install.log`
- `C:\Temp\GoMeshCentralAgent-MSI.log`
- All files in `C:\Temp\GoMeshCentral-Diagnostics\`

## Common Log Entries to Investigate

### ❌ "Service failed to start"
**Look in logs for:**
- Service registration errors (Step 8)
- File path issues (agent.exe not at expected location)
- Windows Event Viewer entries showing Permission Denied

**Action:**
- Verify agent binary was installed: `Test-Path "C:\Program Files\GoMeshCentral\agent.exe"`
- Check service command line: `Get-Item -Path HKLM:\SYSTEM\CurrentControlSet\Services\GoMeshCentralAgent | Select-Object -ExpandProperty ImagePath`

### ❌ "Manifest download failed"
**Look in logs for:**
- Network connectivity issues
- Server hostname/IP incorrect
- Server certificate issues (if using HTTPS)

**Action:**
- Verify connectivity: `Test-NetConnection 10.10.0.242 -Port 8080`
- Try manual download: `Invoke-WebRequest -Uri http://10.10.0.242:8080/api/download/agent/manifest-installer`

### ❌ "MSI installation failed with exit code..."
**Look in MSI log for:**
- "Error 1603" (generic failure - check earlier lines)
- "Return value 3" (action failed)
- File permission errors
- Disk space issues

**Action:**
- Check disk space: `Get-Volume C: | Select-Object SizeRemaining`
- Verify temp directory: `Test-Path $env:TEMP`
- Check if agent is already running: `Get-Process agent -ErrorAction SilentlyContinue`

## Standard Installation (Non-Verbose)

If you want to run the standard installer instead:

```powershell
iwr -useb http://10.10.0.242:8080/api/download/install.ps1 | iex; Install-GoMeshAgent -Server 10.10.0.242:8080
```

## Manual MSI Installation

If PowerShell scripts don't work, download and run the MSI directly:

1. Download from: `http://10.10.0.242:8080/api/download/agent/windows-msi`
2. Right-click, "Run as Administrator"
3. Accept the installation wizard
4. Service should register and start automatically

---

**Need Help?** Share the log files (`GoMeshCentral-Install.log` and `GoMeshCentralAgent-MSI.log`) for detailed analysis.
