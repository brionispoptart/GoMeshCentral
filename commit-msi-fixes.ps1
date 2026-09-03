# PowerShell commit script for MSI fixes
cd "C:\Users\Brion Lund\Documents\GoMeshCentral"

Write-Host "=== GoMeshCentral - Committing MSI Investigation Changes ===" -ForegroundColor Cyan
Write-Host ""

# Add all changes
git add -A

# Show what will be committed
Write-Host "Files staged for commit:" -ForegroundColor Yellow
git status --short

Write-Host ""
Write-Host "Committing..." -ForegroundColor Yellow
Write-Host ""

$message = @"
MSI Installation Troubleshooting - Error 1603 Investigation

Summary:
- Removed ServiceInstall element from WXS (was causing timeout)
- Changed RegistryComponent directory from TARGETDIR to INSTALLFOLDER
- Created minimal test WXS with only essential components  
- Tested minimal MSI - still fails with error 1603
- Issue is fundamental to MSI, not specific components

Findings:
- Error 1603 persists even with minimal configuration
- MSI log shows action failure but specific cause not identified
- WXS compilation completes successfully
- Binary file and build artifacts are valid

Next Steps:
- Investigate MSI error root cause (system-specific or WiX issue)
- Consider PowerShell-only installer as alternative
- Restart remote server for full integration testing

Files Modified:
- packaging/windows/GoMeshCentralAgent.wxs
- docs/MSI_INSTALLATION_STATUS.md (new)
- packaging/windows/GoMeshCentralAgent-minimal.wxs (test reference)
"@

git commit -m $message

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Changes committed successfully" -ForegroundColor Green
    Write-Host ""
    Write-Host "Recent commits:" -ForegroundColor Yellow
    git log --oneline -5
} else {
    Write-Host "✗ Commit failed with exit code $LASTEXITCODE" -ForegroundColor Red
}
