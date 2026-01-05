# Accessing the MCP Gateway from Outside Kubernetes

## Quick Access

The gateway is accessible via port-forward:

```bash
kubectl port-forward service/mcp-gateway 8811:8811
```

Then connect your MCP client to: `http://localhost:8811/mcp`

## Endpoints

### 1. Health Check
```bash
curl http://localhost:8811/health
# Returns empty 200 OK when healthy
```

### 2. MCP Endpoint (Streaming Transport)
```bash
http://localhost:8811/mcp
```

This is the main MCP protocol endpoint using streaming transport. MCP clients should connect here.

**Note:** GET requests will return "GET requires an active session" - this is expected. MCP clients use POST with proper protocol messages.

## Authentication

Authentication is currently **disabled** because the gateway runs with `DOCKER_MCP_IN_CONTAINER=1`. This is the default behavior for containerized deployments.

To enable authentication (optional):
1. Remove or set `DOCKER_MCP_IN_CONTAINER=0` in the deployment
2. The gateway will use the `MCP_GATEWAY_AUTH_TOKEN` environment variable
3. Clients must send: `Authorization: Bearer <token>` header

## Example: Connecting with Claude Desktop

If you want to connect Claude Desktop to this gateway:

1. Start port-forward:
```bash
kubectl port-forward service/mcp-gateway 8811:8811
```

2. Configure Claude Desktop to use SSE transport pointing to:
```
http://localhost:8811/mcp
```

## Example: Testing with curl

MCP uses a specific JSON-RPC 2.0 protocol. Here's a basic test:

```bash
# Initialize a session
curl -X POST http://localhost:8811/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    }
  }'
```

## Exposing Externally

For production use, instead of port-forward:

### Option 1: LoadBalancer Service
```yaml
# In services-multi-pod.yaml, change mcp-gateway service:
spec:
  type: LoadBalancer  # Change from ClusterIP
```

### Option 2: Ingress
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mcp-gateway
spec:
  rules:
  - host: mcp.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: mcp-gateway
            port:
              number: 8811
```

### Option 3: NodePort
```yaml
# In services-multi-pod.yaml:
spec:
  type: NodePort
  ports:
  - port: 8811
    targetPort: 8811
    nodePort: 30811  # Choose a port in range 30000-32767
```

## Troubleshooting

### Can't connect to localhost:8811
- Ensure port-forward is running
- Check if another process is using port 8811: `lsof -i :8811`
- Try a different local port: `kubectl port-forward service/mcp-gateway 8812:8811`

### "GET requires an active session"
- This is expected for GET requests
- MCP clients use POST with proper protocol initialization
- Use the curl example above for testing

### Connection refused
- Check pod is running: `kubectl get pods -l app=mcp-gateway`
- Check logs: `kubectl logs -l app=mcp-gateway`
- Verify service exists: `kubectl get svc mcp-gateway`

## Available Tools

Once connected, the gateway provides access to these tools:
- **duckduckgo** (2 tools): Web search capabilities

To see all available tools, use the MCP `tools/list` method after initializing a session.
