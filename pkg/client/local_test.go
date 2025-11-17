package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalCfgProcessor_SingleWorkingSet(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := tempDir
	projectFile := ".cursor/mcp.json"
	configPath := filepath.Join(projectRoot, projectFile)

	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"MCP_DOCKER": {"command": "docker", "args": ["mcp", "gateway", "run", "--profile", "project-ws"]}}}`), 0o644))

	cfg := localCfg{
		DisplayName: "Test Client",
		ProjectFile: projectFile,
		YQ: YQ{
			List: ".mcpServers | to_entries | map(.value + {\"name\": .key})",
			Set:  ".mcpServers[$NAME] = $JSON",
			Del:  "del(.mcpServers[$NAME])",
		},
	}

	processor, err := NewLocalCfgProcessor(cfg, projectRoot)
	require.NoError(t, err)

	result := processor.Parse()
	assert.True(t, result.IsConfigured)
	assert.True(t, result.IsMCPCatalogConnected)
	assert.Equal(t, "project-ws", result.WorkingSet)
}

func TestLocalCfgProcessor_NoWorkingSet(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := tempDir
	projectFile := ".cursor/mcp.json"
	configPath := filepath.Join(projectRoot, projectFile)

	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"MCP_DOCKER": {"command": "docker", "args": ["mcp", "gateway", "run"]}}}`), 0o644))

	cfg := localCfg{
		DisplayName: "Test Client",
		ProjectFile: projectFile,
		YQ: YQ{
			List: ".mcpServers | to_entries | map(.value + {\"name\": .key})",
			Set:  ".mcpServers[$NAME] = $JSON",
			Del:  "del(.mcpServers[$NAME])",
		},
	}

	processor, err := NewLocalCfgProcessor(cfg, projectRoot)
	require.NoError(t, err)

	result := processor.Parse()
	assert.True(t, result.IsConfigured)
	assert.True(t, result.IsMCPCatalogConnected)
	assert.Empty(t, result.WorkingSet)
}

func TestLocalCfgProcessor_NotConfigured(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := tempDir
	projectFile := ".cursor/mcp.json"

	cfg := localCfg{
		DisplayName: "Test Client",
		ProjectFile: projectFile,
		YQ: YQ{
			List: ".mcpServers | to_entries | map(.value + {\"name\": .key})",
			Set:  ".mcpServers[$NAME] = $JSON",
			Del:  "del(.mcpServers[$NAME])",
		},
	}

	processor, err := NewLocalCfgProcessor(cfg, projectRoot)
	require.NoError(t, err)

	result := processor.Parse()
	assert.False(t, result.IsConfigured)
	assert.False(t, result.IsMCPCatalogConnected)
	assert.Empty(t, result.WorkingSet)
}
