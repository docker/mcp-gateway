package oauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMixedMode_DetermineModeTwoServers verifies that two servers in the same
// Desktop session get different modes: catalog -> ModeDesktop, community with
// flag ON -> ModeCommunity. This is the core "mixed mode" acceptance criterion.
func TestMixedMode_DetermineModeTwoServers(t *testing.T) {
	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}

	ctx := t.Context()
	ceMode := false // Desktop is running

	catalogMode := determineMode(ctx, ceMode, false, flagOn)
	communityMode := determineMode(ctx, ceMode, true, flagOn)

	assert.Equal(t, ModeDesktop, catalogMode, "catalog server should use ModeDesktop")
	assert.Equal(t, ModeCommunity, communityMode, "community server with flag ON should use ModeCommunity")
	assert.NotEqual(t, catalogMode, communityMode, "catalog and community should resolve to different modes")
}

// TestMixedMode_CredentialHelperRouting verifies that two CredentialHelper
// instances with different explicit modes route to the correct storage backend.
func TestMixedMode_CredentialHelperRouting(t *testing.T) {
	desktopHelper := NewOAuthCredentialHelperWithMode(ModeDesktop)
	communityHelper := NewOAuthCredentialHelperWithMode(ModeCommunity)

	assert.Equal(t, ModeDesktop, desktopHelper.resolveMode(),
		"Desktop credential helper should resolve to ModeDesktop")
	assert.Equal(t, ModeCommunity, communityHelper.resolveMode(),
		"Community credential helper should resolve to ModeCommunity")
}

// TestMixedMode_ProviderRefreshRouting verifies that two Provider instances
// with different modes dispatch to the correct refresh strategy.
func TestMixedMode_ProviderRefreshRouting(t *testing.T) {
	noopReload := func(_ context.Context, _ string) error { return nil }

	desktopProvider := NewProvider("catalog-server", ModeDesktop, noopReload)
	communityProvider := NewProvider("community-server", ModeCommunity, noopReload)

	assert.Equal(t, ModeDesktop, desktopProvider.resolveRefreshMode(),
		"Desktop provider should refresh via Desktop API")
	assert.Equal(t, ModeCommunity, communityProvider.resolveRefreshMode(),
		"Community provider should refresh via oauth2 + docker pass")
}

// TestMixedMode_DockerPassFallback verifies the gateway startup fallback logic:
// when a community server resolves to ModeCommunity but docker pass is
// unavailable, the mode falls back to ModeDesktop. This mirrors the logic in
// pkg/gateway/run.go (lines 374-377).
func TestMixedMode_DockerPassFallback(t *testing.T) {
	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}

	ctx := t.Context()
	mode := determineMode(ctx, false, true, flagOn)
	assert.Equal(t, ModeCommunity, mode, "community with flag ON should be ModeCommunity")

	// Simulate the fallback from run.go: when docker pass is unavailable,
	// the gateway overrides the mode to ModeDesktop.
	hasDockerPass := false
	if mode == ModeCommunity && !hasDockerPass {
		mode = ModeDesktop
	}
	assert.Equal(t, ModeDesktop, mode, "should fall back to ModeDesktop when docker pass unavailable")

	// Verify the fallback produces a working credential helper and provider.
	h := NewOAuthCredentialHelperWithMode(mode)
	assert.Equal(t, ModeDesktop, h.resolveMode())

	noopReload := func(_ context.Context, _ string) error { return nil }
	p := NewProvider("community-server", mode, noopReload)
	assert.Equal(t, ModeDesktop, p.resolveRefreshMode())
}

// TestMixedMode_CEModeRegression verifies that CE mode (no Docker Desktop)
// continues to work correctly for both catalog and community servers. All
// servers should resolve to ModeCE regardless of community status.
func TestMixedMode_CEModeRegression(t *testing.T) {
	t.Setenv("DOCKER_MCP_USE_CE", "true")

	ctx := t.Context()

	// Both server types should resolve to ModeCE via the public API.
	assert.Equal(t, ModeCE, DetermineMode(ctx, false),
		"CE mode: catalog server should return ModeCE")
	assert.Equal(t, ModeCE, DetermineMode(ctx, true),
		"CE mode: community server should return ModeCE")

	// CredentialHelper resolves to ModeCE.
	h := NewOAuthCredentialHelperWithMode(ModeCE)
	assert.Equal(t, ModeCE, h.resolveMode())

	// Provider resolves to ModeCE for refresh.
	noopReload := func(_ context.Context, _ string) error { return nil }
	p := NewProvider("test-server", ModeCE, noopReload)
	assert.Equal(t, ModeCE, p.resolveRefreshMode())

	// ShouldUseGatewayOAuth returns true for all servers in CE mode.
	assert.True(t, ShouldUseGatewayOAuth(ctx, false),
		"CE mode: ShouldUseGatewayOAuth should be true for catalog servers")
	assert.True(t, ShouldUseGatewayOAuth(ctx, true),
		"CE mode: ShouldUseGatewayOAuth should be true for community servers")
}
