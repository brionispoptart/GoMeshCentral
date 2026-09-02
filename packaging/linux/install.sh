#!/bin/sh
# GoMeshCentral Agent Linux Installer Script
set -e

SERVER=""
ENROLL_TOKEN=""
INSTALL_DIR="/opt/gomeshcentral"
STATE_DIR="/var/lib/gomeshcentral"

while [ $# -gt 0 ]; do
  case "$1" in
    -server|--server)
      SERVER="$2"; shift 2 ;;
    -enroll-token|--enroll-token)
      ENROLL_TOKEN="$2"; shift 2 ;;
    *)
      shift ;;
  esac
done

if [ -z "$SERVER" ]; then
  echo "Error: -server <host:port> is required" >&2
  exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script must be run as root (use sudo)" >&2
  exit 1
fi

echo "[GoMeshCentral] Installing Linux Agent..."
mkdir -p "$INSTALL_DIR" "$STATE_DIR"
chmod 700 "$STATE_DIR"

if [ ! -f "$INSTALL_DIR/gomesh-agent" ]; then
  if [ -f "./gomesh-agent" ]; then
    cp "./gomesh-agent" "$INSTALL_DIR/gomesh-agent"
  else
    echo "[GoMeshCentral] Downloading gomesh-agent from http://$SERVER/api/download/agent/linux-amd64..."
    if command -v curl >/dev/null 2>&1; then
      curl -sSL "http://$SERVER/api/download/agent/linux-amd64" -o "$INSTALL_DIR/gomesh-agent"
    elif command -v wget >/dev/null 2>&1; then
      wget -qO "$INSTALL_DIR/gomesh-agent" "http://$SERVER/api/download/agent/linux-amd64"
    else
      echo "Error: Neither curl nor wget is available to download agent binary" >&2
      exit 1
    fi
  fi
fi

chmod +x "$INSTALL_DIR/gomesh-agent"

echo "[GoMeshCentral] Registering systemd service..."
if [ -n "$ENROLL_TOKEN" ]; then
  "$INSTALL_DIR/gomesh-agent" -install-service -server "$SERVER" -enroll-token "$ENROLL_TOKEN"
else
  "$INSTALL_DIR/gomesh-agent" -install-service -server "$SERVER"
fi

echo "[GoMeshCentral] Linux agent installed and started successfully!"
