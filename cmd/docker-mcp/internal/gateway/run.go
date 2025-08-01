package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/health"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/interceptors"
)

type Gateway struct {
	Options
	docker       docker.Client
	configurator Configurator
	clientPool   *clientPool
	mcpServer    *mcp.Server
	health       health.State
}

func NewGateway(config Config, docker docker.Client) *Gateway {
	return &Gateway{
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
		clientPool: newClientPool(config.Options, docker),
	}
}

func (g *Gateway) Run(ctx context.Context) error {
	defer g.clientPool.Close()

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
		SubscribeHandler:        nil,
		UnsubscribeHandler:      nil,
		RootsListChangedHandler: nil,
		CompletionHandler:       nil,
		InitializedHandler:      nil,
		HasPrompts:              true,
		HasResources:            true,
		HasTools:                true,
	})

	// Add interceptor middleware to the server
	middlewares := interceptors.Callbacks(g.LogCalls, g.BlockSecrets, parsedInterceptors)
	if len(middlewares) > 0 {
		g.mcpServer.AddReceivingMiddleware(middlewares...)
	}

	if err := g.reloadConfiguration(ctx, configuration, nil); err != nil {
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

					if err := g.reloadConfiguration(ctx, configuration, nil); err != nil {
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

func (g *Gateway) reloadConfiguration(ctx context.Context, configuration Configuration, serverNames []string) error {
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
	capabilities, err := g.listCapabilities(ctx, configuration, serverNames)
	if err != nil {
		return fmt.Errorf("listing resources: %w", err)
	}
	log(">", len(capabilities.Tools), "tools listed in", time.Since(startList))

	// Update capabilities
	// Clear existing capabilities and register new ones
	// Note: The new SDK doesn't have bulk set methods, so we register individually

	for _, tool := range capabilities.Tools {
		g.mcpServer.AddTool(tool.Tool, tool.Handler)
	}

	for _, prompt := range capabilities.Prompts {
		g.mcpServer.AddPrompt(prompt.Prompt, prompt.Handler)
	}

	for _, resource := range capabilities.Resources {
		g.mcpServer.AddResource(resource.Resource, resource.Handler)
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
	}

	g.health.SetHealthy()

	return nil
}
