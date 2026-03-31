package oauth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldUseGatewayOAuth(t *testing.T) {
	// Test the internal function directly so we can control ceMode and the
	// feature-flag checker without depending on the OS, env vars, or a
	// running Docker Desktop backend.

	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}
	flagOff := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}
	flagErr := func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("Desktop not running")
	}

	tests := []struct {
		name        string
		ceMode      bool
		isCommunity bool
		checkFlag   featureFlagChecker
		expected    bool
	}{
		// --- CE mode: Gateway always owns OAuth regardless of server type ---
		{
			name:        "CE mode, catalog server",
			ceMode:      true,
			isCommunity: false,
			checkFlag:   flagOff, // should not be called
			expected:    true,
		},
		{
			name:        "CE mode, community server",
			ceMode:      true,
			isCommunity: true,
			checkFlag:   flagOff, // should not be called
			expected:    true,
		},

		// --- Desktop + catalog server: Desktop always owns OAuth ---
		{
			name:        "Desktop, catalog server",
			ceMode:      false,
			isCommunity: false,
			checkFlag:   flagOn, // should not be called
			expected:    false,
		},

		// --- Desktop + community server: gated on feature flag ---
		{
			name:        "Desktop, community server, flag ON",
			ceMode:      false,
			isCommunity: true,
			checkFlag:   flagOn,
			expected:    true,
		},
		{
			name:        "Desktop, community server, flag OFF",
			ceMode:      false,
			isCommunity: true,
			checkFlag:   flagOff,
			expected:    false,
		},
		{
			name:        "Desktop, community server, flag error",
			ceMode:      false,
			isCommunity: true,
			checkFlag:   flagErr,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseGatewayOAuth(t.Context(), tt.ceMode, tt.isCommunity, tt.checkFlag)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestShouldUseGatewayOAuth_CEModeIntegration(t *testing.T) {
	// Verify the public function wiring: when DOCKER_MCP_USE_CE=true,
	// IsCEMode() returns true and the public ShouldUseGatewayOAuth must
	// return true regardless of isCommunity.
	t.Setenv("DOCKER_MCP_USE_CE", "true")

	assert.True(t, ShouldUseGatewayOAuth(t.Context(), false),
		"CE mode override should make ShouldUseGatewayOAuth return true for catalog servers")
	assert.True(t, ShouldUseGatewayOAuth(t.Context(), true),
		"CE mode override should make ShouldUseGatewayOAuth return true for community servers")
}

func TestShouldUseGatewayOAuth_FeatureFlagName(t *testing.T) {
	// Verify the unexported function passes the correct feature flag name
	// to the checker.
	var capturedName string
	spy := func(_ context.Context, name string) (bool, error) {
		capturedName = name
		return false, nil
	}

	shouldUseGatewayOAuth(t.Context(), false, true, spy)
	assert.Equal(t, "McpGatewayOAuth", capturedName,
		"should query the McpGatewayOAuth feature flag")
}
