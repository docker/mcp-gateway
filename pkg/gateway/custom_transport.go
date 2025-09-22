package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/pkg/interceptors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

// RunWithTransport allows the Gateway to be run with a custom transport instead of the default stdio transport.
// This is useful for testing or when you want to connect to the Gateway programmatically.
func (g *Gateway) RunWithTransport(ctx context.Context, transport mcp.Transport) error {
	// Initialize the Gateway (everything from Run except the transport part)

	// Initialize telemetry
	telemetry.Init() // Commented out to avoid duplicate initialization

	// Record gateway start with custom transport mode
	transportMode := "custom"
	telemetry.RecordGatewayStart(ctx, transportMode)

	// Start periodic metric export for long-running gateway
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

	// Read the configuration
	configuration, _, stopConfigWatcher, err := g.configurator.Read(ctx)
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

	// Create the MCP server (this is what Gateway.Run does internally)
	g.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "Docker AI MCP Gateway",
		Version: "2.0.1",
	}, &mcp.ServerOptions{
		SubscribeHandler: func(_ context.Context, req *mcp.SubscribeRequest) error {
			log("- Client subscribed to URI:", req.Params.URI)
			return nil
		},
		UnsubscribeHandler: func(_ context.Context, req *mcp.UnsubscribeRequest) error {
			log("- Client unsubscribed from URI:", req.Params.URI)
			return nil
		},
		RootsListChangedHandler: func(ctx context.Context, req *mcp.RootsListChangedRequest) {
			log("- Client roots list changed")
			_, _ = req.Session.ListRoots(ctx, &mcp.ListRootsParams{})
		},
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			clientInfo := req.Session.InitializeParams().ClientInfo
			log("- Client initialized", clientInfo.Name+"@"+clientInfo.Version, clientInfo.Title)
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

	// Pull and verify images if needed
	if !g.Static {
		if err := g.pullAndVerify(ctx, configuration); err != nil {
			return err
		}
	}

	// Load configuration and capabilities
	if err := g.reloadConfiguration(ctx, configuration, nil, nil); err != nil {
		return err
	}

	if g.DryRun {
		log("Dry run mode enabled, not starting the server.")
		return nil
	}

	// Run the MCP server with the custom transport
	log("Starting MCP server with custom transport")
	return g.mcpServer.Run(ctx, transport)
}
