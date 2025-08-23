package gateway

import "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/provisioners"

type Config struct {
	Options
	ServerNames  []string
	CatalogPath  []string
	ConfigPath   []string
	RegistryPath []string
	ToolsPath    []string
	SecretsPath  string
}

type Options struct {
	Port                    int
	Transport               string
	ToolNames               []string
	Interceptors            []string
	Verbose                 bool
	LongLived               bool
	DebugDNS                bool
	LogCalls                bool
	BlockSecrets            bool
	BlockNetwork            bool
	VerifySignatures        bool
	DryRun                  bool
	Watch                   bool
	Cpus                    int
	Memory                  string
	MaxServerStartupTimeout int // Maximum time in seconds to wait for each server to start
	Static                  bool
	Central                 bool

	// Feature flags
	OAuthInterceptorEnabled bool

	// Provisioner configuration
	Provisioner string // "docker" or "k8s"

	// Docker-specific options
	DockerContext string // Docker context to use

	// Kubernetes-specific options
	Kubeconfig  string // Path to kubeconfig file
	Namespace   string // Kubernetes namespace
	KubeContext string // Kubernetes context

	// Secret provider configuration
	SecretProvider provisioners.SecretProviderType // Secret provider strategy
	SecretName     string                          // Secret resource name for cluster mode (default: "mcp-gateway-secrets")

	// Configuration provider configuration
	ConfigProvider provisioners.ConfigProviderType // Configuration provider strategy
	ConfigName     string                          // ConfigMap resource name for cluster mode (default: "mcp-gateway-config")

	// Session management (for resource cleanup)
	SessionID string // Generated session ID for resource tagging
}
