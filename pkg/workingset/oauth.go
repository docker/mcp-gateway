package workingset

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

// RegisterOAuthProvidersForServers registers OAuth providers with Docker Desktop
// for any remote OAuth servers in the list. This enables servers to appear
// in the OAuth tab for authorization.
//
// This function is idempotent and safe to call multiple times for the same servers.
// In CE mode, this is a no-op since OAuth DCR happens during the authorize command.
func RegisterOAuthProvidersForServers(ctx context.Context, servers []Server) {
	// Skip in CE mode - DCR happens during oauth authorize command
	if oauth.IsCEMode() {
		return
	}

	for _, server := range servers {
		if server.Snapshot == nil {
			continue
		}
		if !server.Snapshot.Server.IsRemoteOAuthServer() {
			continue
		}

		serverName := server.Snapshot.Server.Name
		if err := oauth.RegisterProviderForLazySetup(ctx, serverName); err != nil {
			// Log warning but don't fail - user can authorize later via CLI
			log.Log(fmt.Sprintf("Warning: Failed to register OAuth provider for %s: %v", serverName, err))
		}
	}
}
