package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/interceptors"
)

type Gateway struct {
	Options
	docker       docker.Client
	configurator Configurator
	clientPool   *clientPool
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
			Watch:        config.Watch,
			docker:       docker,
		},
		clientPool: newClientPool(config.Options, docker),
	}
}

func (g *Gateway) Run(ctx context.Context) error {
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

	// Which servers are enabled in the registry.yaml?
	serverNames := configuration.ServerNames()
	if len(serverNames) == 0 {
		log("- No server is enabled")
	} else {
		log("- Those servers are enabled:", strings.Join(serverNames, ", "))
	}

	// Which docker images are used?
	// Pull them and verify them if possible.
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

	// List all the available tools.
	startList := time.Now()
	log("- Listing MCP tools...")
	capabilities, err := g.listCapabilities(ctx, configuration, serverNames)
	if err != nil {
		return fmt.Errorf("listing resources: %w", err)
	}
	log(">", len(capabilities.Tools), "tools listed in", time.Since(startList))

	// Build a list of interceptors.
	customInterceptors, err := interceptors.Parse(g.Interceptors)
	if err != nil {
		return fmt.Errorf("parsing interceptors: %w", err)
	}
	toolCallbacks := interceptors.Callbacks(g.LogCalls, g.BlockSecrets, customInterceptors)

	// TODO: cleanup stopped servers. That happens in stdio over TCP mode.
	var (
		lock            sync.Mutex
		changeListeners []func(*Capabilities)
	)

	newMCPServer := func() *server.MCPServer {
		mcpServer := server.NewMCPServer(
			"Docker AI MCP Gateway",
			"2.0.1",
			server.WithToolHandlerMiddleware(toolCallbacks),
		)

		current := capabilities
		mcpServer.AddTools(current.Tools...)
		mcpServer.AddPrompts(current.Prompts...)
		mcpServer.AddResources(current.Resources...)
		for _, v := range current.ResourceTemplates {
			mcpServer.AddResourceTemplate(v.ResourceTemplate, v.Handler)
		}

		lock.Lock()
		changeListeners = append(changeListeners, func(newConfig *Capabilities) {
			mcpServer.DeleteTools(current.ToolNames()...)
			mcpServer.DeletePrompts(current.PromptNames()...)
			mcpServer.AddTools(newConfig.Tools...)
			mcpServer.AddPrompts(newConfig.Prompts...)

			// TODO: sync other things than tools

			current = newConfig
		})
		lock.Unlock()

		return mcpServer
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

					capabilities, err := g.listCapabilities(ctx, configuration, configuration.ServerNames())
					if err != nil {
						logf("> Unable to list capabilities: %s", err)
						continue
					}

					lock.Lock()
					for _, listener := range changeListeners {
						listener(capabilities)
					}
					lock.Unlock()
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
		if g.Port == 0 {
			log("> Starting stdio server")
			return startStdioServer(ctx, newMCPServer, os.Stdin, os.Stdout)
		}

		log("> Starting stdio over TCP server on port", g.Port)
		return startStdioOverTCPServer(ctx, newMCPServer, ln)

	case "sse":
		if g.Port == 0 {
			return errors.New("missing 'port' for 'sse' server")
		}

		log("> Starting sse server on port", g.Port)
		return startSseServer(ctx, newMCPServer, ln)

	case "streaming":
		if g.Port == 0 {
			return errors.New("missing 'port' for streaming server")
		}

		log("> Starting streaming server on port", g.Port)
		return startStreamingServer(ctx, newMCPServer, ln)

	default:
		return fmt.Errorf("unknown transport %q, expected 'stdio', 'sse' or 'streaming", g.Transport)
	}
}
