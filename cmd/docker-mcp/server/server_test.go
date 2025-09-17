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

	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
)

func TestListVolumeNotFound(t *testing.T) {
	ctx, home, docker := setup(t, withoutPromptsVolume())

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListEmptyVolume(t *testing.T) {
	ctx, home, docker := setup(t, withEmptyPromptsVolume())

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListImportVolume(t *testing.T) {
	ctx, home, docker := setup(t, withRegistryYamlInPromptsVolume("registry:\n  github-official:\n    ref: \"\""))

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Equal(t, []string{"github-official"}, enabled)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListEmpty(t *testing.T) {
	ctx, _, docker := setup(t, withEmptyRegistryYaml())

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Empty(t, enabled)
}

func TestList(t *testing.T) {
	ctx, _, docker := setup(t, withRegistryYaml("registry:\n  git:\n    ref: \"\""))

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, enabled)
}

func TestEnableNotFound(t *testing.T) {
	ctx, _, docker := setup(t, withEmptyRegistryYaml(), withEmptyCatalog())

	err := Enable(ctx, docker, []string{"duckduckgo"}, false)
	require.ErrorContains(t, err, "server duckduckgo not found in catalog")
}

func TestEnable(t *testing.T) {
	ctx, _, docker := setup(t, withEmptyRegistryYaml(), withCatalog("registry:\n  duckduckgo:\n"))

	err := Enable(ctx, docker, []string{"duckduckgo"}, false)
	require.NoError(t, err)

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Equal(t, []string{"duckduckgo"}, enabled)
}

func TestDisable(t *testing.T) {
	ctx, _, docker := setup(t, withRegistryYaml("registry:\n  duckduckgo:\n    ref: \"\"\n  git:\n    ref: \"\""), withCatalog("registry:\n  git:\n  duckduckgo:\n"))

	err := Disable(ctx, docker, []string{"duckduckgo"}, false)
	require.NoError(t, err)

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, enabled)
}

func TestDisableUnknown(t *testing.T) {
	ctx, _, docker := setup(t, withRegistryYaml("registry:\n  duckduckgo:\n    ref: \"\""), withCatalog("registry:\n  duckduckgo:\n"))

	err := Disable(ctx, docker, []string{"unknown"}, false)
	require.NoError(t, err)

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Equal(t, []string{"duckduckgo"}, enabled)
}

func TestRemoveOutdatedServerOnEnable(t *testing.T) {
	ctx, _, docker := setup(t, withRegistryYaml("registry:\n  outdated:\n    ref: \"\""), withCatalog("registry:\n  git:\n"))

	err := Enable(ctx, docker, []string{"git"}, false)
	require.NoError(t, err)

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, enabled)
}

func TestRemoveOutdatedServerOnDisable(t *testing.T) {
	ctx, _, docker := setup(t, withRegistryYaml("registry:\n  outdated:\n    ref: \"\""), withEmptyCatalog())

	err := Disable(ctx, docker, []string{"git"}, false)
	require.NoError(t, err)

	enabled, err := List(ctx, docker)
	require.NoError(t, err)
	assert.Empty(t, enabled)
}

// Fixtures

func setup(t *testing.T, options ...option) (context.Context, string, docker.Client) {
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

	return t.Context(), home, docker
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
