# GoMeshCentral Agent Installation & Deployment

## Quick Start (Recommended)

### For Administrators
1. Log in to GoMeshCentral dashboard
2. Go to **Admin → Download Agent**
3. Click **Create Agent** (Windows or Linux)
4. Choose installation method:
   - **Option 1:** Download `agent.exe` + copy enrollment command
   - **Option 2:** Copy full PowerShell installation command
5. Run the command on target machine
6. Device appears in **Devices** page within 10 seconds

### For End Users (IT Managed)
IT administrator provides you with a link or PowerShell command.
Simply run it as Administrator and agent installs automatically.

---

## Installation Methods

### Method 1: PowerShell (Recommended)

```powershell
# Copy this full command from the dashboard
powershell -NoProfile -ExecutionPolicy Bypass -Command `
  "Invoke-WebRequest -Uri 'http://gomeshcentral.servr.tech/api/download/install.ps1' -OutFile 'install.ps1' -UseBasicParsing; `
   .\install.ps1 -Server 'gomeshcentral.servr.tech' -EnrollToken 'YOUR_TOKEN_HERE'"
```

**What it does:**
1. ✅ Downloads installer from server
2. ✅ Auto-elevates to Administrator
3. ✅ Detects and removes old installation
4. ✅ Downloads MSI or EXE (fallback)
5. ✅ Installs Windows service
6. ✅ Auto-enrolls device
7. ✅ Service starts automatically

### Method 2: Manual Download + Install

```powershell
# 1. Download installer script
Invoke-WebRequest -Uri 'http://server/api/download/install.ps1' -OutFile 'install.ps1' -UseBasicParsing

# 2. Run installer with enrollment token
.\install.ps1 -Server 'server:8080' -EnrollToken 'TOKEN_FROM_DASHBOARD'

# 3. Verify (should show "Connected" in web UI within 10s)
Get-Service GoMeshCentralAgent
```

### Method 3: Direct Binary Download

```powershell
# Download agent.exe and manual register service
Invoke-WebRequest -Uri 'http://server/api/download/agent/windows-amd64' -OutFile 'agent.exe' -UseBasicParsing

# Install as service with enrollment
.\agent.exe -install-service -server server:8080 -enroll-token TOKEN123
```

---

## Uninstallation

### Option 1: Add/Remove Programs (GUI)
1. Open Settings → **Apps** → **Apps & features**
2. Search for **"GoMeshCentral Agent"**
3. Click **→ Uninstall**

### Option 2: Command Line (Quick)

```powershell
# Method A: If installed via MSI
msiexec.exe /x GoMeshCentralAgent.msi /qn /norestart

# Method B: Run uninstall script
C:\Program Files\GoMeshCentral\uninstall.bat

# Method C: PowerShell (full cleanup)
Stop-Service GoMeshCentralAgent -Force -ErrorAction SilentlyContinue
sc.exe delete GoMeshCentralAgent
Remove-Item 'C:\Program Files\GoMeshCentral' -Recurse -Force
Remove-Item 'C:\ProgramData\GoMeshCentral' -Recurse -Force
```

---

## System Tray Monitor (Optional)

After installation, you can monitor agent status with the tray icon app:

```powershell
# Launch tray monitor (optional)
& 'C:\Program Files\GoMeshCentral\tray-ui.exe'

# Tray icon shows:
# ✅ Connected/Disconnected status
# ✅ Device name
# ✅ Link to dashboard
# ✅ Uninstall option
```

---

## Troubleshooting

### Service Not Running?
```powershell
# Check status
Get-Service GoMeshCentralAgent

# Check for errors
Get-Content "C:\ProgramData\GoMeshCentral\install.log" -Tail 20
```

### Device Not Appearing in Dashboard?
```powershell
# Check agent state
Get-Content "C:\ProgramData\GoMeshCentral\agent-state.json" | ConvertFrom-Json

# Restart service
Restart-Service GoMeshCentralAgent
```

### Installation Failed?
```powershell
# 1. Ensure admin privileges
([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

# 2. Check internet connection to server
Test-Connection gomeshcentral.servr.tech

# 3. Clean and retry
Stop-Service GoMeshCentralAgent -Force -ErrorAction SilentlyContinue
Remove-Item "C:\ProgramData\GoMeshCentral\agent-state.json" -Force -ErrorAction SilentlyContinue
.\install.ps1 -Server 'server:8080' -EnrollToken 'NEW_TOKEN'
```

---

## Linux Installation

```bash
# Download installer
wget http://server/api/download/install.sh -O install.sh
chmod +x install.sh

# Run with enrollment token
./install.sh -Server server:8080 -EnrollToken TOKEN123

# Verify
systemctl status gomeshcentral-agent
```

---

## FAQ

### Q: What happens if I close the installer window?
**A:** Service runs in background. Installer automatically closes after success.

### Q: Can I re-enroll with a new token?
**A:** Yes, run the installer with a new token. Old enrollment will be replaced.

### Q: Does it work without internet?
**A:** No, agent requires connection to GoMeshCentral server.

### Q: Is it safe?
**A:** Yes, agent only runs commands from authenticated server. All communication encrypted.

### Q: Can I uninstall without admin?
**A:** No, agent service requires administrator to uninstall.

---

## Advanced Options

### Custom Heartbeat Interval

```powershell
# Edit service args (requires reboot)
# Default: 10 seconds

Stop-Service GoMeshCentralAgent -Force
sc.exe stop GoMeshCentralAgent

# Via registry (for IT deployment)
$svcPath = 'HKLM:\SYSTEM\CurrentControlSet\Services\GoMeshCentralAgent'
Set-ItemProperty $svcPath ImagePath 'C:\Program Files\GoMeshCentral\agent.exe -server server:8080 -heartbeat-seconds 60 -report-seconds 300'

Restart-Service GoMeshCentralAgent
```

### Enrollment Without Token

```powershell
# Service starts but waits for enrollment
.\install.ps1 -Server server:8080

# Later, enroll manually
$stateFile = 'C:\ProgramData\GoMeshCentral\agent-state.json'
# Agent will use enrollment token passed to service or stored in state

# Or re-run installer with token
.\install.ps1 -Server server:8080 -EnrollToken TOKEN123
```

---

## Support & Feedback

- **Dashboard URL:** https://gomeshcentral.servr.tech
- **Documentation:** See docs/WINDOWS_INSTALLATION.md
- **Report Issues:** Contact administrator
