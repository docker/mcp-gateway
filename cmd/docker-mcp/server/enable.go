package server

import (
	"bytes"
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
)

func Disable(ctx context.Context, docker docker.Client, serverNames []string, mcpOAuthDcrEnabled bool) error {
	// Get catalog including user-configured catalogs to find OAuth-enabled remote servers for DCR cleanup
	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	// Clean up OAuth for disabled servers first
	for _, serverName := range serverNames {
		if server, found := catalog.Servers[serverName]; found {
			// Three-condition check: DCR flag enabled AND type="remote" AND oauth present
			if mcpOAuthDcrEnabled && server.IsRemoteOAuthServer() {
				cleanupOAuthForRemoteServer(ctx, serverName)
			}
		}
	}

	return update(ctx, docker, nil, serverNames, mcpOAuthDcrEnabled)
}

func Enable(ctx context.Context, docker docker.Client, serverNames []string, mcpOAuthDcrEnabled bool) error {
	return update(ctx, docker, serverNames, nil, mcpOAuthDcrEnabled)
}

func update(ctx context.Context, docker docker.Client, add []string, remove []string, mcpOAuthDcrEnabled bool) error {
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
				if err := registerProviderForLazySetup(ctx, serverName); err != nil {
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

	return nil
}

// registerProviderForLazySetup registers a provider for lazy DCR setup
// This shows the provider in the OAuth tab immediately without doing network calls
func registerProviderForLazySetup(ctx context.Context, serverName string) error {
	client := desktop.NewAuthClient()

	// Check if DCR client already exists to avoid double-registration
	_, err := client.GetDCRClient(ctx, serverName)
	if err == nil {
		// Provider already registered, no need to register again
		return nil
	}

	// Get catalog to extract provider name
	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := catalog.Servers[serverName]
	if !found {
		return fmt.Errorf("server %s not found in catalog", serverName)
	}

	// Extract provider name from OAuth config
	if server.OAuth == nil || len(server.OAuth.Providers) == 0 {
		return fmt.Errorf("server %s has no OAuth providers configured", serverName)
	}

	providerName := server.OAuth.Providers[0].Provider // Use first provider

	fmt.Printf("Configuring OAuth provider %s (provider: %s) for authentication...\n", serverName, providerName)

	// Use the existing DCR endpoint with pending=true to register provider without DCR
	dcrRequest := desktop.RegisterDCRRequest{
		ProviderName: providerName,
	}

	if err := client.RegisterDCRClientPending(ctx, serverName, dcrRequest); err != nil {
		return fmt.Errorf("failed to register pending DCR provider: %w", err)
	}

	return nil
}

// cleanupOAuthForRemoteServer removes OAuth provider and DCR client for clean slate UX
// This ensures disabled servers disappear completely from the Docker Desktop OAuth tab
func cleanupOAuthForRemoteServer(ctx context.Context, serverName string) {
	client := desktop.NewAuthClient()

	fmt.Printf("Cleaning up OAuth for %s...\n", serverName)

	// 1. Revoke OAuth tokens (idempotent - fails gracefully if not exists)
	if err := client.DeleteOAuthApp(ctx, serverName); err != nil {
		fmt.Printf("   • No OAuth tokens to revoke\n")
	} else {
		fmt.Printf("   • OAuth tokens revoked\n")
	}

	// 2. Delete DCR client data (idempotent - fails gracefully if not exists)
	if err := client.DeleteDCRClient(ctx, serverName); err != nil {
		fmt.Printf("   • No DCR client to remove\n")
	} else {
		fmt.Printf("   • DCR client data removed\n")
	}

	fmt.Printf("OAuth cleanup complete for %s\n", serverName)
}
