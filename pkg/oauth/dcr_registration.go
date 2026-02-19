package oauth

import (
	"context"
	"fmt"
	"time"

	oauthhelpers "github.com/docker/mcp-gateway-oauth-helpers"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

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

	// Verify this is a remote OAuth server (Type="remote" && OAuth providers exist)
	if !server.IsRemoteOAuthServer() {
		return fmt.Errorf("server %s is not a remote OAuth server", serverName)
	}

	providerName := server.OAuth.Providers[0].Provider

	// Register with DD (pending DCR state)
	dcrRequest := desktop.RegisterDCRRequest{
		ProviderName: providerName,
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

	// Probe the server with a timeout to discover OAuth requirements
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	discovery, err := prober.DiscoverOAuthRequirements(probeCtx, serverURL)
	if err != nil || !discovery.RequiresOAuth {
		return nil // Server doesn't need OAuth, not an error
	}

	// Register with DD (pending DCR state) using server name as provider name
	return client.RegisterDCRClientPending(ctx, serverName, desktop.RegisterDCRRequest{
		ProviderName: serverName,
	})
}

// RegisterProviderWithSnapshot registers a DCR provider using OAuth metadata from the server snapshot
// This avoids querying the catalog since the snapshot already contains all necessary OAuth information
// Idempotent - safe to call multiple times for the same server
func RegisterProviderWithSnapshot(ctx context.Context, serverName, providerName string) error {
	client := desktop.NewAuthClient()

	// Idempotent check - already registered?
	c, err := client.GetDCRClient(ctx, serverName)
	if err == nil && c.State == "registered" {
		return nil // Already registered
	}

	// Register with Docker Desktop (pending DCR state)
	dcrRequest := desktop.RegisterDCRRequest{
		ProviderName: providerName,
	}

	return client.RegisterDCRClientPending(ctx, serverName, dcrRequest)
}
