# Kubernetes Dashboard Setup

Quick scripts to install and access the Kubernetes Dashboard on your cluster.

## Quick Start

```bash
# From the kubernetes/ directory:

# 1. Install the dashboard
./scripts/install-dashboard.sh

# 2. Set up admin account
./scripts/setup-dashboard-admin.sh

# 3. Start port forwarding (in one terminal)
./scripts/port-forward-dashboard.sh

# 4. Get login token (in another terminal)
./scripts/get-dashboard-token.sh
```

Then open: **https://localhost:8443** and paste the token.

**Note:** Scripts can be run from any directory - they automatically resolve paths relative to their location.

## Scripts

### 1. `scripts/install-dashboard.sh`

Installs Kubernetes Dashboard v2.7.0 to your cluster.

```bash
./scripts/install-dashboard.sh
```

**What it does:**
- Checks if dashboard is already installed
- Applies the official dashboard manifest
- Waits for pods to be ready
- Shows installation status

**Safe to re-run:** Prompts before reinstalling if already exists.

### 2. `scripts/setup-dashboard-admin.sh`

Creates a service account with cluster-admin permissions.

```bash
./scripts/setup-dashboard-admin.sh
```

**What it does:**
- Creates `dashboard-admin` ServiceAccount in `kubernetes-dashboard` namespace
- Creates ClusterRoleBinding with `cluster-admin` role
- Verifies the setup

**Permissions:** Full cluster access (cluster-admin role).

**Safe to re-run:** Skips if resources already exist.

### 3. `scripts/get-dashboard-token.sh`

Generates an authentication token for the dashboard.

```bash
# Default: 1 hour token
./scripts/get-dashboard-token.sh

# Custom duration
./scripts/get-dashboard-token.sh 24h   # 24 hours
./scripts/get-dashboard-token.sh 30m   # 30 minutes
./scripts/get-dashboard-token.sh 7d    # 7 days
```

**What it does:**
- Generates a token for the `dashboard-admin` service account
- Copies token to clipboard (if available)
- Shows access instructions
- Displays token expiration time

**Note:** Tokens are temporary and expire. Run this script again to generate a new token.

### 4. `scripts/port-forward-dashboard.sh`

Starts port-forwarding to access the dashboard.

```bash
# Default port (8443)
./scripts/port-forward-dashboard.sh

# Custom port
./scripts/port-forward-dashboard.sh 9443
```

**What it does:**
- Checks dashboard is installed and ready
- Kills any existing process on the port
- Starts port-forward to `https://localhost:8443`
- Shows access instructions
- Keeps running until you press Ctrl+C

**When to use:** Run this anytime you need to access the dashboard (after cluster restart, port-forward died, etc.)

## Accessing the Dashboard

### Method 1: Port Forward (Recommended for Local)

```bash
# Terminal 1: Start port forward
kubectl port-forward -n kubernetes-dashboard service/kubernetes-dashboard 8443:443

# Terminal 2: Get token
./scripts/get-dashboard-token.sh

# Browser: Open https://localhost:8443
```

### Method 2: NodePort (Requires Service Edit)

```bash
kubectl edit service kubernetes-dashboard -n kubernetes-dashboard
# Change type: ClusterIP to type: NodePort

kubectl get svc -n kubernetes-dashboard
# Note the NodePort (e.g., 30443)

# Access at https://localhost:30443
```

### Method 3: LoadBalancer (Cloud Clusters)

```bash
kubectl edit service kubernetes-dashboard -n kubernetes-dashboard
# Change type: ClusterIP to type: LoadBalancer

kubectl get svc -n kubernetes-dashboard
# Wait for EXTERNAL-IP

# Access at https://<external-ip>:443
```

## Security Considerations

### Current Setup (Development)

The `dashboard-admin` account has **full cluster access** (cluster-admin role). This is convenient for development but should **not** be used in production.

### Production Recommendations

1. **Use Read-Only Access:**
```bash
kubectl create clusterrolebinding dashboard-readonly \
  --clusterrole=view \
  --serviceaccount=kubernetes-dashboard:dashboard-admin
```

2. **Namespace-Specific Access:**
```bash
kubectl create rolebinding dashboard-namespace-admin \
  --clusterrole=admin \
  --serviceaccount=kubernetes-dashboard:dashboard-admin \
  --namespace=your-namespace
```

3. **Use Short-Lived Tokens:**
```bash
./scripts/get-dashboard-token.sh 1h   # 1 hour only
```

4. **Consider OIDC/SSO Integration:**
   - Configure dashboard with corporate identity provider
   - See: https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/README.md

## Troubleshooting

### "Connection refused" or "Connection reset"

**Problem:** Dashboard uses HTTPS but accessed via HTTP.

**Solution:** Use `https://` not `http://`
```bash
# Wrong
curl http://localhost:8443

# Correct
curl -k https://localhost:8443
```

### "Forbidden" errors in dashboard

**Problem:** Service account doesn't have sufficient permissions.

**Solution:** Verify ClusterRoleBinding exists:
```bash
kubectl get clusterrolebinding dashboard-admin
```

If missing, run:
```bash
./scripts/setup-dashboard-admin.sh
```

### Token expired

**Problem:** Token is only valid for 1 hour by default.

**Solution:** Generate a new token:
```bash
./scripts/get-dashboard-token.sh

# Or with longer duration
./scripts/get-dashboard-token.sh 24h
```

### Dashboard pod not starting

**Problem:** Resources not available or image pull issues.

**Solution:** Check pod status:
```bash
kubectl get pods -n kubernetes-dashboard
kubectl describe pod -l k8s-app=kubernetes-dashboard -n kubernetes-dashboard
kubectl logs -l k8s-app=kubernetes-dashboard -n kubernetes-dashboard
```

### Port 8443 already in use

**Problem:** Another process is using port 8443.

**Solution:** Use a different local port:
```bash
kubectl port-forward -n kubernetes-dashboard service/kubernetes-dashboard 9443:443
# Then access at https://localhost:9443
```

## Uninstalling

To completely remove the dashboard:

```bash
# Delete the dashboard
kubectl delete -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# Delete the service account and role binding
kubectl delete clusterrolebinding dashboard-admin
kubectl delete serviceaccount dashboard-admin -n kubernetes-dashboard

# Verify removal
kubectl get namespace kubernetes-dashboard
# Should show: Error from server (NotFound)
```

## References

- [Official Dashboard Docs](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/)
- [Dashboard GitHub](https://github.com/kubernetes/dashboard)
- [Access Control](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/README.md)

## Related Scripts

Also available in the `../scripts/` directory:
- `scripts/deploy.sh` - Deploy MCP Gateway
- `scripts/cleanup.sh` - Remove MCP Gateway
- `scripts/test.sh` - Test MCP Gateway
- `scripts/port-forward-mcp.sh` - Port-forward MCP Gateway

See `../README.md` for complete MCP Gateway documentation.
