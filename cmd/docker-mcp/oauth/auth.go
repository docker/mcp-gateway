package oauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
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
	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := catalog.Servers[serverName]
	if !found {
		return fmt.Errorf("server %s not found in catalog", serverName)
	}

	// Check if the server has OAuth configuration
	if server.OAuth == nil || !server.OAuth.Enabled {
		return fmt.Errorf("server %s does not support OAuth authentication", serverName)
	}

	// Use the OAuth provider from the server configuration
	provider := server.OAuth.Provider
	if provider == "" {
		return fmt.Errorf("OAuth provider not configured for server %s", serverName)
	}

	// Use server's configured scopes if not provided
	if scopes == "" && len(server.OAuth.Scopes) > 0 {
		scopes = strings.Join(server.OAuth.Scopes, " ")
	}

	// For remote MCP servers, use the server name as the provider ID for DCR
	// This tells pinata to use Dynamic Client Registration
	client := desktop.NewAuthClient()
	authResponse, err := client.PostOAuthApp(ctx, serverName, scopes)
	if err != nil {
		return fmt.Errorf("failed to authorize OAuth for %s: %w", serverName, err)
	}

	fmt.Printf("Opening your browser for %s authentication. If it doesn't open automatically, please visit: %s\n", provider, authResponse.BrowserURL)
	fmt.Printf("Once authenticated, %s will have access to your %s account\n", serverName, provider)

	return nil
}
