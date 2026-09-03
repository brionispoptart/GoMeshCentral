#!/bin/bash
# Commit script for MSI investigation and fixes

cd "$(dirname "$0")" || exit 1

echo "=== GoMeshCentral - Committing MSI Investigation Changes ==="
echo ""

# Add changes
git add -A

# Check status
echo "Files to commit:"
git status --short

echo ""
echo "Committing changes..."

# Commit with message
git commit -m "MSI Installation Troubleshooting - Error 1603 Investigation

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
- packaging/windows/GoMeshCentralAgent-minimal.wxs (test reference)"

if [ $? -eq 0 ]; then
    echo "✓ Changes committed successfully"
    echo ""
    echo "Recent commits:"
    git log --oneline -5
else
    echo "✗ Commit failed"
    exit 1
fi
