#!/bin/bash
# Cleanup MCP Gateway from Kubernetes

set -e

echo "=========================================="
echo "MCP Gateway Cleanup"
echo "=========================================="
echo ""

echo "Deleting deployments..."
kubectl delete -f deployment-multi-pod.yaml --ignore-not-found=true

echo ""
echo "Deleting services..."
kubectl delete -f services-multi-pod.yaml --ignore-not-found=true

echo ""
echo "Waiting for pods to terminate..."
kubectl wait --for=delete pod -l 'app in (mcp-gateway,mcp-server)' --timeout=60s 2>/dev/null || true

echo ""
echo "✓ Cleanup complete!"
echo ""

# Verify
REMAINING=$(kubectl get pods -l 'app in (mcp-gateway,mcp-server)' 2>/dev/null | wc -l)
if [ "$REMAINING" -gt 1 ]; then
    echo "Warning: Some pods may still be terminating:"
    kubectl get pods -l 'app in (mcp-gateway,mcp-server)'
else
    echo "All MCP Gateway resources have been removed."
fi
