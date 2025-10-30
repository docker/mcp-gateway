package commands

import (
	"testing"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/stretchr/testify/assert"
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
		assert.False(t, enabled, "dynamic-tools should default to disabled when missing")
	})

	t.Run("nil features map", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: nil,
		}
		enabled := isFeatureEnabledFromConfig(configFile, "dynamic-tools")
		assert.False(t, enabled, "dynamic-tools should default to disabled when Features is nil")
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
	assert.True(t, isKnownFeature("oauth-interceptor"))
	assert.True(t, isKnownFeature("mcp-oauth-dcr"))
	assert.True(t, isKnownFeature("dynamic-tools"))

	// Test invalid features
	assert.False(t, isKnownFeature("invalid-feature"))
	assert.False(t, isKnownFeature("configured-catalogs")) // No longer supported
	assert.False(t, isKnownFeature(""))
}
