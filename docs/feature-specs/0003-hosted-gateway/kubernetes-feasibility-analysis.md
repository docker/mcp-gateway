# Docker MCP Gateway: Kubernetes Deployment Feasibility Analysis

**Date:** 2025-12-12
**Analysis:** Kubernetes deployment feasibility for Docker MCP Gateway

## Executive Summary

The Docker MCP Gateway **can** run inside Kubernetes, but with important caveats. The primary challenge is that the gateway's default operational mode relies on dynamic container provisioning via the Docker API, which is incompatible with standard Kubernetes deployments. However, the gateway includes a **static mode** specifically designed for containerized environments, which is well-suited for Kubernetes.

**Recommendation:** Use static mode (`--static=true`) for Kubernetes deployments, where MCP server containers are managed by Kubernetes rather than dynamically provisioned by the gateway.

---

## Supported Deployment Models

Based on the examples directory, the Docker MCP Gateway supports the following deployment models:

### 1. Local Process (Default)
- **Location:** Host machine
- **Configuration:** Mounts `~/.docker/mcp/` for config
- **Container Management:** Uses Docker API to dynamically create/destroy MCP server containers
- **Kubernetes Compatible:** ❌ No

### 2. Container with Mounted Config
- **Example:** `examples/container/`
- **Configuration:** Mounts host config directory into container
- **Container Management:** Uses Docker socket (`--use-api-socket`) to manage sibling containers
- **Kubernetes Compatible:** ⚠️ Requires Docker socket access (security risk)

### 3. Docker-in-Docker (DinD)
- **Example:** `examples/docker-in-docker/`
- **Configuration:** Gateway and MCP servers run in same privileged container
- **Container Management:** Full Docker daemon inside container
- **Kubernetes Compatible:** ⚠️ Requires privileged containers (significant security concerns)
- **File:** `docker-mcp-gateway/examples/docker-in-docker/compose.yaml:4`

```yaml
privileged: true  # Required for DinD
```

### 4. Static Mode (Pre-Started Servers)
- **Example:** `examples/compose-static/`
- **Configuration:** All MCP servers started in advance as separate containers
- **Container Management:** No dynamic provisioning - connects to running containers via network
- **Kubernetes Compatible:** ✅ Yes - ideal for Kubernetes
- **File:** `docker-mcp-gateway/examples/compose-static/README.md:6`

---

## Dynamic Provisioning: The Core Issue

### How Dynamic Provisioning Works

The gateway's default mode dynamically creates MCP server containers on-demand using the Docker API:

**Code Reference:** `docker-mcp-gateway/pkg/gateway/clientpool.go:460`

```go
client = mcpclient.NewStdioCmdClient(
    cg.serverConfig.Name,
    "docker",  // Executes docker CLI
    env,
    runArgs... // docker run arguments
)
```

This directly executes `docker run` commands to start containers with configurations like:

**Code Reference:** `docker-mcp-gateway/pkg/gateway/clientpool.go:268-291`

```go
func (cp *clientPool) baseArgs(name string) []string {
    args := []string{"run"}
    args = append(args, "--rm", "-i", "--init", "--security-opt", "no-new-privileges")
    // ... cpus, memory limits
    args = append(args, "--pull", "never")
    // ... labels for container identification
    return args
}
```

### Why This Doesn't Work in Kubernetes

1. **No Docker API Access:** Kubernetes pods don't have access to Docker/container runtime APIs by default
2. **Different Abstractions:** Kubernetes uses Pods, not raw containers
3. **Security Boundaries:** Container creation requires elevated privileges
4. **Networking:** Docker bridge networks don't map to Kubernetes networking

### Workarounds (Not Recommended)

#### Option A: Mount Docker Socket
```yaml
volumeMounts:
  - name: docker-sock
    mountPath: /var/run/docker.sock
volumes:
  - name: docker-sock
    hostPath:
      path: /var/run/docker.sock
```

**Problems:**
- Major security vulnerability (pod can control entire node)
- Breaks Kubernetes isolation model
- Containers created outside Kubernetes management
- Resource limits not enforced by Kubernetes

#### Option B: Privileged DinD Container
```yaml
securityContext:
  privileged: true
```

**Problems:**
- Requires privileged containers (most clusters prohibit this)
- Security risk - full host kernel access
- Resource inefficient (nested container runtimes)
- Complex networking setup

---

## Static Mode: The Kubernetes Solution

### How Static Mode Works

**Code Reference:** `docker-mcp-gateway/pkg/gateway/clientpool.go:430-431`

```go
} else if cg.cp.Static {
    client = mcpclient.NewStdioCmdClient(
        cg.serverConfig.Name,
        "socat",  // Network bridge tool
        nil,
        "STDIO",
        fmt.Sprintf("TCP:mcp-%s:4444", cg.serverConfig.Name)
    )
```

In static mode:
1. **No container creation** - Gateway expects containers already running
2. **Network communication** - Connects via TCP using `socat`
3. **Service discovery** - Uses DNS names like `mcp-duckduckgo:4444`
4. **Pre-provisioned** - All servers started before gateway

### Static Mode Configuration

**Flag:** `docker-mcp-gateway/cmd/docker-mcp/commands/gateway.go:215`

```bash
--static=true  # Enable static mode (aka pre-started servers)
```

**Behavior:** `docker-mcp-gateway/cmd/docker-mcp/commands/gateway.go:110-112`

```go
if options.Static {
    options.Watch = false  // Disable config watching
}
```

### Example Static Deployment

**File:** `docker-mcp-gateway/examples/compose-static/compose.yaml`

```yaml
services:
  gateway:
    image: docker/mcp-gateway
    ports:
      - "8811:8811"
    command:
      - --transport=streaming
      - --servers=duckduckgo,fetch
      - --port=8811
      - --static=true  # Key flag
    depends_on:
      - mcp-duckduckgo
      - mcp-fetch

  mcp-duckduckgo:
    image: mcp/duckduckgo@sha256:...
    entrypoint: ["/docker-mcp/misc/docker-mcp-bridge", "python", "-m", "duckduckgo_mcp_server.server"]
    labels:
      - docker-mcp=true
      - docker-mcp-name=duckduckgo
      - docker-mcp-transport=stdio
    # ... security options, resource limits

  mcp-fetch:
    image: mcp/fetch@sha256:...
    # ... similar configuration
```

---

## Kubernetes Deployment Architecture

### Recommended Approach

```
┌─────────────────────────────────────────────────┐
│                  Kubernetes Pod                  │
│                                                  │
│  ┌────────────────┐        ┌─────────────────┐ │
│  │  MCP Gateway   │◄──TCP──│  MCP Server     │ │
│  │  Container     │        │  (duckduckgo)   │ │
│  │                │   │    │                 │ │
│  │  (socat proxy) │   │    │  Bridge: stdio  │ │
│  └────────────────┘   │    │  ↔ TCP:4444     │ │
│         │              │    └─────────────────┘ │
│         │              │                         │
│         │              │    ┌─────────────────┐ │
│         │              └───►│  MCP Server     │ │
│         │                   │  (fetch)        │ │
│         │                   │                 │ │
│         │                   │  Bridge: stdio  │ │
│         ▼                   │  ↔ TCP:4444     │ │
│    Kubernetes               └─────────────────┘ │
│    Service                                       │
└─────────────────────────────────────────────────┘
```

### Key Components

1. **MCP Gateway Container:**
   - Runs with `--static=true`
   - Uses `socat` to connect to MCP servers via TCP
   - No Docker API access needed

2. **MCP Server Containers:**
   - Each server runs as a sidecar or separate pod
   - Uses `docker-mcp-bridge` to expose stdio over TCP port 4444
   - Named with `mcp-<servername>` for DNS resolution

3. **Communication:**
   - Gateway → Server: TCP connection via Kubernetes DNS
   - Format: `mcp-{server-name}:4444`
   - No Docker socket or privileged access required

### Docker MCP Bridge

**File:** `docker-mcp-gateway/Dockerfile:128-133`

The bridge tool enables static mode by converting stdio MCP protocols to TCP:

```dockerfile
FROM alpine:3.22 AS mcp-gateway
RUN apk add --no-cache docker-cli socat jq
COPY --from=build-mcp-bridge /docker-mcp-bridge /misc/
```

Each MCP server uses this bridge in its entrypoint:

```yaml
entrypoint: ["/docker-mcp/misc/docker-mcp-bridge", "python", "-m", "duckduckgo_mcp_server.server"]
```

The bridge:
- Listens on TCP port 4444
- Forwards stdio ↔ TCP for MCP protocol communication
- Enables network-based MCP server access

---

## Limitations in Kubernetes

### 1. No Dynamic Server Provisioning

**Impact:** Cannot use dynamic management tools

The gateway includes tools for runtime server management:

**File:** `docker-mcp-gateway/docs/feature-specs/dynamic_servers.md:19-148`

- `mcp-find`: Search for servers in catalog
- `mcp-add`: Dynamically add servers at runtime
- `mcp-remove`: Remove servers at runtime
- `mcp-config-set`: Configure servers dynamically

**In Kubernetes Static Mode:**
- These tools won't work without Docker API access
- Server list must be pre-configured
- Changes require pod restart or Kubernetes deployment update
- Feature automatically disabled: `docker-mcp-gateway/cmd/docker-mcp/commands/gateway.go:172`

```go
if len(options.ServerNames) > 0 && !enableAllServers {
    if options.DynamicTools {
        options.DynamicTools = false
        // "dynamic-tools disabled when using --servers flag"
    }
}
```

### 2. Static Container Set

- All MCP servers must be defined in Kubernetes manifests
- No on-demand container creation
- Resource allocation happens at deployment time
- Unused servers still consume resources

### 3. Configuration Updates

**No File Watching:** `docker-mcp-gateway/cmd/docker-mcp/commands/gateway.go:111`

```go
if options.Static {
    options.Watch = false
}
```

- Config changes require gateway restart
- No dynamic reloading of server configurations
- Requires Kubernetes ConfigMap/Secret updates + pod restart

### 4. Volume and Network Constraints

MCP servers may need:
- File system access (would require Persistent Volumes)
- Network policies (Kubernetes NetworkPolicies instead of Docker networks)
- Secrets (Kubernetes Secrets instead of Docker Desktop secrets)

---

## Kubernetes Deployment Example

### Deployment Manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mcp-gateway
  template:
    metadata:
      labels:
        app: mcp-gateway
    spec:
      # No privileged or Docker socket mounting
      containers:
      - name: gateway
        image: docker/mcp-gateway:latest
        args:
        - --transport=streaming
        - --port=8811
        - --static=true
        - --servers=duckduckgo,fetch
        ports:
        - containerPort: 8811
          name: mcp
        resources:
          limits:
            cpu: "1"
            memory: 2Gi

      # Sidecar: duckduckgo MCP server
      - name: mcp-duckduckgo
        image: mcp/duckduckgo@sha256:68eb20db6109f5c312a695fc5ec3386ad15d93ffb765a0b4eb1baf4328dec14f
        command: ["/docker-mcp/misc/docker-mcp-bridge"]
        args: ["python", "-m", "duckduckgo_mcp_server.server"]
        resources:
          limits:
            cpu: "1"
            memory: 2Gi
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          capabilities:
            drop: ["ALL"]
        volumeMounts:
        - name: docker-mcp-tools
          mountPath: /docker-mcp

      # Sidecar: fetch MCP server
      - name: mcp-fetch
        image: mcp/fetch@sha256:ef9535a3f07249142f9ca5a6033d7024950afdb6dc05e98292794a23e9f5dfbe
        command: ["/docker-mcp/misc/docker-mcp-bridge"]
        args: ["mcp-server-fetch"]
        resources:
          limits:
            cpu: "1"
            memory: 2Gi
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          capabilities:
            drop: ["ALL"]
        volumeMounts:
        - name: docker-mcp-tools
          mountPath: /docker-mcp

      volumes:
      - name: docker-mcp-tools
        emptyDir: {}

      # Init container to copy bridge tool
      initContainers:
      - name: copy-bridge
        image: docker/mcp-gateway:latest
        command: ["sh", "-c", "cp -r /misc/* /docker-mcp/misc/"]
        volumeMounts:
        - name: docker-mcp-tools
          mountPath: /docker-mcp

---
apiVersion: v1
kind: Service
metadata:
  name: mcp-gateway
spec:
  selector:
    app: mcp-gateway
  ports:
  - port: 8811
    targetPort: 8811
    name: mcp
  type: LoadBalancer  # or ClusterIP, NodePort depending on access needs
```

### Multi-Pod Alternative

For better resource isolation, deploy each MCP server as a separate pod:

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: mcp-duckduckgo
spec:
  selector:
    app: mcp-server
    server: duckduckgo
  ports:
  - port: 4444
    targetPort: 4444
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-server-duckduckgo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mcp-server
      server: duckduckgo
  template:
    metadata:
      labels:
        app: mcp-server
        server: duckduckgo
    spec:
      containers:
      - name: server
        image: mcp/duckduckgo@sha256:...
        # ... similar configuration
```

---

## Security Considerations

### Advantages in Kubernetes Static Mode

1. **No Privileged Containers:** Standard security context
2. **No Docker Socket:** No host access required
3. **Network Policies:** Kubernetes-native network isolation
4. **Resource Limits:** Enforced by Kubernetes scheduler
5. **RBAC:** Standard Kubernetes access control

### Security Context Example

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

---

## Performance Considerations

### Static Mode Overhead

- **Network hop:** Extra TCP connection vs direct stdio
- **socat process:** Additional process per connection
- **Always running:** All servers consume resources even when idle

### Mitigation Strategies

1. **Horizontal Pod Autoscaling:** Scale based on CPU/memory
2. **Resource Requests/Limits:** Proper sizing prevents overcommitment
3. **Pod Disruption Budgets:** Ensure availability during updates
4. **Liveness/Readiness Probes:** Detect and restart unhealthy containers

---

## Code References Summary

| Component | File | Line | Purpose |
|-----------|------|------|---------|
| Static mode flag | `cmd/docker-mcp/commands/gateway.go` | 215 | CLI flag definition |
| Static mode logic | `pkg/gateway/clientpool.go` | 430-431 | socat TCP connection |
| Dynamic mode logic | `pkg/gateway/clientpool.go` | 460 | Docker run execution |
| Base container args | `pkg/gateway/clientpool.go` | 268-293 | Container creation params |
| Docker client | `pkg/docker/client.go` | 20-38 | Docker API interface |
| Container operations | `pkg/docker/containers.go` | 30-41 | ContainerCreate/Start |
| Static example | `examples/compose-static/compose.yaml` | 10 | Static flag usage |
| DinD example | `examples/docker-in-docker/compose.yaml` | 4 | Privileged requirement |

---

## Conclusions

### ✅ Feasibility: YES (with Static Mode)

The Docker MCP Gateway **can** run in Kubernetes using static mode (`--static=true`), where:
- MCP servers are pre-deployed as Kubernetes containers/pods
- Gateway connects via TCP using socat
- No Docker API or privileged access required
- Standard Kubernetes security practices apply

### ⚠️ Limitations

1. **No dynamic provisioning:** Cannot add/remove MCP servers at runtime
2. **Static configuration:** Changes require pod restarts
3. **Resource overhead:** All servers run continuously
4. **Network latency:** Additional TCP hop vs direct stdio

### ❌ Not Feasible: Dynamic Mode in Standard Kubernetes

The default dynamic provisioning mode is **not compatible** with standard Kubernetes deployments without:
- Mounting Docker socket (security risk, breaks isolation)
- Running privileged DinD containers (security risk, resource inefficient)
- Custom admission controllers / CRI integrations (complex, non-standard)

### Recommendation

**For Production Kubernetes deployments:**
- Use `--static=true` mode
- Deploy MCP servers as sidecars or separate pods
- Pre-configure all needed servers in deployment manifests
- Use Kubernetes-native tools for scaling and management
- Accept the trade-off: predictability over dynamic flexibility

**For Development/Testing:**
- Docker Compose with static mode matches Kubernetes behavior
- Test configurations before Kubernetes deployment
- Use the compose-static example as a template

---

## Future Enhancements

To improve Kubernetes support, consider:

1. **Kubernetes-native provisioning:**
   - Use Kubernetes Jobs/Pods instead of Docker containers
   - Implement a Kubernetes operator for dynamic server management
   - Use CRDs for MCP server definitions

2. **Service mesh integration:**
   - Istio/Linkerd for mTLS between gateway and servers
   - Distributed tracing for MCP calls
   - Traffic management and retries

3. **Helm chart:**
   - Parameterized deployment templates
   - Easy server configuration
   - Best practices codified

4. **Autoscaling:**
   - HPA based on MCP request metrics
   - VPA for right-sizing resources
   - KEDA for event-driven scaling

---

## References

- Docker MCP Gateway Repository: https://github.com/docker/mcp-gateway
- Examples: `docker-mcp-gateway/examples/`
- Static Mode Example: `docker-mcp-gateway/examples/compose-static/`
- Dynamic Servers Documentation: `docker-mcp-gateway/docs/feature-specs/dynamic_servers.md`
- Compose Static README: `docker-mcp-gateway/examples/compose-static/README.md:1-15`
