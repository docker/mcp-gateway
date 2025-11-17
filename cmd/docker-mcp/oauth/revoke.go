package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

func Revoke(ctx context.Context, app string) error {
	fmt.Printf("Revoking OAuth access for %s...\n", app)

	// Check if CE mode
	if pkgoauth.IsCEMode() {
		return revokeCEMode(ctx, app)
	}

	// Desktop mode - existing implementation
	return revokeDesktopMode(ctx, app)
}

// revokeDesktopMode handles revoke via Docker Desktop (existing behavior)
func revokeDesktopMode(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()

	// Get catalog to check if this is a remote OAuth server
	catalogData, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := catalogData.Servers[app]
	isRemoteOAuth := found && server.IsRemoteOAuthServer()

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

	fmt.Printf("OAuth access revoked for %s\n", app)
	return nil
}

// revokeCEMode handles revoke in standalone CE mode
// Matches Desktop behavior: deletes both token and DCR client
func revokeCEMode(ctx context.Context, app string) error {
	credHelper := pkgoauth.NewReadWriteCredentialHelper()
	manager := pkgoauth.NewManager(credHelper)

	// Delete OAuth token
	if err := manager.RevokeToken(ctx, app); err != nil {
		// Token might not exist, continue to DCR deletion
		fmt.Printf("Note: %v\n", err)
	}

	// Delete DCR client (matches Desktop behavior)
	if err := manager.DeleteDCRClient(app); err != nil {
		return fmt.Errorf("failed to delete DCR client: %w", err)
	}

	fmt.Printf("OAuth access revoked for %s\n", app)
	return nil
}
