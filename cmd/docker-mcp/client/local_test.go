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
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"MCP_DOCKER": {"command": "docker", "args": ["mcp", "gateway", "run", "--working-set", "project-ws"]}}}`), 0o644))

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
	require.Len(t, result.WorkingSets, 1)
	assert.Equal(t, "project-ws", result.WorkingSets[0])
}

func TestLocalCfgProcessor_MultipleWorkingSets(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := tempDir
	projectFile := ".vscode/mcp.json"
	configPath := filepath.Join(projectRoot, projectFile)

	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"mcpServers": {"MCP_DOCKER": {"command": "docker", "args": ["mcp", "gateway", "run", "--working-set", "ws1", "-w", "ws2"]}}}`), 0o644))

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
	require.Len(t, result.WorkingSets, 2)
	assert.Equal(t, "ws1", result.WorkingSets[0])
	assert.Equal(t, "ws2", result.WorkingSets[1])
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
	assert.Empty(t, result.WorkingSets)
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
	assert.Nil(t, result.WorkingSets)
}
