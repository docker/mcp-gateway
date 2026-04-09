package oauth

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

// mockAuthorizeRouting overrides the function pointers so Authorize() does not
// contact Docker Desktop, credential helpers, or the catalog. The returned
// string pointer records which handler was dispatched.
func mockAuthorizeRouting(t *testing.T) *string {
	t.Helper()
	oldLookup := lookupIsCommunityFunc
	oldIsCE := isCEModeFunc
	oldDetermineMode := determineModeFunc
	oldCE := authorizeCEModeFunc
	oldDesktop := authorizeDesktopModeFunc
	oldCommunity := authorizeCommunityModeFunc

	t.Cleanup(func() {
		lookupIsCommunityFunc = oldLookup
		isCEModeFunc = oldIsCE
		determineModeFunc = oldDetermineMode
		authorizeCEModeFunc = oldCE
		authorizeDesktopModeFunc = oldDesktop
		authorizeCommunityModeFunc = oldCommunity
	})

	var called string
	authorizeCEModeFunc = func(_ context.Context, _, _ string, _ bool) error {
		called = "ce"
		return nil
	}
	authorizeDesktopModeFunc = func(_ context.Context, _, _ string) error {
		called = "desktop"
		return nil
	}
	authorizeCommunityModeFunc = func(_ context.Context, _, _ string, _ bool) error {
		called = "community"
		return nil
	}
	return &called
}

// TestAuthorize_CEMode_CatalogLookupFails verifies that when the server is not
// found in the catalog AND we are in CE mode, the authorize falls back to CE.
func TestAuthorize_CEMode_CatalogLookupFails(t *testing.T) {
	called := mockAuthorizeRouting(t)
	isCEModeFunc = func() bool { return true }
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("server not found")
	}

	err := Authorize(t.Context(), "unknown-server", "", false)
	require.NoError(t, err)
	assert.Equal(t, "ce", *called)
}

// TestAuthorize_DesktopMode_CatalogLookupFails verifies that when the server
// is not found in the catalog AND we are NOT in CE mode, the authorize falls
// back to Desktop.
func TestAuthorize_DesktopMode_CatalogLookupFails(t *testing.T) {
	called := mockAuthorizeRouting(t)
	isCEModeFunc = func() bool { return false }
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("server not found")
	}

	err := Authorize(t.Context(), "unknown-server", "", false)
	require.NoError(t, err)
	assert.Equal(t, "desktop", *called)
}

// TestAuthorize_CatalogServer_DesktopMode verifies that a catalog (non-community)
// server in Desktop mode routes to authorizeDesktopMode.
func TestAuthorize_CatalogServer_DesktopMode(t *testing.T) {
	called := mockAuthorizeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return false, nil // catalog server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeDesktop
	}

	err := Authorize(t.Context(), "catalog-server", "", false)
	require.NoError(t, err)
	assert.Equal(t, "desktop", *called)
}

// TestAuthorize_CommunityServer_FlagOn verifies that a community server with
// the McpGatewayOAuth flag ON routes to authorizeCommunityMode.
func TestAuthorize_CommunityServer_FlagOn(t *testing.T) {
	called := mockAuthorizeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return true, nil // community server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeCommunity
	}

	err := Authorize(t.Context(), "community-server", "", false)
	require.NoError(t, err)
	assert.Equal(t, "community", *called)
}

// TestAuthorize_CommunityServer_FlagOff verifies that a community server with
// the McpGatewayOAuth flag OFF routes to authorizeDesktopMode.
func TestAuthorize_CommunityServer_FlagOff(t *testing.T) {
	called := mockAuthorizeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return true, nil // community server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeDesktop
	}

	err := Authorize(t.Context(), "community-server", "", false)
	require.NoError(t, err)
	assert.Equal(t, "desktop", *called)
}

// TestAuthorize_CEMode_CommunityServer verifies that CE mode routes all
// servers through authorizeCEMode regardless of community status.
func TestAuthorize_CEMode_CommunityServer(t *testing.T) {
	called := mockAuthorizeRouting(t)
	lookupIsCommunityFunc = func(_ context.Context, _ string) (bool, error) {
		return true, nil // community server
	}
	determineModeFunc = func(_ context.Context, _ bool) pkgoauth.Mode {
		return pkgoauth.ModeCE
	}

	err := Authorize(t.Context(), "community-server", "", false)
	require.NoError(t, err)
	assert.Equal(t, "ce", *called)
}

// TestAuthorizeCommunityMode_NoCleanupOnFailure verifies that
// authorizeCommunityMode does NOT clean stale Desktop entries when the
// authorize flow fails before token storage. This ensures the user
// retains their existing Desktop authorization as a fallback if the
// community flow fails mid-way (port conflict, user closes browser, etc.).
// Cleanup only runs after the fresh token is safely stored in docker pass.
func TestAuthorizeCommunityMode_NoCleanupOnFailure(t *testing.T) {
	// Save and restore all function pointers touched by this test.
	oldDesktopCleanup := cleanStaleDesktopEntriesFunc
	oldCheckPass := checkHasDockerPassFunc
	oldNewCallback := newCallbackServerFunc
	t.Cleanup(func() {
		cleanStaleDesktopEntriesFunc = oldDesktopCleanup
		checkHasDockerPassFunc = oldCheckPass
		newCallbackServerFunc = oldNewCallback
	})

	// Mock docker pass check to succeed.
	checkHasDockerPassFunc = func(_ context.Context) error { return nil }

	// Mock callback server creation to fail — simulates a mid-flow failure.
	newCallbackServerFunc = func() (*pkgoauth.CallbackServer, error) {
		return nil, fmt.Errorf("test: port conflict")
	}

	var desktopCleanupCalled bool
	cleanStaleDesktopEntriesFunc = func(_ context.Context, _ string) {
		desktopCleanupCalled = true
	}

	// Call the real authorizeCommunityMode directly.
	err := authorizeCommunityMode(t.Context(), "my-community-server", "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create callback server")
	assert.False(t, desktopCleanupCalled,
		"community authorize should NOT clean Desktop entries when flow fails before token storage")
}
