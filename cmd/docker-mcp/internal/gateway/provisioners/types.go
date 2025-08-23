package provisioners

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

// ProvisionerType represents the type of provisioner strategy
type ProvisionerType int

const (
	// DockerProvisioner represents local Docker container provisioning
	DockerProvisioner ProvisionerType = iota
	// KubernetesProvisioner represents Kubernetes pod-based provisioning
	KubernetesProvisioner
	// CloudProvisioner represents cloud container service provisioning
	CloudProvisioner
)

// String returns the string representation of the provisioner type
func (pt ProvisionerType) String() string {
	switch pt {
	case DockerProvisioner:
		return "docker"
	case KubernetesProvisioner:
		return "kubernetes"
	case CloudProvisioner:
		return "cloud"
	default:
		return "unknown"
	}
}

// ParseProvisionerType converts a string to a ProvisionerType
func ParseProvisionerType(s string) (ProvisionerType, error) {
	switch s {
	case "docker":
		return DockerProvisioner, nil
	case "kubernetes", "k8s":
		return KubernetesProvisioner, nil
	case "cloud":
		return CloudProvisioner, nil
	default:
		return DockerProvisioner, fmt.Errorf("unknown provisioner type: %s", s)
	}
}

// IsValid checks if the provisioner type is valid
func (pt ProvisionerType) IsValid() bool {
	return pt >= DockerProvisioner && pt <= CloudProvisioner
}

// SecretProviderType represents the type of secret provider strategy
type SecretProviderType int

const (
	// DockerEngineSecretProvider represents using Docker Desktop credential store
	DockerEngineSecretProvider SecretProviderType = iota
	// ClusterSecretProvider represents using pre-existing cluster secrets
	ClusterSecretProvider
)

// String returns the string representation of the secret provider type
func (spt SecretProviderType) String() string {
	switch spt {
	case DockerEngineSecretProvider:
		return "docker-engine"
	case ClusterSecretProvider:
		return "cluster"
	default:
		return "unknown"
	}
}

// ParseSecretProviderType converts a string to a SecretProviderType
func ParseSecretProviderType(s string) (SecretProviderType, error) {
	switch s {
	case "docker-engine", "docker":
		return DockerEngineSecretProvider, nil
	case "cluster", "k8s", "kubernetes-cluster":
		return ClusterSecretProvider, nil
	default:
		return DockerEngineSecretProvider, fmt.Errorf("unknown secret provider type: %s", s)
	}
}

// IsValid checks if the secret provider type is valid
func (spt SecretProviderType) IsValid() bool {
	return spt >= DockerEngineSecretProvider && spt <= ClusterSecretProvider
}

// ConfigProviderType represents the type of configuration provider strategy
type ConfigProviderType int

const (
	// DockerEngineConfigProvider represents using Docker Desktop configuration and template resolution
	DockerEngineConfigProvider ConfigProviderType = iota
	// ClusterConfigProvider represents using pre-existing cluster ConfigMaps/configuration sources
	ClusterConfigProvider
)

// String returns the string representation of the config provider type
func (cpt ConfigProviderType) String() string {
	switch cpt {
	case DockerEngineConfigProvider:
		return "docker-engine"
	case ClusterConfigProvider:
		return "cluster"
	default:
		return "unknown"
	}
}

// ParseConfigProviderType converts a string to a ConfigProviderType
func ParseConfigProviderType(s string) (ConfigProviderType, error) {
	switch s {
	case "docker-engine", "docker":
		return DockerEngineConfigProvider, nil
	case "cluster", "k8s", "kubernetes-cluster":
		return ClusterConfigProvider, nil
	default:
		return DockerEngineConfigProvider, fmt.Errorf("unknown config provider type: %s", s)
	}
}

// IsValid checks if the config provider type is valid
func (cpt ConfigProviderType) IsValid() bool {
	return cpt >= DockerEngineConfigProvider && cpt <= ClusterConfigProvider
}

// Provisioner defines the interface for all provisioning strategies.
// Each provisioner implements a specific deployment target (Docker, Kubernetes, Cloud).
type Provisioner interface {
	// GetName returns the unique identifier for this provisioner type.
	// Used for provisioner selection via command flags.
	GetName() string

	// PreValidateDeployment checks if the given spec can be provisioned
	// without actually deploying anything. This allows for early failure
	// detection and resource validation.
	PreValidateDeployment(ctx context.Context, spec ProvisionerSpec) error

	// ProvisionServer deploys the server according to the spec and returns
	// a client handle for MCP communication. The cleanup function should
	// be called to release any resources when the client is no longer needed.
	ProvisionServer(ctx context.Context, spec ProvisionerSpec, ss *mcp.ServerSession, server *mcp.Server) (mcpclient.Client, func(), error)

	// Initialize sets up the provisioner with configuration and dependencies.
	// This is called after provisioner creation when configuration is loaded.
	Initialize(ctx context.Context, configResolver ConfigResolver, serverConfigs map[string]*catalog.ServerConfig) error

	// Shutdown performs cleanup of all resources managed by this provisioner.
	// This is called during gateway shutdown to ensure proper resource cleanup.
	Shutdown(ctx context.Context) error

	// ApplyToolProviders applies secret and config provider settings to a POCI tool container spec.
	// This uses the same secret/config provider logic (docker-engine vs cluster) as MCP servers.
	// Each provisioner implements this based on their deployment target (Docker, Kubernetes, etc.).
	ApplyToolProviders(spec *runtime.ContainerSpec, toolName string)
}

// ConfigResolver provides just-in-time resolution of server configuration values.
// This interface allows provisioners to access secrets and environment variables
// without exposing them in the generic ProvisionerSpec.
type ConfigResolver interface {
	// ResolveSecrets returns the resolved secret values for the given server
	ResolveSecrets(serverName string) map[string]string

	// ResolveEnvironment returns the resolved environment variables for the given server
	ResolveEnvironment(serverName string) map[string]string

	// ResolveCommand returns the resolved command with template substitution
	ResolveCommand(serverName string) []string
}

// KubernetesSecretManager manages Kubernetes Secret resources and environment variable mappings
type KubernetesSecretManager interface {
	// GetSecretSpecs returns Kubernetes Secret resource specifications
	// for the given server. Returns map[secretName]secretData for Secret creation.
	GetSecretSpecs(serverName string) map[string]map[string]string

	// GetSecretKeyRefs returns environment variable to secretKeyRef mappings
	// for Pod manifest generation. Returns map[envVarName]SecretKeyRef.
	GetSecretKeyRefs(serverName string) map[string]SecretKeyRef
}

// KubernetesConfigManager manages Kubernetes ConfigMap resources and environment variable mappings
type KubernetesConfigManager interface {
	// GetConfigSpecs returns Kubernetes ConfigMap resource specifications
	// for the given server. Returns map[configMapName]configData for ConfigMap creation.
	GetConfigSpecs(serverName string) map[string]map[string]string

	// GetConfigMapRefs returns ConfigMap names for envFrom injection
	// for Pod manifest generation. Returns []string of ConfigMap names.
	GetConfigMapRefs(serverName string) []string
}

// SecretKeyRef represents a Kubernetes secretKeyRef for environment variables
type SecretKeyRef struct {
	Name string // Secret resource name
	Key  string // Key within the secret data
}

// ProvisionerSpec represents a standardized provisioning specification
// that is decoupled from catalog.ServerConfig to avoid tight coupling.
// This allows the provisioner interface to remain stable while catalog
// structures evolve.
type ProvisionerSpec struct {
	// Name is the unique identifier for this server instance
	Name string

	// Image is the container image to deploy (e.g., "mcp/server:latest")
	Image string

	// Command overrides the default container command
	Command []string

	// Environment contains non-sensitive environment variables to set in the container.
	// Secrets and sensitive config are resolved via ConfigResolver, not stored in this spec.
	Environment map[string]string

	// Volumes specifies volume mounts for the container
	Volumes []string

	// Ports specifies port mappings for containerized HTTP servers
	Ports []PortMapping

	// Networks specifies which networks the container should join
	Networks []string

	// Resources specifies resource limits for the container
	Resources ResourceLimits

	// DisableNetwork indicates whether to disable network access
	DisableNetwork bool

	// AllowHosts specifies which external hosts this server can access
	// Format: ["hostname:port", "api.github.com:443", "example.com:80/tcp"]
	// Used by provisioners to implement network access control
	AllowHosts []string

	// LongLived indicates whether this server should be kept running
	// for multiple requests rather than created per-request
	LongLived bool
}

// PortMapping represents a container port that should be exposed
type PortMapping struct {
	// ContainerPort is the port number inside the container
	ContainerPort int

	// Protocol is the network protocol (tcp, udp)
	Protocol string
}

// ResourceLimits specifies resource constraints for container deployment
type ResourceLimits struct {
	// CPUs specifies the CPU limit (e.g., 1.5 for 1.5 cores)
	CPUs float64

	// Memory specifies the memory limit (e.g., "512m", "1Gb")
	Memory string
}
