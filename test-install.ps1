# GoMeshCentral Agent Windows 1-Line PowerShell Installer
param(
    [string]$Server = "gomeshcentral.servr.tech",
    [string]$EnrollToken = ""
)

$ErrorActionPreference = "Stop"

if (-not $Server) {
    Write-Error "-Server <host:port> parameter is required"
    exit 1
}

# Check Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[GoMeshCentral] Relaunching installer with Administrator privileges..."
    $scriptPath = $MyInvocation.MyCommand.Path
    Start-Process powershell.exe -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" -Server `"$Server`" -EnrollToken `"$EnrollToken`""
    exit 0
}

$InstallDir = "C:\Program Files\GoMeshCentral"
$ExePath = "$InstallDir\agent.exe"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
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
