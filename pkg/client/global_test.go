package client

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGlobalCfg creates a standard globalCfg for testing
func newTestGlobalCfg() globalCfg {
	return globalCfg{
		DisplayName: "Test Client",
		YQ: YQ{
			List: ".mcpServers | to_entries | map(.value + {\"name\": .key})",
			Set:  ".mcpServers[$NAME] = $JSON",
			Del:  "del(.mcpServers[$NAME])",
		},
	}
}

// setPathsForCurrentOS sets the appropriate OS-specific paths field for testing
func setPathsForCurrentOS(cfg *globalCfg, paths []string) {
	switch runtime.GOOS {
	case "windows":
		cfg.Windows = paths
	case "darwin":
		cfg.Darwin = paths
	default:
		cfg.Linux = paths
	}
}

func TestGlobalCfgProcessor_MultiplePaths(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		setupFiles    map[string]string
		configPaths   []string
		expectedFound bool
		expectedError bool
	}{
		{
			name: "single_path_exists",
			setupFiles: map[string]string{
				"config.json": `{"mcpServers": {"test": {"command": "echo"}}}`,
			},
			configPaths:   []string{"config.json"},
			expectedFound: true,
		},
		{
			name: "multiple_paths_first_exists",
			setupFiles: map[string]string{
				"config1.json": `{"mcpServers": {"test": {"command": "echo"}}}`,
				"config2.json": `{"mcpServers": {"other": {"command": "ls"}}}`,
			},
			configPaths:   []string{"config1.json", "config2.json"},
			expectedFound: true,
		},
		{
			name: "multiple_paths_second_exists",
			setupFiles: map[string]string{
				"config2.json": `{"mcpServers": {"fallback": {"command": "fallback"}}}`,
			},
			configPaths:   []string{"config1.json", "config2.json"},
			expectedFound: true,
		},
		{
			name:          "no_paths_exist",
			setupFiles:    map[string]string{},
			configPaths:   []string{"config1.json", "config2.json"},
			expectedFound: false,
		},
		{
			name: "file_is_directory",
			setupFiles: map[string]string{
				"config1.json/": "", // Directory instead of file
				"config2.json":  `{"mcpServers": {"backup": {"command": "backup"}}}`,
			},
			configPaths:   []string{"config1.json", "config2.json"},
			expectedFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testDir := filepath.Join(tempDir, tc.name)
			require.NoError(t, os.MkdirAll(testDir, 0o755))

			var paths []string
			for _, path := range tc.configPaths {
				paths = append(paths, filepath.Join(testDir, path))
			}

			for path, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, path)
				if filepath.Ext(path) == "/" {
					require.NoError(t, os.MkdirAll(fullPath, 0o755))
				} else {
					require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
				}
			}

			cfg := newTestGlobalCfg()
			setPathsForCurrentOS(&cfg, paths)

			processor, err := NewGlobalCfgProcessor(cfg)
			if tc.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			result := processor.ParseConfig()

			if tc.expectedFound {
				assert.True(t, result.IsInstalled)
				assert.Nil(t, result.Err)
				assert.NotNil(t, result.Cfg)
			} else {
				assert.False(t, result.IsInstalled)
			}
		})
	}
}

func TestGlobalCfgProcessor_Update_MultiplePaths(t *testing.T) {
	tempDir := t.TempDir()

	config1Path := filepath.Join(tempDir, "config1.json")
	config2Path := filepath.Join(tempDir, "config2.json")

	require.NoError(t, os.WriteFile(config1Path, []byte(`{"mcpServers": {"existing": {"command": "test"}}}`), 0o644))

	cfg := newTestGlobalCfg()
	paths := []string{config1Path, config2Path}
	setPathsForCurrentOS(&cfg, paths)

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	err = processor.Update("new-server", &MCPServerSTDIO{
		Name:    "new-server",
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(config1Path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new-server")

	_, err = os.ReadFile(config2Path)
	assert.True(t, os.IsNotExist(err))
}

func TestGlobalCfgProcessor_Update_NoExistingFiles(t *testing.T) {
	tempDir := t.TempDir()

	config1Path := filepath.Join(tempDir, "config1.json")
	config2Path := filepath.Join(tempDir, "config2.json")

	cfg := newTestGlobalCfg()
	paths := []string{config1Path, config2Path}
	setPathsForCurrentOS(&cfg, paths)

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	err = processor.Update("new-server", &MCPServerSTDIO{
		Name:    "new-server",
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(config1Path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new-server")

	_, err = os.ReadFile(config2Path)
	assert.True(t, os.IsNotExist(err))
}

func TestGlobalCfgProcessor_EmptyPaths(t *testing.T) {
	cfg := newTestGlobalCfg()

	_, err := NewGlobalCfgProcessor(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no paths configured for OS")
}

func TestGlobalCfgProcessor_SinglePath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"test": {"command": "echo"}}}`), 0o644))

	cfg := newTestGlobalCfg()
	setPathsForCurrentOS(&cfg, []string{configPath})

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	result := processor.ParseConfig()
	assert.True(t, result.IsInstalled)
	assert.True(t, result.IsOsSupported)
	assert.Nil(t, result.Err)
}

func TestGlobalCfgProcessor_SingleWorkingSet(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"MCP_DOCKER": {"command": "docker", "args": ["mcp", "gateway", "run", "--profile", "test-set"]}}}`), 0o644))

	cfg := newTestGlobalCfg()
	setPathsForCurrentOS(&cfg, []string{configPath})

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	result := processor.ParseConfig()
	assert.True(t, result.IsMCPCatalogConnected)
	assert.Equal(t, "test-set", result.WorkingSet)
}

func TestGlobalCfgProcessor_NoWorkingSet(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"MCP_DOCKER": {"command": "docker", "args": ["mcp", "gateway", "run"]}}}`), 0o644))

	cfg := newTestGlobalCfg()
	setPathsForCurrentOS(&cfg, []string{configPath})

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	result := processor.ParseConfig()
	assert.True(t, result.IsMCPCatalogConnected)
	assert.Empty(t, result.WorkingSet)
}

func TestIsPathValid(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "no_env_vars",
			path:     "/absolute/path/config.json",
			envVars:  map[string]string{},
			expected: true,
		},
		{
			name:     "defined_env_var",
			path:     "$HOME/.config/app/config.json",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: true,
		},
		{
			name:     "undefined_env_var",
			path:     "$UNDEFINED_VAR/.config/app/config.json",
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name:     "empty_env_var",
			path:     "$EMPTY_VAR/.config/app/config.json",
			envVars:  map[string]string{"EMPTY_VAR": ""},
			expected: false,
		},
		{
			name:     "multiple_defined_env_vars",
			path:     "$HOME/$CONFIG_DIR/config.json",
			envVars:  map[string]string{"HOME": "/home/user", "CONFIG_DIR": ".config"},
			expected: true,
		},
		{
			name:     "multiple_mixed_env_vars",
			path:     "$HOME/$UNDEFINED_VAR/config.json",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: false,
		},
		{
			name:     "windows_style_defined",
			path:     "$USERPROFILE\\.config\\app\\config.json",
			envVars:  map[string]string{"USERPROFILE": "C:\\Users\\user"},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			result := isPathValid(tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGlobalCfgProcessor_Update_WithEnvVarPaths(t *testing.T) {
	tempDir := t.TempDir()

	// Set up a custom config dir
	customConfigDir := filepath.Join(tempDir, "custom-config")
	require.NoError(t, os.MkdirAll(customConfigDir, 0o755))

	// Create existing config in custom dir
	customConfigPath := filepath.Join(customConfigDir, ".claude.json")
	require.NoError(t, os.WriteFile(customConfigPath, []byte(`{"mcpServers": {"existing": {"command": "test"}}}`), 0o644))

	// Set CLAUDE_CONFIG_DIR environment variable
	t.Setenv("CLAUDE_CONFIG_DIR", customConfigDir)

	// Create home config path (should be ignored when CLAUDE_CONFIG_DIR is set)
	homeDir := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	homeConfigPath := filepath.Join(homeDir, ".claude.json")
	t.Setenv("HOME", homeDir)

	cfg := newTestGlobalCfg()
	paths := []string{"$CLAUDE_CONFIG_DIR/.claude.json", "$HOME/.claude.json"}
	setPathsForCurrentOS(&cfg, paths)

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	err = processor.Update("new-server", &MCPServerSTDIO{
		Name:    "new-server",
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
	})
	require.NoError(t, err)

	// Verify update went to the custom config dir
	content, err := os.ReadFile(customConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new-server")

	// Verify home config was not created
	_, err = os.ReadFile(homeConfigPath)
	assert.True(t, os.IsNotExist(err), "home config should not be created when CLAUDE_CONFIG_DIR is set")
}

func TestGlobalCfgProcessor_Update_FallbackWhenEnvVarUndefined(t *testing.T) {
	tempDir := t.TempDir()

	// Set CLAUDE_CONFIG_DIR to empty - it should fall back to HOME
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	homeDir := filepath.Join(tempDir, "home")
	homeConfigPath := filepath.Join(homeDir, ".claude.json")
	t.Setenv("HOME", homeDir)

	cfg := newTestGlobalCfg()
	paths := []string{"$CLAUDE_CONFIG_DIR/.claude.json", "$HOME/.claude.json"}
	setPathsForCurrentOS(&cfg, paths)

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	err = processor.Update("new-server", &MCPServerSTDIO{
		Name:    "new-server",
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
	})
	require.NoError(t, err)

	// Verify update went to the home config dir
	content, err := os.ReadFile(homeConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new-server")
}

func TestGlobalCfgProcessor_Update_AllPathsInvalid(t *testing.T) {
	cfg := newTestGlobalCfg()
	// Only provide paths with undefined environment variables
	paths := []string{"$UNDEFINED_VAR1/.config.json", "$UNDEFINED_VAR2/.config.json"}
	setPathsForCurrentOS(&cfg, paths)

	processor, err := NewGlobalCfgProcessor(cfg)
	require.NoError(t, err)

	err = processor.Update("new-server", &MCPServerSTDIO{
		Name:    "new-server",
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "no valid config paths found")
}
