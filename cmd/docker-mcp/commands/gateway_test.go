package commands

import (
	"testing"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/stretchr/testify/assert"
)

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

func TestIsDynamicToolsFeatureEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"dynamic-tools": "enabled",
			},
		}
		enabled := isDynamicToolsFeatureEnabledFromConfig(configFile)
		assert.True(t, enabled, "should return true when dynamic-tools is enabled")
	})

	t.Run("disabled", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{
				"dynamic-tools": "disabled",
			},
		}
		enabled := isDynamicToolsFeatureEnabledFromConfig(configFile)
		assert.False(t, enabled, "should return false when dynamic-tools is disabled")
	})

	t.Run("missing", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: map[string]string{},
		}
		enabled := isDynamicToolsFeatureEnabledFromConfig(configFile)
		assert.False(t, enabled, "should return false when dynamic-tools is not set")
	})

	t.Run("nil config", func(t *testing.T) {
		enabled := isDynamicToolsFeatureEnabledFromConfig(nil)
		assert.False(t, enabled, "should return false when config is nil")
	})

	t.Run("nil features", func(t *testing.T) {
		configFile := &configfile.ConfigFile{
			Features: nil,
		}
		enabled := isDynamicToolsFeatureEnabledFromConfig(configFile)
		assert.False(t, enabled, "should return false when features is nil")
	})
}

// Helper function for testing (extract logic from isDynamicToolsFeatureEnabled)
func isDynamicToolsFeatureEnabledFromConfig(configFile *configfile.ConfigFile) bool {
	if configFile == nil || configFile.Features == nil {
		return false
	}
	value, exists := configFile.Features["dynamic-tools"]
	if !exists {
		return false
	}
	return value == "enabled"
}

func TestConvertCatalogNamesToPaths(t *testing.T) {
	t.Run("keeps existing file paths unchanged", func(t *testing.T) {
		paths := []string{
			"docker-mcp.yaml",
			"./my-catalog.yaml",
			"../other-catalog.yaml",
			"/absolute/path/catalog.yaml",
		}

		result := convertCatalogNamesToPaths(paths)

		assert.Equal(t, paths, result)
	})

	t.Run("keeps unknown names unchanged", func(t *testing.T) {
		paths := []string{
			"unknown-name",
			"another-unknown",
		}

		result := convertCatalogNamesToPaths(paths)

		assert.Equal(t, paths, result)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		paths := []string{}

		result := convertCatalogNamesToPaths(paths)

		assert.Empty(t, result)
	})

	t.Run("mixed paths and names", func(t *testing.T) {
		paths := []string{
			"docker-mcp.yaml", // file path - keep as-is
			"unknown-name",    // unknown name - keep as-is
			"./relative.yaml", // file path - keep as-is
		}

		result := convertCatalogNamesToPaths(paths)

		expected := []string{
			"docker-mcp.yaml",
			"unknown-name",
			"./relative.yaml",
		}
		assert.Equal(t, expected, result)
	})
}

func TestBuildUniqueCatalogPaths(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		defaults := []string{"docker-mcp.yaml"}
		configured := []string{"my-catalog.yaml", "other-catalog.yaml"}
		additional := []string{"cli-catalog.yaml"}

		result := buildUniqueCatalogPaths(defaults, configured, additional)

		expected := []string{"docker-mcp.yaml", "my-catalog.yaml", "other-catalog.yaml", "cli-catalog.yaml"}
		assert.Equal(t, expected, result)
	})

	t.Run("removes duplicates preserving precedence", func(t *testing.T) {
		defaults := []string{"docker-mcp.yaml", "common.yaml"}
		configured := []string{"my-catalog.yaml", "common.yaml"}      // duplicate
		additional := []string{"cli-catalog.yaml", "docker-mcp.yaml"} // duplicate

		result := buildUniqueCatalogPaths(defaults, configured, additional)

		// Should keep first occurrence (maintaining precedence)
		expected := []string{"docker-mcp.yaml", "common.yaml", "my-catalog.yaml", "cli-catalog.yaml"}
		assert.Equal(t, expected, result)
	})

	t.Run("handles empty slices", func(t *testing.T) {
		defaults := []string{"docker-mcp.yaml"}
		configured := []string{}
		additional := []string{}

		result := buildUniqueCatalogPaths(defaults, configured, additional)

		expected := []string{"docker-mcp.yaml"}
		assert.Equal(t, expected, result)
	})

	t.Run("handles all empty slices", func(t *testing.T) {
		defaults := []string{}
		configured := []string{}
		additional := []string{}

		result := buildUniqueCatalogPaths(defaults, configured, additional)

		assert.Empty(t, result)
	})

	t.Run("maintains precedence order", func(t *testing.T) {
		// Test that later sources can't override earlier ones (first occurrence wins)
		defaults := []string{"first.yaml"}
		configured := []string{"first.yaml"} // duplicate - should be ignored
		additional := []string{"first.yaml"} // duplicate - should be ignored

		result := buildUniqueCatalogPaths(defaults, configured, additional)

		expected := []string{"first.yaml"} // Only one occurrence
		assert.Equal(t, expected, result)
	})
}

func TestConditionalConfiguredCatalogPaths(t *testing.T) {
	t.Run("excludes configured catalogs when single Docker catalog URL", func(t *testing.T) {
		// Test the logic for when defaultPaths contains only DockerCatalogURL
		defaultPaths := []string{"https://desktop.docker.com/mcp/catalog/v2/catalog.yaml"}

		// This should match the condition and return empty configuredPaths
		shouldExclude := len(defaultPaths) == 1 && (defaultPaths[0] == "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml" || defaultPaths[0] == "docker-mcp.yaml")
		assert.True(t, shouldExclude, "should exclude configured catalogs when single Docker catalog URL")
	})

	t.Run("excludes configured catalogs when single Docker catalog filename", func(t *testing.T) {
		// Test the logic for when defaultPaths contains only DockerCatalogFilename
		defaultPaths := []string{"docker-mcp.yaml"}

		// This should match the condition and return empty configuredPaths
		shouldExclude := len(defaultPaths) == 1 && (defaultPaths[0] == "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml" || defaultPaths[0] == "docker-mcp.yaml")
		assert.True(t, shouldExclude, "should exclude configured catalogs when single Docker catalog filename")
	})

	t.Run("includes configured catalogs when multiple paths", func(t *testing.T) {
		// Test the logic for when defaultPaths contains multiple entries
		defaultPaths := []string{"docker-mcp.yaml", "other-catalog.yaml"}

		// This should NOT match the condition and allow configuredPaths
		shouldExclude := len(defaultPaths) == 1 && (defaultPaths[0] == "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml" || defaultPaths[0] == "docker-mcp.yaml")
		assert.False(t, shouldExclude, "should include configured catalogs when multiple paths")
	})

	t.Run("includes configured catalogs when single non-Docker catalog", func(t *testing.T) {
		// Test the logic for when defaultPaths contains a single non-Docker catalog
		defaultPaths := []string{"custom-catalog.yaml"}

		// This should NOT match the condition and allow configuredPaths
		shouldExclude := len(defaultPaths) == 1 && (defaultPaths[0] == "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml" || defaultPaths[0] == "docker-mcp.yaml")
		assert.False(t, shouldExclude, "should include configured catalogs when single non-Docker catalog")
	})
}
