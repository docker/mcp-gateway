package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"

	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/health"
	"github.com/docker/mcp-gateway/pkg/interceptors"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth"
	"github.com/docker/mcp-gateway/pkg/telemetry"
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

// ServerCapabilities tracks the capabilities registered for a specific server
type ServerCapabilities struct {
	ToolNames            []string
	PromptNames          []string
	ResourceURIs         []string
	ResourceTemplateURIs []string
}

type Gateway struct {
	Options
	docker         docker.Client
	configurator   Configurator
	configuration  Configuration
	clientPool     *clientPool
	mcpServer      *mcp.Server
	health         health.State
	oauthProviders map[string]*oauth.Provider
	providersMu    sync.RWMutex
	// subsChannel  chan SubsMessage

	sessionCacheMu sync.RWMutex
	sessionCache   map[*mcp.ServerSession]*ServerSessionCache

	// Track registered capabilities per server for proper reload handling
	capabilitiesMu              sync.RWMutex
	serverCapabilities          map[string]*ServerCapabilities
	serverAvailableCapabilities map[string]*Capabilities

	// Track all tool registrations for mcp-exec
	toolRegistrations map[string]ToolRegistration

	// authToken stores the authentication token for SSE/streaming modes
	authToken string
	// authTokenWasGenerated indicates whether the token was auto-generated or from environment
	authTokenWasGenerated bool
}

func NewGateway(config Config, docker docker.Client) *Gateway {
	var configurator Configurator
	if config.WorkingSet != "" {
		configurator = &WorkingSetConfiguration{
			WorkingSet: config.WorkingSet,
		}
	} else {
		// Prepend session-specific paths if SessionName is set
		registryPath := config.RegistryPath
		configPath := config.ConfigPath
		toolsPath := config.ToolsPath

		if config.SessionName != "" {
			// Prepend session-specific paths to load session configs first
			sessionRegistry := fmt.Sprintf("%s/registry.yaml", config.SessionName)
			sessionConfig := fmt.Sprintf("%s/config.yaml", config.SessionName)
			sessionTools := fmt.Sprintf("%s/tools.yaml", config.SessionName)

			registryPath = append([]string{sessionRegistry}, registryPath...)
			configPath = append([]string{sessionConfig}, configPath...)
			toolsPath = append([]string{sessionTools}, toolsPath...)
		}

		configurator = &FileBasedConfiguration{
			ServerNames:        config.ServerNames,
			CatalogPath:        config.CatalogPath,
			RegistryPath:       registryPath,
			ConfigPath:         configPath,
			SecretsPath:        config.SecretsPath,
			ToolsPath:          toolsPath,
			OciRef:             config.OciRef,
			MCPRegistryServers: config.MCPRegistryServers,
			Watch:              config.Watch,
			McpOAuthDcrEnabled: config.McpOAuthDcrEnabled,
			sessionName:        config.SessionName,
			docker:             docker,
		}
	}

	g := &Gateway{
		Options:                     config.Options,
		docker:                      docker,
		oauthProviders:              make(map[string]*oauth.Provider),
		configurator:                configurator,
		sessionCache:                make(map[*mcp.ServerSession]*ServerSessionCache),
		serverCapabilities:          make(map[string]*ServerCapabilities),
		serverAvailableCapabilities: make(map[string]*Capabilities),
		toolRegistrations:           make(map[string]ToolRegistration),
	}
	g.clientPool = newClientPool(config.Options, docker, g)

	return g
}

func (g *Gateway) Run(ctx context.Context) error {
	// Initialize telemetry
	telemetry.Init()

	// Set up log file redirection if specified
	if g.LogFilePath != "" {
		logFile, err := os.OpenFile(g.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", g.LogFilePath, err)
		}
		defer logFile.Close()

		// Create a multi-writer that writes to both stderr and the log file
		multiWriter := io.MultiWriter(os.Stderr, logFile)
		log.SetLogWriter(multiWriter)
	}

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
	g.configuration = configuration
	if err != nil {
		return err
	}
	defer func() { _ = stopConfigWatcher() }()

	// Set the session name in the configuration for persistence if specified via --session flag
	if fbc, ok := g.configurator.(*FileBasedConfiguration); ok {
		if fbc.sessionName != "" {
			g.configuration.SessionName = fbc.sessionName
		}
	}

	// Parse interceptors
	var parsedInterceptors []interceptors.Interceptor
	if len(g.Interceptors) > 0 {
		var err error
		parsedInterceptors, err = interceptors.Parse(g.Interceptors)
		if err != nil {
			return fmt.Errorf("parsing interceptors: %w", err)
		}
		log.Log("- Interceptors enabled:", strings.Join(g.Interceptors, ", "))
	}

	g.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "Docker AI MCP Gateway",
		Version: "2.0.1",
	}, &mcp.ServerOptions{
		SubscribeHandler: func(_ context.Context, req *mcp.SubscribeRequest) error {
			log.Log("- Client subscribed to URI:", req.Params.URI)
			// The MCP SDK doesn't provide ServerSession in SubscribeHandler because it already
			// keeps track of the mapping between ServerSession and subscribed resources in the Server
			// g.subsChannel <- SubsMessage{uri: req.Params.URI, action: subscribe , ss: ss}
			return nil
		},
		UnsubscribeHandler: func(_ context.Context, req *mcp.UnsubscribeRequest) error {
			log.Log("- Client unsubscribed from URI:", req.Params.URI)
			// The MCP SDK doesn't provide ServerSession in UnsubscribeHandler because it already
			// keeps track of the mapping ServerSession and subscribed resources in the Server
			// g.subsChannel <- SubsMessage{uri: req.Params.URI, action: unsubscribe , ss: ss}
			return nil
		},
		RootsListChangedHandler: func(ctx context.Context, req *mcp.RootsListChangedRequest) {
			log.Log("- Client roots list changed")
			// We can't get the ServerSession from the request anymore, so we'll need to handle this differently
			_, _ = req.Session.ListRoots(ctx, &mcp.ListRootsParams{})
		},
		CompletionHandler: nil,
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			clientInfo := req.Session.InitializeParams().ClientInfo
			log.Log(fmt.Sprintf("- Client initialized %s@%s %s", clientInfo.Name, clientInfo.Version, clientInfo.Title))
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

	if err := g.reloadConfiguration(ctx, configuration, nil, nil); err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// When running in Container mode, disable OAuth notification monitoring and authentication
	inContainer := os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1"

	if g.McpOAuthDcrEnabled && !inContainer {
		// Start OAuth notification monitor to receive OAuth related events from Docker Desktop
		log.Log("- Starting OAuth notification monitor")
		monitor := oauth.NewNotificationMonitor()
		monitor.OnOAuthEvent = func(event oauth.Event) {
			// Route event to specific provider
			g.routeEventToProvider(event)
		}
		monitor.Start(ctx)

		// Start OAuth provider for each OAuth server
		// Each provider runs in its own goroutine with dynamic timing based on token expiry
		log.Log("- Starting OAuth provider loops...")
		for _, serverName := range configuration.ServerNames() {
			serverConfig, _, found := configuration.Find(serverName)
			if !found || serverConfig == nil || !serverConfig.Spec.IsRemoteOAuthServer() {
				continue
			}

			g.startProvider(ctx, serverName)
		}
	}

	// Optionally watch for configuration updates.
	if configurationUpdates != nil {
		log.Log("- Watching for configuration updates...")
		go func() {
			for {
				select {
				case <-ctx.Done():
					log.Log("> Stop watching for updates")
					return
				case configuration := <-configurationUpdates:
					log.Log("> Configuration updated, reloading...")

					if err := g.pullAndVerify(ctx, configuration); err != nil {
						log.Logf("> Unable to pull and verify images: %s", err)
						continue
					}

					if err := g.reloadConfiguration(ctx, configuration, nil, nil); err != nil {
						log.Logf("> Unable to list capabilities: %s", err)
						g.configuration = configuration
						continue
					}
				}
			}
		}()
	}

	log.Log("> Initialized in", time.Since(start))
	if g.DryRun {
		log.Log("Dry run mode enabled, not starting the server.")
		return nil
	}

	// Initialize authentication token for SSE and streaming modes
	// Skip authentication when running in container (DOCKER_MCP_IN_CONTAINER=1)
	transport := strings.ToLower(g.Transport)
	if (transport == "sse" || transport == "http" || transport == "streamable" || transport == "streaming" || transport == "streamable-http") && !inContainer {
		token, wasGenerated, err := getOrGenerateAuthToken()
		if err != nil {
			return fmt.Errorf("failed to initialize auth token: %w", err)
		}
		g.authToken = token
		g.authTokenWasGenerated = wasGenerated
	}

	// Start the server
	switch transport {
	case "stdio":
		log.Log("> Start stdio server")
		return g.startStdioServer(ctx, os.Stdin, os.Stdout)

	case "sse":
		log.Log("> Start sse server on port", g.Port)
		endpoint := "/sse"
		url := formatGatewayURL(g.Port, endpoint)
		if inContainer {
			log.Logf("> Gateway URL: %s", url)
			log.Logf("> Authentication disabled (running in container)")
		} else if g.authTokenWasGenerated {
			log.Logf("> Gateway URL: %s", url)
			log.Logf("> Use Bearer token: %s", formatBearerToken(g.authToken))
		} else {
			log.Logf("> Gateway URL: %s", url)
			log.Logf("> Use Bearer token from MCP_GATEWAY_AUTH_TOKEN environment variable")
		}
		return g.startSseServer(ctx, ln)

	case "http", "streamable", "streaming", "streamable-http":
		log.Log("> Start streaming server on port", g.Port)
		endpoint := "/mcp"
		url := formatGatewayURL(g.Port, endpoint)
		if inContainer {
			log.Logf("> Gateway URL: %s", url)
			log.Logf("> Authentication disabled (running in container)")
		} else if g.authTokenWasGenerated {
			log.Logf("> Gateway URL: %s", url)
			log.Logf("> Use Bearer token: %s", formatBearerToken(g.authToken))
		} else {
			log.Logf("> Gateway URL: %s", url)
			log.Logf("> Use Bearer token from MCP_GATEWAY_AUTH_TOKEN environment variable")
		}
		return g.startStreamingServer(ctx, ln)

	default:
		return fmt.Errorf("unknown transport %q, expected 'stdio', 'sse' or 'streaming", g.Transport)
	}
}

// RefreshCapabilities implements the CapabilityRefresher interface
// This method updates the server's capabilities by reloading the configuration
func (g *Gateway) RefreshCapabilities(ctx context.Context, server *mcp.Server, serverSession *mcp.ServerSession, serverName string) error {
	// Create a clientConfig to reuse the existing session for the server that triggered the notification
	clientConfig := &clientConfig{
		serverSession: serverSession,
		server:        server,
	}

	log.Log("- RefreshCapabilities called for session, refreshing servers:", serverName)

	oldCaps, err := g.reloadServerCapabilities(ctx, serverName, clientConfig)
	if err != nil {
		log.Log("! Failed to refresh capabilities:", err)
		return err
	}

	// Now update g.mcpServer with the new capabilities
	g.capabilitiesMu.Lock()
	newCaps := g.allCapabilities(serverName)
	err = g.updateServerCapabilities(serverName, oldCaps, newCaps, nil)
	g.capabilitiesMu.Unlock()

	if err != nil {
		log.Log("! Failed to update server capabilities:", err)
		return err
	}

	log.Log("- RefreshCapabilities completed successfully")
	return nil
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
		log.Log("- Client does not support roots or error listing roots:", err)
		cache.Roots = nil
	} else {
		log.Log("- Client supports roots, found", len(rootsResult.Roots), "roots")
		for _, root := range rootsResult.Roots {
			log.Log("  - Root:", root.URI)
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

// OAuth Provider Management Methods

// startProvider creates and starts an OAuth provider goroutine for a server
func (g *Gateway) startProvider(ctx context.Context, serverName string) {
	g.providersMu.Lock()
	defer g.providersMu.Unlock()

	// Check if provider already running
	if _, exists := g.oauthProviders[serverName]; exists {
		return
	}

	// Create reload function for this provider
	reloadFn := func(ctx context.Context, name string) error {
		log.Logf("> Reloading OAuth server: %s", name)

		// Close old client connection with stale token
		g.clientPool.InvalidateOAuthClients(name)

		// Reload server configuration
		oldCaps, err := g.reloadServerCapabilities(ctx, name, nil)
		if err != nil {
			return err
		}

		// Now update g.mcpServer with the new capabilities
		g.capabilitiesMu.Lock()
		newCaps := g.allCapabilities(name)
		err = g.updateServerCapabilities(name, oldCaps, newCaps, nil)
		g.capabilitiesMu.Unlock()

		if err != nil {
			return err
		}

		log.Logf("> OAuth server %s reconnected and tools registered", name)
		return nil
	}

	// Create and start provider
	provider := oauth.NewProvider(serverName, reloadFn)
	g.oauthProviders[serverName] = provider

	// Wrapper goroutine handles cleanup after provider exits
	go func() {
		provider.Run(ctx) // Blocks until provider stops

		// Provider exited - remove from map
		g.providersMu.Lock()
		delete(g.oauthProviders, serverName)
		g.providersMu.Unlock()

		log.Logf("- Removed provider %s from map after exit", serverName)
	}()
}

// stopProvider stops an OAuth provider goroutine for a server
func (g *Gateway) stopProvider(serverName string) {
	g.providersMu.Lock()
	defer g.providersMu.Unlock()

	if provider, exists := g.oauthProviders[serverName]; exists {
		provider.Stop()
		delete(g.oauthProviders, serverName)
	}
}

// routeEventToProvider routes SSE events to the appropriate provider
func (g *Gateway) routeEventToProvider(event oauth.Event) {
	g.providersMu.RLock()
	provider, exists := g.oauthProviders[event.Provider]
	g.providersMu.RUnlock()

	switch event.Type {
	case oauth.EventLoginSuccess:
		// User just authorized - ensure provider exists
		if !exists {
			log.Logf("- Creating provider for %s after login", event.Provider)
			g.startProvider(context.Background(), event.Provider)
		}

		// Always send event to trigger reload (connects server and lists tools)
		// Wait briefly if we just created the provider
		if !exists {
			time.Sleep(100 * time.Millisecond)
		}

		g.providersMu.RLock()
		provider, exists = g.oauthProviders[event.Provider]
		g.providersMu.RUnlock()

		if exists {
			provider.SendEvent(event)
		}

	case oauth.EventTokenRefresh:
		// Token refreshed - route to provider if exists
		if exists {
			provider.SendEvent(event)
		}
		// If doesn't exist, drop (another gateway or disabled server)

	case oauth.EventLogoutSuccess:
		// User logged out - stop provider if exists
		if exists {
			log.Logf("- Stopping provider for %s after logout", event.Provider)
			g.stopProvider(event.Provider)
		}

	default:
		// Other events (login-start, code-received, error) - ignore
	}
}
