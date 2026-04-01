package workingset

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

// registerWithSnapshotFunc and registerForDynamicDiscoveryFunc are the
// Desktop registration functions. Tests can override these to avoid
// requiring a running Docker Desktop backend.
var (
	registerWithSnapshotFunc       = oauth.RegisterProviderWithSnapshot
	registerForDynamicDiscoveryFunc = oauth.RegisterProviderForDynamicDiscovery
)

// RegisterOAuthProvidersForServers registers OAuth providers with Docker Desktop
// for any remote OAuth servers in the list. This enables servers to appear
// in the OAuth tab for authorization.
//
// This function is idempotent and safe to call multiple times for the same servers.
// When the Gateway owns OAuth for a server (CE mode, or Desktop + community
// server + McpGatewayOAuth flag ON), Desktop registration is skipped — DCR
// happens during the authorize command instead.
func RegisterOAuthProvidersForServers(ctx context.Context, servers []Server) {
	// CE mode: all OAuth is Gateway-owned, skip Desktop registration entirely.
	if oauth.IsCEMode() {
		return
	}

	for _, server := range servers {
		if server.Snapshot == nil {
			continue
		}

		// Desktop mode: skip registration for community servers when the
		// McpGatewayOAuth flag is ON — Gateway owns OAuth for those.
		if server.Snapshot.Server.IsCommunity() {
			if oauth.ShouldUseGatewayOAuth(ctx, true) {
				continue
			}
		}

		if server.Snapshot.Server.HasExplicitOAuthProviders() {
			serverName := server.Snapshot.Server.Name
			providerName := server.Snapshot.Server.OAuth.Providers[0].Provider

			if err := registerWithSnapshotFunc(ctx, serverName, providerName); err != nil {
				log.Log(fmt.Sprintf("Warning: Failed to register OAuth provider for %s: %v", serverName, err))
			}
		} else if server.Snapshot.Server.Type == "remote" && server.Snapshot.Server.Remote.URL != "" {
			// Remote servers without oauth.providers: probe for OAuth dynamically
			serverName := server.Snapshot.Server.Name
			serverURL := server.Snapshot.Server.Remote.URL
			if err := registerForDynamicDiscoveryFunc(ctx, serverName, serverURL); err != nil {
				log.Log(fmt.Sprintf("Warning: Failed dynamic OAuth discovery for %s: %v", serverName, err))
			}
		}
	}
}
