# GoMeshCentral Agent Windows PowerShell Installer
# This is the primary installation method - no MSI dependency
param(
    [string]$Server = "",
    [string]$EnrollToken = ""
)

$ErrorActionPreference = "Stop"

# Validate parameters
if (-not $Server) {
    Write-Error "-Server parameter is required (e.g., -Server 'your-server.com:8080')"
    exit 1
}

# Check Administrator privileges
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[GoMeshCentral] Relaunching installer with Administrator privileges..."
    $scriptPath = $MyInvocation.MyCommand.Path
    Start-Process powershell.exe -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" -Server `"$Server`" -EnrollToken `"$EnrollToken`""
    exit 0
}

# Define paths
$TempDir = $env:TEMP
$InstallDir = "$env:ProgramFiles\GoMeshCentral"
$DataDir = "$env:ProgramData\GoMeshCentral"
$AgentExeName = "agent.exe"
$AgentExePath = "$InstallDir\$AgentExeName"
$ManifestPath = "$DataDir\manifest.json"
$StateFilePath = "$DataDir\agent-state.json"
$ServiceName = "GoMeshCentralAgent"
$ServiceDisplayName = "GoMeshCentral Agent"

Write-Host "================================================================================"
Write-Host "[GoMeshCentral] Agent Installation"
Write-Host "================================================================================"
Write-Host "[GoMeshCentral] Server: $Server"
Write-Host "[GoMeshCentral] Install Directory: $InstallDir"
Write-Host "[GoMeshCentral] Data Directory: $DataDir"
Write-Host ""

# Step 1: Download deployment manifest (optional)
Write-Host "[GoMeshCentral] Step 1: Downloading deployment manifest..."
$ManifestUrl = "http://$Server/api/download/agent/manifest-installer"
try {
    $ProgressPreference = 'SilentlyContinue'
    $manifestRequest = @{
        Uri = $ManifestUrl
        OutFile = "$TempDir\manifest-temp.json"
        UseBasicParsing = $true
        TimeoutSec = 30
    }
    Invoke-WebRequest @manifestRequest
    Write-Host "[GoMeshCentral] ✓ Manifest downloaded successfully"
} catch {
    Write-Host "[GoMeshCentral] ⚠ Manifest download failed (optional): $($_.Exception.Message)"
    Write-Host "[GoMeshCentral]   Agent will use default settings"
}
$ProgressPreference = 'Continue'

# Step 2: Download Windows agent binary
Write-Host "[GoMeshCentral] Step 2: Downloading agent binary..."
$BinaryUrl = "http://$Server/api/download/agent/windows-amd64"
$TempBinaryPath = "$TempDir\agent-download.exe"

try {
    $ProgressPreference = 'SilentlyContinue'
    $binaryRequest = @{
        Uri = $BinaryUrl
        OutFile = $TempBinaryPath
        UseBasicParsing = $true
        TimeoutSec = 60
    }
    Invoke-WebRequest @binaryRequest
    
    # Verify binary exists and has reasonable size
    $fileInfo = Get-Item $TempBinaryPath
    if ($fileInfo.Length -lt 1000000) {  # Less than 1MB is suspicious
        throw "Downloaded file is too small ($($fileInfo.Length) bytes)"
    }
    Write-Host "[GoMeshCentral] ✓ Agent binary downloaded ($([math]::Round($fileInfo.Length / 1MB, 2)) MB)"
} catch {
    Write-Error "[GoMeshCentral] Failed to download agent binary: $($_.Exception.Message)"
    exit 1
} finally {
    $ProgressPreference = 'Continue'
}

# Step 3: Prepare installation directories
Write-Host "[GoMeshCentral] Step 3: Preparing installation directories..."
try {
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Write-Host "[GoMeshCentral] ✓ Created $InstallDir"
    }
    
    if (-not (Test-Path $DataDir)) {
        New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
        Write-Host "[GoMeshCentral] ✓ Created $DataDir"
    }
} catch {
    Write-Error "[GoMeshCentral] Failed to create directories: $_"
    exit 1
}

# Step 4: Stop existing service if running
Write-Host "[GoMeshCentral] Step 4: Checking for existing service..."
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($service) {
    if ($service.Status -eq 'Running') {
        Write-Host "[GoMeshCentral] Stopping existing service..."
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue | Out-Null
        Start-Sleep -Seconds 2
    }
}

# Step 5: Copy binary to installation directory
Write-Host "[GoMeshCentral] Step 5: Installing agent binary..."
try {
    Copy-Item -Path $TempBinaryPath -Destination $AgentExePath -Force
    Write-Host "[GoMeshCentral] ✓ Agent binary installed to $AgentExePath"
} catch {
    Write-Error "[GoMeshCentral] Failed to copy agent binary: $_"
    exit 1
}

# Step 6: Install deployment manifest
Write-Host "[GoMeshCentral] Step 6: Installing deployment manifest..."
if (Test-Path "$TempDir\manifest-temp.json") {
    try {
        Copy-Item -Path "$TempDir\manifest-temp.json" -Destination $ManifestPath -Force
        Write-Host "[GoMeshCentral] ✓ Manifest installed to $ManifestPath"
    } catch {
        Write-Warning "[GoMeshCentral] Failed to install manifest: $_"
    }
    Remove-Item "$TempDir\manifest-temp.json" -Force -ErrorAction SilentlyContinue
}

# Step 7: Register Windows service
Write-Host "[GoMeshCentral] Step 7: Registering Windows service..."
if ($service) {
    # Service exists, just ensure it's configured correctly
    Write-Host "[GoMeshCentral] ✓ Service already registered"
} else {
    try {
        # Build service binary path with state file argument
        $binPath = "$AgentExePath -state `"$StateFilePath`""
        
        # Create the service with LocalSystem account
        New-Service -Name $ServiceName `
                   -DisplayName $ServiceDisplayName `
                   -BinaryPathName $binPath `
                   -StartupType "Automatic" `
                   -Account "LocalSystem" `
                   -ErrorAction Stop | Out-Null
        
        Write-Host "[GoMeshCentral] ✓ Service registered successfully"
    } catch {
        Write-Error "[GoMeshCentral] Failed to register service: $_"
        exit 1
    }
}

# Step 8: Start the service
Write-Host "[GoMeshCentral] Step 8: Starting service..."
try {
    Start-Service -Name $ServiceName -ErrorAction Stop | Out-Null
    Start-Sleep -Seconds 2
    $svc = Get-Service -Name $ServiceName
    if ($svc.Status -eq 'Running') {
        Write-Host "[GoMeshCentral] ✓ Service started successfully"
    } else {
        Write-Warning "[GoMeshCentral] ⚠ Service status is $($svc.Status)"
    }
} catch {
    Write-Warning "[GoMeshCentral] Failed to start service immediately: $_"
    Write-Host "[GoMeshCentral]   Service will start on next system boot"
}

# Cleanup
Remove-Item $TempBinaryPath -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "================================================================================"
Write-Host "[GoMeshCentral] Installation Complete!"
Write-Host "================================================================================"
Write-Host "[GoMeshCentral] Service Name: $ServiceName"
Write-Host "[GoMeshCentral] Service Status: $($(Get-Service -Name $ServiceName).Status)"
Write-Host "[GoMeshCentral] Agent Binary: $AgentExePath"
Write-Host "[GoMeshCentral] Data Directory: $DataDir"
Write-Host ""
Write-Host "[GoMeshCentral] The agent will now connect to $Server"
Write-Host "[GoMeshCentral] Check 'Devices' tab in 30 seconds to see the device appear"
Write-Host ""
