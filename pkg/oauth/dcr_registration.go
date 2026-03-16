package oauth

import (
	"context"
	"fmt"

	oauthhelpers "github.com/docker/mcp-gateway-oauth-helpers"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/log"
)

// clientSecretSuffix is the naming convention for OAuth client secrets in the secrets store.
const clientSecretSuffix = ".client_secret"

// dcrRegistrationClient is the subset of desktop.Tools used for DCR registration.
// Extracted as an interface to enable testing.
type dcrRegistrationClient interface {
	GetDCRClient(ctx context.Context, app string) (*desktop.DCRClient, error)
	RegisterDCRClientPending(ctx context.Context, app string, req desktop.RegisterDCRRequest) error
}

// oauthProber abstracts OAuth discovery to enable testing.
type oauthProber interface {
	DiscoverOAuthRequirements(ctx context.Context, serverURL string) (*oauthhelpers.Discovery, error)
}

// defaultOAuthProber wraps the package-level function.
type defaultOAuthProber struct{}

func (defaultOAuthProber) DiscoverOAuthRequirements(ctx context.Context, serverURL string) (*oauthhelpers.Discovery, error) {
	return oauthhelpers.DiscoverOAuthRequirements(ctx, serverURL)
}

// RegisterProviderForLazySetup registers a DCR provider with Docker Desktop
// This allows 'docker mcp oauth authorize' to work before full DCR is complete
// Idempotent - safe to call multiple times for the same server
func RegisterProviderForLazySetup(ctx context.Context, serverName string) error {
	client := desktop.NewAuthClient()

	// Idempotent check - already registered?
	_, err := client.GetDCRClient(ctx, serverName)
	if err == nil {
		return nil // Already registered
	}

	// Get server from catalog
	catalogData, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := catalogData.Servers[serverName]
	if !found {
		return fmt.Errorf("server %s not found in catalog", serverName)
	}

	// Verify this server has OAuth providers (remote with explicit providers, or pre-registered)
	if !server.HasExplicitOAuthProviders() && !server.HasPreRegisteredOAuth() {
		return fmt.Errorf("server %s does not have OAuth providers configured", serverName)
	}

	provider := server.OAuth.Providers[0]

	// Build DCR request with pre-registered metadata if available.
	// When registration + server_metadata are provided (from a catalog with
	// embedded OAuth metadata), Pinata skips DCR discovery and uses them directly.
	dcrRequest := desktop.RegisterDCRRequest{
		ProviderName: provider.Provider,
	}
	if provider.Registration != nil {
		dcrRequest.ClientID = provider.Registration.ClientID

		// Look up client_secret from the Secrets Engine (user sets it via docker mcp secret set).
		// The catalog only contains client_id; the secret is never distributed in catalogs.
		clientSecretKey := secret.GetDefaultSecretKey(serverName + clientSecretSuffix)
		if env, err := secret.GetSecret(ctx, clientSecretKey); err == nil && string(env.Value) != "" {
			dcrRequest.ClientSecret = string(env.Value)
			log.Logf("- Registering pre-configured OAuth client for %s with client_secret", serverName)
		} else {
			log.Logf("- Registering pre-configured OAuth client for %s without client_secret (not yet set)", serverName)
		}
	}
	if provider.ServerMetadata != nil {
		dcrRequest.AuthorizationEndpoint = provider.ServerMetadata.AuthorizationEndpoint
		dcrRequest.TokenEndpoint = provider.ServerMetadata.TokenEndpoint
		dcrRequest.Scopes = provider.ServerMetadata.ScopesSupported
	}

	return client.RegisterDCRClientPending(ctx, serverName, dcrRequest)
}

// RegisterProviderForDynamicDiscovery probes a remote server for OAuth support
// and creates a pending DCR entry if the server requires OAuth.
// This is used for community servers that lack oauth.providers metadata in the catalog.
// Idempotent - safe to call multiple times for the same server.
func RegisterProviderForDynamicDiscovery(ctx context.Context, serverName, serverURL string) error {
	return registerProviderForDynamicDiscovery(ctx, serverName, serverURL, desktop.NewAuthClient(), defaultOAuthProber{})
}

func registerProviderForDynamicDiscovery(ctx context.Context, serverName, serverURL string, client dcrRegistrationClient, prober oauthProber) error {
	// Idempotent check - already registered?
	_, err := client.GetDCRClient(ctx, serverName)
	if err == nil {
		return nil // Already registered
	}

	// Probe the server to discover OAuth requirements.
	// The discovery library uses its own 30s HTTP timeout internally.
	discovery, err := prober.DiscoverOAuthRequirements(ctx, serverURL)
	if err != nil {
		log.Logf("Dynamic OAuth discovery failed for %s: %v", serverName, err)
		return nil // Probe failed, not fatal
	}
	if discovery == nil || !discovery.RequiresOAuth {
		return nil // Server doesn't need OAuth
	}

	// Register with DD (pending DCR state) using server name as provider name
	return client.RegisterDCRClientPending(ctx, serverName, desktop.RegisterDCRRequest{
		ProviderName: serverName,
	})
}

// RegisterProviderWithSnapshot registers a DCR provider using OAuth metadata from the server snapshot
// This avoids querying the catalog since the snapshot already contains all necessary OAuth information
// Idempotent - safe to call multiple times for the same server
func RegisterProviderWithSnapshot(ctx context.Context, serverName string, provider catalog.OAuthProvider, scopes []string) error {
	client := desktop.NewAuthClient()

	// Idempotent check - already registered?
	c, err := client.GetDCRClient(ctx, serverName)
	if err == nil && c.State == "registered" {
		return nil // Already registered
	}

	// Register with Docker Desktop (pending DCR state)
	// Include pre-registered client metadata if available from catalog
	dcrRequest := desktop.RegisterDCRRequest{
		ProviderName: provider.Provider,
		Scopes:       scopes,
	}
	if provider.Registration != nil {
		dcrRequest.ClientID = provider.Registration.ClientID

		// Look up client_secret from Secrets Engine (not distributed in catalogs)
		clientSecretKey := secret.GetDefaultSecretKey(serverName + clientSecretSuffix)
		if env, err := secret.GetSecret(ctx, clientSecretKey); err == nil && string(env.Value) != "" {
			dcrRequest.ClientSecret = string(env.Value)
		}
	}
	if provider.ServerMetadata != nil {
		dcrRequest.AuthorizationEndpoint = provider.ServerMetadata.AuthorizationEndpoint
		dcrRequest.TokenEndpoint = provider.ServerMetadata.TokenEndpoint
		if len(dcrRequest.Scopes) == 0 {
			dcrRequest.Scopes = provider.ServerMetadata.ScopesSupported
		}
	}

	return client.RegisterDCRClientPending(ctx, serverName, dcrRequest)
}
