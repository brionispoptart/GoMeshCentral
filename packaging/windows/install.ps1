# GoMeshCentral Agent Windows 1-Line PowerShell Installer
param(
    [string]$Server = "",
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

$TempDir = $env:TEMP
$MsiPath = "$TempDir\GoMeshCentralAgent.msi"
$ManifestPath = "$TempDir\manifest.json"
$DownloadUrl = "http://$Server/api/download/agent/windows-msi"
$ManifestUrl = "http://$Server/api/download/agent/manifest-installer"

Write-Host "[GoMeshCentral] Downloading deployment manifest from $ManifestUrl..."
try {
    Invoke-WebRequest -Uri $ManifestUrl -OutFile $ManifestPath -UseBasicParsing
    Write-Host "[GoMeshCentral] Manifest downloaded successfully"
} catch {
    Write-Warning "[GoMeshCentral] Manifest download failed (will use MSI defaults): $_"
}

Write-Host "[GoMeshCentral] Downloading MSI installer from $DownloadUrl..."
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $MsiPath -UseBasicParsing
} catch {
    Write-Host "[GoMeshCentral] MSI download failed, falling back to EXE installer..."
    # Fallback to old exe-based installation if MSI not available
    & "$PSScriptRoot\install-exe.ps1" -Server $Server -EnrollToken $EnrollToken
    exit $LASTEXITCODE
}

Write-Host "[GoMeshCentral] Running MSI installer..."

# Build MSI arguments
$MsiArgs = @(
    "/i", "$MsiPath",
    "/qn",  # Silent install
    "/norestart",
    "SERVER=$Server"
)

if ($EnrollToken) {
    $MsiArgs += "ENROLL_TOKEN=$EnrollToken"
}

# Execute MSI
$process = Start-Process msiexec.exe -ArgumentList $MsiArgs -Wait -PassThru

if ($process.ExitCode -eq 0) {
    Write-Host "[GoMeshCentral] Agent installed successfully!"
    Write-Host "[GoMeshCentral] Service: GoMeshCentralAgent"
    Write-Host "[GoMeshCentral] To uninstall, use Add/Remove Programs or run: uninstall.bat"
} else {
    Write-Error "MSI installation failed with exit code $($process.ExitCode)"
    exit $process.ExitCode
}

# After MSI installation, place manifest in the data directory
$DataDir = "$env:ProgramData\GoMeshCentral"
if ((Test-Path $ManifestPath) -and (Test-Path $DataDir)) {
    Copy-Item $ManifestPath "$DataDir\manifest.json" -Force -ErrorAction SilentlyContinue
    Write-Host "[GoMeshCentral] Manifest installed to $DataDir\manifest.json"
}

# Register the service (MSI only extracts files, doesn't register service)
$AgentExePath = "$env:ProgramFiles\GoMeshCentral\agent.exe"
$ServiceName = "GoMeshCentralAgent"
$ServiceDisplayName = "GoMeshCentral Agent"

# Check if service already exists
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $service) {
    Write-Host "[GoMeshCentral] Registering Windows service..."
    try {
        # Build service arguments - manifest will be loaded at runtime from manifest.json
        $binPath = "$AgentExePath -state `"$DataDir\agent-state.json`""
        
        # Create the service
        New-Service -Name $ServiceName `
                   -DisplayName $ServiceDisplayName `
                   -BinaryPathName $binPath `
                   -StartupType "Demand" `
                   -Account "LocalSystem" `
                   -ErrorAction Stop | Out-Null
        
        Write-Host "[GoMeshCentral] Service registered successfully"
    } catch {
        Write-Error "Failed to register service: $_"
        exit 1
    }
} else {
    Write-Host "[GoMeshCentral] Service already exists, updating start type..."
    Set-Service -Name $ServiceName -StartupType "Demand" -ErrorAction SilentlyContinue
}

# Start the service now that manifest and binary are in place
Write-Host "[GoMeshCentral] Starting GoMeshCentralAgent service..."
try {
    Start-Service -Name $ServiceName -ErrorAction Stop
    Write-Host "[GoMeshCentral] Service started successfully"
} catch {
    Write-Warning "[GoMeshCentral] Failed to start service immediately; it will start on next boot: $_"
}

# Cleanup
Remove-Item $MsiPath -Force -ErrorAction SilentlyContinue
