#!/usr/bin/env pwsh
# Build and run explicit server binary for GoMeshCentral

Write-Output "========================================="
Write-Output "GoMeshCentral Server - Explicit Binary Build"
Write-Output "========================================="

$ErrorActionPreference = "Stop"

# Kill existing processes
Write-Output "`n[*] Stopping existing processes..."
try {
    Get-Process -Name "go" -ErrorAction SilentlyContinue | Stop-Process -Force
    Get-Process -Name "server" -ErrorAction SilentlyContinue | Stop-Process -Force
    Get-Process -Name "agent" -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Milliseconds 500
    Write-Output "[+] Processes stopped"
} catch {
    Write-Output "[!] Error stopping processes: $_"
}

# Clean Go cache
Write-Output "`n[*] Cleaning Go cache..."
& "C:\Program Files\Go\bin\go.exe" clean
if ($LASTEXITCODE -eq 0) {
    Write-Output "[+] Go cache cleaned"
} else {
    Write-Output "[!] Go clean failed"
}

# Build server binary
Write-Output "`n[*] Building server binary..."
& "C:\Program Files\Go\bin\go.exe" build -v -o server-explicit.exe ./cmd/server
$serverBuildExit = $LASTEXITCODE
if ($serverBuildExit -eq 0) {
    Write-Output "[+] Server binary built: $(Get-Item server-explicit.exe | Select-Object -ExpandProperty FullName)"
} else {
    Write-Output "[!] Server build failed (exit code: $serverBuildExit)"
    exit 1
}

# Build agent binary
Write-Output "`n[*] Building agent binary..."
& "C:\Program Files\Go\bin\go.exe" build -v -o agent-explicit.exe ./cmd/agent
$agentBuildExit = $LASTEXITCODE
if ($agentBuildExit -eq 0) {
    Write-Output "[+] Agent binary built: $(Get-Item agent-explicit.exe | Select-Object -ExpandProperty FullName)"
} else {
    Write-Output "[!] Agent build failed (exit code: $agentBuildExit)"
    exit 1
}

# Start server
Write-Output "`n[*] Starting server..."
Write-Output "Server listening on: http://localhost:8080"
Write-Output "`nPress Ctrl+C to stop server"
Write-Output "========================================="

& .\server-explicit.exe
