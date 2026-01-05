#!/bin/bash
# Port-forward MCP Gateway to localhost for testing

set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# Parent directory contains the YAML files
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

# Default port
PORT="${1:-8811}"

echo "=========================================="
echo "MCP Gateway Port Forward"
echo "=========================================="
echo ""

# Check if MCP gateway is deployed
if ! kubectl get deployment mcp-gateway &> /dev/null; then
    echo "Error: mcp-gateway deployment not found"
    echo "Please run ./deploy.sh first"
    exit 1
fi

# Check if gateway pods exist
echo "Checking MCP Gateway status..."
if ! kubectl get pods -l app=mcp-gateway &> /dev/null; then
    echo "Error: MCP Gateway pods not found"
    echo "Please run ./deploy.sh first"
    exit 1
fi

# Wait for gateway to be ready
echo "Waiting for MCP Gateway to be ready..."
kubectl wait --for=condition=ready pod -l app=mcp-gateway --timeout=30s

# Check if port is already in use
if lsof -ti:$PORT &> /dev/null; then
    echo "⚠️  Port $PORT is already in use"
    echo ""
    echo "Killing existing process on port $PORT..."
    lsof -ti:$PORT | xargs kill -9 2>/dev/null || true
    sleep 2
fi

# Show MCP Gateway info
echo ""
echo "=========================================="
echo "✓ MCP Gateway Ready"
echo "=========================================="
echo ""

echo "Gateway Status:"
kubectl get pods -l app=mcp-gateway -o wide
echo ""

echo "Available MCP Servers:"
kubectl get pods -l app=mcp-server -o wide
echo ""

echo "Gateway will be available at:"
echo "  http://localhost:$PORT/mcp"
echo ""
echo "Transport: streaming"
echo "Authentication: Disabled (container mode)"
echo ""
echo "To test the connection:"
echo "  curl http://localhost:$PORT/mcp"
echo "  (Should return: 'GET requires an active session')"
echo ""
echo "To connect your MCP client:"
echo "  URL: http://localhost:$PORT/mcp"
echo "  Transport: streaming"
echo ""
echo "Press Ctrl+C to stop port forwarding"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Start port forward
kubectl port-forward service/mcp-gateway $PORT:8811
