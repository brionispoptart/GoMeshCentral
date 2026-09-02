#!/usr/bin/env python3
import os
import sys

# Check if the file exists and is valid
filepath = r"c:\Users\Brion Lund\Documents\GoMeshCentral\internal\storage\sqlite.go"

if os.path.exists(filepath):
    with open(filepath, 'r') as f:
        lines = f.readlines()
    
    print(f"File has {len(lines)} lines")
    print(f"Last 10 lines:")
    for i, line in enumerate(lines[-10:], start=len(lines)-9):
        print(f"{i}: {line.rstrip()}")
else:
    print(f"File not found: {filepath}")
