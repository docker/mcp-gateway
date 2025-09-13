package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

func Revoke(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()
	
	// Check if this is a DCR provider by trying to get DCR client directly
	dcrClient, err := client.GetDCRClient(ctx, app)
	if err == nil && dcrClient.State != "" {
		// This is a DCR provider (registered or unregistered) - revoke OAuth access only (preserve DCR client for re-auth)
		return revokeRemoteMCPServer(ctx, app)
	}

	// Traditional OAuth provider revoke (built-in providers)
	return client.DeleteOAuthApp(ctx, app)
}

func revokeRemoteMCPServer(ctx context.Context, serverName string) error {
	client := desktop.NewAuthClient()
	
	fmt.Printf("Revoking OAuth access for %s...\n", serverName)
	
	// Revoke OAuth tokens only - DCR client remains for future authorization
	if err := client.DeleteOAuthApp(ctx, serverName); err != nil {
		return fmt.Errorf("failed to revoke OAuth access for %s: %w", serverName, err)
	}
	
	fmt.Printf("oauth access revoked for %s (DCR client preserved for re-authorization)\n", serverName)
	return nil
}
