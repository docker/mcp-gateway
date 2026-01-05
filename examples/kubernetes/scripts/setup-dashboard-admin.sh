#!/bin/bash
# Set up admin service account for Kubernetes Dashboard

set -e

echo "=========================================="
echo "Setting Up Dashboard Admin Account"
echo "=========================================="
echo ""

# Check if dashboard namespace exists
if ! kubectl get namespace kubernetes-dashboard &> /dev/null; then
    echo "Error: kubernetes-dashboard namespace not found"
    echo "Please run ./install-dashboard.sh first"
    exit 1
fi

echo "Creating service account: dashboard-admin..."
kubectl create serviceaccount dashboard-admin -n kubernetes-dashboard 2>/dev/null || {
    echo "⚠️  ServiceAccount 'dashboard-admin' already exists"
}

echo "Creating cluster role binding..."
kubectl create clusterrolebinding dashboard-admin \
    --clusterrole=cluster-admin \
    --serviceaccount=kubernetes-dashboard:dashboard-admin 2>/dev/null || {
    echo "⚠️  ClusterRoleBinding 'dashboard-admin' already exists"
}

echo ""
echo "=========================================="
echo "✓ Admin Account Configured!"
echo "=========================================="
echo ""

echo "Service Account Details:"
kubectl get serviceaccount dashboard-admin -n kubernetes-dashboard

echo ""
echo "Cluster Role Binding:"
kubectl get clusterrolebinding dashboard-admin

echo ""
echo "⚠️  Security Note:"
echo "This account has cluster-admin privileges (full access)."
echo "For production, consider using more restricted roles."
echo ""
echo "Next step:"
echo "  Run ./get-dashboard-token.sh to get your login token"
echo ""
