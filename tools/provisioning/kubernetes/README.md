# MCP Gateway Kubernetes Cluster Tools

This directory contains comprehensive cluster provisioning tools for managing MCP Gateway resources in Kubernetes environments, including pod management, cluster provider resource provisioning, and end-to-end workflow automation.

## Important: Docker Desktop Kubernetes Dependency

**This is currently an experimental feature designed specifically for Docker Desktop's built-in Kubernetes cluster.** The cluster provider mode relies on Docker Desktop's container registry mirror (`desktop-containerd-registry-mirror:v0.0.2`) for image availability.

### Registry Mirror Requirement

Docker Desktop automatically configures its Kubernetes cluster to use a local registry mirror that acts as a pull-through/push-through cache. This means:

- When you `docker pull` or `docker push` images, they are cached in the local mirror
- The Kubernetes cluster is pre-configured to look in this mirror for images
- MCP server pods can only launch if their images are available in this mirror

**Critical**: If you try to deploy an MCP server image that has never been pulled via regular Docker commands, the pod will fail to start because the image won't be available in the registry mirror.

**How MCP Gateway Mitigates This**: During gateway startup, MCP Gateway automatically pulls all images referenced in the catalog's server specifications, plus any internal utility images needed for its operations (such as sidecar containers). This image pre-pulling populates the registry mirror, ensuring images are available when pods are created. However, if you add new servers or modify catalog configurations after gateway startup, you may need to restart the gateway to pull newly referenced images.

### Critical Architectural Constraint

**MCP Gateway must run on a machine with Docker access, not in the Kubernetes cluster itself.** This is because:

- MCP Gateway performs `docker pull` operations during startup to populate the Docker Desktop registry mirror
- Other Kubernetes clusters don't have this registry mirror, so the `docker pull` operations wouldn't help with image availability
- Even in cluster provider mode (which uses pre-existing ConfigMaps/Secrets), the image pre-pulling still occurs
- Currently, **MCP Gateway running inside a Kubernetes pod is not supported**

**Supported Architecture**:
- ✅ **MCP Gateway**: Runs on local machine with Docker Desktop
- ✅ **MCP Servers**: Run as pods in Docker Desktop's Kubernetes cluster
- ❌ **MCP Gateway in cluster**: Not currently supported

This means the Kubernetes provisioner is designed for **hybrid deployments** where the gateway runs locally but provisions servers in the cluster.

### Prerequisites

Before using cluster provider mode:

1. **Enable Docker Desktop Kubernetes**: Ensure Kubernetes is enabled in Docker Desktop settings
2. **Pre-pull Images**: Use `docker pull <image>` for any MCP server images you plan to deploy
3. **Verify Mirror**: The registry mirror runs automatically as part of Docker Desktop

### Other Kubernetes Clusters

While you may be able to use these tools with other Kubernetes clusters, you must handle all image availability and registry configuration manually. This includes:

- Ensuring MCP server images are accessible to your cluster
- Configuring image pull policies and registry authentication
- Managing network access between cluster and registries

For production deployments outside Docker Desktop, thorough testing and cluster-specific configuration is required.

## Tools

### cluster-tools (cluster-tools.go)

A comprehensive command-line utility for:
- **Pod Management**: Cleaning up MCP server pods and managing gateway-created Kubernetes resources
- **Cluster Provider Provisioning**: Creating ConfigMaps and Secrets for cluster provider mode
- **Configuration Generation**: Generating separate .env files for configs and secrets from MCP catalog data
- **Resource Population**: Creating Kubernetes resources from .env files
- **Data Extraction**: Analyzing MCP catalog data and extracting configuration requirements

#### Usage

```bash
# Build the cluster tools
go build -o cluster-tools cluster-tools.go
# OR
make cluster-tools
# OR (legacy compatibility)
make mcp-tool    # Creates symlink to cluster-tools

# Pod management commands
./cluster-tools session mcp-gateway-abc12345 [namespace]
./cluster-tools stale 24h [namespace]
./cluster-tools list [namespace]

# Configuration generation commands  
./cluster-tools generate-env <server-names> <base-name>
./cluster-tools extract-data <server-name>
./cluster-tools show-example <server-name>

# Resource provisioning commands
./cluster-tools populate-configmap <config-env-file> <configmap-name> [namespace]
./cluster-tools populate-secret <secret-env-file> <secret-name> [namespace]
./cluster-tools create-configmap <name> '<json-data>' [namespace]
./cluster-tools create-secret <name> '<json-data>' [namespace]
```

#### Commands

**Pod Management:**
- **session**: Remove all pods associated with a specific gateway session ID
- **stale**: Remove pods older than the specified duration (e.g., '24h', '2h30m', '45m')
- **list**: Display all pods currently managed by mcp-gateway

**Configuration Generation:**
- **generate-env**: Generate separate .env files for config and secret variables from MCP catalog data
- **extract-data**: Extract and display detailed configuration requirements for a specific MCP server
- **show-example**: Display example JSON data for ConfigMap/Secret creation for known servers

**Resource Provisioning:**
- **populate-configmap**: Create/update Kubernetes ConfigMap from config .env file
- **populate-secret**: Create/update Kubernetes Secret from secret .env file  
- **create-configmap**: Create a Kubernetes ConfigMap with JSON data (direct method)
- **create-secret**: Create a Kubernetes Secret with JSON data (direct method)

#### Examples

**Pod Management:**
```bash
# Clean up all pods from session mcp-gateway-abc12345 in the default namespace
./cluster-tools session mcp-gateway-abc12345

# Clean up all pods older than 24 hours in the 'mcp' namespace  
./cluster-tools stale 24h mcp

# List all MCP gateway pods in the current namespace
./cluster-tools list

# List all MCP gateway pods in a specific namespace
./cluster-tools list production
```

**Configuration Generation Workflow:**
```bash
# Generate separate .env files for apify-mcp-server
./cluster-tools generate-env apify-mcp-server apify
# Creates: apify-config.env (template variables for config) and apify-secret.env (credentials)

# Generate for multiple servers
./cluster-tools generate-env "apify-mcp-server,firewalla-mcp-server" multi
# Creates: multi-config.env and multi-secret.env with combined variables

# Extract detailed requirements for a server
./cluster-tools extract-data apify-mcp-server

# Show example JSON data for manual ConfigMap/Secret creation
./cluster-tools show-example firewalla-mcp-server
```

**Resource Provisioning Workflow:**
```bash
# Method 1: Use .env files (recommended for production)
# Step 1: Edit the generated .env files with actual values
# Step 2: Create ConfigMap and Secret from .env files
./cluster-tools populate-configmap apify-config.env my-mcp-config default
./cluster-tools populate-secret apify-secret.env my-mcp-secrets default

# Method 2: Direct JSON creation (for quick testing)
./cluster-tools create-configmap mcp-config '{"ENABLE_ADDING_ACTORS":"false"}' default
./cluster-tools create-secret mcp-secrets '{"APIFY_TOKEN":"your-token"}' default
```

## End-to-End Cluster Provider Workflow

When using `--cluster-config-provider cluster` with Docker MCP Gateway, you need pre-existing ConfigMaps and Secrets. Here's the complete workflow:

### 1. Generate Configuration Files

First, generate separate .env files for your MCP servers:

```bash
# Single server
./cluster-tools generate-env apify-mcp-server apify

# Multiple servers  
./cluster-tools generate-env "apify-mcp-server,firewalla-mcp-server" production
```

This creates:
- `<base-name>-config.env`: Template variables that need user input (goes to ConfigMap)
- `<base-name>-secret.env`: Credentials that need secret values (goes to Secret)

### 2. Edit Configuration Files

Edit the generated files with actual values:

```bash
# Edit config values (non-sensitive template variables)
vim apify-config.env

# Edit secret values (credentials)
vim apify-secret.env
```

### 3. Create Kubernetes Resources

```bash
# Create ConfigMap from config .env file
./cluster-tools populate-configmap apify-config.env my-mcp-config default

# Create Secret from secret .env file  
./cluster-tools populate-secret apify-secret.env my-mcp-secrets default
```

### 4. Run MCP Gateway

```bash
docker-mcp gateway run \
  --cluster-config-provider cluster \
  --cluster-secret-provider cluster \
  --cluster-config-name my-mcp-config \
  --cluster-secret-name my-mcp-secrets
```

### Configuration Separation

The `generate-env` command intelligently separates variables:

- **ConfigMap** (config file): Template variables from MCP server config parameters
- **Secret** (secret file): Credentials from MCP server secrets definitions
- **Excluded**: Static environment variables (already in container specs)

## Requirements

### RBAC Permissions

The MCP tool requires the following Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: mcp-gateway-provisioning
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["list", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["create", "list", "get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: mcp-gateway-provisioning
subjects:
- kind: User
  name: your-username # or ServiceAccount
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: mcp-gateway-provisioning
  apiGroup: rbac.authorization.k8s.io
```

### Authentication

The tool uses standard Kubernetes client authentication:
- In-cluster service account (when running inside Kubernetes)
- Local kubeconfig file (`~/.kube/config` or `$KUBECONFIG`)

## Pod Labels

MCP gateway creates pods with the following labels for management:

- `app.kubernetes.io/managed-by=mcp-gateway` - Identifies gateway-managed pods
- `app.kubernetes.io/instance=<session-id>` - Session tracking for cleanup
- `app.kubernetes.io/component=mcp-server` - Component identification
- `app.kubernetes.io/name=<server-name>` - MCP server name
- `mcp-gateway.docker.com/session=<session-id>` - Custom session label

## Safety Notes

- The cleanup tool only operates on pods with the `app.kubernetes.io/managed-by=mcp-gateway` label
- Session-specific cleanup only affects pods with matching session IDs
- Stale pod cleanup has age verification to prevent accidental deletion of active pods
- All deletions use foreground propagation for immediate cleanup

## Troubleshooting

### Permission Denied Errors

If you see RBAC permission errors, ensure your user or service account has the required permissions listed above.

### Connection Errors

- Verify `kubectl` is configured and working: `kubectl get pods`
- Check your kubeconfig file location and permissions
- Ensure the Kubernetes cluster is accessible

### No Pods Found

- Verify you're checking the correct namespace
- Confirm the gateway has been running and creating pods
- Check that pods have the expected labels: `kubectl get pods -l app.kubernetes.io/managed-by=mcp-gateway`