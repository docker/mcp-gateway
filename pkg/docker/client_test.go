package docker

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunningInDockerCE_CEModeEnvVar(t *testing.T) {
	t.Run("DOCKER_MCP_USE_CE=true returns true", func(t *testing.T) {
		t.Setenv("DOCKER_MCP_USE_CE", "true")
		result, err := RunningInDockerCE(t.Context(), nil)
		assert.NoError(t, err)
		assert.True(t, result, "Should return true when DOCKER_MCP_USE_CE=true")
	})

	t.Run("DOCKER_MCP_USE_CE=false does not short-circuit", func(t *testing.T) {
		t.Setenv("DOCKER_MCP_USE_CE", "false")
		// Without the env override, the platform default applies (assumes Desktop).
		result, err := RunningInDockerCE(t.Context(), nil)
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("DOCKER_MCP_USE_CE unset does not short-circuit", func(t *testing.T) {
		os.Unsetenv("DOCKER_MCP_USE_CE")
		result, err := RunningInDockerCE(t.Context(), nil)
		assert.NoError(t, err)
		assert.False(t, result)
	})
}
