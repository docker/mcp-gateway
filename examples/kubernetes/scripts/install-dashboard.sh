#!/bin/bash
# Install Kubernetes Dashboard

set -e

echo "=========================================="
echo "Installing Kubernetes Dashboard"
echo "=========================================="
echo ""

# Check if dashboard is already installed
if kubectl get namespace kubernetes-dashboard &> /dev/null; then
    echo "⚠️  Dashboard namespace already exists"
    read -p "Do you want to reinstall? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled"
        exit 0
    fi
    echo "Removing existing dashboard..."
    kubectl delete namespace kubernetes-dashboard
    echo "Waiting for namespace to be removed..."
    sleep 5
fi

echo "Installing Kubernetes Dashboard v2.7.0..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

echo ""
echo "Waiting for dashboard pods to be ready..."
kubectl wait --for=condition=ready pod -l k8s-app=kubernetes-dashboard -n kubernetes-dashboard --timeout=60s

echo ""
echo "=========================================="
echo "✓ Dashboard Installed Successfully!"
echo "=========================================="
echo ""

kubectl get pods -n kubernetes-dashboard

echo ""
echo "Next steps:"
echo "1. Run ./setup-dashboard-admin.sh to create an admin account"
echo "2. Run ./get-dashboard-token.sh to get the login token"
echo "3. Access dashboard at: https://localhost:8443"
echo ""
echo "To start port-forwarding:"
echo "  kubectl port-forward -n kubernetes-dashboard service/kubernetes-dashboard 8443:443"
echo ""
