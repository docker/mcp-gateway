package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"

	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/health"
	"github.com/docker/mcp-gateway/pkg/interceptors"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

const TokenEventFilename = "token-event.json"

type ServerSessionCache struct {
	Roots []*mcp.Root
}

// TokenEvent represents a token refresh or acquisition event
type TokenEvent struct {
	Provider   string    `json:"provider"`
	Timestamp  time.Time `json:"timestamp"`
	EventType  string    `json:"event_type"` // EventTypeTokenAcquired or EventTypeTokenRefreshed
	ServerName string    `json:"server_name"`
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
	docker        docker.Client
	configurator  Configurator
	configuration Configuration
	clientPool    *clientPool
	mcpServer     *mcp.Server
	health        health.State
	// subsChannel  chan SubsMessage

	sessionCacheMu sync.RWMutex
	sessionCache   map[*mcp.ServerSession]*ServerSessionCache

	// Track registered capabilities per server for proper reload handling
	capabilitiesMu     sync.RWMutex
	serverCapabilities map[string]*ServerCapabilities
}

func NewGateway(config Config, docker docker.Client) *Gateway {
	g := &Gateway{
		Options: config.Options,
		docker:  docker,
		configurator: &FileBasedConfiguration{
			ServerNames:        config.ServerNames,
			CatalogPath:        config.CatalogPath,
			RegistryPath:       config.RegistryPath,
			ConfigPath:         config.ConfigPath,
			SecretsPath:        config.SecretsPath,
			ToolsPath:          config.ToolsPath,
			OciRef:             config.OciRef,
			MCPRegistryServers: config.MCPRegistryServers,
			Watch:              config.Watch,
			Central:            config.Central,
			McpOAuthDcrEnabled: config.McpOAuthDcrEnabled,
			docker:             docker,
		},
		sessionCache:       make(map[*mcp.ServerSession]*ServerSessionCache),
		serverCapabilities: make(map[string]*ServerCapabilities),
	}
	g.clientPool = newClientPool(config.Options, docker, g)
	return g
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
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			clientInfo := req.Session.InitializeParams().ClientInfo
			log(fmt.Sprintf("- Client initialized %s@%s %s", clientInfo.Name, clientInfo.Version, clientInfo.Title))
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
					// First, check and handle any token events
					g.handleTokenEvent(ctx)

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
	// Which servers are enabled in the registry.yaml?
	if len(serverNames) == 0 {
		serverNames = configuration.ServerNames()
	}
	if len(serverNames) == 0 {
		log("- No server is enabled")
	} else {
		log("- Those servers are enabled:", strings.Join(serverNames, ", "))
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
	// Clear existing capabilities per server and register new ones

	// Lock for reading/writing capability tracking
	g.capabilitiesMu.Lock()
	defer g.capabilitiesMu.Unlock()

	// Clear all existing capabilities from tracked servers
	for _, oldCaps := range g.serverCapabilities {
		if len(oldCaps.ToolNames) > 0 {
			g.mcpServer.RemoveTools(oldCaps.ToolNames...)
		}
		if len(oldCaps.PromptNames) > 0 {
			g.mcpServer.RemovePrompts(oldCaps.PromptNames...)
		}
		if len(oldCaps.ResourceURIs) > 0 {
			g.mcpServer.RemoveResources(oldCaps.ResourceURIs...)
		}
		if len(oldCaps.ResourceTemplateURIs) > 0 {
			g.mcpServer.RemoveResourceTemplates(oldCaps.ResourceTemplateURIs...)
		}
	}

	// Clear the tracking map - we'll rebuild it
	g.serverCapabilities = make(map[string]*ServerCapabilities)

	// Add new capabilities and track them per server
	for _, tool := range capabilities.Tools {
		g.mcpServer.AddTool(tool.Tool, tool.Handler)

		// Track by server
		if g.serverCapabilities[tool.ServerName] == nil {
			g.serverCapabilities[tool.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[tool.ServerName].ToolNames = append(
			g.serverCapabilities[tool.ServerName].ToolNames,
			tool.Tool.Name,
		)
	}

	// Add internal tools when dynamic-tools feature is enabled
	if g.DynamicTools {
		log("- Adding internal tools (dynamic-tools feature enabled)")

		// Add mcp-find tool
		mcpFindTool := g.createMcpFindTool(configuration)
		g.mcpServer.AddTool(mcpFindTool.Tool, mcpFindTool.Handler)

		// Add mcp-add tool
		mcpAddTool := g.createMcpAddTool(configuration, clientConfig)
		g.mcpServer.AddTool(mcpAddTool.Tool, mcpAddTool.Handler)

		// Add mcp-remove tool
		mcpRemoveTool := g.createMcpRemoveTool(configuration, clientConfig)
		g.mcpServer.AddTool(mcpRemoveTool.Tool, mcpRemoveTool.Handler)

		// Add mcp-registry-import tool
		mcpRegistryImportTool := g.createMcpRegistryImportTool(configuration, clientConfig)
		g.mcpServer.AddTool(mcpRegistryImportTool.Tool, mcpRegistryImportTool.Handler)

		// Add mcp-config-set tool
		mcpConfigSetTool := g.createMcpConfigSetTool(configuration, clientConfig)
		g.mcpServer.AddTool(mcpConfigSetTool.Tool, mcpConfigSetTool.Handler)

		log("  > mcp-find: tool for finding MCP servers in the catalog")
		log("  > mcp-add: tool for adding MCP servers to the registry")
		log("  > mcp-remove: tool for removing MCP servers from the registry")
		log("  > mcp-registry-import: tool for importing servers from MCP registry URLs")
		log("  > mcp-config-set: tool for setting configuration values for MCP servers")
	}

	for _, prompt := range capabilities.Prompts {
		g.mcpServer.AddPrompt(prompt.Prompt, prompt.Handler)

		// Track by server
		if g.serverCapabilities[prompt.ServerName] == nil {
			g.serverCapabilities[prompt.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[prompt.ServerName].PromptNames = append(
			g.serverCapabilities[prompt.ServerName].PromptNames,
			prompt.Prompt.Name,
		)
	}

	for _, resource := range capabilities.Resources {
		g.mcpServer.AddResource(resource.Resource, resource.Handler)

		// Track by server
		if g.serverCapabilities[resource.ServerName] == nil {
			g.serverCapabilities[resource.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[resource.ServerName].ResourceURIs = append(
			g.serverCapabilities[resource.ServerName].ResourceURIs,
			resource.Resource.URI,
		)
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

		// Track by server
		if g.serverCapabilities[template.ServerName] == nil {
			g.serverCapabilities[template.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[template.ServerName].ResourceTemplateURIs = append(
			g.serverCapabilities[template.ServerName].ResourceTemplateURIs,
			resource.URITemplate,
		)
	}

	g.health.SetHealthy()

	return nil
}

// stringSliceToSet converts a slice to a map for efficient lookup
func stringSliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	return set
}

// diffStringSlices returns items that are in 'newer' but not in 'older' (additions),
// and items that are in 'older' but not in 'newer' (removals)
func diffStringSlices(older, newer []string) (additions, removals []string) {
	oldSet := stringSliceToSet(older)
	newSet := stringSliceToSet(newer)

	for s := range newSet {
		if !oldSet[s] {
			additions = append(additions, s)
		}
	}

	for s := range oldSet {
		if !newSet[s] {
			removals = append(removals, s)
		}
	}

	return additions, removals
}

func (g *Gateway) reloadServerConfiguration(ctx context.Context, serverName string, clientConfig *clientConfig) error {
	// Find the server configuration in current config
	serverConfig, _, found := g.configuration.Find(serverName)
	if !found || serverConfig == nil {
		return fmt.Errorf("server %s not found in configuration", serverName)
	}

	// Get current capabilities from the server (this reflects the server's current state after it notified us of changes)
	capabilities, err := g.listCapabilities(ctx, g.configuration, []string{serverName}, clientConfig)
	if err != nil {
		return fmt.Errorf("failed to list capabilities for %s: %w", serverName, err)
	}

	// Lock for reading/writing capability tracking
	g.capabilitiesMu.Lock()
	defer g.capabilitiesMu.Unlock()

	// Get old capabilities for this server
	oldCaps := g.serverCapabilities[serverName]
	if oldCaps == nil {
		oldCaps = &ServerCapabilities{}
	}

	// Build new capability name/URI lists
	newCaps := &ServerCapabilities{
		ToolNames:            make([]string, 0, len(capabilities.Tools)),
		PromptNames:          make([]string, 0, len(capabilities.Prompts)),
		ResourceURIs:         make([]string, 0, len(capabilities.Resources)),
		ResourceTemplateURIs: make([]string, 0, len(capabilities.ResourceTemplates)),
	}

	for _, tool := range capabilities.Tools {
		newCaps.ToolNames = append(newCaps.ToolNames, tool.Tool.Name)
	}
	for _, prompt := range capabilities.Prompts {
		newCaps.PromptNames = append(newCaps.PromptNames, prompt.Prompt.Name)
	}
	for _, resource := range capabilities.Resources {
		newCaps.ResourceURIs = append(newCaps.ResourceURIs, resource.Resource.URI)
	}
	for _, template := range capabilities.ResourceTemplates {
		newCaps.ResourceTemplateURIs = append(newCaps.ResourceTemplateURIs, template.ResourceTemplate.URITemplate)
	}

	// Determine what changed
	addedTools, removedTools := diffStringSlices(oldCaps.ToolNames, newCaps.ToolNames)
	addedPrompts, removedPrompts := diffStringSlices(oldCaps.PromptNames, newCaps.PromptNames)
	addedResources, removedResources := diffStringSlices(oldCaps.ResourceURIs, newCaps.ResourceURIs)
	addedTemplates, removedTemplates := diffStringSlices(oldCaps.ResourceTemplateURIs, newCaps.ResourceTemplateURIs)

	// Remove old capabilities that are no longer present
	if len(removedTools) > 0 {
		g.mcpServer.RemoveTools(removedTools...)
		log("  - Removed", len(removedTools), "tools for", serverName)
	}

	if len(removedPrompts) > 0 {
		g.mcpServer.RemovePrompts(removedPrompts...)
		log("  - Removed", len(removedPrompts), "prompts for", serverName)
	}

	if len(removedResources) > 0 {
		g.mcpServer.RemoveResources(removedResources...)
		log("  - Removed", len(removedResources), "resources for", serverName)
	}

	if len(removedTemplates) > 0 {
		g.mcpServer.RemoveResourceTemplates(removedTemplates...)
		log("  - Removed", len(removedTemplates), "resource templates for", serverName)
	}

	// Add/update all capabilities from this server
	for _, tool := range capabilities.Tools {
		g.mcpServer.AddTool(tool.Tool, tool.Handler)
	}
	if len(addedTools) > 0 {
		log("  - Added/updated", len(addedTools), "tools for", serverName)
	}

	for _, prompt := range capabilities.Prompts {
		g.mcpServer.AddPrompt(prompt.Prompt, prompt.Handler)
	}
	if len(addedPrompts) > 0 {
		log("  - Added/updated", len(addedPrompts), "prompts for", serverName)
	}

	for _, resource := range capabilities.Resources {
		g.mcpServer.AddResource(resource.Resource, resource.Handler)
	}
	if len(addedResources) > 0 {
		log("  - Added/updated", len(addedResources), "resources for", serverName)
	}

	for _, template := range capabilities.ResourceTemplates {
		resourceTemplate := &mcp.ResourceTemplate{
			URITemplate: template.ResourceTemplate.URITemplate,
			Name:        template.ResourceTemplate.Name,
			Description: template.ResourceTemplate.Description,
			MIMEType:    template.ResourceTemplate.MIMEType,
		}
		g.mcpServer.AddResourceTemplate(resourceTemplate, template.Handler)
	}
	if len(addedTemplates) > 0 {
		log("  - Added/updated", len(addedTemplates), "resource templates for", serverName)
	}

	// Update tracking with new capabilities
	g.serverCapabilities[serverName] = newCaps

	return nil
}

// RefreshCapabilities implements the CapabilityRefresher interface
// This method updates the server's capabilities by reloading the configuration
func (g *Gateway) RefreshCapabilities(ctx context.Context, server *mcp.Server, serverSession *mcp.ServerSession, serverName string) error {
	// Create a clientConfig to reuse the existing session for the server that triggered the notification
	clientConfig := &clientConfig{
		serverSession: serverSession,
		server:        server,
	}

	log("- RefreshCapabilities called for session, refreshing servers:", serverName)

	err := g.reloadServerConfiguration(ctx, serverName, clientConfig)
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

// handleTokenEvent checks for and processes OAuth token events
func (g *Gateway) handleTokenEvent(_ context.Context) {
	// Small delay to ensure token is fully written to credential store
	// This handles the race condition where registry.yaml updates before token is stored
	time.Sleep(200 * time.Millisecond)

	tokenEventPath := filepath.Join(os.Getenv("HOME"), ".docker", "mcp", TokenEventFilename)

	// Check if token event file exists
	if _, err := os.Stat(tokenEventPath); os.IsNotExist(err) {
		// File doesn't exist, no token event
		return
	}

	// Read and parse token event
	data, err := os.ReadFile(tokenEventPath)
	if err != nil {
		log(fmt.Sprintf("Failed to read token event file: %v", err))
		return
	}

	// Skip if file is empty or just placeholder
	if len(data) == 0 || string(data) == "{}" {
		return
	}

	var event TokenEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log(fmt.Sprintf("Failed to parse token event: %v", err))
		return
	}

	log(fmt.Sprintf("Processing %s event for provider %s at %v",
		event.EventType, event.Provider, event.Timestamp.Format(time.RFC3339)))

	// Invalidate OAuth clients for the specified provider
	g.clientPool.InvalidateOAuthClients(event.Provider)

	// Don't delete the file - allow all MCP Gateway instances to process the event
	// File will be overwritten on next token event or cleaned up on DD startup
	log(fmt.Sprintf("Token event processed for %s", event.Provider))
}
