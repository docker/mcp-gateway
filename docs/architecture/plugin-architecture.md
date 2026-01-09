# Plugin Architecture

This document describes the plugin architecture for MCP Gateway, based on [Design Decision 0000: Plugin Model and Sidecar Strategy](https://github.com/docker/mcp-gateway-enterprise/blob/main/docs/design/0000-plugin-model-and-sidecar-strategy.md).

## Overview

The plugin architecture provides a way to extend MCP Gateway functionality without modifying the core codebase. It supports both Desktop and Kubernetes (BYOC) deployments from a shared core.

**Key Requirements**:
- Support Desktop deployment (maintain existing functionality, no regressions)
- Support Kubernetes deployment (multi-user, multi-tenant, enterprise auth, access control)
- Pluggable architecture (customers can integrate with their infrastructure)
- Open source core (no proprietary dependencies)

## Provider Types

The architecture supports two provider types:

### 1. In-Memory Provider

Runs Go code directly in the gateway process.

**Characteristics**:
- In-process (no network calls, no containers)
- Direct function calls
- Lowest latency
- No isolation from gateway process

**Use Cases**:
- Simple plugins (always-allow policy, stdout logger)
- Performance-critical hot paths (authentication validation)
- Plugins without external dependencies

**Configuration**:
```yaml
plugins:
  auth_provider:
    provider: in-memory
    implementation: desktop-implicit

  audit_sink:
    provider: in-memory
    implementation: stdout
```

### 2. MCP Provider

Uses MCP protocol (JSON-RPC over HTTP) to communicate with plugin implementations.

**Protocol**: MCP strict subset
- Tools only (tools/list, tools/call)
- No prompts, no resources, no sampling
- Lifecycle management (health checks, graceful shutdown)

**MCP Server Types**:

**Local MCP (Containerized)**:
- Desktop: Gateway uses Docker Engine API to start container
- Kubernetes: Container runs as sidecar in gateway pod
- Same container image works for both

**Remote MCP (HTTP Endpoint)**:
- MCP server already running (customer infrastructure)
- Gateway communicates via remote URL

**Configuration**:
```yaml
plugins:
  # Catalog reference (string)
  credential_storage:
    provider: mcp
    server: catalog://docker.io/docker/mcp-plugins:v1/credstore-k8s-secret

  # Inline server definition - type: image
  auth_proxy:
    provider: mcp
    server:
      type: image
      image: docker.io/docker/mcp-authproxy:latest

  # Inline server definition - type: remote
  policy_evaluator:
    provider: mcp
    server:
      type: remote
      endpoint: https://policy.customer.internal:9000
```

## Architecture Diagram

```
┌───────────────────────────────────────────────────────────────────────┐
│                             MCP Gateway                               │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                       Plugin Registry                           │  │
│  │                                                                 │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │  │
│  │  │    Auth     │  │ Credential  │  │   Policy    │   ...        │  │
│  │  │  Provider   │  │  Storage    │  │  Evaluator  │              │  │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │  │
│  └─────────┼────────────────┼────────────────┼─────────────────────┘  │
│            │                │                │                        │
│  ┌─────────▼────────────────▼────────────────▼─────────────────────┐  │
│  │                    PluginProvider Interface                     │  │
│  │                                                                 │  │
│  │  - CreateAuthProvider(config) → AuthProvider                    │  │
│  │  - CreateCredentialStorage(config) → CredentialStorage          │  │
│  │  - CreateAuditSink(config) → AuditSink                          │  │
│  └─────────┬────────────────────────────────┬──────────────────────┘  │
│            │                                │                         │
└────────────┼────────────────────────────────┼─────────────────────────┘
             │                                │
    ┌────────▼────────┐              ┌────────▼────────┐
    │   In-Memory     │              │      MCP        │
    │    Provider     │              │    Provider     │
    │                 │              │                 │
    │  - Direct Go    │              │  - JSON-RPC/HTTP│
    │  - In-process   │              │  - Containers   │
    │  - Lowest lat.  │              │  - Remote URLs  │
    └─────────────────┘              └────────┬────────┘
                                              │
                                     ┌────────▼────────┐
                                     │  MCP Servers    │
                                     │                 │
                                     │  - Local (K8s   │
                                     │    sidecars)    │
                                     │  - Remote URLs  │
                                     └─────────────────┘
```

## Plugin Code Interfaces (The Stable Contract)

Gateway defines Go interfaces for all plugin types:

```go
type AuthProvider interface {
    ValidateCredential(ctx context.Context, creds Credentials) (*UserPrincipal, error)
}

type CredentialStorage interface {
    Store(ctx context.Context, userID, server, credType, value string) error
    Retrieve(ctx context.Context, userID, server, credType string) (string, error)
    Delete(ctx context.Context, userID, server, credType string) error
    List(ctx context.Context, userID string) ([]CredentialInfo, error)
}

type AuthProxy interface {
    InjectCredentials(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)
}

type AuditSink interface {
    LogEvent(ctx context.Context, event *AuditEvent) error
}

type PolicyEvaluator interface {
    CheckAccess(ctx context.Context, principal *UserPrincipal, mcpServer string) error
}

type MCPProvisioner interface {
    Provision(ctx context.Context, server *ServerDef, userID string) (*ProvisionedServer, error)
    Deprovision(ctx context.Context, serverID string) error
    List(ctx context.Context, userID string) ([]*ProvisionedServer, error)
}

type TelemetryPlugin interface {
    RecordCounter(ctx context.Context, name string, value int64, attrs map[string]string)
    RecordHistogram(ctx context.Context, name string, value float64, attrs map[string]string)
    RecordGauge(ctx context.Context, name string, value int64, attrs map[string]string)
    Close() error
}
```

## Desktop Deployment

**Local MCP Plugin Containers**:
1. Gateway resolves catalog reference to server definition
2. Server definition contains Docker image reference with digest
3. Gateway uses Docker Engine API to start container
4. Container lifecycle managed by gateway (start, health check, stop)
5. Communication via localhost:port

**In-Memory Plugins**:
- Auth provider: `desktop-implicit` (always returns desktop-user principal)
- Policy evaluator: `always-allow`
- Audit sink: `stdout` or `stderr`

## Kubernetes Deployment

**Sidecar MCP Plugin Containers**:
1. Plugin server references resolved from catalog at deployment time
2. Helm chart templates use catalog references to populate sidecar images
3. Kubernetes starts containers alongside gateway (pod lifecycle)
4. Communication via localhost:port (standard sidecar pattern)

**Pod Configuration** (Helm template):
```yaml
spec:
  containers:
  - name: gateway
    image: mcp-gateway:latest
    ports:
    - containerPort: 8811

  # Auth provider sidecar
  - name: auth-provider
    image: {{ .Values.plugins.authProvider.image }}
    ports:
    - containerPort: 8081

  # Credential storage sidecar
  - name: credential-storage
    image: {{ .Values.plugins.credentialStorage.image }}
    ports:
    - containerPort: 8083
```

## MCP Tool Conventions

Plugin MCP servers implement tools following these conventions:

### Auth Provider
- Tool: `validate_credential`
- Input: `{type: "api_key", value: "..."}`
- Output: `{user_id: "...", tenant_id: "...", roles: [...], groups: [...]}`

### Credential Storage
- Tools: `store_credential`, `retrieve_credential`, `delete_credential`, `list_credentials`

### Auth Proxy
- Tool: `inject_credentials`
- Input: `{user_id: "...", mcp_server: "...", target_url: "...", method: "...", headers: {...}, body: "..."}`
- Output: `{headers: {...}, body: "..."}`

### Audit Sink
- Tool: `log_event`
- Input: `{timestamp: "...", event_type: "...", user_id: "...", mcp_server: "...", tool: "...", result: "..."}`

### Policy Evaluator
- Tool: `check_access`
- Input: `{user_id: "...", tenant_id: "...", roles: [...], groups: [...], mcp_server: "..."}`
- Output: `{allowed: true/false, reason: "..."}`

### Telemetry
- Tools: `record-counter`, `record-histogram`, `record-gauge`

## Package Structure

```
pkg/plugins/
├── interface.go              # Plugin interfaces and types
├── loader.go                 # PluginRegistry and loading
├── mcp/
│   └── telemetry_adapter.go  # Existing MCP telemetry adapter
└── providers/
    ├── inmemory/
    │   ├── provider.go       # In-memory provider
    │   └── defaults.go       # Default implementations
    └── mcp/
        ├── provider.go       # MCP provider
        └── adapters.go       # MCP tool adapters
```

## Configuration Example

### Desktop Mode
```yaml
plugins:
  auth_provider:
    provider: in-memory
    implementation: desktop-implicit

  credential_storage:
    provider: mcp
    server:
      type: image
      image: docker.io/docker/mcp-credstore-keychain:latest

  audit_sink:
    provider: in-memory
    implementation: stdout

  policy_evaluator:
    provider: in-memory
    implementation: always-allow
```

### Kubernetes Mode
```yaml
plugins:
  auth_provider:
    provider: mcp
    server: catalog://docker.io/docker/mcp-plugins:v1/auth-k8s-secret

  credential_storage:
    provider: mcp
    server: catalog://docker.io/docker/mcp-plugins:v1/credstore-k8s-secret

  auth_proxy:
    provider: mcp
    server: catalog://docker.io/docker/mcp-plugins:v1/authproxy-bearer

  audit_sink:
    provider: in-memory
    implementation: stdout

  policy_evaluator:
    provider: mcp
    server:
      type: remote
      endpoint: https://policy.internal.example.com:9000
```

## Implementation Status

### Completed
- [x] Plugin code interfaces (AuthProvider, CredentialStorage, etc.)
- [x] PluginRegistry with provider pattern
- [x] In-memory provider with default implementations
- [x] MCP provider with tool adapters
- [x] TelemetryPlugin interface and MCP adapter
- [x] Configuration types

### Planned
- [ ] Catalog reference resolution
- [ ] Container manager for Desktop (Docker Engine API)
- [ ] Integration tests
- [ ] V1 MCP plugin containers (auth-k8s-secret, credstore-k8s-secret)

## References

- [Design Decision 0000: Plugin Model and Sidecar Strategy](https://github.com/docker/mcp-gateway-enterprise/blob/main/docs/design/0000-plugin-model-and-sidecar-strategy.md)
- [MCP Protocol Specification](https://modelcontextprotocol.io)
