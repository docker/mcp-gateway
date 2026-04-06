package server

import (
	"context"
	"testing"

	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOAuthHooks overrides the OAuth function pointers for the enable path.
// Returns pointers to track whether lazy setup and dynamic discovery were called.
func mockOAuthHooks(t *testing.T, gatewayOwns bool) (lazySetupCalled *[]string, dynamicDiscoveryCalled *[]string) {
	t.Helper()
	oldShouldUse := shouldUseGatewayOAuthForEnableFunc
	oldLazy := registerProviderForLazySetupFunc
	oldDiscovery := registerForDynamicDiscoveryEnableFunc

	t.Cleanup(func() {
		shouldUseGatewayOAuthForEnableFunc = oldShouldUse
		registerProviderForLazySetupFunc = oldLazy
		registerForDynamicDiscoveryEnableFunc = oldDiscovery
	})

	shouldUseGatewayOAuthForEnableFunc = func(_ context.Context, _ bool) bool {
		return gatewayOwns
	}

	var lazyCalls, discoveryCalls []string
	registerProviderForLazySetupFunc = func(_ context.Context, serverName string) error {
		lazyCalls = append(lazyCalls, serverName)
		return nil
	}
	registerForDynamicDiscoveryEnableFunc = func(_ context.Context, serverName, _ string) error {
		discoveryCalls = append(discoveryCalls, serverName)
		return nil
	}

	return &lazyCalls, &discoveryCalls
}

// catalogWithCommunityOAuthServer returns a catalog YAML with a community
// server that has explicit OAuth providers.
func catalogWithCommunityOAuthServer() string {
	return `registry:
  community-oauth:
    type: remote
    description: "Community OAuth server"
    remote:
      url: "https://community.example.com/mcp"
    oauth:
      providers:
        - provider: github
    metadata:
      tags:
        - community`
}

// catalogWithCatalogOAuthServer returns a catalog YAML with a catalog
// server that has explicit OAuth providers (no community tag).
func catalogWithCatalogOAuthServer() string {
	return `registry:
  catalog-oauth:
    type: remote
    description: "Catalog OAuth server"
    remote:
      url: "https://catalog.example.com/mcp"
    oauth:
      providers:
        - provider: github`
}

// catalogWithCommunityRemoteServer returns a catalog YAML with a community
// remote server that has no explicit OAuth (dynamic discovery candidate).
func catalogWithCommunityRemoteServer() string {
	return `registry:
  community-remote:
    type: remote
    description: "Community remote server"
    remote:
      url: "https://community.example.com/mcp"
    metadata:
      tags:
        - community`
}

// TestEnable_OAuthServer_GatewayOwns verifies that when Gateway owns OAuth
// (CE mode or community + flag ON), no Desktop registration occurs.
func TestEnable_OAuthServer_GatewayOwns(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t,
		withEmptyRegistryYaml(),
		withCatalog(catalogWithCommunityOAuthServer()),
		withEmptyConfig(),
	)

	lazyCalls, discoveryCalls := mockOAuthHooks(t, true)

	err := Enable(ctx, docker, dockerCli, []string{"community-oauth"}, true)
	require.NoError(t, err)

	assert.Empty(t, *lazyCalls,
		"Gateway-owned OAuth should not call RegisterProviderForLazySetup")
	assert.Empty(t, *discoveryCalls,
		"Gateway-owned OAuth should not call RegisterProviderForDynamicDiscovery")
}

// TestEnable_OAuthServer_DesktopCatalog verifies that a catalog server in
// Desktop mode calls RegisterProviderForLazySetup.
func TestEnable_OAuthServer_DesktopCatalog(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t,
		withEmptyRegistryYaml(),
		withCatalog(catalogWithCatalogOAuthServer()),
		withEmptyConfig(),
	)

	lazyCalls, _ := mockOAuthHooks(t, false)

	err := Enable(ctx, docker, dockerCli, []string{"catalog-oauth"}, true)
	require.NoError(t, err)

	assert.Equal(t, []string{"catalog-oauth"}, *lazyCalls,
		"Desktop mode should call RegisterProviderForLazySetup for catalog OAuth server")
}

// TestEnable_OAuthServer_DesktopCommunityFlagOn verifies that a community
// server with flag ON does not call Desktop registration.
func TestEnable_OAuthServer_DesktopCommunityFlagOn(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t,
		withEmptyRegistryYaml(),
		withCatalog(catalogWithCommunityOAuthServer()),
		withEmptyConfig(),
	)

	// shouldUseGatewayOAuthForEnableFunc = true simulates flag ON for community
	lazyCalls, _ := mockOAuthHooks(t, true)

	err := Enable(ctx, docker, dockerCli, []string{"community-oauth"}, true)
	require.NoError(t, err)

	assert.Empty(t, *lazyCalls,
		"community + flag ON should not call Desktop registration")
}

// TestEnable_DynamicDiscovery_GatewayOwns verifies that a community remote
// server without explicit OAuth does not trigger dynamic discovery when
// Gateway owns OAuth.
func TestEnable_DynamicDiscovery_GatewayOwns(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t,
		withEmptyRegistryYaml(),
		withCatalog(catalogWithCommunityRemoteServer()),
		withEmptyConfig(),
	)

	_, discoveryCalls := mockOAuthHooks(t, true)

	err := Enable(ctx, docker, dockerCli, []string{"community-remote"}, true)
	require.NoError(t, err)

	assert.Empty(t, *discoveryCalls,
		"Gateway-owned OAuth should not trigger dynamic discovery")
}

// Verify the function pointers are wired to the real functions by default.
func TestEnableOAuth_FunctionPointersDefault(t *testing.T) {
	// These should point to the real implementations, not nil.
	assert.NotNil(t, shouldUseGatewayOAuthForEnableFunc)
	assert.NotNil(t, registerProviderForLazySetupFunc)
	assert.NotNil(t, registerForDynamicDiscoveryEnableFunc)

	// Verify shouldUseGatewayOAuthForEnableFunc is the real function by
	// checking that it matches the real function's behavior in CE mode.
	t.Setenv("DOCKER_MCP_USE_CE", "true")
	assert.True(t, shouldUseGatewayOAuthForEnableFunc(t.Context(), false),
		"default shouldUseGatewayOAuthForEnableFunc should match pkgoauth.ShouldUseGatewayOAuth")
	// Restore so it doesn't affect later test runs.
	_ = pkgoauth.ShouldUseGatewayOAuth
}
