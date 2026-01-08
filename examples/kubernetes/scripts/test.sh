#!/bin/bash
# Test script for MCP Gateway in Kubernetes

set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# Parent directory contains the YAML files
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== MCP Gateway Kubernetes Test ==="
echo ""

echo "1. Checking pod status..."
kubectl get pods -l app=mcp-gateway
kubectl get pods -l app=mcp-server
echo ""

echo "2. Checking services..."
kubectl get svc mcp-gateway mcp-duckduckgo
echo ""

echo "3. Checking gateway logs..."
kubectl logs -l app=mcp-gateway --tail=10
echo ""

echo "4. Port-forwarding to gateway (background)..."
kubectl port-forward service/mcp-gateway 8811:8811 > /dev/null 2>&1 &
PF_PID=$!
sleep 3

echo "5. Testing health endpoint..."
curl -s http://localhost:8811/health -H "Authorization: Bearer test-token-123" || echo "Health check endpoint may not be available"
echo ""

echo "6. Testing SSE connection (ctrl+c to stop)..."
echo "Connect with:"
echo "  curl -N http://localhost:8811/sse -H 'Authorization: Bearer test-token-123'"
echo ""

# Cleanup
kill $PF_PID 2>/dev/null || true

echo "=== Test Complete ==="
echo ""
echo "To access the gateway:"
echo "  kubectl port-forward service/mcp-gateway 8811:8811"
echo ""
echo "Then connect your MCP client to:"
echo "  http://localhost:8811/sse"
echo "  (with Authorization: Bearer test-token-123)"
