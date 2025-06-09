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

	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/config"
)

func TestListVolumeNotFound(t *testing.T) {
	ctx, home, dockerAPI := setup(t, withoutPromptsVolume())

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListEmptyVolume(t *testing.T) {
	ctx, home, dockerAPI := setup(t, withEmptyPromptsVolume())

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListImportVolume(t *testing.T) {
	ctx, home, dockerAPI := setup(t, withRegistryYamlInPromptsVolume("registry:\n  github-official:\n    ref: \"\""))

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"github-official"}, enabled)

	assert.FileExists(t, filepath.Join(home, ".docker/mcp/registry.yaml"))
}

func TestListEmpty(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withEmptyRegistryYaml())

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Empty(t, enabled)
}

func TestList(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withRegistryYaml("registry:\n  git:\n    ref: \"\""))

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, enabled)
}

func TestEnableNotFound(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withEmptyRegistryYaml(), withEmptyCatalog())

	err := Enable(ctx, dockerAPI, []string{"duckduckgo"})
	require.ErrorContains(t, err, "server duckduckgo not found in catalog")
}

func TestEnable(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withEmptyRegistryYaml(), withCatalog("registry:\n  duckduckgo:\n"))

	err := Enable(ctx, dockerAPI, []string{"duckduckgo"})
	require.NoError(t, err)

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"duckduckgo"}, enabled)
}

func TestDisable(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withRegistryYaml("registry:\n  duckduckgo:\n    ref: \"\"\n  git:\n    ref: \"\""), withCatalog("registry:\n  git:\n  duckduckgo:\n"))

	err := Disable(ctx, dockerAPI, []string{"duckduckgo"})
	require.NoError(t, err)

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, enabled)
}

func TestDisableUnknown(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withRegistryYaml("registry:\n  duckduckgo:\n    ref: \"\""), withCatalog("registry:\n  duckduckgo:\n"))

	err := Disable(ctx, dockerAPI, []string{"unknown"})
	require.NoError(t, err)

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"duckduckgo"}, enabled)
}

func TestRemoveOutdatedServerOnEnable(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withRegistryYaml("registry:\n  outdated:\n    ref: \"\""), withCatalog("registry:\n  git:\n"))

	err := Enable(ctx, dockerAPI, []string{"git"})
	require.NoError(t, err)

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, enabled)
}

func TestRemoveOutdatedServerOnDisable(t *testing.T) {
	ctx, _, dockerAPI := setup(t, withRegistryYaml("registry:\n  outdated:\n    ref: \"\""), withEmptyCatalog())

	err := Disable(ctx, dockerAPI, []string{"git"})
	require.NoError(t, err)

	enabled, err := List(ctx, dockerAPI)
	require.NoError(t, err)
	assert.Empty(t, enabled)
}

// Fixtures

func setup(t *testing.T, options ...option) (context.Context, string, config.VolumeInspecter) {
	t.Helper()

	// Mock for Docker API
	dockerAPI := &fakeDockerAPI{}

	// Create a temporary directory for the home directory
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}

	for _, o := range options {
		o(t, home, dockerAPI)
	}

	return t.Context(), home, dockerAPI
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(path, content, 0o644)
	require.NoError(t, err)
}

type fakeDockerAPI struct {
	volume     volume.Volume
	inspectErr error
}

func (f *fakeDockerAPI) VolumeInspect(context.Context, string) (volume.Volume, error) {
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

type option func(*testing.T, string, *fakeDockerAPI)

func withoutPromptsVolume() option {
	return func(_ *testing.T, _ string, dockerAPI *fakeDockerAPI) {
		dockerAPI.inspectErr = errdefs.NotFound(errors.New("volume not found"))
	}
}

func withEmptyPromptsVolume() option {
	return func(t *testing.T, _ string, dockerAPI *fakeDockerAPI) {
		t.Helper()
		dockerAPI.inspectErr = nil

		cmdOutput := config.CmdOutput
		t.Cleanup(func() { config.CmdOutput = cmdOutput })
		config.CmdOutput = func(*exec.Cmd) ([]byte, error) {
			return nil, &exitCodeErr{exitCode: 42}
		}
	}
}

func withRegistryYamlInPromptsVolume(yaml string) option {
	return func(t *testing.T, _ string, dockerAPI *fakeDockerAPI) {
		t.Helper()
		dockerAPI.inspectErr = nil

		cmdOutput := config.CmdOutput
		t.Cleanup(func() { config.CmdOutput = cmdOutput })
		config.CmdOutput = func(*exec.Cmd) ([]byte, error) {
			return []byte(yaml), nil
		}
	}
}

func withRegistryYaml(yaml string) option {
	return func(t *testing.T, home string, _ *fakeDockerAPI) {
		t.Helper()
		writeFile(t, filepath.Join(home, ".docker/mcp/registry.yaml"), []byte(yaml))
	}
}

func withEmptyRegistryYaml() option {
	return withRegistryYaml("")
}

func withCatalog(yaml string) option {
	return func(t *testing.T, home string, _ *fakeDockerAPI) {
		t.Helper()
		writeFile(t, filepath.Join(home, ".docker/mcp/catalogs/docker-mcp.yaml"), []byte(yaml))
	}
}

func withEmptyCatalog() option {
	return withCatalog("")
}
