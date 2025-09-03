package oauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

func Revoke(ctx context.Context, app string) error {
	// Check if this is a remote MCP server
	if strings.HasSuffix(app, "-remote") {
		return revokeRemoteMCPServer(ctx, app)
	}

	client := desktop.NewAuthClient()
	return client.DeleteOAuthApp(ctx, app)
}

func revokeRemoteMCPServer(ctx context.Context, serverName string) error {
	client := desktop.NewAuthClient()
	
	// First, check if this server exists as a DCR OAuth provider.
	// After DCR implementation, remote servers like "notion-remote" become 
	// OAuth providers directly, so we should revoke them via Docker Desktop API.
	apps, err := client.ListOAuthApps(ctx)
	if err == nil {
		for _, app := range apps {
			if app.App == serverName {
				// This is a DCR provider - revoke OAuth access only (keep DCR client)
				fmt.Printf("Revoking OAuth access for %s...\n", serverName)
				
				// Revoke OAuth tokens only - DCR client remains for future authorization
				if err := client.DeleteOAuthApp(ctx, serverName); err != nil {
					return fmt.Errorf("failed to revoke OAuth access for %s: %w", serverName, err)
				}
				
				fmt.Printf("✅ OAuth access revoked for %s (DCR client preserved for re-authorization)\n", serverName)
				
				return nil
			}
		}
	}
	
	// Fallback to old logic for non-DCR servers (backward compatibility)
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
		return fmt.Errorf("server %s does not have OAuth configured", serverName)
	}

	// This handles the legacy case where catalog servers use separate OAuth providers
	if server.OAuth.Provider != serverName {
		fmt.Printf("Note: %s uses %s OAuth provider.\n", serverName, server.OAuth.Provider)
		fmt.Printf("To revoke access, use: docker mcp oauth revoke %s\n", server.OAuth.Provider)
		return nil
	}

	// If we get here, the server should have been a DCR provider but wasn't found
	return fmt.Errorf("OAuth provider %s not found - it may not be authorized", serverName)
}
