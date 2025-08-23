# Pluggable Provisioning for Docker MCP Gateway

## 1. Introduction & Overview

Pluggable provisioning introduces a unified provisioner architecture that abstracts MCP server deployment across multiple environments. This feature enables Docker MCP Gateway to provision servers not only in local Docker containers (existing functionality) but also in Kubernetes clusters, with a foundation for future cloud provider support.

The implementation follows a **Strategy Pattern** where different provisioner implementations handle environment-specific deployment logic while maintaining a consistent interface for server provisioning and lifecycle management.

### Architecture Benefits

- **Environment Abstraction**: Deploy MCP servers consistently across Docker and Kubernetes
- **Backward Compatibility**: 100% compatibility with existing Docker-based workflows
- **Extensible Design**: Foundation for future cloud provider support
- **Unified Interface**: Same commands and configurations work across provisioners

## 2. Core Architecture & Interfaces

### Provisioner Interface

All provisioners implement a unified interface:
```go
type Provisioner interface {
    GetName() string
    PreValidateDeployment(ctx context.Context, spec ProvisionerSpec) error
    ProvisionServer(ctx context.Context, spec ProvisionerSpec) (mcpclient.Client, func(), error)
    Initialize(ctx context.Context, configResolver ConfigResolver, serverConfigs map[string]*catalog.ServerConfig) error
    Shutdown(ctx context.Context) error
    ApplyToolProviders(spec *runtime.ContainerSpec, toolName string)
}
```

### Container Runtime Abstraction

Provisioners use Container Runtime implementations for actual container operations:
```go
type ContainerRuntime interface {
    RunContainer(ctx context.Context, spec ContainerSpec) (*ContainerResult, error)
    StartContainer(ctx context.Context, spec ContainerSpec) (*ContainerHandle, error)
    StopContainer(ctx context.Context, handle *ContainerHandle) error
    GetName() string
    Shutdown(ctx context.Context) error
}
```

### Configuration Resolution Interface

Just-in-time secret and configuration resolution through ConfigResolver pattern:
```go
type ConfigResolver interface {
    ResolveSecrets(serverName string) map[string]string
    ResolveEnvironment(serverName string) map[string]string
    ResolveCommand(serverName string) []string
}
```

### Gateway-Managed Architecture

The gateway creates and manages both Container Runtime and Provisioners, eliminating coordination issues:

```
Gateway (Infrastructure Authority)
├── Creates Container Runtime (coordinates with gateway's runtime environment)
├── Creates Provisioners (coordinates with container runtime)  
├── Manages network configuration
└── clientPool (Pure Client Management)
    ├── Receives Container Runtime as parameter
    ├── Receives Provisioners as parameter
    └── Completely agnostic to specific strategies
```

## 3. Docker Provisioner

### 3.1 Overview & Status

**Status**: ✅ Complete - Full backward compatibility with existing functionality

**Provisioner**: `--provisioner=docker` (default)

### 3.2 Default Behavior

Everything is resolved through the Docker engine by default. No provider configuration is needed or applicable - the Docker Provisioner handles all secrets and configuration resolution automatically.

- **Secrets**: Resolved from Docker Desktop credential store
- **Configuration**: Resolved from local YAML files and Docker environment
- **No provider options**: Secret and config providers are not applicable - Docker engine handles everything

### 3.3 Capabilities & Features

#### Remote HTTP Endpoints
Direct connection to MCP servers exposing HTTP interfaces

#### Containerized Stdio
Traditional container-based stdio communication via Docker

#### POCI Tool Support
Ephemeral containers for tools with native stdout/stderr separation via Docker API

#### Secret Injection
Just-in-time secret resolution from Docker Desktop credential store

#### Network Management
Automatic network detection and container attachment with full `allowHosts` support:
- **Per-server isolation**: Each MCP server gets dedicated proxy infrastructure
- **L4/L7 proxy support**: Automatic protocol detection (`api.github.com:443` → L7, `database:5432/tcp` → L4)
- **Container-level enforcement**: Docker links, custom networks, and proxy environment variables
- **Zero cross-server access**: Servers cannot access each other's allowed hosts

### 3.4 Container Runtime

Uses `DockerContainerRuntime` with Docker API for both ephemeral (tools) and persistent (servers) containers.

### 3.5 Configuration Resolution

Uses Docker Desktop credential store for template resolution (`{{dockerhub.username}}` → actual values).

**Configuration Sources**:
- **Secrets**: Docker Desktop credential store (`docker-credential-desktop`) - only secrets defined in catalog `secrets` array
- **Config Variables**: YAML configuration files from:
  - `--config` flag paths (if specified)
  - `~/.docker/mcp/config.yaml` (default when no `--config` flag)
  - Legacy Docker volume import (fallback if file doesn't exist)
  - Empty configuration (if no sources available)
- **Command Templates**: Same YAML configuration files as config variables

### 3.6 Command Line Usage

```bash
# Docker provisioning (default)
docker-mcp gateway run --provisioner=docker

# Docker-specific flags
docker-mcp gateway run --provisioner=docker --docker-context CONTEXT
```

**Provider Flags**: Not applicable - `--cluster-config-provider` and `--cluster-secret-provider` flags are ignored.

## 4. Kubernetes Provisioner

### 4.1 Overview & Status

**Status**: ⚠️ Highly experimental with Docker Desktop Kubernetes

**Provisioner**: `--provisioner=kubernetes`

**Feature Flag**: The Kubernetes provisioner is gated behind the `kubernetes-provisioning` experimental feature flag, which is **disabled by default**.

**Enabling the Feature**:
```bash
# Check current feature status
docker mcp feature list

# Enable Kubernetes provisioning
docker mcp feature enable kubernetes-provisioning

# Verify it's enabled
docker mcp feature list
```

**Behavior**: Requires explicit provider configuration to determine where secrets and configuration are sourced. Supports two distinct modes with different operational characteristics.

### 4.2 Provider Mode Architecture

The Kubernetes provisioner supports two distinct provider modes for handling configuration templates and secrets, enabling both development flexibility and production security requirements.

**Important**: These provider modes **only apply to the Kubernetes Provisioner**. The Docker Provisioner automatically uses the Docker engine for all resolution without provider configuration.

#### Configuration Summary Table

| Provisioner | Provider Configuration | Secrets Source | Config Source | Use Case |
|-------------|----------------------|----------------|---------------|----------|
| **Docker** | Not applicable | Docker Desktop credential store | Local YAML files | Default behavior |
| **Kubernetes + docker-engine** | `--cluster-*-provider docker-engine` | Docker Desktop credential store | Local YAML files | K8s development |
| **Kubernetes + cluster** | `--cluster-*-provider cluster` | Pre-existing K8s Secrets (out-of-band) | Pre-existing K8s ConfigMaps + catalog static values | K8s production |

### 4.3 Docker-Engine Provider Mode

**Purpose**: Development and local deployment scenarios where Docker Desktop manages credentials

**CLI Configuration**:
```bash
--provisioner=kubernetes --cluster-config-provider docker-engine --cluster-secret-provider docker-engine
```

**Behavior**: Uses Docker engine as the source for secrets and config, just like Docker Provisioner
- **Secrets**: Resolved from Docker Desktop credential store 
- **Configuration**: Resolved from local YAML files
- **Use Case**: Development and testing in Kubernetes with Docker Desktop convenience

**Template Resolution Process**:
1. **Template Detection**: System identifies templates like `{{dockerhub.username}}`, `{{github.token}}` in server configurations
2. **Just-in-Time Resolution**: ConfigResolver queries Docker Desktop credential store when servers are provisioned
3. **Direct Injection**: Resolved values injected directly into container environment variables
4. **Dynamic Resource Creation**: For Kubernetes, ConfigMaps and Secrets created dynamically with resolved values

**Architecture**:
```go
// Docker-engine mode flow for secrets
ConfigResolver.ResolveSecrets("server-name") 
→ serverConfig.Secrets (from Docker Desktop credential store)
→ {"dockerhub.username": "actualuser", "github.token": "ghp_abc123"}

// Docker-engine mode flow for config variables  
ConfigResolver.ResolveEnvironment("server-name")
→ serverConfig.Config (from YAML config files) 
→ Template evaluation: eval.Evaluate("{{server.setting}}", serverConfig.Config)
→ {"SOME_CONFIG": "resolved-value"}
```

### 4.4 Cluster Provider Mode

**Purpose**: Production deployments where ConfigMaps and Secrets are pre-provisioned and managed out-of-band

**CLI Configuration**:
```bash
--provisioner=kubernetes --cluster-config-provider cluster --cluster-secret-provider cluster \
--cluster-config-name my-mcp-config --cluster-secret-name my-mcp-secrets
```

**Behavior**: Uses pre-configured Kubernetes ConfigMaps and Secrets in the cluster
- **Static Config Values**: Automatically configured in ConfigMap from catalog definitions (if permissions allow)
- **Dynamic Config Values**: Must already exist in the cluster's ConfigMap
- **All Secrets**: Must be configured via out-of-band processes - no automatic secret provisioning
- **Value Hierarchy**: Only values not hard-coded in catalog definitions are looked up from cluster resources
- **Use Case**: Production deployments with centralized secret management and out-of-band configuration

**Template Resolution Process**:
1. **Template Preservation**: Templates like `{{service.api_key}}` left unresolved in container specifications
2. **Resource Reference**: Container environment variables use `secretKeyRef` and `configMapKeyRef` references
3. **Pre-Existing Resources**: Assumes ConfigMaps and Secrets already exist in cluster with matching keys
4. **Kubernetes-Native Security**: Leverages Kubernetes RBAC, encryption, and secret rotation

**Architecture**:
```go
// Cluster mode flow
Template: "{{github.token}}" 
→ Preserved as reference in Pod spec
→ Environment variable: valueFrom.secretKeyRef{name: "mcp-secrets", key: "github.token"}
→ Kubernetes resolves reference at runtime
```

**Resource Mapping**:
```yaml
# Template in catalog: GITHUB_TOKEN={{github.token}}
# Becomes Pod environment variable:
env:
- name: GITHUB_TOKEN
  valueFrom:
    secretKeyRef:
      name: mcp-gateway-secrets
      key: github.token
```

**Configuration Sources**:
- **Secrets**: Pre-existing Kubernetes Secret resources (`kubectl create secret`)
- **Configuration**: Pre-existing Kubernetes ConfigMap resources (`kubectl create configmap`)
- **Template Variables**: Direct key mapping from template to resource key

**Template Variable Categorization**:

The system intelligently categorizes template variables based on MCP catalog metadata:

**Secret Variables** (→ Secrets):
- Variables corresponding to entries in catalog `secrets` array
- Examples: `{{github.token}}`, `{{api.key}}`, `{{database.password}}`
- Always stored in Kubernetes Secrets or resolved from Docker Desktop credential store

**Configuration Variables** (→ ConfigMaps):
- Variables corresponding to entries in catalog `config` array  
- Examples: `{{server.actors}}`, `{{cache.size}}`, `{{debug.level}}`
- Stored in Kubernetes ConfigMaps or resolved from Docker Desktop environment

**Static Values** (→ Container Spec):
- Non-templated environment variables like `NODE_ENV=production`
- Included directly in container specifications
- Not extracted to external resources

### 4.5 Capabilities & Features

#### Pod-Based Deployment
MCP servers run as Kubernetes pods with full lifecycle management

#### Native Secret Management
Kubernetes Secrets with `secretKeyRef` environment variable injection

#### ConfigMap Support
Native Kubernetes ConfigMaps for non-sensitive configuration

#### Session-Based Cleanup
Automatic pod cleanup on gateway shutdown with session tracking

#### POCI Tool Implementation

The POCI tool support required sophisticated technical solutions due to fundamental differences between Docker and Kubernetes APIs.

**The Problem**: Kubernetes doesn't natively support stdout/stderr separation for ephemeral containers like Docker does. Multiple API limitations prevent direct stream separation:
- **Pod logs API**: Always combines stdout/stderr streams
- **Pod attach API**: Only works reliably for running containers
- **Jobs API**: Also combines stdout/stderr streams (attempted but unsuccessful)

**The Solution**: A dual-container sidecar approach with ENTRYPOINT override and file-based stream separation:

**1. Image Inspection and Command Reconstruction**:
```go
// Inspect original container image to get ENTRYPOINT and CMD
entrypoint, cmd, err := k.inspectImage(ctx, spec.Image)

// Reconstruct what would normally run: ENTRYPOINT + CMD + args
fullCommand := append(entrypoint, spec.Command...)
```

**2. ENTRYPOINT Override with Stream Redirection**:
```bash
# Original command: /entrypoint.sh curl https://example.com
# Wrapped command with redirection:
/entrypoint.sh curl https://example.com >/logs/stdout.log 2>/logs/stderr.log; echo $? >/logs/exit_code.log; touch /logs/complete.marker
```

**3. Dual-Container Pod Architecture**:
- **Main Container**: Runs POCI tool with overridden ENTRYPOINT, writes separated logs to shared volume
- **Sidecar Container**: Alpine-based (`alpine:3.22.1`) with `sleep 3600`, provides file access after main container completes
- **Shared Volume**: EmptyDir volume mounted at `/logs/` for both containers

**4. Stream Retrieval Process**:
```go
// Wait for main container completion
err := k.waitForPodCompletion(ctx, podName, spec.StartupTimeout)

// Wait for completion marker file
err = k.waitForCompletionMarker(ctx, podName, 30*time.Second)

// Read separated streams via kubectl exec on sidecar
stdoutBytes := k.execInContainer(ctx, podName, "sidecar", []string{"cat", "/logs/stdout.log"})
stderrBytes := k.execInContainer(ctx, podName, "sidecar", []string{"cat", "/logs/stderr.log"})
exitCodeBytes := k.execInContainer(ctx, podName, "sidecar", []string{"cat", "/logs/exit_code.log"})
```

**5. Docker Compatibility Validation**:
The implementation achieves identical behavior to Docker provisioner:
- **Docker**: `stdout` = HTML content, `stderr` = curl progress bars
- **Kubernetes**: `stdout` = HTML content (identical), `stderr` = curl progress bars (identical)
- **Exit codes**: Preserved across both provisioners with proper error handling

This technical solution overcomes Kubernetes API limitations while maintaining full compatibility with existing POCI catalog entries and the `{{args|into}}` template pattern.

### 4.6 Container Runtime

Uses `KubernetesContainerRuntime` with client-go v0.33.4 for Pod management and attach API for stdio streams.

**Key Technical Features**:
- Real stdio communication via Pod attach API (`/attach` subresource)
- Automatic image inspection using go-containerregistry for POCI tools
- Comprehensive session-based resource labeling and cleanup
- Race condition elimination through shared resource initialization

### 4.7 Command Line Usage

#### Provisioner Selection
```bash
# Kubernetes provisioning  
docker-mcp gateway run --provisioner=kubernetes
```

#### Kubernetes-Specific Flags
```bash
--kubeconfig PATH           # Path to kubeconfig file
--namespace NAMESPACE       # Target Kubernetes namespace
--kube-context CONTEXT      # Kubernetes context name
```

#### Configuration Provider Modes
```bash
# Docker-engine mode (development) - uses Docker Desktop credential store
--cluster-config-provider docker-engine
--cluster-secret-provider docker-engine

# Cluster mode (production) - uses pre-existing Kubernetes resources
--cluster-config-provider cluster
--cluster-secret-provider cluster
--cluster-config-name my-mcp-config
--cluster-secret-name my-mcp-secrets
```

**Provider Flags**: Required - Must specify both `--cluster-config-provider` and `--cluster-secret-provider`.

### 4.8 Cluster Provider Workflow

For production Kubernetes deployments where ConfigMaps and Secrets are managed out-of-band:

#### 1. Configuration Discovery
```bash
./cluster-tools extract-data firewalla-mcp-server
# Analyzes catalog requirements and shows what needs configuration
```

#### 2. Template Generation
```bash
./cluster-tools generate-env firewalla-mcp-server,apify-mcp-server multi
# Creates: multi-config.env (only templated env vars) + multi-secret.env (all secrets)
```

#### 3. Resource Provisioning
```bash
./cluster-tools populate-configmap multi-config.env my-mcp-config default
./cluster-tools populate-secret multi-secret.env my-mcp-secrets default
```

#### 4. Gateway Execution
```bash
docker-mcp gateway run \
  --provisioner=kubernetes \
  --cluster-config-provider cluster \
  --cluster-secret-provider cluster \
  --cluster-config-name my-mcp-config \
  --cluster-secret-name my-mcp-secrets
```

### 4.9 Utility Tools

**Comprehensive cluster-tools utility** (`tools/provisioning/kubernetes/cluster-tools.go`):
- **Pod Management**: Session cleanup, stale resource detection, resource listing
- **Configuration Generation**: Template file generation from catalog data with security separation
- **Resource Provisioning**: ConfigMap and Secret creation from .env files
- **Analysis Tools**: Configuration requirement extraction and example data display

### 4.10 Current Limitations

#### Docker Desktop Kubernetes Dependency

**⚠️ Critical Constraint**: The Kubernetes provisioner is currently **experimental and designed specifically for Docker Desktop's built-in Kubernetes cluster**.

**Registry Mirror Dependency**:
- Docker Desktop uses a local registry mirror (`desktop-containerd-registry-mirror:v0.0.2`) as a pull-through cache
- MCP Gateway performs `docker pull` during startup to populate this mirror
- Other Kubernetes clusters lack this registry mirror, making image availability problematic

**Architectural Constraint**:
- **MCP Gateway must run on a machine with Docker access, not in the Kubernetes cluster itself**
- Even in cluster provider mode, the image pre-pulling logic still executes
- Running MCP Gateway inside a Kubernetes pod would likely cause failures

**Supported Architecture**:
- ✅ **MCP Gateway**: Runs on local machine with Docker Desktop
- ✅ **MCP Servers**: Run as pods in Docker Desktop's Kubernetes cluster
- ❌ **MCP Gateway in cluster**: Not currently supported

#### Configuration Provider Limitations

**Template Resolution Context**: While cluster provider mode supports pre-existing ConfigMaps/Secrets for credentials, non-secret template resolution (environment variables, command templates) still assumes Docker Desktop availability for development scenarios.

#### Network Access Control Limitations

**⚠️ Kubernetes Proxy Limitations**: Network access control via `allowHosts` configuration is **only implemented for the Docker provisioner**.

**Docker Provisioner Network Security** ✅:
- **Per-server isolation**: Each MCP server gets dedicated proxy infrastructure
- **L4/L7 proxy support**: Automatic protocol detection (`api.github.com:443` → L7, `database:5432/tcp` → L4)
- **Container-level enforcement**: Docker links, custom networks, and proxy environment variables
- **Zero cross-server access**: Servers cannot access each other's allowed hosts

**Kubernetes Provisioner Network Security** ⚠️:
- **No network restrictions**: `allowHosts` configuration is **ignored**
- **Full cluster network access**: MCP servers can reach any network destination accessible from the cluster
- **No proxy infrastructure**: Kubernetes provisioner does not implement proxy containers
- **Alternative security approaches**: Must rely on Kubernetes NetworkPolicies, service mesh, or cluster-level firewalls

**Impact**:
```yaml
# Same configuration, different network security behavior
servers:
  example-server:
    image: "mcp/example:latest" 
    allowHosts: ["api.github.com:443"]  # Network restriction specification
    
# Docker provisioner behavior:
# ✅ Server can ONLY access api.github.com:443 via dedicated L7 proxy
# ❌ Server CANNOT access any other hosts (blocked by network isolation)

# Kubernetes provisioner behavior:  
# ⚠️ allowHosts configuration IGNORED
# ⚠️ Server can access ANY network destination available to the cluster
# ⚠️ No automatic network restriction enforcement
```

**Future Enhancement**: Network access control for Kubernetes would require implementing NetworkPolicies or service mesh integration rather than container-based proxies.

## 5. Configuration Summary & Quick Reference

### Provider Flag Matrix

| Scenario | Provisioner | Config Provider | Secret Provider | Use Case |
|----------|-------------|-----------------|-----------------|----------|
| Local Docker | `docker` | N/A (ignored) | N/A (ignored) | Default development |
| K8s Development | `kubernetes` | `docker-engine` | `docker-engine` | K8s with Docker Desktop convenience |
| K8s Production | `kubernetes` | `cluster` | `cluster` | Production with out-of-band config |

### Command Examples

#### Default Docker Development
```bash
docker-mcp gateway run
# or explicitly:
docker-mcp gateway run --provisioner=docker
```

#### Kubernetes Development Mode
```bash
docker-mcp gateway run \
  --provisioner=kubernetes \
  --cluster-config-provider docker-engine \
  --cluster-secret-provider docker-engine
```

#### Kubernetes Production Mode
```bash
docker-mcp gateway run \
  --provisioner=kubernetes \
  --cluster-config-provider cluster \
  --cluster-secret-provider cluster \
  --cluster-config-name my-mcp-config \
  --cluster-secret-name my-mcp-secrets
```

### Mode-Specific Behavior

| Aspect | Docker | K8s + docker-engine | K8s + cluster |
|--------|--------|-------------------|---------------|
| **Secret Resolution** | Docker Desktop credential store | Docker Desktop credential store | Pre-existing Kubernetes Secrets |
| **Template Processing** | Resolved to actual values | Resolved to actual values | Preserved as secretKeyRef references |
| **Resource Creation** | Container environment variables | Dynamic creation during provisioning | Assumes pre-existing resources |
| **Security Model** | Docker Desktop trust model | Docker Desktop trust model | Kubernetes RBAC + encryption |
| **Operational Model** | Development-focused, automatic | Development-focused, automatic | Production-focused, manual pre-provisioning |
| **Network Security** | Full allowHosts support | No allowHosts support | No allowHosts support |

## 6. Testing & Validation

### Test Coverage
- **Unit Tests**: Interface definitions, provisioner implementations, adapter functions
- **Integration Tests**: Real Docker and Kubernetes cluster scenarios
- **Regression Tests**: Comprehensive baseline validation preventing functionality loss
- **Cross-Provisioner Tests**: Compatibility testing between provisioner types

### Validation Results
- **All existing tests passing**: No regressions introduced
- **Docker provisioner**: Complete feature parity with legacy implementation
- **Kubernetes provisioner**: Full lifecycle tested with real clusters
- **POCI tools**: stdout/stderr separation working across both provisioners

## 7. Technical Achievements

### Zero-Regression Implementation
- **100% Backward Compatibility**: All existing Docker functionality preserved
- **Comprehensive Regression Testing**: 61+ tests with baseline validation
- **Legacy Path Elimination**: Clean migration to provisioner interface

### Container Runtime Abstraction
- **Unified Interface**: Both ephemeral (tools) and persistent (servers) container operations
- **Transport Agnostic**: Works with stdio streams, HTTP endpoints, and container handles
- **Lifecycle Management**: Proper cleanup and resource management across runtimes

### Experimental Kubernetes Support
- **Native Resource Management**: Uses Kubernetes-native ConfigMaps, Secrets, and RBAC
- **Session Tracking**: Comprehensive cleanup with gateway session IDs
- **Real MCP Communication**: Working stdio streams via Pod attach API
- **Security Best Practices**: secretKeyRef injection, encrypted storage, access controls

### Operational Excellence
- **Comprehensive Tooling**: Complete workflow from catalog analysis to resource provisioning
- **Security Separation**: Configs and secrets handled separately with appropriate protection
- **Error Handling**: Detailed diagnostics, RBAC-aware error messages, timeout management
- **Documentation**: Complete workflows with troubleshooting guides

## 8. Future Work

### Cloud Provisioner Implementation
- Target cloud platform integration (AWS/Azure/GCP)
- Cloud container service API integration
- Cloud-native secret management (AWS Secrets Manager, Azure Key Vault, etc.)
- Service discovery and load balancer integration
- Cost estimation and resource optimization

### Gateway Runtime Abstraction
- **Problem**: Gateway currently assumes Docker runtime for network detection
- **Solution**: Pluggable Gateway Runtime abstraction (similar to MCP provisioner pattern)
- **Impact**: Would enable MCP Gateway to run in various environments (Docker, K8s, host, cloud)

### Enhanced Image Management
- **Problem**: Current Docker Desktop registry mirror dependency
- **Solutions**:
  - OCI registry integration for direct image management
  - Kubernetes-native image pull mechanisms
  - Cloud registry integration
- **Impact**: Would enable true multi-cluster Kubernetes support

### Configuration Provider Enhancement
- **Extract ConfigProvider interface** from ConfigResolver for broader template resolution
- **Support cluster-based configuration** for non-secret templates
- **External configuration service integration**

## Summary

The pluggable provisioning feature successfully introduces a unified, extensible architecture for MCP server deployment while maintaining 100% backward compatibility. The implementation provides a production-ready Docker provisioner and a highly experimental Kubernetes provisioner with comprehensive tooling, security best practices, and operational excellence.

While currently constrained to Docker Desktop environments for Kubernetes provisioning, the architecture provides a solid foundation for future enhancements including cloud provider support and enhanced multi-cluster capabilities.