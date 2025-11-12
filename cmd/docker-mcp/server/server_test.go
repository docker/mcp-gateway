package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
)

func TestListVolumeNotFound(t *testing.T) {
	ctx, home, docker := setupForList(t, withoutPromptsVolume(), withEmptyConfig())

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	assert.Empty(t, entries)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListEmptyVolume(t *testing.T) {
	ctx, home, docker := setupForList(t, withEmptyPromptsVolume(), withEmptyConfig())

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	assert.Empty(t, entries)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListImportVolume(t *testing.T) {
	ctx, home, docker := setupForList(t, withRegistryYamlInPromptsVolume("registry:\n  github-official:\n    ref: \"\""), withEmptyConfig(), withCatalog("registry:\n  github-official:\n    description: \"GitHub server\""))

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "github-official", entries[0].Name)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListEmpty(t *testing.T) {
	ctx, _, docker := setupForList(t, withEmptyRegistryYaml(), withEmptyConfig())

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestList(t *testing.T) {
	ctx, _, docker := setupForList(t, withRegistryYaml("registry:\n  git:\n    ref: \"\""), withEmptyConfig(), withCatalog("registry:\n  git:\n    description: \"Git server\""))

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "git", entries[0].Name)
}

func TestEnableNotFound(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t, withEmptyRegistryYaml(), withEmptyCatalog(), withEmptyConfig())

	err := Enable(ctx, docker, dockerCli, []string{"duckduckgo"}, false, true)
	require.ErrorContains(t, err, "server duckduckgo not found in catalog")
}

func TestEnable(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t, withEmptyRegistryYaml(), withCatalog("registry:\n  duckduckgo:\n    description: \"DuckDuckGo server\""), withEmptyConfig())

	err := Enable(ctx, docker, dockerCli, []string{"duckduckgo"}, false, true)
	require.NoError(t, err)

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "duckduckgo", entries[0].Name)
}

func TestDisable(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t, withRegistryYaml("registry:\n  duckduckgo:\n    ref: \"\"\n  git:\n    ref: \"\""), withCatalog("registry:\n  git:\n    description: \"Git server\"\n  duckduckgo:\n    description: \"DuckDuckGo server\""), withEmptyConfig())

	err := Disable(ctx, docker, dockerCli, []string{"duckduckgo"}, false)
	require.NoError(t, err)

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "git", entries[0].Name)
}

func TestDisableUnknown(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t, withRegistryYaml("registry:\n  duckduckgo:\n    ref: \"\""), withCatalog("registry:\n  duckduckgo:\n    description: \"DuckDuckGo server\""), withEmptyConfig())

	err := Disable(ctx, docker, dockerCli, []string{"unknown"}, false)
	require.NoError(t, err)

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "duckduckgo", entries[0].Name)
}

func TestRemoveOutdatedServerOnEnable(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t, withRegistryYaml("registry:\n  outdated:\n    ref: \"\""), withCatalog("registry:\n  git:\n    description: \"Git server\""), withEmptyConfig())

	err := Enable(ctx, docker, dockerCli, []string{"git"}, false, true)
	require.NoError(t, err)

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "git", entries[0].Name)
}

func TestRemoveOutdatedServerOnDisable(t *testing.T) {
	ctx, _, docker, dockerCli := setup(t, withRegistryYaml("registry:\n  outdated:\n    ref: \"\""), withEmptyCatalog(), withEmptyConfig())

	err := Disable(ctx, docker, dockerCli, []string{"git"}, false)
	require.NoError(t, err)

	entries, err := List(ctx, docker, false)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// Fixtures

func setup(t *testing.T, options ...option) (context.Context, string, docker.Client, command.Cli) {
	t.Helper()

	// Mock for Docker API
	docker := &fakeDocker{}

	// Create a temporary directory for the home directory
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}

	for _, o := range options {
		o(t, home, docker)
	}

	return t.Context(), home, docker, &fakeCli{}
}

func setupForList(t *testing.T, options ...option) (context.Context, string, docker.Client) {
	t.Helper()
	ctx, home, docker, _ := setup(t, options...)
	return ctx, home, docker
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

type fakeCli struct {
	command.Cli
}

func (fakeCli) ConfigFile() *configfile.ConfigFile {
	return &configfile.ConfigFile{}
}

type exitCodeErr struct {
	exitCode int
}

func (e *exitCodeErr) ExitCode() int {
	return e.exitCode
}

func (e *exitCodeErr) Error() string {
	return strconv.Itoa(e.exitCode)
}

type option func(*testing.T, string, *fakeDocker)

func withoutPromptsVolume() option {
	return func(_ *testing.T, _ string, dockerCLI *fakeDocker) {
		dockerCLI.inspectErr = errdefs.NotFound(errors.New("volume not found"))
	}
}

func withEmptyPromptsVolume() option {
	return func(t *testing.T, _ string, dockerCLI *fakeDocker) {
		t.Helper()
		dockerCLI.inspectErr = nil

		cmdOutput := config.CmdOutput
		t.Cleanup(func() { config.CmdOutput = cmdOutput })
		config.CmdOutput = func(*exec.Cmd) ([]byte, error) {
			return nil, &exitCodeErr{exitCode: 42}
		}
	}
}

func withRegistryYamlInPromptsVolume(yaml string) option {
	return func(t *testing.T, _ string, dockerCLI *fakeDocker) {
		t.Helper()
		dockerCLI.inspectErr = nil

		cmdOutput := config.CmdOutput
		t.Cleanup(func() { config.CmdOutput = cmdOutput })
		config.CmdOutput = func(*exec.Cmd) ([]byte, error) {
			return []byte(yaml), nil
		}
	}
}

func withRegistryYaml(yaml string) option {
	return func(t *testing.T, home string, _ *fakeDocker) {
		t.Helper()
		writeFile(t, filepath.Join(home, ".docker/mcp/registry.yaml"), []byte(yaml))
	}
}

func withEmptyRegistryYaml() option {
	return withRegistryYaml("")
}

func withCatalog(yaml string) option {
	return func(t *testing.T, home string, _ *fakeDocker) {
		t.Helper()
		writeFile(t, filepath.Join(home, ".docker/mcp/catalogs/docker-mcp.yaml"), []byte(yaml))

		// Create catalog.json registry file to register the docker-mcp catalog
		catalogRegistry := `{
  "catalogs": {
    "docker-mcp": {
      "displayName": "Docker MCP Default Catalog",
      "url": "docker-mcp.yaml",
      "lastUpdate": "2024-01-01T00:00:00Z"
    }
  }
}`
		writeFile(t, filepath.Join(home, ".docker/mcp/catalog.json"), []byte(catalogRegistry))
	}
}

func withEmptyCatalog() option {
	return withCatalog("")
}

func withEmptyConfig() option {
	return func(t *testing.T, home string, _ *fakeDocker) {
		t.Helper()
		writeFile(t, filepath.Join(home, ".docker/mcp/config.yaml"), []byte(""))
	}
}
