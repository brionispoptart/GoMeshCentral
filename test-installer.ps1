# Test PowerShell Installer
# Run as Administrator

$Server = "localhost:8080"
$TempDir = $env:TEMP
$TestLog = "$TempDir\GoMeshCentral-Install-Test.log"

Write-Host "========================================" | Tee-Object -FilePath $TestLog -Append
Write-Host "GoMeshCentral Installer Test" | Tee-Object -FilePath $TestLog -Append
Write-Host "========================================" | Tee-Object -FilePath $TestLog -Append
Write-Host "Server: $Server" | Tee-Object -FilePath $TestLog -Append
Write-Host "Log: $TestLog" | Tee-Object -FilePath $TestLog -Append
Write-Host "" | Tee-Object -FilePath $TestLog -Append

# Step 1: Download installer
Write-Host "Step 1: Downloading installer from http://$Server/api/download/install.ps1" | Tee-Object -FilePath $TestLog -Append
$InstallerPath = "$TempDir\GoMeshCentral-install.ps1"
try {
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri "http://$Server/api/download/install.ps1" -OutFile $InstallerPath -ErrorAction Stop
    Write-Host "✓ Installer downloaded" | Tee-Object -FilePath $TestLog -Append
} catch {
    Write-Host "✗ Failed to download installer: $_" | Tee-Object -FilePath $TestLog -Append
    exit 1
}

# Step 2: Verify installer exists
if (-not (Test-Path $InstallerPath)) {
    Write-Host "✗ Installer file not found at $InstallerPath" | Tee-Object -FilePath $TestLog -Append
    exit 1
}
Write-Host "✓ Installer file exists" | Tee-Object -FilePath $TestLog -Append

# Step 3: Run installer
Write-Host "Step 2: Running installer..." | Tee-Object -FilePath $TestLog -Append
Write-Host "" | Tee-Object -FilePath $TestLog -Append

try {
    & $InstallerPath -Server $Server 2>&1 | Tee-Object -FilePath $TestLog -Append
    $installResult = $LASTEXITCODE
} catch {
    Write-Host "✗ Installer execution failed: $_" | Tee-Object -FilePath $TestLog -Append
    $installResult = 1
}

Write-Host "" | Tee-Object -FilePath $TestLog -Append
Write-Host "Step 3: Verifying installation..." | Tee-Object -FilePath $TestLog -Append

# Step 4: Check if service exists
$ServiceName = "GoMeshCentralAgent"
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($service) {
    Write-Host "✓ Service '$ServiceName' registered" | Tee-Object -FilePath $TestLog -Append
    Write-Host "  Status: $($service.Status)" | Tee-Object -FilePath $TestLog -Append
    Write-Host "  StartType: $($service.StartType)" | Tee-Object -FilePath $TestLog -Append
} else {
    Write-Host "✗ Service '$ServiceName' not found" | Tee-Object -FilePath $TestLog -Append
}

# Step 5: Check if binary exists
$AgentPath = "$env:ProgramFiles\GoMeshCentral\agent.exe"
if (Test-Path $AgentPath) {
    Write-Host "✓ Agent binary found at $AgentPath" | Tee-Object -FilePath $TestLog -Append
    $fileInfo = Get-Item $AgentPath
    Write-Host "  Size: $($fileInfo.Length) bytes" | Tee-Object -FilePath $TestLog -Append
} else {
    Write-Host "✗ Agent binary not found at $AgentPath" | Tee-Object -FilePath $TestLog -Append
}

# Step 6: Check data directory
$DataDir = "$env:ProgramData\GoMeshCentral"
if (Test-Path $DataDir) {
    Write-Host "✓ Data directory found at $DataDir" | Tee-Object -FilePath $TestLog -Append
    Get-ChildItem $DataDir -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "  - $($_.Name)" | Tee-Object -FilePath $TestLog -Append
    }
} else {
    Write-Host "✗ Data directory not found at $DataDir" | Tee-Object -FilePath $TestLog -Append
}

Write-Host "" | Tee-Object -FilePath $TestLog -Append
Write-Host "========================================" | Tee-Object -FilePath $TestLog -Append
Write-Host "Test Complete" | Tee-Object -FilePath $TestLog -Append
Write-Host "Log saved to: $TestLog" | Tee-Object -FilePath $TestLog -Append
Write-Host "========================================" | Tee-Object -FilePath $TestLog -Append
