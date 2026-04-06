package oauth

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

// mockRevokeRouting overrides the function pointers so Revoke() does not
// contact Docker Desktop, credential helpers, or the catalog. The returned
// string pointer records which handler was dispatched.
func mockRevokeRouting(t *testing.T) *string {
	t.Helper()
	oldLookup := lookupIsCommunityFunc
	oldIsCE := isCEModeFunc
	oldDetermineMode := determineModeFunc
	oldCE := revokeCEModeFunc
	oldDesktop := revokeDesktopModeFunc
	oldCommunity := revokeCommunityModeFunc

	t.Cleanup(func() {
		lookupIsCommunityFunc = oldLookup
		isCEModeFunc = oldIsCE
		determineModeFunc = oldDetermineMode
		revokeCEModeFunc = oldCE
		revokeDesktopModeFunc = oldDesktop
		revokeCommunityModeFunc = oldCommunity
	})

	var called string
	revokeCEModeFunc = func(_ context.Context, _ string) error {
		called = "ce"
		return nil
	}
	revokeDesktopModeFunc = func(_ context.Context, _ string) error {
		called = "desktop"
		return nil
	}
	revokeCommunityModeFunc = func(_ context.Context, _ string) error {
		called = "community"
		return nil
	}
	return &called
}

// TestRevoke_CEMode_CatalogLookupFails verifies that when the server is not
// found in the catalog AND we are in CE mode, the revoke falls back to CE.
func TestRevoke_CEMode_CatalogLookupFails(t *testing.T) {
	called := mockRevokeRouting(t)
	isCEModeFunc = func() bool { return true }
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("server not found")
	}

	err := Revoke(t.Context(), "unknown-server")
	require.NoError(t, err)
	assert.Equal(t, "ce", *called)
}

// TestRevoke_DesktopMode_CatalogLookupFails verifies that when the server
// is not found in the catalog AND we are NOT in CE mode, the revoke falls
// back to Desktop.
func TestRevoke_DesktopMode_CatalogLookupFails(t *testing.T) {
	called := mockRevokeRouting(t)
	isCEModeFunc = func() bool { return false }
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("server not found")
	}

	err := Revoke(t.Context(), "unknown-server")
	require.NoError(t, err)
	assert.Equal(t, "desktop", *called)
}

// TestRevoke_CatalogServer_DesktopMode verifies that a catalog (non-community)
// server in Desktop mode routes to revokeDesktopMode.
func TestRevoke_CatalogServer_DesktopMode(t *testing.T) {
	called := mockRevokeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return false, nil // catalog server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeDesktop
	}

	err := Revoke(t.Context(), "catalog-server")
	require.NoError(t, err)
	assert.Equal(t, "desktop", *called)
}

// TestRevoke_CommunityServer_FlagOn verifies that a community server with
// the McpGatewayOAuth flag ON routes to revokeCommunityMode.
func TestRevoke_CommunityServer_FlagOn(t *testing.T) {
	called := mockRevokeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return true, nil // community server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeCommunity
	}

	err := Revoke(t.Context(), "community-server")
	require.NoError(t, err)
	assert.Equal(t, "community", *called)
}

// TestRevoke_CommunityServer_FlagOff verifies that a community server with
// the McpGatewayOAuth flag OFF routes to revokeDesktopMode.
func TestRevoke_CommunityServer_FlagOff(t *testing.T) {
	called := mockRevokeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return true, nil // community server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeDesktop
	}

	err := Revoke(t.Context(), "community-server")
	require.NoError(t, err)
	assert.Equal(t, "desktop", *called)
}

// TestRevoke_CEMode_CommunityServer verifies that CE mode routes all
// servers through revokeCEMode regardless of community status.
func TestRevoke_CEMode_CommunityServer(t *testing.T) {
	called := mockRevokeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return true, nil // community server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeCE
	}

	err := Revoke(t.Context(), "community-server")
	require.NoError(t, err)
	assert.Equal(t, "ce", *called)
}
