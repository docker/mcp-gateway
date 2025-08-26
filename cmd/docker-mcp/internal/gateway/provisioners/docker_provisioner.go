package provisioners

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

// ProxyRunner provides proxy functionality for network isolation
type ProxyRunner interface {
	RunProxies(ctx context.Context, allowedHosts []string, longRunning bool) (proxies.TargetConfig, func(context.Context) error, error)
}

// DockerProvisionerImpl implements the Provisioner interface for Docker container deployment
type DockerProvisionerImpl struct {
	docker           docker.Client
	containerRuntime runtime.ContainerRuntime
	configResolver   ConfigResolver // Just-in-time secret/config resolution
	proxyRunner      ProxyRunner    // Network proxy functionality for AllowHosts
	networks         []string
	verbose          bool
	static           bool
	blockNetwork     bool
	cpus             int
	memory           string
	longLived        bool
	capRefresher     mcpclient.CapabilityRefresher
}

// DockerProvisionerConfig holds configuration for creating a Docker provisioner
type DockerProvisionerConfig struct {
	Docker           docker.Client
	ContainerRuntime runtime.ContainerRuntime
	ConfigResolver   ConfigResolver // Just-in-time config resolution
	ProxyRunner      ProxyRunner    // Network proxy functionality
	Networks         []string
	Verbose          bool
	Static           bool
	BlockNetwork     bool
	Cpus             int
	Memory           string
	LongLived        bool
	CapRefresher     mcpclient.CapabilityRefresher
}

// NewDockerProvisioner creates a new Docker provisioner instance
func NewDockerProvisioner(config DockerProvisionerConfig) *DockerProvisionerImpl {
	return &DockerProvisionerImpl{
		docker:           config.Docker,
		containerRuntime: config.ContainerRuntime,
		configResolver:   config.ConfigResolver,
		proxyRunner:      config.ProxyRunner,
		networks:         config.Networks,
		verbose:          config.Verbose,
		static:           config.Static,
		blockNetwork:     config.BlockNetwork,
		cpus:             config.Cpus,
		memory:           config.Memory,
		longLived:        config.LongLived,
		capRefresher:     config.CapRefresher,
	}
}

// GetName returns the unique identifier for this provisioner type
func (dp *DockerProvisionerImpl) GetName() string {
	return DockerProvisioner.String()
}

// SetNetworks updates the networks configuration for this provisioner
func (dp *DockerProvisionerImpl) SetNetworks(networks []string) {
	dp.networks = networks
	dp.debugLog("Updated networks configuration:", networks)
}

// Initialize sets up the provisioner with configuration and dependencies
func (dp *DockerProvisionerImpl) Initialize(_ context.Context, configResolver ConfigResolver, _ map[string]*catalog.ServerConfig) error {
	dp.debugLog("Initialize called")

	// Set up the config resolver
	dp.configResolver = configResolver
	dp.debugLog("ConfigResolver injected")

	return nil
}

// SetConfigResolver injects a config resolver for just-in-time secret/config resolution
// Deprecated: Use Initialize method instead
func (dp *DockerProvisionerImpl) SetConfigResolver(resolver ConfigResolver) {
	dp.configResolver = resolver
	dp.debugLog("ConfigResolver injected")
}

// PreValidateDeployment checks if the given spec can be provisioned without actually deploying
func (dp *DockerProvisionerImpl) PreValidateDeployment(_ context.Context, spec ProvisionerSpec) error {
	dp.debugLog("PreValidateDeployment called for server:", spec.Name)
	// For Docker provisioner, basic validation:
	// 1. If it's a remote endpoint, no validation needed
	// 2. If it's containerized, we could validate image availability, but that's expensive
	// 3. For static mode, we could validate socat availability
	// For now, minimal validation - just check that we have required fields
	if spec.Name == "" {
		dp.debugLog("Validation failed: server name is required")
		return fmt.Errorf("server name is required")
	}

	// If it's not a remote endpoint and not static, we need an image
	if spec.Image == "" && !dp.static {
		dp.debugLog("Validation failed for server:", spec.Name, "- container image is required for containerized deployment")
		return fmt.Errorf("container image is required for containerized deployment")
	}

	dp.debugLog("PreValidateDeployment passed for server:", spec.Name)
	return nil
}

// ProvisionServer deploys the server according to the spec and returns a client handle
func (dp *DockerProvisionerImpl) ProvisionServer(ctx context.Context, spec ProvisionerSpec, ss *mcp.ServerSession, server *mcp.Server) (mcpclient.Client, func(), error) {
	// This method implements the logic that was previously in clientGetter.GetClient()
	dp.debugLog("ProvisionServer called for server:", spec.Name)
	cleanup := func() {}
	var client mcpclient.Client

	// Convert spec back to serverConfig for compatibility with existing logic
	// This is temporary until we fully migrate to the provisioner interface
	serverConfig := dp.specToServerConfig(spec)
	dp.debugLog("Converted spec to server config for:", spec.Name)

	// Decision tree for client type (copied from original logic)
	if serverConfig.Spec.SSEEndpoint != "" {
		// Deprecated: Use Remote instead
		dp.debugLog("Using deprecated SSE endpoint for:", spec.Name)
		client = mcpclient.NewRemoteMCPClient(serverConfig)
	} else if serverConfig.Spec.Remote.URL != "" {
		// Remote HTTP endpoint
		dp.debugLog("Using remote HTTP endpoint for:", spec.Name, "URL:", serverConfig.Spec.Remote.URL)
		client = mcpclient.NewRemoteMCPClient(serverConfig)
	} else if dp.static {
		// Static deployment mode (socat tunnel)
		dp.debugLog("Using static deployment mode (socat) for:", spec.Name)
		client = mcpclient.NewStdioCmdClient(
			serverConfig.Name,
			"socat",
			nil,
			"STDIO",
			fmt.Sprintf("TCP:mcp-%s:4444", serverConfig.Name),
		)
	} else if spec.LongLived {
		// Long-lived containerized stdio - use Container Runtime for persistent containers
		dp.debugLog("Using persistent containerized stdio via Container Runtime for long-lived MCP server:", spec.Name)

		// Set up network proxies if AllowHosts specified
		var proxyCleanup func(context.Context) error
		containerSpec := dp.buildContainerSpec(spec)

		if len(spec.AllowHosts) > 0 && dp.proxyRunner != nil {
			dp.debugLog("Setting up network proxies for:", spec.Name, "AllowHosts:", spec.AllowHosts)
			targetConfig, cleanup, err := dp.proxyRunner.RunProxies(ctx, spec.AllowHosts, spec.LongLived)
			if err != nil {
				dp.debugLog("Failed to set up proxies for:", spec.Name, "error:", err)
				return nil, func() {}, fmt.Errorf("proxy setup failed: %w", err)
			}
			proxyCleanup = cleanup

			// Apply proxy configuration to container spec
			dp.applyProxyConfig(&containerSpec, targetConfig)
			dp.debugLog("Applied proxy configuration for:", spec.Name, "network:", targetConfig.NetworkName)
		}

		containerSpec.Persistent = true // Ensure persistent for long-lived

		if len(containerSpec.Command) == 0 {
			dp.debugLog("Starting persistent container", containerSpec.Image, "via Container Runtime")
		} else {
			dp.debugLog("Starting persistent container", containerSpec.Image, "with command", containerSpec.Command, "via Container Runtime")
		}

		// Start persistent container using Container Runtime
		handle, err := dp.containerRuntime.StartContainer(ctx, containerSpec)
		if err != nil {
			dp.debugLog("Failed to start container for:", spec.Name, "error:", err)
			return nil, cleanup, fmt.Errorf("failed to start container: %w", err)
		}

		// Create MCP client using Container Runtime handles
		client = mcpclient.NewStdioHandleClient(serverConfig.Name, handle.Stdin, handle.Stdout)

		// Update cleanup to stop the container and proxies
		originalCleanup := cleanup
		cleanup = func() {
			// Stop container first
			ctx := context.Background() // Use background context for cleanup
			if stopErr := dp.containerRuntime.StopContainer(ctx, handle); stopErr != nil {
				dp.debugLog("Error stopping container for:", spec.Name, "error:", stopErr)
			}
			// Clean up proxies if they were set up
			if proxyCleanup != nil {
				if proxyErr := proxyCleanup(ctx); proxyErr != nil {
					dp.debugLog("Error cleaning up proxies for:", spec.Name, "error:", proxyErr)
				}
			}
			// Then run original cleanup
			originalCleanup()
		}
	} else {
		// Short-lived containerized stdio - use Container Runtime but with ephemeral behavior
		dp.debugLog("Using ephemeral containerized stdio via Container Runtime for short-lived MCP server:", spec.Name)

		// Set up network proxies if AllowHosts specified
		var proxyCleanup func(context.Context) error
		containerSpec := dp.buildContainerSpec(spec)

		if len(spec.AllowHosts) > 0 && dp.proxyRunner != nil {
			dp.debugLog("Setting up network proxies for:", spec.Name, "AllowHosts:", spec.AllowHosts)
			targetConfig, cleanup, err := dp.proxyRunner.RunProxies(ctx, spec.AllowHosts, spec.LongLived)
			if err != nil {
				dp.debugLog("Failed to set up proxies for:", spec.Name, "error:", err)
				return nil, func() {}, fmt.Errorf("proxy setup failed: %w", err)
			}
			proxyCleanup = cleanup

			// Apply proxy configuration to container spec
			dp.applyProxyConfig(&containerSpec, targetConfig)
			dp.debugLog("Applied proxy configuration for:", spec.Name, "network:", targetConfig.NetworkName)
		}

		containerSpec.Persistent = false    // Ephemeral behavior - will be cleaned up automatically
		containerSpec.RemoveAfterRun = true // Equivalent to docker run --rm

		if len(containerSpec.Command) == 0 {
			dp.debugLog("Starting ephemeral container", containerSpec.Image, "via Container Runtime")
		} else {
			dp.debugLog("Starting ephemeral container", containerSpec.Image, "with command", containerSpec.Command, "via Container Runtime")
		}

		// Even ephemeral MCP servers need StartContainer for stdio streams
		// The difference is in the Persistent and RemoveAfterRun flags
		handle, err := dp.containerRuntime.StartContainer(ctx, containerSpec)
		if err != nil {
			dp.debugLog("Failed to start ephemeral container for:", spec.Name, "error:", err)
			return nil, cleanup, fmt.Errorf("failed to start ephemeral container: %w", err)
		}

		// Create MCP client using Container Runtime handles
		client = mcpclient.NewStdioHandleClient(serverConfig.Name, handle.Stdin, handle.Stdout)

		// For ephemeral servers, cleanup immediately when client closes
		originalCleanup := cleanup
		cleanup = func() {
			// Stop container immediately for ephemeral behavior
			ctx := context.Background() // Use background context for cleanup
			if stopErr := dp.containerRuntime.StopContainer(ctx, handle); stopErr != nil {
				dp.debugLog("Error stopping ephemeral container for:", spec.Name, "error:", stopErr)
			}
			// Clean up proxies if they were set up
			if proxyCleanup != nil {
				if proxyErr := proxyCleanup(ctx); proxyErr != nil {
					dp.debugLog("Error cleaning up proxies for:", spec.Name, "error:", proxyErr)
				}
			}
			// Then run original cleanup
			originalCleanup()
		}
	}

	// Initialize the client (copied from original logic)
	initParams := &mcp.InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: &mcp.Implementation{
			Name:    "docker",
			Version: "1.0.0",
		},
	}

	// Initialize the client
	dp.debugLog("Initializing MCP client for:", spec.Name)
	if err := client.Initialize(ctx, initParams, dp.verbose, ss, server, dp.capRefresher); err != nil {
		dp.debugLog("Failed to initialize MCP client for:", spec.Name, "error:", err)
		return nil, cleanup, err
	}

	dp.debugLog("Successfully provisioned server:", spec.Name)
	return client, cleanup, nil
}

// specToServerConfig creates a minimal catalog.ServerConfig for compatibility
// Note: This method doesn't include secrets since they're resolved just-in-time
func (dp *DockerProvisionerImpl) specToServerConfig(spec ProvisionerSpec) *catalog.ServerConfig {
	// Convert environment map to catalog.Env slice
	var env []catalog.Env
	for name, value := range spec.Environment {
		env = append(env, catalog.Env{Name: name, Value: value})
	}

	return &catalog.ServerConfig{
		Name: spec.Name,
		Spec: catalog.Server{
			Image:          spec.Image,
			Command:        spec.Command,
			Env:            env,
			Secrets:        []catalog.Secret{}, // Empty - secrets resolved via ConfigResolver
			Volumes:        spec.Volumes,
			DisableNetwork: spec.DisableNetwork,
			LongLived:      spec.LongLived,
		},
		Config:  map[string]any{},    // Empty for now
		Secrets: map[string]string{}, // Empty - secrets resolved via ConfigResolver
	}
}

// buildContainerSpec converts a ProvisionerSpec to a ContainerSpec for Container Runtime
func (dp *DockerProvisionerImpl) buildContainerSpec(spec ProvisionerSpec) runtime.ContainerSpec {
	// Start with non-sensitive environment variables from spec
	env := make(map[string]string)
	for name, value := range spec.Environment {
		env[name] = value
	}

	// Just-in-time resolution of secrets via ConfigResolver
	if dp.configResolver != nil {
		secrets := dp.configResolver.ResolveSecrets(spec.Name)
		for envName, secretValue := range secrets {
			env[envName] = secretValue
		}

		// Just-in-time resolution of sensitive environment variables
		resolvedEnv := dp.configResolver.ResolveEnvironment(spec.Name)
		for envName, envValue := range resolvedEnv {
			env[envName] = envValue // Overrides non-sensitive env if needed
		}
	}

	// Just-in-time command resolution (for template substitution)
	command := spec.Command
	if dp.configResolver != nil {
		command = dp.configResolver.ResolveCommand(spec.Name)
	}

	return runtime.ContainerSpec{
		Name:    spec.Name,
		Image:   spec.Image,
		Command: command,

		// Runtime configuration
		Networks: dp.networks,
		Volumes:  spec.Volumes,
		Env:      env,

		// Resource limits
		CPUs:   dp.cpus,
		Memory: dp.memory,

		// Container configuration for MCP servers (ephemeral vs persistent determined by caller)
		Persistent:    spec.LongLived, // Use spec setting - long-lived servers are persistent
		AttachStdio:   true,           // Need stdin/stdout for MCP protocol
		KeepStdinOpen: true,           // Keep stdin open for ongoing MCP communication
		RestartPolicy: "no",           // No auto-restart for MCP servers

		// Security and behavior
		RemoveAfterRun: !spec.LongLived, // Remove ephemeral containers automatically (like --rm)
		Interactive:    true,            // Required for MCP communication
		Init:           true,            // Use init process
		Privileged:     false,           // Never privileged for MCP servers

		// Network isolation
		DisableNetwork: spec.DisableNetwork,
	}
}

// ApplyToolProviders applies secret and config provider settings to a POCI tool container spec
// For Docker provisioner, this mainly uses ConfigResolver for environment variable resolution
func (dp *DockerProvisionerImpl) ApplyToolProviders(spec *runtime.ContainerSpec, toolName string) {
	dp.debugLog("ApplyToolProviders called for tool:", toolName)

	// Docker provisioner uses ConfigResolver for environment variable resolution
	if dp.configResolver != nil {
		// Resolve environment variables for the tool (if any defined in catalog)
		resolvedEnv := dp.configResolver.ResolveEnvironment(toolName)
		for envName, envValue := range resolvedEnv {
			spec.Env[envName] = envValue
		}
		dp.debugLog("Applied config resolver for tool:", toolName, "env vars:", len(resolvedEnv))

		// Note: Docker provisioner doesn't use complex secret providers like Kubernetes
		// Secrets are typically handled through Docker's existing credential mechanisms
		// or passed as environment variables through the ConfigResolver
	} else {
		dp.debugLog("No config resolver available for tool:", toolName)
	}
}

// applyProxyConfig applies proxy network configuration to a container spec
func (dp *DockerProvisionerImpl) applyProxyConfig(containerSpec *runtime.ContainerSpec, targetConfig proxies.TargetConfig) {
	// Apply proxy network - overrides default networks
	if targetConfig.NetworkName != "" {
		containerSpec.Networks = []string{targetConfig.NetworkName}
	}

	// Apply container links for hostname resolution
	containerSpec.Links = targetConfig.Links

	// Apply DNS configuration
	containerSpec.DNS = targetConfig.DNS

	// Merge proxy environment variables
	for _, envVar := range targetConfig.Env {
		// Parse "KEY=VALUE" format
		if idx := strings.Index(envVar, "="); idx != -1 {
			key := envVar[:idx]
			value := envVar[idx+1:]
			if containerSpec.Env == nil {
				containerSpec.Env = make(map[string]string)
			}
			containerSpec.Env[key] = value
		}
	}
}

// debugLog prints debug messages to stderr only when verbose mode is enabled
func (dp *DockerProvisionerImpl) debugLog(args ...any) {
	if dp.verbose {
		prefixedArgs := append([]any{"[DockerProvisioner]"}, args...)
		fmt.Fprintln(os.Stderr, prefixedArgs...)
	}
}

// Shutdown performs cleanup of all resources managed by this provisioner
func (dp *DockerProvisionerImpl) Shutdown(ctx context.Context) error {
	dp.debugLog("Shutting down Docker provisioner")

	// Let the container runtime perform its own shutdown
	if err := dp.containerRuntime.Shutdown(ctx); err != nil {
		dp.debugLog("Error during container runtime shutdown:", err)
		// Don't fail shutdown for cleanup errors, just log them
	}

	dp.debugLog("Docker provisioner shutdown completed")
	return nil
}
