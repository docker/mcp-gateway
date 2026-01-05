#!/bin/bash
# Get authentication token for Kubernetes Dashboard

set -e

# Check if service account exists
if ! kubectl get serviceaccount dashboard-admin -n kubernetes-dashboard &> /dev/null; then
    echo "Error: dashboard-admin service account not found"
    echo "Please run ./setup-dashboard-admin.sh first"
    exit 1
fi

# Check for optional duration parameter
DURATION="${1:-1h}"

echo "=========================================="
echo "Kubernetes Dashboard Token"
echo "=========================================="
echo ""
echo "Generating token (valid for: $DURATION)..."
echo ""

TOKEN=$(kubectl create token dashboard-admin -n kubernetes-dashboard --duration="$DURATION")

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "$TOKEN"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Copy to clipboard if available
if command -v pbcopy &> /dev/null; then
    echo "$TOKEN" | pbcopy
    echo "✓ Token copied to clipboard (macOS)"
elif command -v xclip &> /dev/null; then
    echo "$TOKEN" | xclip -selection clipboard
    echo "✓ Token copied to clipboard (Linux)"
elif command -v clip.exe &> /dev/null; then
    echo "$TOKEN" | clip.exe
    echo "✓ Token copied to clipboard (Windows/WSL)"
fi

echo ""
echo "To access the dashboard:"
echo "1. Start port-forward (if not already running):"
echo "   kubectl port-forward -n kubernetes-dashboard service/kubernetes-dashboard 8443:443"
echo ""
echo "2. Open in browser:"
echo "   https://localhost:8443"
echo ""
echo "3. Select 'Token' authentication method"
echo ""
echo "4. Paste the token above"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Token expires in: $DURATION"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To generate a new token with different duration:"
echo "  ./get-dashboard-token.sh 24h    # 24 hours"
echo "  ./get-dashboard-token.sh 30m    # 30 minutes"
echo "  ./get-dashboard-token.sh 7d     # 7 days"
echo ""
