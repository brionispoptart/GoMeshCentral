#!/usr/bin/env pwsh
# Kill existing go processes
Write-Output "Stopping existing processes..."
Get-Process | Where-Object { $_.ProcessName -eq 'go' } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

# Kill any server processes
Get-Process | Where-Object { $_.ProcessName -like 'server*' } | Stop-Process -Force -ErrorAction SilentlyContinue

# Clean build cache
Write-Output "Cleaning build cache..."
& 'C:\Program Files\Go\bin\go.exe' clean

# Build explicit binary
Write-Output "Building server binary..."
& 'C:\Program Files\Go\bin\go.exe' build -o server-explicit.exe ./cmd/server
$buildExit = $LASTEXITCODE
Write-Output "Build exit code: $buildExit"

if ($buildExit -ne 0) {
    Write-Output "BUILD FAILED"
    exit 1
}

if (!(Test-Path "server-explicit.exe")) {
    Write-Output "Binary file not found!"
    exit 1
}

Write-Output "Binary created: $(Get-Item server-explicit.exe | Select-Object -ExpandProperty FullName)"
Write-Output "Starting server..."
& ".\server-explicit.exe"
