package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

func Revoke(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()

	// Check if this is a DCR provider
	dcrClient, err := client.GetDCRClient(ctx, app)
	if err == nil && dcrClient.State != "" {
		// Handle UNREGISTERED providers - they don't have tokens yet
		if dcrClient.State == "unregistered" {
			return fmt.Errorf("provider %s is not authenticated yet - nothing to revoke", app)
		}

		// REGISTERED DCR provider - revoke tokens but preserve DCR client for re-auth
		fmt.Printf("Revoking OAuth access for %s...\n", app)
		if err := client.DeleteOAuthApp(ctx, app); err != nil {
			return fmt.Errorf("failed to revoke OAuth access for %s: %w", app, err)
		}
		fmt.Printf("OAuth access revoked for %s\n", app)
		fmt.Printf("Note: DCR client registration preserved. Run 'docker mcp oauth authorize %s' to re-authenticate\n", app)
		return nil
	}

	// Built-in OAuth provider - just revoke tokens
	return client.DeleteOAuthApp(ctx, app)
}
