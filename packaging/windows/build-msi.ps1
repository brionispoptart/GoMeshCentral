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

# 3. Copy uninstall.bat to dist
$UninstallSrc = "$ScriptDir\uninstall.bat"
$UninstallDst = "$OutDir\uninstall.bat"
if (Test-Path $UninstallSrc) {
    Copy-Item $UninstallSrc $UninstallDst -Force
    Write-Host "[GoMeshCentral] Copied uninstall.bat to $UninstallDst"
} else {
    Write-Warning "[GoMeshCentral] uninstall.bat not found at $UninstallSrc"
}

# 4. Check for WiX Toolset
$MsiPath = "$OutDir\GoMeshCentralAgent.msi"
$wixFound = $false

# Try modern WiX (v4+) first
if (Get-Command wix -ErrorAction SilentlyContinue) {
    Write-Host "[GoMeshCentral] Found WiX v4+ CLI..."
    $wixFound = $true
    $WxsPath = "$ScriptDir\GoMeshCentralAgent.wxs"
    
    try {
        wix build "$WxsPath" -o "$MsiPath" -d SourceDir="$OutDir"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[GoMeshCentral] MSI built successfully: $MsiPath"
            Write-Host "[GoMeshCentral] File size: $((Get-Item $MsiPath).Length / 1MB)MB"
        } else {
            Write-Error "WiX build failed with exit code $LASTEXITCODE"
            exit 1
        }
    } catch {
        Write-Error "Failed to build MSI: $_"
        exit 1
    }
}
# Try classic WiX (v3) tools
elseif ((Get-Command candle -ErrorAction SilentlyContinue) -and (Get-Command light -ErrorAction SilentlyContinue)) {
    Write-Host "[GoMeshCentral] Found WiX v3 (candle/light)..."
    $wixFound = $true
    $WxsPath = "$ScriptDir\GoMeshCentralAgent.wxs"
    $WixObjPath = "$OutDir\GoMeshCentralAgent.wixobj"
    
    try {
        & candle "-dSourceDir=$OutDir" -out "$WixObjPath" "$WxsPath"
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Candle compilation failed"
            exit 1
        }
        
        & light -out "$MsiPath" "$WixObjPath"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[GoMeshCentral] MSI built successfully: $MsiPath"
            Write-Host "[GoMeshCentral] File size: $((Get-Item $MsiPath).Length / 1MB)MB"
        } else {
            Write-Error "Light linking failed with exit code $LASTEXITCODE"
            exit 1
        }
    } catch {
        Write-Error "Failed to build MSI: $_"
        exit 1
    }
}
else {
    Write-Error "WiX Toolset not found. Please install WiX (v3 or v4+) and ensure candle/light/wix are on PATH"
    Write-Host "Download WiX from: https://wixtoolset.org/"
    exit 1
}

if (-not (Test-Path $MsiPath)) {
    Write-Error "MSI file was not created"
    exit 1
}

Write-Host "[GoMeshCentral] Build complete!"
Write-Host "[GoMeshCentral] MSI created at: $MsiPath"
exit 0
