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
				// This is a DCR provider - revoke OAuth access and clean up DCR data
				fmt.Printf("Revoking OAuth access for %s...\n", serverName)
				
				// 1. Revoke OAuth token (removes from OAuth providers)
				if err := client.DeleteOAuthApp(ctx, serverName); err != nil {
					return fmt.Errorf("failed to revoke OAuth access for %s: %w", serverName, err)
				}
				
				// 2. Clean up DCR client data for complete cleanup
				if err := client.DeleteDCRClient(ctx, serverName); err != nil {
					// Don't fail the revoke if DCR cleanup fails - token revocation succeeded
					fmt.Printf("⚠️ Warning: Failed to clean up DCR client data for %s: %v\n", serverName, err)
					fmt.Printf("✅ OAuth access revoked for %s (DCR data cleanup failed)\n", serverName)
				} else {
					fmt.Printf("✅ OAuth access revoked and DCR client cleaned up for %s\n", serverName)
				}
				
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
