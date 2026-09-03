#!/usr/bin/env powershell
<#
Deploy the fixed MSI and installer to remote server
#>

$ErrorActionPreference = "Continue"
$sourceDir = "c:\Users\Brion Lund\Documents\GoMeshCentral"
$remoteServer = "root@10.10.0.99"
$remotePath = "~/gomeshcentral/"
$sshKey = "c:\Users\Brion Lund\.ssh\id_rsa"

Write-Host "=== GoMeshCentral MSI Deployment ==="
Write-Host "Source directory: $sourceDir"
Write-Host "Remote server: $remoteServer"
Write-Host "Remote path: $remotePath"
Write-Host ""

# Files to deploy
$filesToDeploy = @(
    "$sourceDir\dist\agent.exe"
    "$sourceDir\dist\GoMeshCentralAgent.msi"
    "$sourceDir\packaging\windows\install.ps1"
)

# Check files exist
Write-Host "Checking files..."
foreach ($file in $filesToDeploy) {
    if (Test-Path $file) {
        $size = (Get-Item $file).Length / 1MB
        Write-Host "✓ $(Split-Path -Leaf $file) ($([math]::Round($size, 2)) MB)"
    } else {
        Write-Host "✗ $(Split-Path -Leaf $file) - NOT FOUND"
    }
}
Write-Host ""

# Deploy using SCP
Write-Host "Deploying files to $remoteServer..."
Write-Host ""

foreach ($file in $filesToDeploy) {
    $fileName = Split-Path -Leaf $file
    Write-Host "Deploying $fileName..."
    
    & scp -i $sshKey -p $file "$remoteServer`:$remotePath" 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ $fileName deployed"
    } else {
        Write-Host "✗ $fileName deployment failed (exit code: $LASTEXITCODE)"
    }
}

Write-Host ""
Write-Host "Verifying deployment on remote server..."
& ssh -i $sshKey $remoteServer "ls -lh $remotePath/agent.exe $remotePath/GoMeshCentralAgent.msi $remotePath/install.ps1 2>&1" 2>&1

Write-Host ""
Write-Host "Deployment complete!"
