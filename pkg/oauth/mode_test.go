package oauth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldUseGatewayOAuth(t *testing.T) {
	// ShouldUseGatewayOAuth is a wrapper: DetermineMode(...) != ModeDesktop.
	// The full decision-tree coverage lives in TestDetermineMode; here we
	// verify the bool mapping via determineMode (testable core).

	flagOn := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}
	flagOff := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	tests := []struct {
		name        string
		ceMode      bool
		isCommunity bool
		checkFlag   featureFlagChecker
		expected    bool
	}{
		{"CE mode -> true (ModeCE)", true, false, flagOff, true},
		{"Desktop catalog -> false (ModeDesktop)", false, false, flagOn, false},
		{"Desktop community flag ON -> true (ModeCommunity)", false, true, flagOn, true},
		{"Desktop community flag OFF -> false (ModeDesktop)", false, true, flagOff, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := determineMode(t.Context(), tt.ceMode, tt.isCommunity, tt.checkFlag)
			got := mode != ModeDesktop
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestShouldUseGatewayOAuth_CEModeIntegration(t *testing.T) {
	// Verify the public function wiring: when DOCKER_MCP_USE_CE=true,
	// ShouldUseGatewayOAuth returns true regardless of isCommunity.
	t.Setenv("DOCKER_MCP_USE_CE", "true")

	assert.True(t, ShouldUseGatewayOAuth(t.Context(), false),
		"CE mode override should make ShouldUseGatewayOAuth return true for catalog servers")
	assert.True(t, ShouldUseGatewayOAuth(t.Context(), true),
		"CE mode override should make ShouldUseGatewayOAuth return true for community servers")
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

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeAuto, "Auto"},
		{ModeDesktop, "Desktop"},
		{ModeCE, "CE"},
		{ModeCommunity, "Community"},
		{Mode(99), "Auto"}, // unknown falls through to default
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}
