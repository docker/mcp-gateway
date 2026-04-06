package oauth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFlagDisable_CommunityFallsBackToDesktop verifies that when the
// McpGatewayOAuth flag is toggled OFF, community servers fall back from
// ModeCommunity to ModeDesktop.
func TestFlagDisable_CommunityFallsBackToDesktop(t *testing.T) {
	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}
	flagOff := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	ctx := t.Context()
	ceMode := false

	modeWhenOn := determineMode(ctx, ceMode, true, flagOn)
	modeWhenOff := determineMode(ctx, ceMode, true, flagOff)

	assert.Equal(t, ModeCommunity, modeWhenOn, "flag ON: community server should be ModeCommunity")
	assert.Equal(t, ModeDesktop, modeWhenOff, "flag OFF: community server should fall back to ModeDesktop")
}

// TestFlagDisable_CatalogUnaffected verifies that catalog servers always use
// ModeDesktop regardless of the McpGatewayOAuth flag state.
func TestFlagDisable_CatalogUnaffected(t *testing.T) {
	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}
	flagOff := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	ctx := t.Context()
	ceMode := false

	assert.Equal(t, ModeDesktop, determineMode(ctx, ceMode, false, flagOn),
		"flag ON: catalog server should still be ModeDesktop")
	assert.Equal(t, ModeDesktop, determineMode(ctx, ceMode, false, flagOff),
		"flag OFF: catalog server should still be ModeDesktop")
}

// TestFlagDisable_CEModeUnaffected verifies that CE mode always returns ModeCE
// regardless of the McpGatewayOAuth flag state or server type.
func TestFlagDisable_CEModeUnaffected(t *testing.T) {
	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}
	flagOff := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	ctx := t.Context()
	ceMode := true

	assert.Equal(t, ModeCE, determineMode(ctx, ceMode, false, flagOn),
		"CE mode + flag ON: catalog should be ModeCE")
	assert.Equal(t, ModeCE, determineMode(ctx, ceMode, false, flagOff),
		"CE mode + flag OFF: catalog should be ModeCE")
	assert.Equal(t, ModeCE, determineMode(ctx, ceMode, true, flagOn),
		"CE mode + flag ON: community should be ModeCE")
	assert.Equal(t, ModeCE, determineMode(ctx, ceMode, true, flagOff),
		"CE mode + flag OFF: community should be ModeCE")
}

// TestFlagDisable_FlagErrorFallsBackToDesktop verifies that when the feature
// flag check returns an error, community servers gracefully fall back to
// ModeDesktop rather than crashing or choosing an unexpected mode.
func TestFlagDisable_FlagErrorFallsBackToDesktop(t *testing.T) {
	flagErr := func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("Desktop not reachable")
	}

	ctx := t.Context()
	mode := determineMode(ctx, false, true, flagErr)
	assert.Equal(t, ModeDesktop, mode, "flag error: community server should fall back to ModeDesktop")
}

// TestFlagDisable_ExistingProviderModePreserved verifies that a Provider
// created with ModeCommunity retains its mode even when the feature flag is
// conceptually toggled OFF. The mode is cached at construction time and only
// changes when the provider is stopped and re-created (gateway restart).
func TestFlagDisable_ExistingProviderModePreserved(t *testing.T) {
	noopReload := func(_ context.Context, _ string) error { return nil }

	// Create a provider while flag is "ON" (ModeCommunity).
	p := NewProvider("community-server", ModeCommunity, noopReload)
	assert.Equal(t, ModeCommunity, p.resolveRefreshMode(),
		"provider should start with ModeCommunity")

	// The flag is now "OFF" conceptually, but the provider mode is immutable.
	// A new determineMode call would return ModeDesktop, but the existing
	// provider retains its cached mode.
	assert.Equal(t, ModeCommunity, p.resolveRefreshMode(),
		"in-flight provider should retain cached mode after flag toggle")

	// A new provider created after the flag toggle would get ModeDesktop.
	p2 := NewProvider("community-server", ModeDesktop, noopReload)
	assert.Equal(t, ModeDesktop, p2.resolveRefreshMode(),
		"new provider after flag toggle should use ModeDesktop")
}

// TestFlagDisable_DetermineModeIsPure verifies that determineMode is a pure
// function: calling it with different flag states produces different results
// but does not mutate any shared state.
func TestFlagDisable_DetermineModeIsPure(t *testing.T) {
	callCount := 0
	flagOn := func(_ context.Context, _ string) (bool, error) {
		callCount++
		return true, nil
	}
	flagOff := func(_ context.Context, _ string) (bool, error) {
		callCount++
		return false, nil
	}

	ctx := t.Context()

	// Call multiple times with alternating flag states.
	r1 := determineMode(ctx, false, true, flagOn)
	r2 := determineMode(ctx, false, true, flagOff)
	r3 := determineMode(ctx, false, true, flagOn)
	r4 := determineMode(ctx, false, true, flagOff)

	assert.Equal(t, ModeCommunity, r1)
	assert.Equal(t, ModeDesktop, r2)
	assert.Equal(t, ModeCommunity, r3)
	assert.Equal(t, ModeDesktop, r4)

	// Each call invoked the flag checker exactly once (community path checks flag).
	assert.Equal(t, 4, callCount, "flag checker should be called once per determineMode invocation")
}
