package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/health"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/interceptors"
)

type Gateway struct {
	Options
	docker       docker.Client
	configurator Configurator
	clientPool   *clientPool
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

	// Build a list of interceptors.
	customInterceptors, err := interceptors.Parse(g.Interceptors)
	if err != nil {
		return fmt.Errorf("parsing interceptors: %w", err)
	}
	toolCallbacks := interceptors.Callbacks(g.LogCalls, g.BlockSecrets, customInterceptors)

	mcpServer := server.NewMCPServer(
		"Docker AI MCP Gateway",
		"2.0.1",
		server.WithToolHandlerMiddleware(toolCallbacks),
	)

	if err := g.reloadConfiguration(ctx, mcpServer, configuration); err != nil {
		return fmt.Errorf("loading configuration: %w", err)
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

					if err := g.reloadConfiguration(ctx, mcpServer, configuration); err != nil {
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
		return g.startStdioServer(ctx, mcpServer, os.Stdin, os.Stdout)

	case "sse":
		log("> Start sse server on port", g.Port)
		return g.startSseServer(ctx, mcpServer, ln)

	case "streaming":
		log("> Start streaming server on port", g.Port)
		return g.startStreamingServer(ctx, mcpServer, ln)

	default:
		return fmt.Errorf("unknown transport %q, expected 'stdio', 'sse' or 'streaming", g.Transport)
	}
}

func (g *Gateway) reloadConfiguration(ctx context.Context, mcpServer *server.MCPServer, configuration Configuration) error {
	// Which servers are enabled in the registry.yaml?
	serverNames := configuration.ServerNames()
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

	// Update the server's capabilities.
	g.health.SetUnhealthy()
	mcpServer.SetTools(capabilities.Tools...)
	mcpServer.SetPrompts(capabilities.Prompts...)
	mcpServer.SetResources(capabilities.Resources...)
	mcpServer.RemoveAllResourceTemplates()
	for _, v := range capabilities.ResourceTemplates {
		mcpServer.AddResourceTemplate(v.ResourceTemplate, v.Handler)
	}
	g.health.SetHealthy()

	return nil
}
