package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/provisioners"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/health"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/interceptors"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/telemetry"
)

type ServerSessionCache struct {
	Roots []*mcp.Root
}

// type SubsAction int

// const (
// subscribe   SubsAction = 0
// unsubscribe SubsAction = 1
// )

// type SubsMessage struct {
// uri    string
// action SubsAction
// ss     *mcp.ServerSession
// }

type Gateway struct {
	Options
	docker       docker.Client
	configurator Configurator
	clientPool   *clientPool
	mcpServer    *mcp.Server
	health       health.State
	// subsChannel  chan SubsMessage

	sessionCacheMu sync.RWMutex
	sessionCache   map[*mcp.ServerSession]*ServerSessionCache

	// Track registered capabilities for cleanup during reload
	registeredToolNames            []string
	registeredPromptNames          []string
	registeredResourceURIs         []string
	registeredResourceTemplateURIs []string

	// Gateway-owned resources for proper lifecycle management
	containerRuntime runtime.ContainerRuntime
	provisionerMap   map[provisioners.ProvisionerType]provisioners.Provisioner
}

func NewGateway(config Config, docker docker.Client) *Gateway {
	gateway := &Gateway{
		Options: config.Options,
		docker:  docker,
		configurator: &FileBasedConfiguration{
			ServerNames:  config.ServerNames,
			CatalogPath:  config.CatalogPath,
			RegistryPath: config.RegistryPath,
			ConfigPath:   config.ConfigPath,
			SecretsPath:  config.SecretsPath,
			ToolsPath:    config.ToolsPath,
			Watch:        config.Watch,
			Central:      config.Central,
			docker:       docker,
		},
		sessionCache: make(map[*mcp.ServerSession]*ServerSessionCache),
	}
	// Generate session ID for resource tracking and cleanup
	if config.SessionID == "" {
		sessionID, err := GenerateSessionID()
		if err != nil {
			log("Warning: Failed to generate session ID, using default:", err)
			sessionID = "mcp-gateway-unknown"
		}
		config.SessionID = sessionID
	}
	log("Gateway session ID:", config.SessionID)

	// Parse provisioner type from config
	provisionerType, err := provisioners.ParseProvisionerType(config.Provisioner)
	if err != nil {
		// This should be caught by command line validation, but handle gracefully
		log("Warning: Invalid provisioner type, defaulting to docker:", err)
		provisionerType = provisioners.DockerProvisioner
	}

	// Gateway creates and manages Container Runtime at its level based on provisioner choice
	// This allows the gateway to coordinate with its own runtime environment
	// IMPORTANT: Container runtime for POCI tools should match the provisioner selection
	var containerRuntime runtime.ContainerRuntime

	if provisionerType == provisioners.KubernetesProvisioner {
		// Use Kubernetes container runtime for POCI tools when Kubernetes provisioner is selected
		kubernetesRuntime, err := runtime.NewKubernetesContainerRuntime(runtime.KubernetesContainerRuntimeConfig{
			ContainerRuntimeConfig: runtime.ContainerRuntimeConfig{
				Verbose: config.Verbose,
			},
			Namespace:   config.Namespace,   // From --namespace flag
			Kubeconfig:  config.Kubeconfig,  // From --kubeconfig flag
			KubeContext: config.KubeContext, // From --kube-context flag
		})
		if err != nil {
			log("ERROR: Failed to create Kubernetes container runtime (--provisioner kubernetes was specified):", err)
			log("FATAL: Cannot proceed without Kubernetes container runtime")
			os.Exit(1)
		}
		containerRuntime = kubernetesRuntime
		log("Using Kubernetes container runtime for POCI tools")
	} else {
		// Use Docker container runtime for POCI tools when Docker provisioner is selected
		containerRuntime = runtime.NewDockerContainerRuntime(runtime.DockerContainerRuntimeConfig{
			ContainerRuntimeConfig: runtime.ContainerRuntimeConfig{
				Verbose: config.Verbose,
			},
			PullPolicy:    "never",              // Match existing behavior
			DockerContext: config.DockerContext, // Pass through Docker context from CLI
		})
		log("Using Docker container runtime for POCI tools")
	}

	// We need to create a temporary default provisioner value first
	tempDefaultProvisioner := provisionerType

	// Create clientPool first so we can pass it to provisioners
	clientPool := newClientPool(ClientPoolConfig{
		Options:            config.Options,
		Docker:             docker,
		ContainerRuntime:   containerRuntime,
		Provisioners:       nil, // Will be set after provisioners are created
		DefaultProvisioner: tempDefaultProvisioner,
		Gateway:            gateway,
	})

	// Gateway creates and manages Provisioners at its level
	// Networks will be set when gateway detects its runtime environment
	provisionerMap := make(map[provisioners.ProvisionerType]provisioners.Provisioner)

	// Always create Docker provisioner (fully implemented)
	dockerProvisioner := provisioners.NewDockerProvisioner(provisioners.DockerProvisionerConfig{
		Docker:           docker,
		ContainerRuntime: containerRuntime,
		ProxyRunner:      clientPool, // Pass clientPool as ProxyRunner
		Networks:         []string{}, // Will be updated when Gateway detects networks
		Verbose:          config.Verbose,
		Static:           config.Static,
		BlockNetwork:     config.BlockNetwork,
		Cpus:             config.Cpus,
		Memory:           config.Memory,
		LongLived:        false, // This will be overridden per-server
		CapRefresher:     gateway,
	})
	provisionerMap[provisioners.DockerProvisioner] = dockerProvisioner

	// Create Kubernetes provisioner if needed (reuse the same runtime instance for consistency)
	if provisionerType == provisioners.KubernetesProvisioner {
		// Reuse the same Kubernetes runtime instance that's being used for POCI tools
		if kubernetesRuntime, ok := containerRuntime.(*runtime.KubernetesContainerRuntime); ok {
			kubernetesProvisioner := provisioners.NewKubernetesProvisioner(provisioners.KubernetesProvisionerConfig{
				ContainerRuntime: kubernetesRuntime,
				Namespace:        config.Namespace,
				SecretName:       config.SecretName,     // From --secret-name flag
				SecretProvider:   config.SecretProvider, // From --secret-provider flag
				ConfigProvider:   config.ConfigProvider, // From --config-provider flag
				ConfigName:       config.ConfigName,     // From --config-name flag
				Verbose:          config.Verbose,
				SessionID:        config.SessionID,
				// ConfigResolver and SecretManager will be injected later in reloadConfiguration
				CapRefresher: gateway,
			})
			provisionerMap[provisioners.KubernetesProvisioner] = kubernetesProvisioner
			log("Created Kubernetes provisioner using shared container runtime")
		} else {
			log("ERROR: Expected Kubernetes container runtime but got:", containerRuntime.GetName())
			// This should never happen given the logic above, but fail hard if it does
			log("FATAL: Container runtime and provisioner type mismatch")
			os.Exit(1)
		}
	}

	// Set default provisioner based on configuration
	if _, exists := provisionerMap[provisionerType]; !exists {
		log("ERROR: Requested provisioner", provisionerType.String(), "is not available")
		log("FATAL: Provisioner creation failed - no fallback allowed for explicit provisioner selection")
		os.Exit(1)
	}

	// Update clientPool with the created provisioners
	clientPool.provisionerMap = provisionerMap

	// Store gateway-owned resources for lifecycle management
	gateway.containerRuntime = containerRuntime
	gateway.provisionerMap = provisionerMap
	gateway.clientPool = clientPool

	return gateway
}

func (g *Gateway) Run(ctx context.Context) error {
	// Initialize telemetry
	telemetry.Init()

	// Record gateway start
	transportMode := "stdio"
	if g.Port != 0 {
		transportMode = "sse"
	}
	telemetry.RecordGatewayStart(ctx, transportMode)

	// Start periodic metric export for long-running gateway
	// This is critical because Docker CLI's ManualReader only exports on shutdown
	// which is inappropriate for gateways that can run for hours, days, or weeks
	// ALL gateway run commands are long-lived regardless of transport (stdio, sse, streaming)
	// Even stdio mode runs as long as the client (e.g., Claude Code) is connected
	if !g.DryRun {
		go g.periodicMetricExport(ctx)
	}

	// Perform shutdown cleanup of all gateway-owned provisioners and runtimes
	defer func() {
		// Use background context with timeout for cleanup to avoid cancellation issues during shutdown
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCleanup()

		// Shutdown all provisioners
		for provisionerType, provisioner := range g.provisionerMap {
			log("Shutting down provisioner:", provisionerType.String())
			if err := provisioner.Shutdown(cleanupCtx); err != nil {
				log("Warning: Error shutting down provisioner", provisionerType.String(), ":", err)
			}
		}

		// Shutdown container runtime
		if g.containerRuntime != nil {
			log("Shutting down container runtime:", g.containerRuntime.GetName())
			if err := g.containerRuntime.Shutdown(cleanupCtx); err != nil {
				log("Warning: Error shutting down container runtime:", err)
			}
		}
	}()
	defer g.clientPool.Close()
	defer func() {
		// Clean up all session cache entries
		g.sessionCacheMu.Lock()
		g.sessionCache = make(map[*mcp.ServerSession]*ServerSessionCache)
		g.sessionCacheMu.Unlock()
	}()

	start := time.Now()

	// Listen as early as possible to not lose client connections.
	var ln net.Listener
	if port := g.Port; port != 0 {
		var (
			lc  net.ListenConfig
			err error
		)
		ln, err = lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return err
		}
	}

	// Read the configuration.
	configuration, configurationUpdates, stopConfigWatcher, err := g.configurator.Read(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = stopConfigWatcher() }()

	// Parse interceptors
	var parsedInterceptors []interceptors.Interceptor
	if len(g.Interceptors) > 0 {
		var err error
		parsedInterceptors, err = interceptors.Parse(g.Interceptors)
		if err != nil {
			return fmt.Errorf("parsing interceptors: %w", err)
		}
		log("- Interceptors enabled:", strings.Join(g.Interceptors, ", "))
	}

	g.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "Docker AI MCP Gateway",
		Version: "2.0.1",
	}, &mcp.ServerOptions{
		SubscribeHandler: func(_ context.Context, req *mcp.SubscribeRequest) error {
			log("- Client subscribed to URI:", req.Params.URI)
			// The MCP SDK doesn't provide ServerSession in SubscribeHandler because it already
			// keeps track of the mapping between ServerSession and subscribed resources in the Server
			// g.subsChannel <- SubsMessage{uri: req.Params.URI, action: subscribe , ss: ss}
			return nil
		},
		UnsubscribeHandler: func(_ context.Context, req *mcp.UnsubscribeRequest) error {
			log("- Client unsubscribed from URI:", req.Params.URI)
			// The MCP SDK doesn't provide ServerSession in UnsubscribeHandler because it already
			// keeps track of the mapping ServerSession and subscribed resources in the Server
			// g.subsChannel <- SubsMessage{uri: req.Params.URI, action: unsubscribe , ss: ss}
			return nil
		},
		RootsListChangedHandler: func(ctx context.Context, req *mcp.RootsListChangedRequest) {
			log("- Client roots list changed")
			// We can't get the ServerSession from the request anymore, so we'll need to handle this differently
			_, _ = req.Session.ListRoots(ctx, &mcp.ListRootsParams{})
		},
		CompletionHandler: nil,
		InitializedHandler: func(_ context.Context, _ *mcp.InitializedRequest) {
			log("- Client initialized")
		},
		HasPrompts:   true,
		HasResources: true,
		HasTools:     true,
	})

	// Add interceptor middleware to the server (includes telemetry)
	middlewares := interceptors.Callbacks(g.LogCalls, g.BlockSecrets, g.OAuthInterceptorEnabled, parsedInterceptors)
	if len(middlewares) > 0 {
		g.mcpServer.AddReceivingMiddleware(middlewares...)
	}

	if err := g.reloadConfiguration(ctx, configuration, nil, nil); err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Central mode.
	if g.Central {
		log("> Initialized (in central mode) in", time.Since(start))
		if g.DryRun {
			log("Dry run mode enabled, not starting the server.")
			return nil
		}

		log("> Start streaming server on port", g.Port)
		return g.startCentralStreamingServer(ctx, ln, configuration)
	}

	// Which docker images are used?
	// Pull them and verify them if possible.
	if !g.Static {
		if err := g.pullAndVerify(ctx, configuration); err != nil {
			return err
		}

		// When running in a container, find on which network we are running.
		if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
			networks, err := g.guessNetworks(ctx)
			if err != nil {
				return fmt.Errorf("guessing network: %w", err)
			}
			g.clientPool.SetNetworks(networks)
		}
	}

	// Optionally watch for configuration updates.
	if configurationUpdates != nil {
		log("- Watching for configuration updates...")
		go func() {
			for {
				select {
				case <-ctx.Done():
					log("> Stop watching for updates")
					return
				case configuration := <-configurationUpdates:
					log("> Configuration updated, reloading...")

					if err := g.pullAndVerify(ctx, configuration); err != nil {
						logf("> Unable to pull and verify images: %s", err)
						continue
					}

					if err := g.reloadConfiguration(ctx, configuration, nil, nil); err != nil {
						logf("> Unable to list capabilities: %s", err)
						continue
					}
				}
			}
		}()
	}

	log("> Initialized in", time.Since(start))
	if g.DryRun {
		log("Dry run mode enabled, not starting the server.")
		return nil
	}

	// Start the server
	switch strings.ToLower(g.Transport) {
	case "stdio":
		log("> Start stdio server")
		return g.startStdioServer(ctx, os.Stdin, os.Stdout)

	case "sse":
		log("> Start sse server on port", g.Port)
		return g.startSseServer(ctx, ln)

	case "http", "streamable", "streaming", "streamable-http":
		log("> Start streaming server on port", g.Port)
		return g.startStreamingServer(ctx, ln)

	default:
		return fmt.Errorf("unknown transport %q, expected 'stdio', 'sse' or 'streaming", g.Transport)
	}
}

func (g *Gateway) reloadConfiguration(ctx context.Context, configuration Configuration, serverNames []string, clientConfig *clientConfig) error {
	// Create and inject ConfigResolver for Docker virtual remote provisioner
	// This gives the provisioner just-in-time access to secrets and config
	serverConfigs := make(map[string]*catalog.ServerConfig)

	// Which servers are enabled in the registry.yaml?
	if len(serverNames) == 0 {
		serverNames = configuration.ServerNames()
	}
	if len(serverNames) == 0 {
		log("- No server is enabled")
	} else {
		log("- Those servers are enabled:", strings.Join(serverNames, ", "))
	}

	// Build server config map for ConfigResolver
	for _, serverName := range serverNames {
		if serverConfig, _, found := configuration.Find(serverName); found && serverConfig != nil {
			serverConfigs[serverName] = serverConfig
		}
	}

	// Initialize only the selected provisioner with configuration and dependencies
	configResolver := provisioners.NewGatewayConfigResolver(serverConfigs)

	// Get the selected provisioner type from gateway config
	selectedProvisionerType, err := provisioners.ParseProvisionerType(g.Provisioner)
	if err != nil {
		selectedProvisionerType = provisioners.DockerProvisioner // Default fallback
	}

	// Initialize only the selected provisioner
	selectedProvisioner := g.provisionerMap[selectedProvisionerType]
	if g.Verbose {
		log("[" + selectedProvisionerType.String() + "Provisioner] Initializing provisioner")
	}
	if err := selectedProvisioner.Initialize(ctx, configResolver, serverConfigs); err != nil {
		log("Warning: Failed to initialize", selectedProvisionerType.String(), "provisioner:", err)
		// Continue - this is not fatal
	}

	// Deprecated: Also inject into client pool for backward compatibility
	if err := g.clientPool.SetConfigResolver(configResolver); err != nil {
		log("Warning: Failed to inject ConfigResolver into client pool:", err)
		// Continue - this is not fatal, just means no just-in-time config resolution
	}

	// List all the available tools.
	startList := time.Now()
	log("- Listing MCP tools...")
	capabilities, err := g.listCapabilities(ctx, configuration, serverNames, clientConfig)
	if err != nil {
		return fmt.Errorf("listing resources: %w", err)
	}
	log(">", len(capabilities.Tools), "tools listed in", time.Since(startList))

	// Update capabilities
	// Clear existing capabilities and register new ones
	// Note: The new SDK doesn't have bulk set methods, so we register individually

	// Clear all existing capabilities by tracking them in the Gateway struct
	if g.registeredToolNames != nil {
		g.mcpServer.RemoveTools(g.registeredToolNames...)
	}
	if g.registeredPromptNames != nil {
		g.mcpServer.RemovePrompts(g.registeredPromptNames...)
	}
	if g.registeredResourceURIs != nil {
		g.mcpServer.RemoveResources(g.registeredResourceURIs...)
	}
	if g.registeredResourceTemplateURIs != nil {
		g.mcpServer.RemoveResourceTemplates(g.registeredResourceTemplateURIs...)
	}

	// Reset tracking slices
	g.registeredToolNames = nil
	g.registeredPromptNames = nil
	g.registeredResourceURIs = nil
	g.registeredResourceTemplateURIs = nil

	// Add new capabilities and track them
	for _, tool := range capabilities.Tools {
		g.mcpServer.AddTool(tool.Tool, tool.Handler)
		g.registeredToolNames = append(g.registeredToolNames, tool.Tool.Name)
	}

	for _, prompt := range capabilities.Prompts {
		g.mcpServer.AddPrompt(prompt.Prompt, prompt.Handler)
		g.registeredPromptNames = append(g.registeredPromptNames, prompt.Prompt.Name)
	}

	for _, resource := range capabilities.Resources {
		g.mcpServer.AddResource(resource.Resource, resource.Handler)
		g.registeredResourceURIs = append(g.registeredResourceURIs, resource.Resource.URI)
	}

	// Resource templates are handled as regular resources in the new SDK
	for _, template := range capabilities.ResourceTemplates {
		// Convert ResourceTemplate to Resource
		resource := &mcp.ResourceTemplate{
			URITemplate: template.ResourceTemplate.URITemplate,
			Name:        template.ResourceTemplate.Name,
			Description: template.ResourceTemplate.Description,
			MIMEType:    template.ResourceTemplate.MIMEType,
		}
		g.mcpServer.AddResourceTemplate(resource, template.Handler)
		g.registeredResourceTemplateURIs = append(g.registeredResourceTemplateURIs, resource.URITemplate)
	}

	g.health.SetHealthy()

	return nil
}

// RefreshCapabilities implements the CapabilityRefresher interface
// This method updates the server's capabilities by reloading the configuration
func (g *Gateway) RefreshCapabilities(ctx context.Context, server *mcp.Server, serverSession *mcp.ServerSession) error {
	// Get current configuration
	configuration, _, _, err := g.configurator.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	// Create a clientConfig to reuse the existing session for the server that triggered the notification
	clientConfig := &clientConfig{
		serverSession: serverSession,
		server:        server,
	}

	// Refresh all servers, but the clientPool will reuse the existing session for the one that matches
	serverNames := configuration.ServerNames()
	log("- RefreshCapabilities called for session, refreshing servers:", strings.Join(serverNames, ", "))

	err = g.reloadConfiguration(ctx, configuration, serverNames, clientConfig)
	if err != nil {
		log("! Failed to refresh capabilities:", err)
	} else {
		log("- RefreshCapabilities completed successfully")
	}
	return err
}

// GetSessionCache returns the cached information for a server session
func (g *Gateway) GetSessionCache(ss *mcp.ServerSession) *ServerSessionCache {
	g.sessionCacheMu.RLock()
	defer g.sessionCacheMu.RUnlock()
	return g.sessionCache[ss]
}

// RemoveSessionCache removes the cached information for a server session
func (g *Gateway) RemoveSessionCache(ss *mcp.ServerSession) {
	g.sessionCacheMu.Lock()
	defer g.sessionCacheMu.Unlock()
	delete(g.sessionCache, ss)
}

// ListRoots checks if client supports Roots, gets them, and caches the result
func (g *Gateway) ListRoots(ctx context.Context, ss *mcp.ServerSession) {
	// Check if client supports Roots and get them if available
	rootsResult, err := ss.ListRoots(ctx, nil)

	g.sessionCacheMu.Lock()
	defer g.sessionCacheMu.Unlock()

	// Get existing cache or create new one
	cache, exists := g.sessionCache[ss]
	if !exists {
		cache = &ServerSessionCache{}
		g.sessionCache[ss] = cache
	}

	if err != nil {
		log("- Client does not support roots or error listing roots:", err)
		cache.Roots = nil
	} else {
		log("- Client supports roots, found", len(rootsResult.Roots), "roots")
		for _, root := range rootsResult.Roots {
			log("  - Root:", root.URI)
		}
		cache.Roots = rootsResult.Roots
	}
	g.clientPool.UpdateRoots(ss, cache.Roots)
}

// periodicMetricExport periodically exports metrics for long-running gateways
// This addresses the critical issue where Docker CLI's ManualReader only exports on shutdown
func (g *Gateway) periodicMetricExport(ctx context.Context) {
	// Get interval from environment or use default
	intervalStr := os.Getenv("DOCKER_MCP_METRICS_INTERVAL")
	interval := 30 * time.Second
	if intervalStr != "" {
		if parsed, err := time.ParseDuration(intervalStr); err == nil {
			interval = parsed
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Get the meter provider to force flush metrics
	meterProvider := otel.GetMeterProvider()

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Starting periodic metric export every %v\n", interval)
	}

	for {
		select {
		case <-ctx.Done():
			if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Stopping periodic metric export\n")
			}
			return
		case <-ticker.C:
			// Force metric export
			if mp, ok := meterProvider.(interface{ ForceFlush(context.Context) error }); ok {
				flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if err := mp.ForceFlush(flushCtx); err != nil {
					if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Periodic flush error: %v\n", err)
					}
				} else {
					if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Periodic metric flush successful\n")
					}
				}
				cancel()
			} else if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] WARNING: MeterProvider does not support ForceFlush\n")
			}
		}
	}
}
