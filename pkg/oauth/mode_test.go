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

func TestDetermineMode(t *testing.T) {
	// Test the internal function directly so we can control ceMode and the
	// feature-flag checker.

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
		expected    Mode
	}{
		// CE mode: always ModeCE regardless of server type
		{
			name:        "CE mode, catalog server",
			ceMode:      true,
			isCommunity: false,
			checkFlag:   flagOff,
			expected:    ModeCE,
		},
		{
			name:        "CE mode, community server",
			ceMode:      true,
			isCommunity: true,
			checkFlag:   flagOff,
			expected:    ModeCE,
		},

		// Desktop + catalog server: always ModeDesktop
		{
			name:        "Desktop, catalog server",
			ceMode:      false,
			isCommunity: false,
			checkFlag:   flagOn,
			expected:    ModeDesktop,
		},

		// Desktop + community server: depends on feature flag
		{
			name:        "Desktop, community server, flag ON",
			ceMode:      false,
			isCommunity: true,
			checkFlag:   flagOn,
			expected:    ModeCommunity,
		},
		{
			name:        "Desktop, community server, flag OFF",
			ceMode:      false,
			isCommunity: true,
			checkFlag:   flagOff,
			expected:    ModeDesktop,
		},
		{
			name:        "Desktop, community server, flag error",
			ceMode:      false,
			isCommunity: true,
			checkFlag:   flagErr,
			expected:    ModeDesktop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineMode(t.Context(), tt.ceMode, tt.isCommunity, tt.checkFlag)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestDetermineMode_CEModeIntegration(t *testing.T) {
	// Verify the public function wiring: when DOCKER_MCP_USE_CE=true,
	// DetermineMode returns ModeCE regardless of isCommunity.
	t.Setenv("DOCKER_MCP_USE_CE", "true")

	assert.Equal(t, ModeCE, DetermineMode(t.Context(), false),
		"CE mode override should return ModeCE for catalog servers")
	assert.Equal(t, ModeCE, DetermineMode(t.Context(), true),
		"CE mode override should return ModeCE for community servers")
}

func TestMode_ResolveMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     Mode
		ceMode   bool // controlled via env var
		expected Mode
	}{
		{
			name:     "explicit Desktop stays Desktop",
			mode:     ModeDesktop,
			expected: ModeDesktop,
		},
		{
			name:     "explicit CE stays CE",
			mode:     ModeCE,
			expected: ModeCE,
		},
		{
			name:     "explicit Community stays Community",
			mode:     ModeCommunity,
			expected: ModeCommunity,
		},
		{
			name:     "Auto in CE mode resolves to CE",
			mode:     ModeAuto,
			ceMode:   true,
			expected: ModeCE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ceMode {
				t.Setenv("DOCKER_MCP_USE_CE", "true")
			}
			h := NewOAuthCredentialHelperWithMode(tt.mode)
			got := h.resolveMode()
			assert.Equal(t, tt.expected, got)
		})
	}
}
