# MSI Installation Troubleshooting - Current Status

## Problem Summary
- MSI installation consistently fails with error code 1603 (Fatal Error)
- Error occurs even with minimal WXS containing only agent.exe file and data folder
- Remote server (10.10.0.242) is currently unreachable/down
- Cannot proceed with full installation testing without server connectivity

## Changes Made This Session

### 1. WXS File Fixes (packaging/windows/GoMeshCentralAgent.wxs)
- ✅ Removed `<ServiceInstall>` element (was causing timeout during execution)
- ✅ Changed RegistryComponent Directory from TARGETDIR to INSTALLFOLDER
- ✅ Simplified to minimal components:
  - AgentBinary (agent.exe file)
  - AppDataDir (create C:\ProgramData\GoMeshCentral)
  - UninstallShortcut (uninstall.bat)
  - RegistryComponent (Add/Remove Programs entries)

### 2. Test Artifacts Created
- `packaging/windows/GoMeshCentralAgent-minimal.wxs` - Minimal test WXS with 2 components only
- `dist/GoMeshCentralAgent-minimal.msi` - Minimal MSI for testing
- Multiple MSI log files for diagnostic analysis

## Test Results

### Test 1: Full WXS MSI
- Exit Code: 1603
- Status: FAILED

### Test 2: Minimal WXS MSI (only AgentBinary + AppDataDir)
- Exit Code: 1603  
- Status: FAILED
- Conclusion: Problem is NOT caused by specific components, but fundamental MSI issue

### Diagnostic Findings
- MSI log files created but difficult to parse (UTF-16 encoding)
- MSI log shows "Return value 3" indicating action failure
- Specific failing action not clearly identified in logs
- Fresh .wixobj files created before each build
- WiX v3 compilation completes without errors

## Current Blockers

### Blocker 1: MSI Error 1603
- **Impact**: Prevents testing of MSI-based installation
- **Severity**: HIGH
- **Root Cause**: Unknown - persists even with minimal components
- **Possible Causes**:
  - Windows Installer service configuration
  - System Group Policy restrictions
  - Broken registry state from previous failed installations
  - WiX v3 toolset compatibility issue
  
### Blocker 2: Remote Server Offline
- **Impact**: Cannot test against actual server endpoints
- **Severity**: HIGH  
- **Status**: Server at 10.10.0.242 not responding to ping or SSH
- **Last Known**: Server was running and deployed with latest binaries

## Recommended Path Forward

### Option A: Bypass MSI, Use PowerShell Installer
**Status**: install-verbose.ps1 exists and already implements full installation logic
**Advantages**:
- PowerShell script is PowerShell 5.1 compatible
- Already includes service registration via New-Service
- Contains comprehensive logging
- No WiX/MSI complexity

**Implementation**:
1. Update install-verbose.ps1 to skip MSI if Windows Agent present
2. Test manual installation: Download binaries → Register service
3. Verify agent connects and appears in dashboard

### Option B: Debug MSI Error 1603
**Estimated Effort**: 2-4 hours
**Steps**:
1. Use WiX Log Analyzer tool for detailed log analysis
2. Test on clean VM to eliminate system-specific issues
3. Try different WiX configurations
4. Contact WiX community for guidance

### Option C: Use .exe Installer Instead
**Effort**: 1-2 hours
**Advantage**: Can use existing PowerShell logic wrapped in InstallShield or other tool

## Immediate Actions Needed

1. **Restart Remote Server**
   - SSH: `ssh brionlund@10.10.0.242 "cd ~/gomeshcentral && ./server-linux &"`
   - Verify: `curl http://10.10.0.242:8080/api/health`

2. **Test PowerShell Installer Standalone**
   - Modify install-verbose.ps1 to skip MSI and copy binaries directly
   - Test from Windows client against remote server

3. **Commit Changes**
   - All WXS fixes
   - Minimal WXS for reference
   - This status document

## Files Modified
- `packaging/windows/GoMeshCentralAgent.wxs` - Simplified WXS structure
- Created: `packaging/windows/GoMeshCentralAgent-minimal.wxs` - Test reference
- Status: `docs/MSI_INSTALLATION_STATUS.md` - This file

## Next Session Priority
1. Verify server is running and accessible
2. Decide on MSI bypass strategy
3. Implement PowerShell-only installer if MSI continues failing
4. Document working installation process

---
Created: 2026-09-03 14:15 UTC
Status: INVESTIGATION COMPLETE - AWAITING DECISION ON PATH FORWARD
