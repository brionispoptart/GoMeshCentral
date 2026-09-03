@echo off
REM GoMeshCentral Agent Uninstaller
setlocal enabledelayedexpansion

echo [GoMeshCentral] Uninstalling GoMeshCentral Agent...

REM Check if running as admin
net.exe session >nul 2>&1
if errorlevel 1 (
    echo [GoMeshCentral] Requesting Administrator privileges...
    powershell.exe -Command "Start-Process cmd -ArgumentList '/c \"%~f0\"' -Verb RunAs" >nul 2>&1
    exit /b 0
)

REM Stop service
echo [GoMeshCentral] Stopping service...
sc.exe stop GoMeshCentralAgent >nul 2>&1

REM Delete service
echo [GoMeshCentral] Removing service...
sc.exe delete GoMeshCentralAgent >nul 2>&1

REM Delete files
echo [GoMeshCentral] Removing files...
if exist "C:\Program Files\GoMeshCentral" (
    rmdir /s /q "C:\Program Files\GoMeshCentral" >nul 2>&1
)

if exist "%PROGRAMDATA%\GoMeshCentral" (
    rmdir /s /q "%PROGRAMDATA%\GoMeshCentral" >nul 2>&1
)

echo [GoMeshCentral] GoMeshCentral Agent has been successfully uninstalled.
timeout /t 3 /nobreak
