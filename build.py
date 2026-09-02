#!/usr/bin/env python3
import os
import sys
import subprocess

os.chdir(r"c:\Users\Brion Lund\Documents\GoMeshCentral")

# Try to restore from git first
try:
    result = subprocess.run([
        'git', 'checkout', 'internal/storage/sqlite.go'
    ], capture_output=True, text=True)
    print(f"Git restore: {result.stdout}")
    if result.returncode != 0:
        print(f"Git error: {result.stderr}")
except Exception as e:
    print(f"Error running git: {e}")

# Now try to build
try:
    go_exe = r"C:\Program Files\Go\bin\go.exe"
    result = subprocess.run([
        go_exe, 'build', '-o', 'server.exe', './cmd/server'
    ], capture_output=True, text=True, cwd=r"c:\Users\Brion Lund\Documents\GoMeshCentral")
    print(f"Build stdout: {result.stdout}")
    if result.returncode == 0:
        print("BUILD SUCCESSFUL")
    else:
        print(f"BUILD FAILED with code {result.returncode}")
        print(f"Build stderr: {result.stderr}")
except Exception as e:
    print(f"Error running go build: {e}")
