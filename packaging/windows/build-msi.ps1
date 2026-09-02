# GoMeshCentral Agent MSI Packaging Script
param(
    [string]$OutDir = "dist",
    [string]$GoPath = "C:\Program Files\Go\bin\go.exe"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Resolve-Path "$ScriptDir\..\.."

Set-Location $RootDir

Write-Host "[GoMeshCentral] Packaging Windows Agent MSI..."

# 1. Ensure output directory exists
if (-not (Test-Path $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir | Out-Null
}

# 2. Build agent binary
$ExePath = "$OutDir\agent.exe"
Write-Host "[GoMeshCentral] Building agent binary at $ExePath..."
if (Test-Path $GoPath) {
    & $GoPath build -o $ExePath ./cmd/agent
} else {
    go build -o $ExePath ./cmd/agent
}

if ($LASTEXITCODE -ne 0 -or -not (Test-Path $ExePath)) {
    Write-Error "Failed to build agent.exe"
    exit 1
}

# 3. Check for WiX Toolset
$MsiPath = "$OutDir\GoMeshCentralAgent.msi"
$wixCandidate = Get-Command candle, light, wix -ErrorAction SilentlyContinue

if ($wixCandidate) {
    Write-Host "[GoMeshCentral] Building MSI using WiX Toolset..."
    $WxsPath = "$ScriptDir\GoMeshCentralAgent.wxs"
    $WixObjPath = "$OutDir\GoMeshCentralAgent.wixobj"
    
    if (Get-Command candle -ErrorAction SilentlyContinue) {
        candle -DSourceDir="$OutDir" -out $WixObjPath $WxsPath
        light -out $MsiPath $WixObjPath
    } else {
        wix build $WxsPath -o $MsiPath -d SourceDir="$OutDir"
    }
} else {
    Write-Host "[GoMeshCentral] WiX CLI not found on PATH."
    Write-Host "[GoMeshCentral] Note: agent.exe includes built-in Windows ARP (Add/Remove Programs) registration when installed via -install-service."
    Write-Host "[GoMeshCentral] To build formal MSIs, install WiX v3/v4 or run build-msi.ps1 with WiX installed."
}

Write-Host "[GoMeshCentral] Windows packaging complete. Agent executable: $ExePath"
