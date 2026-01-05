# Running MCP Gateway in Kubernetes

This example demonstrates how to run the Docker MCP Gateway in Kubernetes using **static mode**.

## Overview

In static mode (`--static=true`), the gateway connects to pre-started MCP server containers via TCP instead of dynamically provisioning them. This is the recommended approach for Kubernetes deployments.

## Architecture

```
┌────────────────────────────────────────┐
│         Kubernetes Pod                  │
│                                         │
│  ┌──────────────┐    ┌──────────────┐ │
│  │   Gateway    │◄──►│ MCP Server   │ │
│  │  Container   │TCP │ (duckduckgo) │ │
│  │              │4444│              │ │
│  └──────────────┘    └──────────────┘ │
│         │            ┌──────────────┐ │
│         └───────────►│ MCP Server   │ │
│                  TCP │   (fetch)    │ │
│                 4444 │              │ │
│                      └──────────────┘ │
└────────────────────────────────────────┘
```

## Files

- `deployment.yaml` - Main deployment with gateway and MCP servers as sidecars
- `service.yaml` - Service to expose the gateway
- `deployment-multi-pod.yaml` - Alternative: separate pods for each component
- `services-multi-pod.yaml` - Services for multi-pod deployment

## Prerequisites

- Kubernetes cluster (Docker Desktop, minikube, kind, etc.)
- kubectl configured
- Docker images pulled (they'll be pulled automatically if not present)

## Quick Start

Deploy the gateway with MCP servers in separate pods:

```bash
# One-command deploy
./deploy.sh

# Start port forwarding (separate terminal)
./port-forward-mcp.sh

# Test the gateway (another terminal)
curl http://localhost:8811/mcp
# Should return: "GET requires an active session"
```

Connect your MCP client to: `http://localhost:8811/mcp`

## Alternative: Multi-Pod Deployment

For better resource isolation and independent scaling:

```bash
# Deploy servers and gateway as separate pods
kubectl apply -f deployment-multi-pod.yaml
kubectl apply -f services-multi-pod.yaml

# Check deployments
kubectl get deployments
kubectl get pods
kubectl get services
```

## Configuration

### Environment Variables

The gateway uses the `MCP_GATEWAY_AUTH_TOKEN` environment variable for authentication in SSE/streaming mode. This is set in the deployment manifests.

### Resource Limits

Each container has resource limits defined:
- Gateway: 1 CPU, 2Gi memory
- MCP Servers: 1 CPU, 2Gi memory each

Adjust these in the deployment files as needed.

### Adding More MCP Servers

To add additional MCP servers:

1. Add a new sidecar container in `deployment.yaml`:
```yaml
- name: mcp-<server-name>
  image: mcp/<server-name>@sha256:...
  command: ["/docker-mcp/misc/docker-mcp-bridge"]
  args: ["<server-command>"]
  # ... rest of configuration
```

2. Update the gateway args to include the new server:
```yaml
args:
  - --static=true
  - --servers=duckduckgo,fetch,<server-name>
```

3. Ensure the initContainer copies the bridge tool to all servers.

## Accessing the Gateway

### From Inside the Cluster

Other pods can access the gateway at:
```
http://mcp-gateway:8811/sse
```

### From Outside the Cluster

Use kubectl port-forward:
```bash
kubectl port-forward service/mcp-gateway 8811:8811
```

Or change the service type to `LoadBalancer` or `NodePort` in `service.yaml`.

**See [docs/accessing.md](docs/accessing.md) for detailed access instructions, authentication configuration, and production deployment options.**

## Security Considerations

- All containers run with `allowPrivilegeEscalation: false`
- Containers run as non-root (UID 1000)
- All capabilities dropped
- Read-only root filesystem where possible
- No Docker socket mounting required
- No privileged containers

## Troubleshooting

### Check pod status
```bash
kubectl get pods
kubectl describe pod <pod-name>
```

### View logs
```bash
# Gateway logs
kubectl logs deployment/mcp-gateway -c gateway

# Specific MCP server logs
kubectl logs deployment/mcp-gateway -c mcp-duckduckgo

# Init container logs (if bridge copy fails)
kubectl logs deployment/mcp-gateway -c copy-bridge
```

### Test connectivity between containers
```bash
# Exec into gateway container
kubectl exec -it deployment/mcp-gateway -c gateway -- sh

# Test connection to MCP server
nc -zv localhost 4444
```

### Common Issues

**Bridge tool not found:**
- Check init container logs
- Ensure volume mount paths match

**Connection refused:**
- Verify MCP servers are listening on port 4444
- Check that bridge tool is running
- Review server logs for errors

**Authentication errors:**
- Ensure `MCP_GATEWAY_AUTH_TOKEN` is set consistently
- Token must match in client requests

## Cleanup

```bash
# Delete sidecar deployment
kubectl delete -f deployment.yaml
kubectl delete -f service.yaml

# Or delete multi-pod deployment
kubectl delete -f deployment-multi-pod.yaml
kubectl delete -f services-multi-pod.yaml
```

## Limitations

Compared to running the gateway locally with Docker:

- **No dynamic server provisioning** - Cannot add/remove servers at runtime
- **Static configuration** - Changes require pod restart
- **All servers run continuously** - Even if not actively used
- **No file watching** - Config updates need pod restart

These are inherent limitations of running in Kubernetes static mode.

## Next Steps

- Configure persistent storage for stateful MCP servers
- Add HorizontalPodAutoscaler for scaling
- Implement Prometheus monitoring
- Add network policies for additional security
- Create Helm chart for easier deployment

## Additional Documentation

- **[docs/accessing.md](docs/accessing.md)** - Detailed guide for accessing the gateway from outside the cluster, including authentication, external exposure options (LoadBalancer, Ingress, NodePort), and troubleshooting
- **[docs/dashboard.md](docs/dashboard.md)** - Optional guide for installing and using the Kubernetes Dashboard to monitor your cluster
