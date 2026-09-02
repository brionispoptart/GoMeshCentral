#!/bin/bash

# GoMeshCentral Deployment Script for Linux Server

set -e

cd ~/gomeshcentral

echo "=== GoMeshCentral Server Setup ==="
echo ""

# Create data directory if it doesn't exist
mkdir -p data

# Check if binaries exist
if [ ! -f server-linux ]; then
    echo "ERROR: server-linux binary not found"
    exit 1
fi

if [ ! -f agent-linux ]; then
    echo "ERROR: agent-linux binary not found"
    exit 1
fi

echo "✓ Binaries found"
echo ""

# Set environment variables for this session
export GMC_LISTEN_ADDR=:8080
export GMC_JWT_SECRET=dev-secret-change-in-production
export GMC_BOOTSTRAP_ADMIN_USER=admin
export GMC_BOOTSTRAP_ADMIN_PASS=admin123
export GMC_DB_PATH=data/gomeshcentral.db

echo "Starting GoMeshCentral Server..."
nohup ./server-linux > server.log 2>&1 &
SERVER_PID=$!
echo "Server started with PID: $SERVER_PID"

# Wait for server to start
sleep 3

# Check if server is running
if ps -p $SERVER_PID > /dev/null; then
    echo "✓ Server process is running"
else
    echo "✗ Server process died. Check server.log:"
    cat server.log
    exit 1
fi

echo ""
echo "Starting GoMeshCentral Agent..."
nohup ./agent-linux -server localhost:8080 > agent.log 2>&1 &
AGENT_PID=$!
echo "Agent started with PID: $AGENT_PID"

# Wait for agent to start
sleep 2

# Check if agent is running
if ps -p $AGENT_PID > /dev/null; then
    echo "✓ Agent process is running"
else
    echo "✗ Agent process died. Check agent.log:"
    cat agent.log
    exit 1
fi

echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Server: http://10.10.0.242:8080/client"
echo "Username: admin"
echo "Password: admin123"
echo ""
echo "Logs:"
echo "  Server: ~/gomeshcentral/server.log"
echo "  Agent: ~/gomeshcentral/agent.log"
echo ""
echo "To stop the services, run:"
echo "  kill $SERVER_PID $AGENT_PID"
echo ""
