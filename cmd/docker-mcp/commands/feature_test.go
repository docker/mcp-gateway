package commands

import (
	"testing"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/stretchr/testify/assert"

	"github.com/docker/mcp-gateway/pkg/features"
)

func TestIsFeatureEnabledOAuthInterceptor(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"oauth-interceptor": "enabled",
			},
		}
		enabled := isFeatureEnabledFromConfig(configFile, "oauth-interceptor")
		assert.True(t, enabled)
	})

	t.Run("disabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"oauth-interceptor": "disabled",
			},
		}
		enabled := isFeatureEnabledFromConfig(configFile, "oauth-interceptor")
		assert.False(t, enabled)
	})

	t.Run("missing", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: make(map[string]string),
		}
		enabled := isFeatureEnabledFromConfig(configFile, "oauth-interceptor")
		assert.False(t, enabled, "missing features should default to disabled")
	})
}

func TestIsFeatureEnabledDynamicTools(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"dynamic-tools": "enabled",
			},
		}
		enabled := isFeatureEnabledFromConfig(configFile, "dynamic-tools")
		assert.True(t, enabled)
	})

	t.Run("disabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"dynamic-tools": "disabled",
			},
		}
		enabled := isFeatureEnabledFromConfig(configFile, "dynamic-tools")
		assert.False(t, enabled)
	})

	t.Run("missing", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: make(map[string]string),
		}
		enabled := isFeatureEnabledFromConfig(configFile, "dynamic-tools")
		assert.True(t, enabled, "dynamic-tools should default to enabled when missing")
	})

	t.Run("nil features map", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: nil,
		}
		enabled := isFeatureEnabledFromConfig(configFile, "dynamic-tools")
		assert.True(t, enabled, "dynamic-tools should default to enabled when Features is nil")
	})
}

func TestIsFeatureEnabledMcpOAuthDcr(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"mcp-oauth-dcr": "enabled",
			},
		}
		enabled := isFeatureEnabledFromConfig(configFile, "mcp-oauth-dcr")
		assert.True(t, enabled)
	})

	t.Run("disabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"mcp-oauth-dcr": "disabled",
			},
		}
		enabled := isFeatureEnabledFromConfig(configFile, "mcp-oauth-dcr")
		assert.False(t, enabled)
	})

	t.Run("missing", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: make(map[string]string),
		}
		enabled := isFeatureEnabledFromConfig(configFile, "mcp-oauth-dcr")
		assert.True(t, enabled, "mcp-oauth-dcr should default to enabled when missing")
	})

	t.Run("nil features map", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: nil,
		}
		enabled := isFeatureEnabledFromConfig(configFile, "mcp-oauth-dcr")
		assert.True(t, enabled, "mcp-oauth-dcr should default to enabled when Features is nil")
	})
}

func TestIsKnownFeature(t *testing.T) {
	// Test valid features
	assert.True(t, isKnownFeature("oauth-interceptor", &mockFeatures{}))
	assert.True(t, isKnownFeature("mcp-oauth-dcr", &mockFeatures{}))
	assert.True(t, isKnownFeature("dynamic-tools", &mockFeatures{}))

	// Test invalid features
	assert.False(t, isKnownFeature("invalid-feature", &mockFeatures{}))
	assert.False(t, isKnownFeature("configured-catalogs", &mockFeatures{})) // No longer supported
	assert.False(t, isKnownFeature("", &mockFeatures{}))

	// Test profiles feature - unknown in Docker Desktop, known in CE
	assert.True(t, isKnownFeature("profiles", &mockFeatures{
		runningDockerDesktop: false,
	}))
	assert.False(t, isKnownFeature("profiles", &mockFeatures{
		runningDockerDesktop: true,
	}))
}

type mockFeatures struct {
	initErr              error
	runningDockerDesktop bool
	profilesEnabled      bool
}

var _ features.Features = &mockFeatures{}

func (m *mockFeatures) InitError() error {
	return m.initErr
}

func (m *mockFeatures) IsRunningInDockerDesktop() bool {
	return m.runningDockerDesktop
}

func (m *mockFeatures) IsProfilesFeatureEnabled() bool {
	return m.profilesEnabled
}
