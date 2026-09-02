@echo off
cd /d "c:\Users\Brion Lund\Documents\GoMeshCentral"
"C:\Program Files\Go\bin\go.exe" build -o server.exe ./cmd/server
if %errorlevel% equ 0 (
  echo Build successful
) else (
  echo Build failed with code %errorlevel%
  exit /b %errorlevel%
)
