package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
	"github.com/docker/mcp-gateway/pkg/workingset"
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

	// Revoke tokens via Docker Desktop
	if err := client.DeleteOAuthApp(ctx, app); err != nil {
		return fmt.Errorf("failed to revoke OAuth access: %w", err)
	}

	fmt.Printf("OAuth access revoked for %s\n", app)

	// Clean up DCR entry if the server is not in any profile.
	// If the server is still in a profile, keep the DCR entry so it
	// remains visible in the OAuth UI for re-authorization.
	dao, err := db.New()
	if err == nil {
		workingset.CleanupOrphanedDCREntries(ctx, dao, []string{app})
	}

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
