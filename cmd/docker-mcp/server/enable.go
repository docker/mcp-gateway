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
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oauth"
)

func Disable(ctx context.Context, docker docker.Client, serverNames []string) error {
	// Get catalog to check server types for OAuth cleanup
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}
	
	// Clean up OAuth for disabled servers first
	for _, serverName := range serverNames {
		if server, found := cat.Servers[serverName]; found {
			// Only cleanup OAuth for remote servers with OAuth config
			if server.Type == "remote" && server.OAuth != nil && len(server.OAuth.Providers) > 0 {
				if err := cleanupOAuthForRemoteServer(ctx, serverName); err != nil {
					fmt.Printf("⚠️ Warning: Failed to cleanup OAuth for %s: %v\n", serverName, err)
				}
			}
		}
	}
	
	return update(ctx, docker, nil, serverNames)
}

func Enable(ctx context.Context, docker docker.Client, serverNames []string) error {
	return update(ctx, docker, serverNames, nil)
}

func update(ctx context.Context, docker docker.Client, add []string, remove []string) error {
	// Read registry.yaml that contains which servers are enabled.
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return fmt.Errorf("reading registry config: %w", err)
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return fmt.Errorf("parsing registry config: %w", err)
	}

	// Get catalog including user-configured catalogs
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return err
	}

	updatedRegistry := config.Registry{
		Servers: map[string]config.Tile{},
	}

	// Keep only servers that are still in the catalog.
	for serverName := range registry.Servers {
		if _, found := cat.Servers[serverName]; found {
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref: "",
			}
		}
	}

	// Enable servers.
	for _, serverName := range add {
		if server, found := cat.Servers[serverName]; found {
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref: "",
			}
			
			// For remote MCP servers with OAuth config, perform OAuth discovery and DCR
			if server.Type == "remote" && server.OAuth != nil && len(server.OAuth.Providers) > 0 {
				if err := setupOAuthForRemoteServer(ctx, serverName, &cat); err != nil {
					fmt.Printf("⚠️ Warning: Failed to setup OAuth for %s: %v\n", serverName, err)
					fmt.Printf("   You can run 'docker mcp oauth authorize %s' later to set up authentication.\n", serverName)
				}
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

// setupOAuthForRemoteServer performs OAuth discovery and DCR for remote MCP servers
// This enables OAuth providers to appear in the Docker Desktop UI immediately after enable
func setupOAuthForRemoteServer(ctx context.Context, serverName string, cat *catalog.Catalog) error {
	server, found := cat.Servers[serverName]
	if !found {
		return fmt.Errorf("server %s not found in catalog", serverName)
	}

	// Get server URL
	serverURL := server.Remote.URL
	if serverURL == "" {
		serverURL = server.SSEEndpoint
		if serverURL == "" {
			return fmt.Errorf("server %s has no remote URL configured", serverName)
		}
	}

	fmt.Printf("🔍 Discovering OAuth requirements for %s...\n", serverName)

	// Perform OAuth discovery
	discovery, err := oauth.DiscoverOAuthRequirements(ctx, serverURL)
	if err != nil {
		return fmt.Errorf("OAuth discovery failed: %w", err)
	}

	// Check if DCR client already exists (avoid re-registration)
	client := desktop.NewAuthClient()
	existing, err := client.GetDCRClient(ctx, serverName)
	if err == nil && existing.ClientID != "" {
		fmt.Printf("✅ DCR client already exists for %s, registering as provider...\n", serverName)
		
		// Still need to register as OAuth provider so it appears in UI
		if err := client.RegisterDCRProvider(ctx, serverName); err != nil {
			return fmt.Errorf("failed to register existing DCR client as provider: %w", err)
		}
		
		fmt.Printf("✅ OAuth provider registered for %s (appears in Docker Desktop OAuth tab)\n", serverName)
		return nil
	}

	// Perform DCR to get client credentials
	fmt.Printf("🔧 Registering OAuth client for %s...\n", serverName)
	
	credentials, err := oauth.PerformDCR(ctx, discovery, serverName)
	if err != nil {
		return fmt.Errorf("DCR registration failed: %w", err)
	}

	// Store DCR client in Docker Desktop
	dcrRequest := desktop.RegisterDCRRequest{
		ClientID:              credentials.ClientID,
		AuthorizationEndpoint: credentials.AuthorizationEndpoint,
		TokenEndpoint:         credentials.TokenEndpoint,
	}

	if err := client.RegisterDCRClient(ctx, serverName, dcrRequest); err != nil {
		return fmt.Errorf("failed to store DCR client: %w", err)
	}

	// Register as OAuth provider so it appears in Docker Desktop UI
	if err := client.RegisterDCRProvider(ctx, serverName); err != nil {
		return fmt.Errorf("failed to register OAuth provider: %w", err)
	}

	fmt.Printf("✅ OAuth setup complete for %s\n", serverName)
	fmt.Printf("   • Client registered with authorization server\n")
	fmt.Printf("   • Provider appears in Docker Desktop OAuth tab\n")
	fmt.Printf("   • Ready for authorization (click Authorize in UI or run 'docker mcp oauth authorize %s')\n", serverName)

	return nil
}

// cleanupOAuthForRemoteServer removes OAuth provider and DCR client for clean slate UX
// This ensures disabled servers disappear completely from the Docker Desktop OAuth tab
func cleanupOAuthForRemoteServer(ctx context.Context, serverName string) error {
	client := desktop.NewAuthClient()
	
	// Check if OAuth provider exists
	apps, err := client.ListOAuthApps(ctx)
	if err != nil {
		return fmt.Errorf("failed to list OAuth apps: %w", err)
	}
	
	var hasOAuthProvider bool
	for _, app := range apps {
		if app.App == serverName {
			hasOAuthProvider = true
			break
		}
	}
	
	if hasOAuthProvider {
		fmt.Printf("🧹 Cleaning up OAuth for %s...\n", serverName)
		
		// 1. Revoke OAuth tokens (removes from OAuth providers)
		if err := client.DeleteOAuthApp(ctx, serverName); err != nil {
			fmt.Printf("⚠️ Warning: Failed to revoke OAuth access for %s: %v\n", serverName, err)
		} else {
			fmt.Printf("   • OAuth tokens revoked\n")
		}
		
		// 2. Delete DCR client data for complete cleanup
		if err := client.DeleteDCRClient(ctx, serverName); err != nil {
			fmt.Printf("⚠️ Warning: Failed to clean up DCR client for %s: %v\n", serverName, err)
		} else {
			fmt.Printf("   • DCR client data removed\n")
		}
		
		fmt.Printf("✅ OAuth cleanup complete for %s (clean slate achieved)\n", serverName)
	} else {
		// Still try to clean up DCR client if it exists (silent cleanup)
		client.DeleteDCRClient(ctx, serverName) // Ignore errors for silent cleanup
	}
	
	return nil
}
