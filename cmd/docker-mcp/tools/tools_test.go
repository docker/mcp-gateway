package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
)

func TestEnableToolsEmpty(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Enable(ctx, docker, []string{"search_duckduckgo"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
}

func TestEnableToolsExistingServer(t *testing.T) {
	ctx, docker := setup(t,
		withToolsConfig("duckduckgo:\n  - other_tool"),
		withSampleCatalog())

	err := Enable(ctx, docker, []string{"search_duckduckgo"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestEnableToolsExistingTool(t *testing.T) {
	ctx, docker := setup(t,
		withToolsConfig("duckduckgo:\n  - other_tool"),
		withSampleCatalog())

	err := Enable(ctx, docker, []string{"other_tool"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Len(t, toolsConfig.ServerTools["duckduckgo"], 1)
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestEnableToolsDuplicateTool(t *testing.T) {
	ctx, docker := setup(t,
		withEmptyToolsConfig(),
		withSampleCatalog())

	err := Enable(ctx, docker, []string{"other_tool"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
	assert.NotContains(t, toolsConfig.ServerTools["other_server"], "other_tool")

	err = Enable(ctx, docker, []string{"other_tool"}, "other_server")
	require.NoError(t, err)

	toolsYAML, err = config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err = config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
	assert.Contains(t, toolsConfig.ServerTools["other_server"], "other_tool")
}

func TestEnableToolNotFound(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Enable(ctx, docker, []string{"nonexistent_tool"}, "duckduckgo")
	require.ErrorContains(t, err, "tool \"nonexistent_tool\" not found in server \"duckduckgo\"")
}

func TestEnableServerNotFound(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Enable(ctx, docker, []string{"nonexistent_tool"}, "nonexistent_server")
	require.ErrorContains(t, err, "server \"nonexistent_server\" not found in catalog")
}

func TestEnableToolAutoDiscoverServer(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Enable(ctx, docker, []string{"search_duckduckgo"}, "")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
}

func TestEnableToolAutoDiscoverServerExistingServer(t *testing.T) {
	ctx, docker := setup(t, withToolsConfig("duckduckgo:\n  - other_tool"), withSampleCatalog())

	err := Enable(ctx, docker, []string{"search_duckduckgo"}, "")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestEnableToolAutoDiscoverServerExistingTool(t *testing.T) {
	ctx, docker := setup(t,
		withToolsConfig("duckduckgo:\n  - other_tool"),
		withSampleCatalog())

	err := Enable(ctx, docker, []string{"other_tool"}, "")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Len(t, toolsConfig.ServerTools["duckduckgo"], 1)
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestEnableToolAutoDiscoverNotFound(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Enable(ctx, docker, []string{"nonexistent_tool"}, "")
	require.ErrorContains(t, err, "tool \"nonexistent_tool\" not found in any server")
}

func TestEnableMultipleTools(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Enable(ctx, docker, []string{"search_duckduckgo", "other_tool"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
	assert.Len(t, toolsConfig.ServerTools["duckduckgo"], 2)
}

func TestDisableEmpty(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Disable(ctx, docker, []string{"search_duckduckgo"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.NotContains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestDisableToolExistingServer(t *testing.T) {
	ctx, docker := setup(t,
		withToolsConfig("duckduckgo:\n  - search_duckduckgo\n  - other_tool"),
		withSampleCatalog())

	err := Disable(ctx, docker, []string{"search_duckduckgo"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.NotContains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestDisableToolExistingServerToolAlreadyDisabled(t *testing.T) {
	ctx, docker := setup(t,
		withToolsConfig("duckduckgo:\n  - search_duckduckgo"),
		withSampleCatalog())

	err := Disable(ctx, docker, []string{"other_tool"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.Contains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.NotContains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
}

func TestDisableServerNotFound(t *testing.T) {
	ctx, docker := setup(t, withEmptyToolsConfig(), withSampleCatalog())

	err := Disable(ctx, docker, []string{"nonexistent_tool"}, "nonexistent_server")
	require.ErrorContains(t, err, "server \"nonexistent_server\" not found in catalog")
}

func TestDisableMultipleTools(t *testing.T) {
	ctx, docker := setup(t,
		withToolsConfig("duckduckgo:\n  - search_duckduckgo\n  - other_tool"),
		withSampleCatalog())

	err := Disable(ctx, docker, []string{"search_duckduckgo", "other_tool"}, "duckduckgo")
	require.NoError(t, err)

	toolsYAML, err := config.ReadTools(ctx, docker)
	require.NoError(t, err)
	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	require.NoError(t, err)

	assert.NotContains(t, toolsConfig.ServerTools["duckduckgo"], "search_duckduckgo")
	assert.NotContains(t, toolsConfig.ServerTools["duckduckgo"], "other_tool")
	assert.Empty(t, toolsConfig.ServerTools["duckduckgo"])
}

// Fixtures and helpers

func setup(t *testing.T, options ...option) (context.Context, docker.Client) {
	t.Helper()

	docker := &fakeDocker{}

	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}

	for _, o := range options {
		o(t, home, docker)
	}

	return t.Context(), docker
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(path, content, 0o644)
	require.NoError(t, err)
}

type fakeDocker struct {
	docker.Client
	volume     volume.Volume
	inspectErr error
}

func (f *fakeDocker) InspectVolume(context.Context, string) (volume.Volume, error) {
	return f.volume, f.inspectErr
}

type option func(*testing.T, string, *fakeDocker)

func withEmptyToolsConfig() option {
	return withToolsConfig("")
}

func withToolsConfig(yaml string) option {
	return func(t *testing.T, home string, _ *fakeDocker) {
		t.Helper()
		writeFile(t, filepath.Join(home, ".docker/mcp/tools.yaml"), []byte(yaml))
	}
}

func withSampleCatalog() option {
	return func(t *testing.T, home string, _ *fakeDocker) {
		t.Helper()
		catalogContent := `registry:
  duckduckgo:
    tools:
      - name: "search_duckduckgo"
        description: "Search DuckDuckGo"
      - name: "other_tool"
        description: "Another tool"
  other_server:
    tools:
      - name: "other_tool"
        description: "Another tool"
`
		writeFile(t, filepath.Join(home, ".docker/mcp/catalogs/docker-mcp.yaml"), []byte(catalogContent))
	}
}
