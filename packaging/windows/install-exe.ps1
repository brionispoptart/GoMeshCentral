# GoMeshCentral Agent Windows EXE Fallback Installer
# This is used if MSI is not available from server
param(
    [string]$Server = "",
    [string]$EnrollToken = ""
)

$ErrorActionPreference = "Stop"

$InstallDir = "C:\Program Files\GoMeshCentral"
$DataDir = "$env:PROGRAMDATA\GoMeshCentral"
$ExePath = "$InstallDir\agent.exe"
$StateFile = "$DataDir\agent-state.json"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

if (-not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Path $DataDir | Out-Null
}

# If re-enrolling with a new token, uninstall the old service first
if ($EnrollToken) {
    $existingService = Get-Service -Name "GoMeshCentralAgent" -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Host "[GoMeshCentral] Removing existing service for re-enrollment..."
        Stop-Service -Name "GoMeshCentralAgent" -Force -ErrorAction SilentlyContinue
        [System.Threading.Thread]::Sleep(500)
        sc.exe delete GoMeshCentralAgent 2>$null | Out-Null
        [System.Threading.Thread]::Sleep(500)
    }
    # Also delete old state file (forces re-enrollment)
    if (Test-Path $StateFile) {
        Write-Host "[GoMeshCentral] Removing old state file for fresh enrollment..."
        Remove-Item -Path $StateFile -Force -ErrorAction SilentlyContinue
    }
}

$DownloadUrl = "http://$Server/api/download/agent/windows-amd64"
Write-Host "[GoMeshCentral] Downloading agent.exe from $DownloadUrl..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath -UseBasicParsing

Write-Host "[GoMeshCentral] Registering Windows Service..."
if ($EnrollToken) {
    & $ExePath -install-service -server $Server -enroll-token $EnrollToken
} else {
    & $ExePath -install-service -server $Server
}

Write-Host "[GoMeshCentral] Windows agent installed and started successfully!"
Write-Host "[GoMeshCentral] Service: GoMeshCentralAgent"
Write-Host "[GoMeshCentral] To uninstall, use: C:\Program Files\GoMeshCentral\uninstall.bat"
