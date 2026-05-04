package workingset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

// mockDesktopModeForMixedTests sets up Desktop mode mocks including the
// shouldUseGatewayOAuthFunc override. It extends the existing mockDesktopModeForMixedTests
// pattern from oauth_test.go.
func mockDesktopModeForMixedTests(t *testing.T) {
	t.Helper()
	oldCE := isCEModeFunc
	oldGateway := shouldUseGatewayOAuthFunc
	oldSnapshot := registerWithSnapshotFunc
	oldDiscovery := registerForDynamicDiscoveryFunc

	isCEModeFunc = func() bool { return false }
	shouldUseGatewayOAuthFunc = func(_ context.Context, isCommunity bool) bool {
		// In Desktop mode, Gateway owns OAuth for community servers.
		return isCommunity
	}

	t.Cleanup(func() {
		isCEModeFunc = oldCE
		shouldUseGatewayOAuthFunc = oldGateway
		registerWithSnapshotFunc = oldSnapshot
		registerForDynamicDiscoveryFunc = oldDiscovery
	})
}

// newCatalogServer creates a test Server with explicit OAuth (catalog-style).
func newCatalogServer(name string) Server {
	return Server{
		Type: ServerTypeRemote,
		Snapshot: &ServerSnapshot{
			Server: catalog.Server{
				Name: name,
				Type: "remote",
				OAuth: &catalog.OAuth{
					Providers: []catalog.OAuthProvider{{Provider: "github"}},
				},
			},
		},
	}
}

// newCommunityServer creates a test Server tagged as community with explicit OAuth.
func newCommunityServer(name string) Server {
	return Server{
		Type: ServerTypeRemote,
		Snapshot: &ServerSnapshot{
			Server: catalog.Server{
				Name: name,
				Type: "remote",
				Metadata: &catalog.Metadata{
					Tags: []string{"community"},
				},
				OAuth: &catalog.OAuth{
					Providers: []catalog.OAuthProvider{{Provider: "notion"}},
				},
			},
		},
	}
}

// --- Registration tests ---

// TestRegisterOAuthProviders_MixedCatalogAndCommunity verifies that catalog
// and community servers are both registered with Desktop for OAuth tab visibility.
func TestRegisterOAuthProviders_MixedCatalogAndCommunity(t *testing.T) {
	mockDesktopModeForMixedTests(t)

	var registeredServers []string
	registerWithSnapshotFunc = func(_ context.Context, serverName, _ string) error {
		registeredServers = append(registeredServers, serverName)
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, _, _ string) error {
		return nil
	}

	servers := []Server{
		newCatalogServer("catalog-oauth"),
		newCommunityServer("community-oauth"),
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, []string{"catalog-oauth", "community-oauth"}, registeredServers,
		"both catalog and community servers should be registered with Desktop for OAuth tab visibility")
}

// --- Dynamic discovery tests ---

// newCommunityRemoteServer creates a community server with a remote URL but
// no explicit OAuth providers. This is a dynamic discovery candidate.
func newCommunityRemoteServer(name, url string) Server {
	return Server{
		Type: ServerTypeRemote,
		Snapshot: &ServerSnapshot{
			Server: catalog.Server{
				Name:   name,
				Type:   "remote",
				Remote: catalog.Remote{URL: url},
				Metadata: &catalog.Metadata{
					Tags: []string{"community"},
				},
			},
		},
	}
}

// TestRegisterOAuthProviders_CommunityDynamicDiscovery verifies that
// community servers with a URL but no explicit OAuth ARE registered for
// dynamic discovery. Registration is for OAuth tab visibility -- Gateway
// still owns the authorize/revoke lifecycle.
func TestRegisterOAuthProviders_CommunityDynamicDiscovery(t *testing.T) {
	mockDesktopModeForMixedTests(t)

	var discoveredServers []string
	registerWithSnapshotFunc = func(_ context.Context, _, _ string) error {
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, serverName, _ string) error {
		discoveredServers = append(discoveredServers, serverName)
		return nil
	}

	servers := []Server{
		newCommunityRemoteServer("community-remote", "https://example.com/mcp"),
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, []string{"community-remote"}, discoveredServers,
		"community remote server should trigger dynamic discovery for OAuth tab visibility")
}

// --- Cleanup filtering tests ---

// TestCleanupOrphanedDCREntries_SkipsCommunity verifies that community
// servers are filtered out of the cleanup list. When all servers are
// community, the filtered list is empty and doCleanupOrphanedDCREntries
// is never called (safe with nil dao).
func TestCleanupOrphanedDCREntries_SkipsCommunity(t *testing.T) {
	mockDesktopModeForMixedTests(t)

	communityServers := map[string]bool{
		"comm-a": true,
		"comm-b": true,
	}

	// Pass nil dao: if the filter works correctly, doCleanupOrphanedDCREntries
	// is never reached (filtered list is empty), so nil dao is safe.
	CleanupOrphanedDCREntries(
		context.Background(),
		nil, // dao -- not reached when filtered list is empty
		[]string{"comm-a", "comm-b"},
		communityServers,
	)
	// No panic = community servers were correctly filtered out.
}
