package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

func Revoke(ctx context.Context, app string) error {
	// Load catalog to check server type and OAuth configuration
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	// Check if this server requires DCR OAuth flow
	if server, found := cat.Servers[app]; found {
		if server.Type == "remote" && server.OAuth != nil && len(server.OAuth.Providers) > 0 {
			return revokeRemoteMCPServer(ctx, app)
		}
	}

	// Traditional OAuth provider revoke
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
	if server.OAuth == nil || len(server.OAuth.Providers) == 0 {
		return fmt.Errorf("server %s does not have OAuth configured", serverName)
	}

	// If we get here, the server should have been a DCR provider but wasn't found
	return fmt.Errorf("OAuth provider %s not found - it may not be authorized", serverName)
}
