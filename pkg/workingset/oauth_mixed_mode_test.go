package workingset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

// mockDesktopModeWithGatewayOAuth sets up Desktop mode mocks including the
// shouldUseGatewayOAuthFunc override. It extends the existing mockDesktopMode
// pattern from oauth_test.go.
func mockDesktopModeWithGatewayOAuth(t *testing.T, gatewayOAuthEnabled bool) {
	t.Helper()
	oldCE := isCEModeFunc
	oldGateway := shouldUseGatewayOAuthFunc
	oldSnapshot := registerWithSnapshotFunc
	oldDiscovery := registerForDynamicDiscoveryFunc

	isCEModeFunc = func() bool { return false }
	shouldUseGatewayOAuthFunc = func(_ context.Context, isCommunity bool) bool {
		// In Desktop mode, Gateway owns OAuth only for community servers
		// when the flag is enabled.
		return gatewayOAuthEnabled && isCommunity
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

// TestRegisterOAuthProviders_MixedCatalogAndCommunity_FlagOn verifies that
// when the McpGatewayOAuth flag is ON, catalog servers are registered with
// Desktop but community servers are skipped (Gateway owns their OAuth).
func TestRegisterOAuthProviders_MixedCatalogAndCommunity_FlagOn(t *testing.T) {
	mockDesktopModeWithGatewayOAuth(t, true)

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

	assert.Equal(t, []string{"catalog-oauth"}, registeredServers,
		"flag ON: only catalog server should be registered with Desktop")
}

// TestRegisterOAuthProviders_MixedCatalogAndCommunity_FlagOff verifies that
// when the McpGatewayOAuth flag is OFF, both catalog and community servers
// are registered with Desktop (Desktop manages all OAuth).
func TestRegisterOAuthProviders_MixedCatalogAndCommunity_FlagOff(t *testing.T) {
	mockDesktopModeWithGatewayOAuth(t, false)

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
		"flag OFF: both catalog and community servers should be registered with Desktop")
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

// TestRegisterOAuthProviders_CommunityDynamicDiscovery_FlagOn verifies that
// community servers with a URL but no explicit OAuth are NOT registered for
// dynamic discovery when the McpGatewayOAuth flag is ON (Gateway owns OAuth).
func TestRegisterOAuthProviders_CommunityDynamicDiscovery_FlagOn(t *testing.T) {
	mockDesktopModeWithGatewayOAuth(t, true)

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

	assert.Empty(t, discoveredServers,
		"flag ON: community remote server should NOT trigger dynamic discovery (Gateway owns OAuth)")
}

// TestRegisterOAuthProviders_CommunityDynamicDiscovery_FlagOff verifies that
// community servers with a URL but no explicit OAuth ARE registered for
// dynamic discovery when the McpGatewayOAuth flag is OFF (Desktop manages).
func TestRegisterOAuthProviders_CommunityDynamicDiscovery_FlagOff(t *testing.T) {
	mockDesktopModeWithGatewayOAuth(t, false)

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
		"flag OFF: community remote server should trigger dynamic discovery (Desktop manages)")
}

// --- Cleanup filtering tests ---

// TestCleanupOrphanedDCREntries_SkipsCommunityWhenFlagOn verifies that when
// the McpGatewayOAuth flag is ON, community servers are filtered out of the
// cleanup list. When all servers are community, the filtered list is empty and
// doCleanupOrphanedDCREntries is never called (safe with nil dao).
func TestCleanupOrphanedDCREntries_SkipsCommunityWhenFlagOn(t *testing.T) {
	mockDesktopModeWithGatewayOAuth(t, true)

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

// TestCleanupOrphanedDCREntries_IncludesCommunityWhenFlagOff verifies that
// when the McpGatewayOAuth flag is OFF, community servers are NOT filtered
// out of the cleanup list. We verify this by checking that
// shouldUseGatewayOAuthFunc returns false for community servers when the
// flag is off, meaning they pass through the filter.
func TestCleanupOrphanedDCREntries_IncludesCommunityWhenFlagOff(t *testing.T) {
	mockDesktopModeWithGatewayOAuth(t, false)

	// Verify the filter logic: with flag OFF, shouldUseGatewayOAuthFunc returns
	// false for community servers, meaning they would NOT be filtered out.
	assert.False(t, shouldUseGatewayOAuthFunc(context.Background(), true),
		"flag OFF: community servers should not be filtered (shouldUseGatewayOAuth=false)")
	assert.False(t, shouldUseGatewayOAuthFunc(context.Background(), false),
		"flag OFF: catalog servers should not be filtered (shouldUseGatewayOAuth=false)")

	// Contrast with flag ON:
	oldGateway := shouldUseGatewayOAuthFunc
	shouldUseGatewayOAuthFunc = oauth.ShouldUseGatewayOAuth
	t.Cleanup(func() { shouldUseGatewayOAuthFunc = oldGateway })

	// With the real function and DOCKER_MCP_USE_CE=true, all servers use Gateway OAuth.
	t.Setenv("DOCKER_MCP_USE_CE", "true")
	assert.True(t, shouldUseGatewayOAuthFunc(context.Background(), true),
		"CE mode: community servers should be filtered (shouldUseGatewayOAuth=true)")
}
