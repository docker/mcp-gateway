#!/bin/bash
# Deploy MCP Gateway to Kubernetes
# This script sets up the complete MCP Gateway with MCP servers in a Kubernetes cluster

set -e

echo "=========================================="
echo "MCP Gateway Kubernetes Deployment"
echo "=========================================="
echo ""

# Check prerequisites
echo "Checking prerequisites..."
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed"
    exit 1
fi

if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster"
    echo "Please ensure your kubeconfig is configured correctly"
    exit 1
fi

echo "✓ kubectl found"
echo "✓ Connected to cluster: $(kubectl config current-context)"
echo ""

# Deploy the gateway and servers
echo "Deploying MCP Gateway and servers..."
kubectl apply -f deployment-multi-pod.yaml

echo ""
echo "Deploying services..."
kubectl apply -f services-multi-pod.yaml

echo ""
echo "Waiting for pods to be ready (this may take 30-60 seconds)..."
kubectl wait --for=condition=ready pod -l app=mcp-gateway --timeout=60s
kubectl wait --for=condition=ready pod -l app=mcp-server,server=duckduckgo --timeout=60s

echo ""
echo "=========================================="
echo "✓ Deployment Complete!"
echo "=========================================="
echo ""

# Show status
echo "Pod Status:"
kubectl get pods -l 'app in (mcp-gateway,mcp-server)'
echo ""

echo "Service Status:"
kubectl get svc mcp-gateway mcp-duckduckgo
echo ""

# Show logs
echo "Gateway Logs (last 10 lines):"
kubectl logs -l app=mcp-gateway --tail=10
echo ""

echo "=========================================="
echo "How to Access the Gateway:"
echo "=========================================="
echo ""
echo "1. Start port forwarding (in a separate terminal):"
echo "   kubectl port-forward service/mcp-gateway 8811:8811"
echo ""
echo "2. Test the connection:"
echo "   curl http://localhost:8811/mcp"
echo "   (You should see: 'GET requires an active session')"
echo ""
echo "3. Connect your MCP client to:"
echo "   http://localhost:8811/mcp"
echo ""
echo "Available Tools:"
echo "  - duckduckgo (2 tools)"
echo ""
echo "For more information, see:"
echo "  - README.md for full documentation"
echo "  - ACCESSING.md for connection details"
echo "  - STATUS.md for testing results"
echo ""
