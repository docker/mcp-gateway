# Current Provisioning Behavior Documentation

## Purpose
This document serves as a regression prevention guide by documenting the exact current behavior of the Docker MCP Gateway provisioning system before implementing pluggable provisioning strategies.

**CRITICAL**: This document must be preserved and referenced during implementation to ensure no existing functionality is broken.

## Overview of Current System

The current provisioning system is centered around the **clientGetter.GetClient()** method in `cmd/docker-mcp/internal/gateway/clientpool.go` (lines 352-428). This method handles all provisioning decisions using a single strategy based on server configuration.

## Current Provisioning Decision Tree

The system follows this exact decision flow in `clientGetter.GetClient()`:

```go
func (cg *clientGetter) GetClient(ctx context.Context) (mcpclient.Client, error) {
    cg.once.Do(func() {
        // Decision tree for provisioning strategy:
        
        if cg.serverConfig.Spec.SSEEndpoint != "" {
            // Path 1: Deprecated SSE endpoint (remote HTTP)
            client = mcpclient.NewRemoteMCPClient(cg.serverConfig)
            
        } else if cg.serverConfig.Spec.Remote.URL != "" {
            // Path 2: Remote HTTP endpoint
            client = mcpclient.NewRemoteMCPClient(cg.serverConfig)
            
        } else if cg.cp.Static {
            // Path 3: Static deployment mode (socat tunnel)
            client = mcpclient.NewStdioCmdClient(cg.serverConfig.Name, "socat", nil, "STDIO", fmt.Sprintf("TCP:mcp-%s:4444", cg.serverConfig.Name))
            
        } else {
            // Path 4: Containerized stdio (Docker)
            // This is the most complex path with proxies, args, env, etc.
        }
    })
}
```

## Transport Scenarios Currently Supported

### 1. Remote HTTP Endpoints
**Current Behavior**: Direct connection to existing HTTP MCP servers
- **Trigger**: `serverConfig.Spec.Remote.URL != ""`
- **Implementation**: `mcpclient.NewRemoteMCPClient(serverConfig)`
- **Notes**: Also triggered by deprecated `SSEEndpoint` field

### 2. Static Deployment Mode  
**Current Behavior**: Connect to pre-deployed containers via socat tunnel
- **Trigger**: `clientPool.Static == true`
- **Implementation**: `mcpclient.NewStdioCmdClient` with socat command
- **Connection**: TCP connection to `mcp-{serverName}:4444`

### 3. Containerized Stdio
**Current Behavior**: Deploy Docker container and connect via stdio
- **Trigger**: Default case when image is specified
- **Implementation**: `mcpclient.NewStdioCmdClient` with `docker run` command
- **Features**: 
  - Network proxies for `allowHosts`
  - Secret injection via environment variables
  - Volume mounts with read-only override support
  - Resource limits (CPU, memory)
  - Network isolation options

## Current Client Lifecycle Management

### Client Caching (Long-Lived Clients)
**Current Behavior** in `clientPool.AcquireClient()`:

```go
// Long-lived determination (line 69)
keep := config != nil && config.serverSession != nil && (serverConfig.Spec.LongLived || cp.LongLived)

// Cache key structure (line 82)
key := clientKey{serverName: serverConfig.Name, session: session}

// Caching logic (lines 94-104)
if cp.longLived(serverConfig, config) {
    c = context.Background() // Use background context for long-lived
    cp.clientLock.Lock()
    cp.keptClients[key] = keptClient{...}
    cp.clientLock.Unlock()
}
```

### Client Release
**Current Behavior** in `clientPool.ReleaseClient()`:
- Checks if client is in `keptClients` cache
- If kept: does nothing (client stays alive)
- If not kept: calls `client.Session().Close()` immediately

### Cleanup on Pool Close
**Current Behavior** in `clientPool.Close()`:
- Copies `keptClients` map and replaces with empty map
- Iterates through all kept clients and closes their sessions

## Docker Argument Generation

### Base Arguments (clientPool.baseArgs)
**Current Behavior** generates these Docker run arguments:

```go
args := []string{"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges"}

// Resource limits (conditional)
if cp.Cpus > 0 {
    args = append(args, "--cpus", fmt.Sprintf("%d", cp.Cpus))
}
if cp.Memory != "" {
    args = append(args, "--memory", cp.Memory)
}

// Image pulling
args = append(args, "--pull", "never")

// Docker-in-Docker support
if os.Getenv("DOCKER_MCP_IN_DIND") == "1" {
    args = append(args, "--privileged")
}

// Labels for identification
args = append(args,
    "-l", "docker-mcp=true",
    "-l", "docker-mcp-tool-type=mcp", 
    "-l", "docker-mcp-name="+name,
    "-l", "docker-mcp-transport=stdio",
)
```

### Server-Specific Arguments (clientPool.argsAndEnv)
**Current Behavior** adds server-specific arguments:

1. **Network Configuration**:
   - If `DisableNetwork`: `--network none`
   - Otherwise: `--network {each network in cp.networks}`
   - Proxy networks: `--network {targetConfig.NetworkName}`
   - Container links: `--link {each link}`

2. **Environment Variables**:
   - Secrets: `-e {secret.Env}` with value from `serverConfig.Secrets[secret.Name]`
   - Regular env: `-e {env.Name}` with template evaluation
   - Proxy env: `-e {each env from targetConfig}`

3. **Volume Mounts**:
   - Template evaluation: `eval.EvaluateList(serverConfig.Spec.Volumes, serverConfig.Config)`
   - Read-only override: appends `:ro` if not present and `readOnly == true`

4. **DNS and Proxy Support**:
   - Custom DNS: `--dns {targetConfig.DNS}`

## Network Proxy Integration

**Current Behavior** when `BlockNetwork && len(AllowHosts) > 0`:

```go
if cg.cp.BlockNetwork && len(cg.serverConfig.Spec.AllowHosts) > 0 {
    targetConfig, cleanup, err = cg.cp.runProxies(ctx, cg.serverConfig.Spec.AllowHosts, cg.serverConfig.Spec.LongLived)
    // Creates proxy containers for allowed hosts
    // Returns targetConfig with proxy network settings
}
```

## Template Evaluation

**Current Behavior** uses `eval` package for:
- Command expansion: `eval.EvaluateList(serverConfig.Spec.Command, serverConfig.Config)`
- Volume evaluation: `eval.EvaluateList(serverConfig.Spec.Volumes, serverConfig.Config)`
- Environment variable values with `{{...}}` syntax

## Error Handling Patterns

### Client Creation Errors
**Current Behavior**:
- Errors in `GetClient()` are cached in `clientGetter.err`
- Failed long-lived clients are removed from `keptClients` map
- `sync.Once` ensures only one creation attempt per `clientGetter`

### Timeout Handling
**Current Behavior**:
- Client initialization timeout: 20 seconds (currently commented out)
- Context propagation: Uses provided context for creation

## MCP Protocol Integration

### Initialization Parameters
**Current Behavior**:
```go
initParams := &mcp.InitializeParams{
    ProtocolVersion: "2024-11-05",
    ClientInfo: &mcp.Implementation{
        Name:    "docker",
        Version: "1.0.0",
    },
}
```

### Session Management
**Current Behavior**:
- Supports server session reuse for long-lived clients
- Passes `serverSession` and `server` objects to `client.Initialize()`
- Verbose logging support during initialization

## Critical Regression Prevention Points

### 1. Exact Command Generation
The current Docker command generation must remain **byte-for-byte identical** for existing configurations:
- Argument order preservation
- Environment variable injection patterns
- Label format and content
- Volume mount syntax including read-only handling

### 2. Client Caching Behavior
The long-lived client caching must maintain **exact same logic**:
- Cache key generation using `{serverName, session}`
- Context handling (background context for long-lived)
- Thread-safety with `clientLock`

### 3. Error Propagation
Error handling patterns must be **preserved exactly**:
- `sync.Once` error caching pattern
- Long-lived client removal on errors
- Context timeout behavior

### 4. Network Proxy Integration
The proxy system integration must remain **unchanged**:
- `runProxies()` call conditions and parameters
- `targetConfig` application to Docker arguments
- Cleanup function handling

### 5. Template Evaluation
Template evaluation behavior must be **identical**:
- Evaluation timing (during arg generation)
- Context passing to `eval` functions
- Environment variable expansion patterns

## Testing Strategy for Regression Prevention

### Required Baseline Tests
1. **Argument Generation Tests**: Validate exact Docker command generation
2. **Client Caching Tests**: Verify long-lived client behavior
3. **Network Configuration Tests**: Test all network scenarios
4. **Template Evaluation Tests**: Validate template expansion
5. **Error Handling Tests**: Verify error propagation patterns

### Integration Test Scenarios
1. **Remote HTTP**: Connect to existing HTTP endpoint
2. **Static Mode**: socat tunnel connection
3. **Containerized Stdio**: Full Docker container deployment
4. **Network Proxies**: Proxy container creation and configuration
5. **Long-Lived Clients**: Session reuse and cleanup

### Performance Baselines
- Client acquisition time for each transport type
- Memory usage for long-lived client caching
- Docker command execution overhead

This documentation ensures that the pluggable provisioning implementation maintains 100% backward compatibility with existing behavior.