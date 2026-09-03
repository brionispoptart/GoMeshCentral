# GoMeshCentral Agent Windows Verbose Installer with Comprehensive Logging
param(
    [string]$Server = "",
    [string]$EnrollToken = "",
    [string]$LogPath = "$env:TEMP\GoMeshCentral-Install.log"
)

$ErrorActionPreference = "Stop"

# Initialize log file
function Log-Event {
    param(
        [string]$Message,
        [ValidateSet("INFO", "WARN", "ERROR", "DEBUG")]
        [string]$Level = "INFO"
    )
    
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss.fff"
    $logLine = "[$timestamp] [$Level] $Message"
    
    # Write to console with color
    switch ($Level) {
        "ERROR" { Write-Host $logLine -ForegroundColor Red }
        "WARN" { Write-Host $logLine -ForegroundColor Yellow }
        "DEBUG" { Write-Host $logLine -ForegroundColor Gray }
        default { Write-Host $logLine -ForegroundColor Green }
    }
    
    # Write to log file
    Add-Content -Path $LogPath -Value $logLine -ErrorAction SilentlyContinue
}

function Log-Separator {
    $sep = "=" * 80
    Write-Host $sep -ForegroundColor Cyan
    Add-Content -Path $LogPath -Value $sep -ErrorAction SilentlyContinue
}

# Start logging
Log-Separator
Log-Event "GoMeshCentral Agent Windows Installer - VERBOSE MODE"
Log-Event "Log file: $LogPath"
Log-Event "Started at: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Log-Separator

# Validate parameters
Log-Event "Validating parameters..."
if (-not $Server) {
    Log-Event "-Server <host:port> parameter is required" "ERROR"
    exit 1
}
Log-Event "Server: $Server"
Log-Event "Enroll Token: $(if ($EnrollToken) { '***REDACTED***' } else { 'NOT PROVIDED' })"

# Check Administrator
Log-Event "Checking Administrator privileges..."
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Log-Event "Not running as Administrator, relaunching with elevation..." "WARN"
    $scriptPath = $MyInvocation.MyCommand.Path
    Start-Process powershell.exe -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" -Server `"$Server`" -EnrollToken `"$EnrollToken`" -LogPath `"$LogPath`""
    exit 0
}
Log-Event "Running as Administrator: YES"

# System Information
Log-Event "Collecting system information..."
Log-Event "OS: $([System.Environment]::OSVersion.VersionString)"
Log-Event "PowerShell Version: $($PSVersionTable.PSVersion)"
$arch = if ([System.Environment]::Is64BitOperatingSystem) { '64-bit' } else { '32-bit' }
Log-Event "Architecture: $arch"

# Directory setup
$TempDir = $env:TEMP
$MsiPath = "$TempDir\GoMeshCentralAgent.msi"
$MsiLogPath = "$TempDir\GoMeshCentralAgent-MSI.log"
$ManifestPath = "$TempDir\manifest.json"
$DataDir = "$env:ProgramData\GoMeshCentral"
$DownloadUrl = "http://$Server/api/download/agent/windows-msi"
$ManifestUrl = "http://$Server/api/download/agent/manifest-installer"

Log-Event "Temp Directory: $TempDir"
Log-Event "MSI Path: $MsiPath"
Log-Event "MSI Log Path: $MsiLogPath"
Log-Event "Manifest Path: $ManifestPath"
Log-Event "Data Directory: $DataDir"

# Step 1: Download manifest
Log-Separator
Log-Event "STEP 1: Downloading deployment manifest"
Log-Event "URL: $ManifestUrl"

try {
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $ManifestUrl -OutFile $ManifestPath -UseBasicParsing -TimeoutSec 30
    Log-Event "Manifest downloaded successfully"
    Log-Event "Manifest size: $(if (Test-Path $ManifestPath) { (Get-Item $ManifestPath).Length } else { 'UNKNOWN' }) bytes"
    
    # Try to parse and display manifest contents (redacted)
    if (Test-Path $ManifestPath) {
        $manifestContent = Get-Content $ManifestPath -Raw
        Log-Event "Manifest content (first 500 chars): $($manifestContent.Substring(0, [Math]::Min(500, $manifestContent.Length)))"
    }
} catch {
    Log-Event "Manifest download failed: $_" "WARN"
    Log-Event "Exception: $($_.Exception.Message)" "DEBUG"
    Log-Event "Will proceed without manifest; agent will use CLI defaults" "WARN"
}

# Step 2: Download MSI
Log-Separator
Log-Event "STEP 2: Downloading MSI installer"
Log-Event "URL: $DownloadUrl"

try {
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $MsiPath -UseBasicParsing -TimeoutSec 60
    Log-Event "MSI downloaded successfully"
    
    $msiFile = Get-Item $MsiPath
    Log-Event "MSI size: $($msiFile.Length) bytes"
    Log-Event "MSI created: $($msiFile.CreationTime)"
    Log-Event "MSI last modified: $($msiFile.LastWriteTime)"
    
    # Verify MSI is valid
    if ($msiFile.Length -lt 1MB) {
        Log-Event "WARNING: MSI size seems unusually small" "WARN"
    }
} catch {
    Log-Event "MSI download failed: $_" "ERROR"
    Log-Event "Exception: $($_.Exception.Message)" "ERROR"
    exit 1
}

# Step 3: Prepare installation
Log-Separator
Log-Event "STEP 3: Preparing installation directories"

try {
    if (-not (Test-Path $DataDir)) {
        Log-Event "Creating data directory: $DataDir"
        New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
        Log-Event "Directory created successfully"
    } else {
        Log-Event "Data directory already exists"
    }
} catch {
    Log-Event "Failed to create data directory: $_" "ERROR"
}

# Step 4: Run MSI with verbose logging
Log-Separator
Log-Event "STEP 4: Executing MSI installer"
Log-Event "MSI logging will be saved to: $MsiLogPath"

$MsiArgs = @(
    "/i", "$MsiPath",
    "/qn",  # Silent install
    "/norestart",
    "/l*v", "$MsiLogPath"  # Verbose logging to file
)

Log-Event "MSI arguments: $($MsiArgs -join ' ')"
Log-Event "Executing: msiexec.exe $($MsiArgs -join ' ')"

try {
    $process = Start-Process msiexec.exe -ArgumentList $MsiArgs -Wait -PassThru -NoNewWindow
    Log-Event "MSI process completed with exit code: $($process.ExitCode)"
} catch {
    Log-Event "Failed to execute MSI: $_" "ERROR"
    Log-Event "Exception: $($_.Exception.Message)" "ERROR"
    exit 1
}

# Capture MSI log
Log-Separator
Log-Event "STEP 5: Analyzing MSI installation log"

if (Test-Path $MsiLogPath) {
    Log-Event "MSI log file found, size: $(Get-Item $MsiLogPath | Select-Object -ExpandProperty Length) bytes"
    
    # Extract key information from MSI log
    Log-Event "MSI Log Last 50 lines:" "DEBUG"
    $msiLogContent = Get-Content $MsiLogPath -Tail 50
    foreach ($line in $msiLogContent) {
        Log-Event "  $line" "DEBUG"
    }
    
    # Check for errors in MSI log
    $msiErrors = Get-Content $MsiLogPath | Select-String -Pattern "Error|Failed|Return value 3" | Select-Object -First 10
    if ($msiErrors) {
        Log-Event "Errors found in MSI log:" "WARN"
        foreach ($error in $msiErrors) {
            Log-Event "  $error" "WARN"
        }
    }
} else {
    Log-Event "MSI log file not found" "WARN"
}

# Check MSI result
if ($process.ExitCode -ne 0) {
    Log-Event "MSI installation FAILED with exit code $($process.ExitCode)" "ERROR"
    Log-Event "See detailed log at: $MsiLogPath" "ERROR"
    Log-Event "See installation log at: $LogPath" "ERROR"
    exit $process.ExitCode
}

Log-Event "MSI installation completed successfully"

# Step 6: Verify file installation
Log-Separator
Log-Event "STEP 6: Verifying installed files"

$AgentExePath = "$env:ProgramFiles\GoMeshCentral\agent.exe"
$ServiceName = "GoMeshCentralAgent"

Log-Event "Checking for agent binary: $AgentExePath"
if (Test-Path $AgentExePath) {
    $exeFile = Get-Item $AgentExePath
    Log-Event "Agent binary found!"
    Log-Event "  Size: $($exeFile.Length) bytes"
    Log-Event "  Created: $($exeFile.CreationTime)"
    Log-Event "  Modified: $($exeFile.LastWriteTime)"
} else {
    Log-Event "Agent binary NOT found at $AgentExePath" "ERROR"
}

# Step 7: Place manifest
Log-Separator
Log-Event "STEP 7: Installing deployment manifest"

if ((Test-Path $ManifestPath) -and (Test-Path $DataDir)) {
    Log-Event "Copying manifest to: $DataDir\manifest.json"
    try {
        Copy-Item $ManifestPath "$DataDir\manifest.json" -Force -ErrorAction Stop
        Log-Event "Manifest installed successfully"
        $manifestFile = Get-Item "$DataDir\manifest.json"
        Log-Event "  Size: $($manifestFile.Length) bytes"
    } catch {
        Log-Event "Failed to copy manifest: $_" "WARN"
    }
} elseif (Test-Path $ManifestPath) {
    Log-Event "Manifest exists but data directory not found" "WARN"
} else {
    Log-Event "Manifest file not found (agent will use defaults)" "WARN"
}

# Step 8: Service registration
Log-Separator
Log-Event "STEP 8: Service registration"

$ServiceDisplayName = "GoMeshCentral Agent"
Log-Event "Service Name: $ServiceName"
Log-Event "Service Display Name: $ServiceDisplayName"

try {
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    
    if ($service) {
        Log-Event "Service already exists"
        Log-Event "  Status: $($service.Status)"
        Log-Event "  Start Type: $(Get-Service $ServiceName | Select-Object -ExpandProperty StartType)"
        
        # Update start type if needed
        Log-Event "Updating service start type to Demand..."
        Set-Service -Name $ServiceName -StartupType "Demand" -ErrorAction Stop
        Log-Event "Service configuration updated"
    } else {
        Log-Event "Service does not exist, creating new service..."
        
        if (-not (Test-Path $AgentExePath)) {
            Log-Event "ERROR: Agent binary not found at $AgentExePath" "ERROR"
            exit 1
        }
        
        $binPath = "$AgentExePath -state `"$DataDir\agent-state.json`""
        Log-Event "Binary path: $binPath"
        
        New-Service -Name $ServiceName `
                   -DisplayName $ServiceDisplayName `
                   -BinaryPathName $binPath `
                   -StartupType "Demand" `
                   -Account "LocalSystem" `
                   -ErrorAction Stop | Out-Null
        
        Log-Event "Service registered successfully"
    }
} catch {
    Log-Event "Service registration error: $_" "ERROR"
    Log-Event "Exception: $($_.Exception.Message)" "ERROR"
    exit 1
}

# Step 9: Start service
Log-Separator
Log-Event "STEP 9: Starting service"

try {
    Log-Event "Attempting to start service..."
    Start-Service -Name $ServiceName -ErrorAction Stop
    Start-Sleep -Seconds 2
    
    $service = Get-Service -Name $ServiceName
    Log-Event "Service started successfully"
    Log-Event "  Status: $($service.Status)"
} catch {
    Log-Event "Failed to start service: $_" "WARN"
    Log-Event "Service can be started manually later" "WARN"
}

# Step 10: Collect diagnostic information
Log-Separator
Log-Event "STEP 10: Collecting diagnostic information"

# Service details
try {
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        Log-Event "Service Status:"
        Log-Event "  Name: $($service.Name)"
        Log-Event "  Display Name: $($service.DisplayName)"
        Log-Event "  Status: $($service.Status)"
        Log-Event "  Start Type: $(Get-Service $ServiceName | Select-Object -ExpandProperty StartType)"
        
        # Query registry for service command line
        $regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName"
        if (Test-Path $regPath) {
            $regKey = Get-Item $regPath
            $imagePath = $regKey.GetValue("ImagePath")
            Log-Event "  Image Path: $imagePath"
        }
    }
} catch {
    Log-Event "Failed to get service details: $_" "WARN"
}

# Check for recent Windows events
Log-Event "Recent Windows Application Events:"
try {
    $events = Get-EventLog -LogName Application -Newest 20 -ErrorAction SilentlyContinue | 
              Where-Object { $_.Source -like "*GoMesh*" -or $_.Source -like "*Service*" }
    if ($events) {
        foreach ($event in $events) {
            Log-Event "  [$($event.TimeGenerated)] $($event.Source) - $($event.Message.Substring(0, [Math]::Min(100, $event.Message.Length)))"
        }
    } else {
        Log-Event "  (No GoMeshCentral-related events found)"
    }
} catch {
    Log-Event "Failed to query event log: $_" "DEBUG"
}

# Cleanup
Log-Separator
Log-Event "STEP 11: Cleanup"

try {
    if (Test-Path $MsiPath) {
        Remove-Item $MsiPath -Force -ErrorAction SilentlyContinue
        Log-Event "Removed temporary MSI file"
    }
} catch {
    Log-Event "Failed to clean up MSI file: $_" "WARN"
}

# Final summary
Log-Separator
Log-Event "INSTALLATION COMPLETE"
Log-Event "Status: SUCCESSFUL"
Log-Event "Log file saved to: $LogPath"
Log-Event "MSI log saved to: $MsiLogPath"
Log-Event ""
Log-Event "Next steps:"
Log-Event "  1. Verify service is running: Get-Service GoMeshCentralAgent"
Log-Event "  2. Check logs if service not running: Get-EventLog -LogName Application -Newest 20"
Log-Event "  3. Share logs if issues: $LogPath and $MsiLogPath"
Log-Separator
Log-Event "Installation finished at: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"

# Create diagnostic bundle
Log-Event "Creating diagnostic bundle..."
$diagnosticDir = "$env:TEMP\GoMeshCentral-Diagnostics"
if (-not (Test-Path $diagnosticDir)) {
    New-Item -ItemType Directory -Path $diagnosticDir -Force | Out-Null
}

# Copy all relevant logs
Copy-Item $LogPath "$diagnosticDir\Install.log" -Force -ErrorAction SilentlyContinue
Copy-Item $MsiLogPath "$diagnosticDir\MSI.log" -Force -ErrorAction SilentlyContinue

Log-Event "Diagnostic files saved to: $diagnosticDir"
Log-Event "You can send these files for analysis if installation fails"
