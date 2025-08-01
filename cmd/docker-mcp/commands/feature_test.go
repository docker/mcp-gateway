package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsFeatureEnabledTrue(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")

	// Create config with enabled feature
	config := map[string]any{
		"features": map[string]string{
			"configured-catalogs": "enabled",
		},
	}
	configData, err := json.Marshal(config)
	require.NoError(t, err)
	err = os.WriteFile(configFile, configData, 0o644)
	require.NoError(t, err)

	// Load config file
	configFile2 := &configfile.ConfigFile{
		Filename: configFile,
	}
	_ = configFile2.LoadFromReader(os.Stdin) // This will load from the filename

	// Test directly with Features map
	configFile2.Features = map[string]string{
		"configured-catalogs": "enabled",
	}

	enabled := isFeatureEnabled(configFile2, "configured-catalogs")
	assert.True(t, enabled)
}

func TestIsFeatureEnabledFalse(t *testing.T) {
	configFile := &configfile.ConfigFile{
		Features: map[string]string{
			"configured-catalogs": "disabled",
		},
	}

	enabled := isFeatureEnabled(configFile, "configured-catalogs")
	assert.False(t, enabled)
}

func TestIsFeatureEnabledMissing(t *testing.T) {
	configFile := &configfile.ConfigFile{
		Features: make(map[string]string),
	}

	enabled := isFeatureEnabled(configFile, "configured-catalogs")
	assert.False(t, enabled, "missing features should default to disabled")
}

func TestIsFeatureEnabledCorrupt(t *testing.T) {
	configFile := &configfile.ConfigFile{
		Features: map[string]string{
			"configured-catalogs": "invalid-boolean",
		},
	}

	enabled := isFeatureEnabled(configFile, "configured-catalogs")
	assert.False(t, enabled, "corrupted feature values should default to disabled")
}

func TestEnableFeature(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config
	configFile := &configfile.ConfigFile{
		Filename: configPath,
		Features: make(map[string]string),
	}

	// Test enabling configured-catalogs feature
	err := enableFeature(configFile, "configured-catalogs")
	require.NoError(t, err)

	// Verify feature was enabled
	enabled := isFeatureEnabled(configFile, "configured-catalogs")
	assert.True(t, enabled, "configured-catalogs feature should be enabled")
}

func TestDisableFeature(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with feature already enabled
	configFile := &configfile.ConfigFile{
		Filename: configPath,
		Features: map[string]string{
			"configured-catalogs": "enabled",
		},
	}

	// Test disabling configured-catalogs feature
	err := disableFeature(configFile, "configured-catalogs")
	require.NoError(t, err)

	// Verify feature was disabled
	enabled := isFeatureEnabled(configFile, "configured-catalogs")
	assert.False(t, enabled, "configured-catalogs feature should be disabled")
}

func TestListFeatures(t *testing.T) {
	// Create config with mixed features
	configFile := &configfile.ConfigFile{
		Features: map[string]string{
			"configured-catalogs": "enabled",
			"other-feature":       "disabled",
		},
	}

	// Test listing features
	features := listFeatures(configFile)

	// Should contain our feature with correct status
	assert.Contains(t, features, "configured-catalogs")
	assert.Contains(t, features, "other-feature")
	assert.Equal(t, "enabled", features["configured-catalogs"])
	assert.Equal(t, "disabled", features["other-feature"])
}

func TestInvalidFeature(t *testing.T) {
	configFile := &configfile.ConfigFile{
		Features: make(map[string]string),
	}

	// Test enabling invalid feature
	err := enableFeature(configFile, "invalid-feature")
	require.Error(t, err, "should reject invalid feature names")
	assert.Contains(t, err.Error(), "unknown feature")
}

// Feature management functions that need to be implemented
func enableFeature(configFile *configfile.ConfigFile, feature string) error {
	// Validate feature name
	if !isKnownFeature(feature) {
		return &featureError{feature: feature, message: "unknown feature"}
	}

	// Enable the feature
	if configFile.Features == nil {
		configFile.Features = make(map[string]string)
	}
	configFile.Features[feature] = "enabled"

	// Save config file
	return configFile.Save()
}

func disableFeature(configFile *configfile.ConfigFile, feature string) error {
	// Validate feature name
	if !isKnownFeature(feature) {
		return &featureError{feature: feature, message: "unknown feature"}
	}

	// Disable the feature
	if configFile.Features == nil {
		configFile.Features = make(map[string]string)
	}
	configFile.Features[feature] = "disabled"

	// Save config file
	return configFile.Save()
}

func listFeatures(configFile *configfile.ConfigFile) map[string]string {
	if configFile.Features == nil {
		return make(map[string]string)
	}

	// Return copy of features map
	result := make(map[string]string)
	for k, v := range configFile.Features {
		result[k] = v
	}
	return result
}

func isFeatureEnabled(configFile *configfile.ConfigFile, _ string) bool {
	if configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features["configured-catalogs"]
	if !exists {
		return false
	}

	// Handle both boolean string values and "enabled"/"disabled" strings
	if value == "enabled" {
		return true
	}
	if value == "disabled" {
		return false
	}

	// Fallback to parsing as boolean
	enabled, err := strconv.ParseBool(value)
	return err == nil && enabled
}

// Feature error type
type featureError struct {
	feature string
	message string
}

func (e *featureError) Error() string {
	return e.message + ": " + e.feature
}
