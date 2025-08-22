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
		return fmt.Errorf("server %s does not have OAuth configured", serverName)
	}

	// Note: We don't actually revoke the provider's OAuth token here,
	// because it might be used by other services.
	// Instead, we just inform the user.
	fmt.Printf("Note: %s uses %s OAuth provider.\n", serverName, server.OAuth.Provider)
	fmt.Printf("To revoke access, use: docker mcp oauth revoke %s\n", server.OAuth.Provider)

	return nil
}
