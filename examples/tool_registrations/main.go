package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/gateway"
	"github.com/docker/mcp-gateway/pkg/log"
)

// SerializableToolRegistration is a JSON-serializable version of gateway.ToolRegistration
// Since mcp.ToolHandler is a function and can't be serialized, we exclude it
type SerializableToolRegistration struct {
	ServerName        string    `json:"server_name"`
	ServerTitle       string    `json:"server_title,omitempty"`
	ServerDescription string    `json:"server_description,omitempty"`
	Tool              *mcp.Tool `json:"tool"`
}

func main() {
	var (
		catalogPath  = flag.String("catalog", catalog.DockerCatalogFilename, "Path to MCP server catalog")
		registryPath = flag.String("registry", "registry.yaml", "Path to registry file")
		configPath   = flag.String("config", "config.yaml", "Path to config file")
		toolsPath    = flag.String("tools", "tools.yaml", "Path to tools config file")
		secretsPath  = flag.String("secrets", "docker-desktop", "Path to secrets")
		outputPath   = flag.String("output", "tool-registrations.json", "Output file for tool registrations")
		static       = flag.Bool("static", false, "Don't pull or start Docker containers")
		servers      stringSlice
	)
	flag.Var(&servers, "server", "Server name to include (can be specified multiple times, empty = all enabled servers)")
	flag.Parse()

	if err := run(
		*catalogPath,
		*registryPath,
		*configPath,
		*toolsPath,
		*secretsPath,
		*outputPath,
		*static,
		[]string(servers),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(catalogPath, registryPath, configPath, toolsPath, secretsPath, outputPath string, static bool, serverNames []string) error {
	ctx := context.Background()

	// Initialize Docker CLI
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return fmt.Errorf("creating docker CLI: %w", err)
	}
	if err := dockerCli.Initialize(&flags.ClientOptions{}); err != nil {
		return fmt.Errorf("initializing docker CLI: %w", err)
	}

	// Initialize Docker client
	dockerClient := docker.NewClient(dockerCli)

	// Create gateway configuration
	config := gateway.Config{
		ServerNames:  serverNames,
		CatalogPath:  []string{catalogPath},
		RegistryPath: []string{registryPath},
		ConfigPath:   []string{configPath},
		ToolsPath:    []string{toolsPath},
		SecretsPath:  secretsPath,
		Options: gateway.Options{
			Cpus:         1,
			Memory:       "2Gb",
			Transport:    "stdio",
			LogCalls:     false,
			BlockSecrets: false,
			Verbose:      true,
			Static:       static,
			Watch:        false,
		},
	}

	log.Log("Creating gateway...")
	g := gateway.NewGateway(config, dockerClient)

	// Read configuration
	log.Log("Reading configuration...")
	configuration, _, stopConfigWatcher, err := g.Configurator().Read(ctx)
	if err != nil {
		return fmt.Errorf("reading configuration: %w", err)
	}
	defer func() { _ = stopConfigWatcher() }()

	// Pull and verify Docker images (unless static mode is enabled)
	if !static {
		log.Log("Pulling Docker images...")
		if err := g.PullAndVerify(ctx, configuration); err != nil {
			return fmt.Errorf("pulling and verifying images: %w", err)
		}
	}

	// Initialize MCP server (required for reloadConfiguration)
	log.Log("Initializing MCP server...")
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "Tool Registration Extractor",
		Version: "1.0.0",
	}, &mcp.ServerOptions{
		HasPrompts:   true,
		HasResources: true,
		HasTools:     true,
	})
	g.SetMCPServer(mcpServer)

	// Reload configuration to populate tool registrations
	log.Log("Loading tool registrations...")
	if err := g.ReloadConfiguration(ctx, configuration, nil, nil); err != nil {
		return fmt.Errorf("reloading configuration: %w", err)
	}

	// Get tool registrations
	toolRegistrations := g.GetToolRegistrations()
	log.Log(fmt.Sprintf("Found %d tool registrations", len(toolRegistrations)))

	// Convert to serializable format
	serializableRegs := make(map[string]SerializableToolRegistration, len(toolRegistrations))
	for name, reg := range toolRegistrations {
		// Look up server configuration to get description and title
		serverConfig, _, found := configuration.Find(reg.ServerName)

		entry := SerializableToolRegistration{
			ServerName: reg.ServerName,
			Tool:       reg.Tool,
		}

		if found && serverConfig != nil {
			entry.ServerTitle = serverConfig.Spec.Title
			entry.ServerDescription = serverConfig.Spec.Description
		}

		serializableRegs[name] = entry
	}

	// Serialize to JSON
	log.Log(fmt.Sprintf("Writing tool registrations to %s...", outputPath))
	data, err := json.MarshalIndent(serializableRegs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tool registrations: %w", err)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." && outputDir != "" {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}

	log.Log(fmt.Sprintf("Successfully wrote %d tool registrations to %s", len(serializableRegs), outputPath))
	return nil
}

// stringSlice implements flag.Value for repeated string flags
type stringSlice []string

func (s *stringSlice) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}
