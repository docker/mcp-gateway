package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

// Function pointers for testability (same pattern as pkg/workingset/oauth.go).
var (
	revokeCEModeFunc        = revokeCEMode
	revokeDesktopModeFunc   = revokeDesktopMode
	revokeCommunityModeFunc = revokeCommunityMode

	// Internal deps used by mode handlers — overridden in tests to avoid
	// requiring docker pass, Docker Desktop, or a real database.
	deleteOAuthTokenFunc      = secret.DeleteOAuthToken
	deleteDCRClientFunc       = secret.DeleteDCRClient
	desktopDeleteOAuthAppFunc = func(ctx context.Context, app string) error {
		return desktop.NewAuthClient().DeleteOAuthApp(ctx, app)
	}
)

// Revoke revokes OAuth access for a server, routing to the appropriate flow
// based on the per-server mode (Desktop, CE, or Community).
func Revoke(ctx context.Context, app string) error {
	fmt.Printf("Revoking OAuth access for %s...\n", app)

	isCommunity, err := lookupIsCommunityFunc(ctx, app)
	if err != nil {
		// Server not in catalog -- fall back to legacy global routing.
		if isCEModeFunc() {
			return revokeCEModeFunc(ctx, app)
		}
		return revokeDesktopModeFunc(ctx, app)
	}

	switch determineModeFunc(ctx, isCommunity) {
	case pkgoauth.ModeCE:
		return revokeCEModeFunc(ctx, app)
	case pkgoauth.ModeCommunity:
		return revokeCommunityModeFunc(ctx, app)
	default: // ModeDesktop
		return revokeDesktopModeFunc(ctx, app)
	}
}

// revokeDesktopMode handles revoke via Docker Desktop (existing behavior)
func revokeDesktopMode(ctx context.Context, app string) error {
	// Best-effort cleanup of docker pass entries that may exist from a
	// previous authorization with the McpGatewayOAuth flag ON. Run this
	// before the Desktop API call so entries are cleaned even when the
	// Desktop provider has no token for this server.
	cleanStaleDockerPassEntriesFunc(ctx, app)

	// Revoke tokens via Docker Desktop
	if err := desktopDeleteOAuthAppFunc(ctx, app); err != nil {
		return fmt.Errorf("failed to revoke OAuth access: %w", err)
	}

	fmt.Printf("OAuth access revoked for %s\n", app)

	// Clean up DCR entry if the server is not in any profile.
	// If the server is still in a profile, keep the DCR entry so it
	// remains visible in the OAuth UI for re-authorization.
	dao, err := db.New()
	if err == nil {
		workingset.CleanupOrphanedDCREntries(ctx, dao, []string{app}, nil)
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

// revokeCommunityMode handles revoke for community servers in Desktop mode.
// Deletes the OAuth token and DCR client from docker pass, and cleans up
// any stale Desktop Secrets Engine entries left from prior Desktop OAuth
// authorizations (when the McpGatewayOAuth flag was OFF).
func revokeCommunityMode(ctx context.Context, app string) error {
	// Delete OAuth token from docker pass
	if err := deleteOAuthTokenFunc(ctx, app); err != nil {
		// Token might not exist, continue to DCR deletion
		fmt.Printf("Note: %v\n", err)
	}

	// Delete DCR client from docker pass (soft failure -- entry may not exist
	// if authorize was never completed or was already revoked)
	if err := deleteDCRClientFunc(ctx, app); err != nil {
		fmt.Printf("Note: %v\n", err)
	}

	// Best-effort cleanup of stale Desktop Secrets Engine entries. When the
	// flag was previously OFF, Desktop may have stored oauth/dcr entries for
	// this community server. Removing them prevents the Secrets Engine from
	// returning stale Desktop-managed tokens that shadow docker pass entries.
	cleanStaleDesktopEntriesFunc(ctx, app)

	fmt.Printf("OAuth access revoked for %s\n", app)
	return nil
}
