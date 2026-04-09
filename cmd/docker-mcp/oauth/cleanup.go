package oauth

import (
	"context"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/log"
)

// Function pointers for testability.
var (
	cleanStaleDesktopEntriesFunc    = cleanStaleDesktopEntries
	cleanStaleDockerPassEntriesFunc = cleanStaleDockerPassEntries
)

// cleanStaleDesktopEntries removes OAuth and DCR entries from the Desktop
// Secrets Engine for a server. This prevents the docker-desktop-mcp-oauth
// plugin (pattern docker/mcp/oauth/**) from shadowing fresh docker-pass
// tokens (pattern **) when both stores have entries for the same key.
// Best-effort: errors are silently ignored because the entry may not exist
// (normal case for first-time authorizations or single-store workflows).
func cleanStaleDesktopEntries(ctx context.Context, app string) {
	client := desktop.NewAuthClient()
	if err := client.DeleteOAuthApp(ctx, app); err != nil {
		log.Logf("Warning: failed to clean stale Desktop entry for %s: %v", app, err)
	}
}

// cleanStaleDockerPassEntries removes OAuth token and DCR client entries
// from docker pass for a server. This cleans up entries left behind when
// a server was authorized with the Gateway OAuth flag ON but revoked with
// the flag OFF (or vice versa).
// Only attempts deletion when the key actually exists in docker pass to
// avoid noisy error output during normal single-store workflows.
func cleanStaleDockerPassEntries(ctx context.Context, app string) {
	keys, err := secret.List(ctx)
	if err != nil {
		return
	}
	oauthKey := secret.GetOAuthKey(app)
	dcrKey := secret.GetDCRKey(app)
	for _, k := range keys {
		if k == oauthKey {
			if err := secret.DeleteOAuthToken(ctx, app); err != nil {
				log.Logf("Warning: failed to clean stale docker pass OAuth token for %s: %v", app, err)
			}
		}
		if k == dcrKey {
			if err := secret.DeleteDCRClient(ctx, app); err != nil {
				log.Logf("Warning: failed to clean stale docker pass DCR client for %s: %v", app, err)
			}
		}
	}
}
