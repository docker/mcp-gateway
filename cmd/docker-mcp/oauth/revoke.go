package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

func Revoke(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()

	// Get catalog to check if this is a remote OAuth server
	catalogData, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := catalogData.Servers[app]
	isRemoteOAuth := found && server.IsRemoteOAuthServer()

	fmt.Printf("Revoking OAuth access for %s...\n", app)

	// Revoke tokens
	if err := client.DeleteOAuthApp(ctx, app); err != nil {
		return fmt.Errorf("failed to revoke OAuth access: %w", err)
	}

	// For remote OAuth servers, also delete DCR client
	if isRemoteOAuth {
		if err := client.DeleteDCRClient(ctx, app); err != nil {
			return fmt.Errorf("failed to remove DCR client: %w", err)
		}
	}

	return nil
}
