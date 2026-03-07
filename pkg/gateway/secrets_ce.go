package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

// readCEModeOAuthSecrets reads OAuth tokens from the credential helper in CE mode.
// In CE mode, Docker Desktop's secret mechanisms (jcat, Secrets Engine, se:// URIs)
// are not available. Instead, OAuth tokens are stored in the system credential helper
// (e.g., macOS Keychain) by `docker mcp oauth authorize`. This function reads those
// tokens and maps them to the secret names expected by server definitions.
func readCEModeOAuthSecrets(ctx context.Context, servers map[string]catalog.Server, serverNames []string) map[string]string {
	secrets := make(map[string]string)

	if !oauth.IsCEMode() {
		return secrets
	}

	credHelper := oauth.NewOAuthCredentialHelper()

	for _, serverName := range serverNames {
		server, ok := servers[serverName]
		if !ok || server.OAuth == nil {
			continue
		}

		for _, provider := range server.OAuth.Providers {
			token, err := credHelper.GetOAuthToken(ctx, provider.Provider)
			if err != nil {
				log.Logf("  - CE mode: no OAuth token for provider %s: %v", provider.Provider, err)
				continue
			}
			if token != "" {
				secrets[provider.Secret] = token
				log.Logf("  - CE mode: resolved OAuth token for %s -> %s", provider.Provider, provider.Secret)
			}
		}
	}

	return secrets
}
