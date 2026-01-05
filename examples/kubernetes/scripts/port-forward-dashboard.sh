#!/bin/bash
# Port-forward Kubernetes Dashboard to localhost

set -e

# Default port
PORT="${1:-8443}"

echo "=========================================="
echo "Kubernetes Dashboard Port Forward"
echo "=========================================="
echo ""

# Check if dashboard is installed
if ! kubectl get namespace kubernetes-dashboard &> /dev/null; then
    echo "Error: kubernetes-dashboard namespace not found"
    echo "Please run ./install-dashboard.sh first"
    exit 1
fi

# Check if dashboard pods are ready
echo "Checking dashboard status..."
if ! kubectl get pods -n kubernetes-dashboard -l k8s-app=kubernetes-dashboard &> /dev/null; then
    echo "Error: Dashboard pods not found"
    echo "Please run ./install-dashboard.sh first"
    exit 1
fi

# Wait for dashboard to be ready
echo "Waiting for dashboard to be ready..."
kubectl wait --for=condition=ready pod -l k8s-app=kubernetes-dashboard -n kubernetes-dashboard --timeout=30s

# Check if port is already in use
if lsof -ti:$PORT &> /dev/null; then
    echo "⚠️  Port $PORT is already in use"
    echo ""
    echo "Killing existing process on port $PORT..."
    lsof -ti:$PORT | xargs kill -9 2>/dev/null || true
    sleep 2
fi

echo ""
echo "=========================================="
echo "✓ Starting Port Forward"
echo "=========================================="
echo ""
echo "Dashboard will be available at:"
echo "  https://localhost:$PORT"
echo ""
echo "⚠️  Use HTTPS, not HTTP!"
echo "⚠️  Accept the self-signed certificate warning in your browser"
echo ""
echo "To get a login token, run (in another terminal):"
echo "  ./get-dashboard-token.sh"
echo ""
echo "Press Ctrl+C to stop port forwarding"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Start port forward
kubectl port-forward -n kubernetes-dashboard service/kubernetes-dashboard $PORT:443
