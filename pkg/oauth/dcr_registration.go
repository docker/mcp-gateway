package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

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

// RegisterProviderWithSnapshot registers a DCR provider using OAuth metadata from the server snapshot
// This avoids querying the catalog since the snapshot already contains all necessary OAuth information
// Idempotent - safe to call multiple times for the same server
func RegisterProviderWithSnapshot(ctx context.Context, serverName, providerName string) error {
	client := desktop.NewAuthClient()

	// Idempotent check - already registered?
	_, err := client.GetDCRClient(ctx, serverName)
	if err == nil {
		return nil // Already registered
	}

	// Register with Docker Desktop (pending DCR state)
	dcrRequest := desktop.RegisterDCRRequest{
		ProviderName: providerName,
	}

	return client.RegisterDCRClientPending(ctx, serverName, dcrRequest)
}
