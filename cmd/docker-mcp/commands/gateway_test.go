package commands

import (
	"path/filepath"
	"testing"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayUseConfiguredCatalogsEnabled(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with feature enabled
	configFile := &configfile.ConfigFile{
		Filename: configPath,
		Features: map[string]string{
			"configured-catalogs": "enabled",
		},
	}

	// Test validation passes when feature enabled
	err := validateConfiguredCatalogsFeature(configFile, true)
	assert.NoError(t, err, "should allow --use-configured-catalogs when feature enabled")
}

func TestGatewayUseConfiguredCatalogsDisabled(t *testing.T) {
	// Create config with feature disabled
	configFile := &configfile.ConfigFile{
		Features: map[string]string{
			"configured-catalogs": "disabled",
		},
	}

	// Test validation fails when feature disabled
	err := validateConfiguredCatalogsFeature(configFile, true)
	require.Error(t, err, "should reject --use-configured-catalogs when feature disabled")
	assert.Contains(t, err.Error(), "configured catalogs feature is not enabled")
}

func TestGatewayFeatureFlagErrorMessage(t *testing.T) {
	// Create config with no features
	configFile := &configfile.ConfigFile{
		Features: make(map[string]string),
	}

	// Test validation fails with helpful error message
	err := validateConfiguredCatalogsFeature(configFile, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configured catalogs feature is not enabled")
	assert.Contains(t, err.Error(), "docker mcp feature enable configured-catalogs")
}

func TestGatewayContainerModeDetection(t *testing.T) {
	// Test with nil config file (simulating container mode without volume mount)
	err := validateConfiguredCatalogsFeature(nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Docker configuration not accessible")
	assert.Contains(t, err.Error(), "running in container")
}

func TestGatewayNoValidationWhenFlagNotUsed(t *testing.T) {
	// Test that validation is skipped when flag not used, even if config is nil
	err := validateConfiguredCatalogsFeature(nil, false)
	assert.NoError(t, err, "should skip validation when --use-configured-catalogs not used")
}

// Feature validation function that needs to be implemented
func validateConfiguredCatalogsFeature(configFile *configfile.ConfigFile, useConfigured bool) error {
	if !useConfigured {
		return nil // No validation needed when feature not requested
	}

	// Check if config is accessible (container mode check)
	if configFile == nil {
		return &configError{
			message: `Docker configuration not accessible.

If running in container, mount Docker config:
  -v ~/.docker:/root/.docker

Or mount just the config file:  
  -v ~/.docker/config.json:/root/.docker/config.json`,
		}
	}

	// Check if feature is enabled
	if configFile.Features != nil {
		if value, exists := configFile.Features["configured-catalogs"]; exists {
			if value == "enabled" {
				return nil // Feature is enabled
			}
		}
	}

	// Feature not enabled
	return &configError{
		message: `configured catalogs feature is not enabled.

To enable this experimental feature, run:
  docker mcp feature enable configured-catalogs

This feature allows the gateway to automatically include user-managed catalogs
alongside the default Docker catalog.`,
	}
}

// Config error type
type configError struct {
	message string
}

func (e *configError) Error() string {
	return e.message
}

func TestIsOAuthInterceptorFeatureEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"oauth-interceptor": "enabled",
			},
		}
		enabled := isOAuthInterceptorFeatureEnabledFromConfig(configFile)
		assert.True(t, enabled, "should return true when oauth-interceptor is enabled")
	})

	t.Run("disabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"oauth-interceptor": "disabled",
			},
		}
		enabled := isOAuthInterceptorFeatureEnabledFromConfig(configFile)
		assert.False(t, enabled, "should return false when oauth-interceptor is disabled")
	})

	t.Run("missing", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{},
		}
		enabled := isOAuthInterceptorFeatureEnabledFromConfig(configFile)
		assert.False(t, enabled, "should return false when oauth-interceptor is not set")
	})

	t.Run("nil config", func(t *testing.T) {
		enabled := isOAuthInterceptorFeatureEnabledFromConfig(nil)
		assert.False(t, enabled, "should return false when config is nil")
	})

	t.Run("nil features", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: nil,
		}
		enabled := isOAuthInterceptorFeatureEnabledFromConfig(configFile)
		assert.False(t, enabled, "should return false when features is nil")
	})
}

// Helper function for testing (extract logic from isOAuthInterceptorFeatureEnabled)
func isOAuthInterceptorFeatureEnabledFromConfig(configFile *configfile.ConfigFile) bool {
	if configFile == nil || configFile.Features == nil {
		return false
	}
	value, exists := configFile.Features["oauth-interceptor"]
	if !exists {
		return false
	}
	return value == "enabled"
}
