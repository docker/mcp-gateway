package oauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oauth"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	// Check if this is a remote MCP server
	if strings.HasSuffix(app, "-remote") {
		return authorizeRemoteMCPServer(ctx, app, scopes)
	}

	client := desktop.NewAuthClient()

	authResponse, err := client.PostOAuthApp(ctx, app, scopes)
	if err != nil {
		return err
	}

	fmt.Printf("Opening your browser for authentication. If it doesn't open automatically, please visit: %s\n", authResponse.BrowserURL)

	return nil
}

func authorizeRemoteMCPServer(ctx context.Context, serverName string, scopes string) error {
	// Get the catalog including user-configured catalogs
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := cat.Servers[serverName]
	if !found {
		return fmt.Errorf("server %s not found in catalog", serverName)
	}

	// Validate server has remote URL
	serverURL := server.Remote.URL
	if serverURL == "" {
		// Fallback to deprecated SSEEndpoint
		serverURL = server.SSEEndpoint
		if serverURL == "" {
			return fmt.Errorf("server %s has no remote URL configured", serverName)
		}
	}

	// Phase 1: Discover OAuth requirements
	fmt.Printf("Discovering OAuth requirements for %s...\n", serverName)
	discovery, err := oauth.DiscoverOAuthRequirements(ctx, serverURL)
	if err != nil {
		return fmt.Errorf("failed to discover OAuth requirements: %w", err)
	}

	// If server doesn't require OAuth, inform user
	if !discovery.RequiresOAuth {
		fmt.Printf("Server %s does not require OAuth authentication\n", serverName)
		return nil
	}

	fmt.Printf("OAuth required for %s\n", serverName)
	fmt.Printf("Authorization Server: %s\n", discovery.AuthorizationServer)
	if len(discovery.Scopes) > 0 {
		fmt.Printf("Required Scopes: %s\n", strings.Join(discovery.Scopes, " "))
	}

	// Use discovered scopes if not provided by user
	if scopes == "" && len(discovery.Scopes) > 0 {
		scopes = strings.Join(discovery.Scopes, " ")
	}

	// Check if server has static OAuth configuration (for backward compatibility)
	if server.OAuth != nil && server.OAuth.Enabled {
		provider := server.OAuth.Provider
		if provider != "" {
			fmt.Printf("Using configured OAuth provider: %s\n", provider)
		}
		
		// Use server's configured scopes if not already set
		if scopes == "" && len(server.OAuth.Scopes) > 0 {
			scopes = strings.Join(server.OAuth.Scopes, " ")
		}
	}

	// For MCP Gateway-driven DCR, we need to pass the discovery metadata to Docker Desktop
	// The server name will trigger DCR in the backend with the discovered configuration
	// TODO: In Phase 2, we'll implement the full DCR flow here instead of delegating to Desktop
	client := desktop.NewAuthClient()
	authResponse, err := client.PostOAuthApp(ctx, serverName, scopes)
	if err != nil {
		return fmt.Errorf("failed to authorize OAuth for %s: %w", serverName, err)
	}

	fmt.Printf("Opening your browser for OAuth authentication. If it doesn't open automatically, please visit: %s\n", authResponse.BrowserURL)
	fmt.Printf("Once authenticated, %s will have OAuth access\n", serverName)

	return nil
}
