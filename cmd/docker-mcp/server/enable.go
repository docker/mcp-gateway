package server

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/cli/cli/command"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

func Disable(ctx context.Context, docker docker.Client, dockerCli command.Cli, serverNames []string, mcpOAuthDcrEnabled bool) error {
	return update(ctx, docker, dockerCli, nil, serverNames, mcpOAuthDcrEnabled)
}

func Enable(ctx context.Context, docker docker.Client, dockerCli command.Cli, serverNames []string, mcpOAuthDcrEnabled bool) error {
	return update(ctx, docker, dockerCli, serverNames, nil, mcpOAuthDcrEnabled)
}

func update(ctx context.Context, docker docker.Client, dockerCli command.Cli, add []string, remove []string, mcpOAuthDcrEnabled bool) error {
	// Read registry.yaml that contains which servers are enabled.
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return fmt.Errorf("reading registry config: %w", err)
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return fmt.Errorf("parsing registry config: %w", err)
	}

	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return err
	}

	updatedRegistry := config.Registry{
		Servers: map[string]config.Tile{},
	}

	// Keep only servers that are still in the catalog.
	for serverName := range registry.Servers {
		if _, found := catalog.Servers[serverName]; found {
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref: "",
			}
		}
	}

	// Enable servers.
	for _, serverName := range add {
		if server, found := catalog.Servers[serverName]; found {
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref: "",
			}

			// Three-condition check: DCR flag enabled AND type="remote" AND oauth present
			if mcpOAuthDcrEnabled && server.IsRemoteOAuthServer() {
				if err := oauth.RegisterProviderForLazySetup(ctx, serverName); err != nil {
					fmt.Printf("Warning: Failed to register OAuth provider for %s: %v\n", serverName, err)
					fmt.Printf("   You can run 'docker mcp oauth authorize %s' later to set up authentication.\n", serverName)
				} else {
					fmt.Printf("OAuth provider configured for %s - use 'docker mcp oauth authorize %s' to authenticate\n", serverName, serverName)
				}
			} else if !mcpOAuthDcrEnabled && server.IsRemoteOAuthServer() {
				// Provide guidance when DCR is needed but disabled
				fmt.Printf("Server %s requires OAuth authentication but DCR is disabled.\n", serverName)
				fmt.Printf("   To enable automatic OAuth setup, run: docker mcp feature enable mcp-oauth-dcr\n")
				fmt.Printf("   Or set up OAuth manually using: docker mcp oauth authorize %s\n", serverName)
			}
		} else {
			return fmt.Errorf("server %s not found in catalog", serverName)
		}
	}

	// Disable servers.
	for _, serverName := range remove {
		delete(updatedRegistry.Servers, serverName)
	}

	// Save it.
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(updatedRegistry); err != nil {
		return fmt.Errorf("encoding registry config: %w", err)
	}

	if err := config.WriteRegistry(buf.Bytes()); err != nil {
		return fmt.Errorf("writing registry config: %w", err)
	}

	if len(add) > 0 && hints.Enabled(dockerCli) {
		hints.TipCyan.Print("Tip: ")
		hints.TipGreen.Print("✓")
		hints.TipCyan.Print(" Server enabled. To view all enabled servers, use ")
		hints.TipCyanBoldItalic.Println("docker mcp server ls")
		fmt.Println()
	}

	if len(remove) > 0 && hints.Enabled(dockerCli) {
		hints.TipCyan.Print("Tip: ")
		hints.TipGreen.Print("✓")
		hints.TipCyan.Print(" Server disabled. To see remaining enabled servers, use ")
		hints.TipCyanBoldItalic.Println("docker mcp server ls")
		fmt.Println()
	}

	return nil
}
