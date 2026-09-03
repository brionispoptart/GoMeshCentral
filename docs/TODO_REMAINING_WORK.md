# GoMeshCentral Windows Agent - Remaining Tasks

## 🎯 Priority: Build MSI Installer

### Current Status
- ✅ WiX configuration created: `packaging/windows/GoMeshCentralAgent.wxs`
- ✅ Build script created: `packaging/windows/build-msi.ps1`
- ✅ Server endpoint added: `GET /api/download/agent/windows-msi`
- ✅ Fallback installer created: `install.ps1` (MSI-first, EXE fallback)
- ⏳ **MISSING**: WiX Toolset installation + MSI build

### What to Do

#### Step 1: Install WiX Toolset
```powershell
# Option A: WiX v4 (Modern, CLI-based)
# Download from: https://github.com/wixtoolset/wix4/releases
# Install to add 'wix' command to PATH

# Option B: WiX v3 (Classic)
# Download from: https://wixtoolset.org/
# Install to add 'candle' and 'light' commands to PATH

# Verify installation
wix --version
# or
candle -version
```

#### Step 2: Build MSI
```powershell
cd GoMeshCentral\packaging\windows
.\build-msi.ps1

# Output: dist\GoMeshCentralAgent.msi (~10-15 MB)
```

#### Step 3: Deploy MSI to Server
```powershell
# Copy to server
scp dist/GoMeshCentralAgent.msi user@10.10.0.242:~/gomeshcentral/dist/

# Server will serve at: http://gomeshcentral.servr.tech/api/download/agent/windows-msi
```

#### Step 4: Test
```powershell
# Run installer with MSI (should download MSI first)
.\install.ps1 -Server 'gomeshcentral.servr.tech' -EnrollToken 'TOKEN_HERE'

# Verify device appears in dashboard
# Check Add/Remove Programs for "GoMeshCentral Agent"
```

### Benefits of MSI
- ✅ Appears in Windows "Add/Remove Programs"
- ✅ Automatic uninstall registration
- ✅ Can be deployed via Group Policy
- ✅ Uninstall rollback support
- ✅ Transform files for enterprise customization

---

## 🎯 Priority: Deploy Tray UI

### Current Status
- ✅ tray-ui.exe built: `tray-ui.exe` (9.9 MB)
- ✅ Shows connection status (Connected/Disconnected)
- ✅ Links to dashboard
- ✅ Includes uninstall option
- ⏳ **MISSING**: Auto-launch on user login, MSI integration

### What to Do

#### Option 1: Manual Deployment (Users Run Once)
```powershell
# Users can launch manually
C:\Program Files\GoMeshCentral\tray-ui.exe

# Or create a desktop shortcut
# Right-click desktop → New → Shortcut → C:\Program Files\GoMeshCentral\tray-ui.exe
```

#### Option 2: Auto-Launch on Login (Recommended)
```powershell
# Add to registry to run at login
$RegPath = 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Run'
$ExePath = 'C:\Program Files\GoMeshCentral\tray-ui.exe'
New-ItemProperty -Path $RegPath -Name 'GoMeshCentralTrayUI' -Value $ExePath -Force | Out-Null

# OR for current user only (less privileged)
$RegPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
New-ItemProperty -Path $RegPath -Name 'GoMeshCentralTrayUI' -Value $ExePath -Force | Out-Null
```

#### Option 3: Include in MSI (Best for Enterprise)
1. Update `GoMeshCentralAgent.wxs` to include tray-ui.exe
2. Add registry entry for auto-launch
3. Add Start Menu shortcut
4. See "MSI Improvements" section below

### Features Already Implemented
- ✅ System tray icon (cyan/blue 16x16 pixel icon)
- ✅ Menu items:
  - Status: Connected/Disconnected (updates every 5s)
  - Device name
  - Open Dashboard (launches browser)
  - Uninstall Agent
  - Exit Monitor
- ✅ Auto-detection of agent state file

### Features to Add (Optional)
- [ ] Custom icon with GoMeshCentral branding
- [ ] Status indicator color (green=connected, red=disconnected)
- [ ] Right-click context menu
- [ ] Double-click to open dashboard
- [ ] Tooltip showing uptime/statistics
- [ ] Settings dialog for heartbeat interval

---

## 🎯 WiX File Improvements (Next Level)

### Current MSI Includes
- ✅ Service installation with parameterized config
- ✅ Program Files directory
- ✅ ProgramData directory
- ✅ uninstall.bat script
- ✅ Registry entries for Add/Remove Programs

### Enhancements to Add
- [ ] Tray UI binary (tray-ui.exe)
- [ ] Start Menu shortcuts
  - [ ] Uninstall shortcut
  - [ ] Launch Tray UI shortcut
  - [ ] Open Dashboard shortcut
- [ ] Auto-launch Tray UI on first login
- [ ] Desktop shortcut option (user choice)
- [ ] Custom UI dialogs (EULA, server address)
- [ ] Repair functionality
- [ ] System requirements checking (Windows 10+, .NET if needed)

### Updated WiX Template
```xml
<!-- Add to GoMeshCentralAgent.wxs -->

<!-- Tray UI Component -->
<Component Directory="INSTALLFOLDER" Guid="...">
  <File Source="$(var.SourceDir)\tray-ui.exe" />
</Component>

<!-- Start Menu Shortcuts -->
<Directory Id="ProgramMenuFolder">
  <Directory Id="AppMenuFolder" Name="GoMeshCentral">
    <Component Guid="...">
      <Shortcut Id="LaunchTrayUI" Target="[INSTALLFOLDER]tray-ui.exe" />
      <Shortcut Id="LaunchDashboard" Target="http://localhost:8080" />
      <RegistryValue Key="..." Value="1" />
    </Component>
  </Directory>
</Directory>

<!-- Registry for Auto-Launch -->
<Component Guid="...">
  <RegistryKey Root="HKCU" 
    Key="Software\Microsoft\Windows\CurrentVersion\Run">
    <RegistryValue Name="GoMeshCentralTray" 
      Value="[INSTALLFOLDER]tray-ui.exe" Type="string" />
  </RegistryKey>
</Component>
```

---

## 🎯 Testing Checklist

### Windows Installation
- [ ] Download PowerShell installer
- [ ] Run as Administrator
- [ ] Auto-elevation works (no UAC errors)
- [ ] MSI or EXE downloads successfully
- [ ] Service installs with `Get-Service GoMeshCentralAgent`
- [ ] Service starts automatically
- [ ] State file created at `C:\ProgramData\GoMeshCentral\agent-state.json`
- [ ] Device appears in Devices list within 10s
- [ ] Device shows "Connected" status
- [ ] Last heartbeat updates every 10s

### Uninstallation
- [ ] Service stops cleanly
- [ ] Service registry removed
- [ ] Program Files deleted
- [ ] ProgramData cleaned
- [ ] Add/Remove Programs entry gone (if MSI)

### Tray UI
- [ ] tray-ui.exe launches without errors
- [ ] Tray icon appears in system tray
- [ ] Status shows "Connected" when service running
- [ ] "Open Dashboard" button works
- [ ] "Uninstall" button prompts confirmation
- [ ] Closing tray UI doesn't stop service

### Re-Enrollment
- [ ] Run installer with new token while service running
- [ ] Old service uninstalls cleanly
- [ ] New service installs with new token
- [ ] Device re-enrolls (new agentKey)
- [ ] Device appears as same ID in dashboard

---

## 📋 Implementation Order

### Phase 1: MSI Building (This Week)
1. Install WiX Toolset
2. Build MSI: `.\build-msi.ps1`
3. Copy MSI to `~/gomeshcentral/dist/`
4. Test installation via PowerShell script
5. Verify Add/Remove Programs support

### Phase 2: Tray UI Deployment (This Week)
1. Copy tray-ui.exe to `~/gomeshcentral/` or MSI dist
2. Add to MSI package
3. Add registry auto-launch entry
4. Test tray UI shows connected status

### Phase 3: MSI Enhancements (Next Week)
1. Add tray-ui.exe to MSI
2. Add Start Menu shortcuts
3. Add registry auto-launch for tray UI
4. Add desktop shortcut option
5. Test full enterprise deployment

### Phase 4: Advanced Features (Future)
1. Silent installation with configuration file
2. Group Policy support
3. Agent auto-update mechanism
4. Custom branding (logos, company name in UI)

---

## 🔧 Commands to Run

### Build MSI
```powershell
cd GoMeshCentral\packaging\windows
.\build-msi.ps1  # Creates dist\GoMeshCentralAgent.msi
```

### Deploy MSI
```powershell
scp dist/GoMeshCentralAgent.msi user@10.10.0.242:~/gomeshcentral/dist/
```

### Test Installation
```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command `
  "Invoke-WebRequest -Uri 'http://gomeshcentral.servr.tech/api/download/install.ps1' -OutFile 'install.ps1' -UseBasicParsing; `
   .\install.ps1 -Server 'gomeshcentral.servr.tech' -EnrollToken 'TEST_TOKEN'"
```

### View Service Status
```powershell
Get-Service GoMeshCentralAgent
Get-Content "C:\ProgramData\GoMeshCentral\agent-state.json" | ConvertFrom-Json
```

### Add Tray UI Auto-Launch
```powershell
$RegPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$ExePath = 'C:\Program Files\GoMeshCentral\tray-ui.exe'
New-ItemProperty -Path $RegPath -Name 'GoMeshCentralTrayUI' -Value $ExePath -Force
```

---

## 📚 Documentation Links

- [AGENT_DEPLOYMENT.md](./AGENT_DEPLOYMENT.md) - User-facing guide
- [WINDOWS_INSTALLATION.md](./WINDOWS_INSTALLATION.md) - Admin/developer guide
- [DEPLOYMENT_STATUS.md](./DEPLOYMENT_STATUS.md) - Current status and roadmap
- [build-msi.ps1](../packaging/windows/build-msi.ps1) - MSI build script
- [install.ps1](../packaging/windows/install.ps1) - PowerShell installer

---

## 🎯 Success Criteria

- ✅ MSI builds and installs without errors
- ✅ PowerShell installer downloads MSI first, falls back to EXE
- ✅ Device enrolls and connects after installation
- ✅ Add/Remove Programs shows GoMeshCentral Agent
- ✅ Uninstall via Add/Remove Programs works completely
- ✅ Tray UI launches and shows connection status
- ✅ Tray UI uninstall button works
- ✅ Users can re-enroll with new token
- ✅ Documentation is clear and complete

---

**Status**: Ready to Implement ✅  
**Estimated Time**: 2-4 hours  
**Complexity**: Medium (all groundwork done, just execution)
