package desktop

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRunningInDockerDesktop_CEModeEnvVar(t *testing.T) {
	t.Run("DOCKER_MCP_USE_CE=true returns false", func(t *testing.T) {
		t.Setenv("DOCKER_MCP_USE_CE", "true")
		assert.False(t, IsRunningInDockerDesktop(context.Background()),
			"Should return false when DOCKER_MCP_USE_CE=true")
	})

	t.Run("DOCKER_MCP_USE_CE=false does not override", func(t *testing.T) {
		t.Setenv("DOCKER_MCP_USE_CE", "false")
		// Without the override, platform default behavior applies
		// Just verify it doesn't panic
		_ = IsRunningInDockerDesktop(context.Background())
	})

	t.Run("DOCKER_MCP_USE_CE unset does not override", func(t *testing.T) {
		os.Unsetenv("DOCKER_MCP_USE_CE")
		// Should not panic and should use default platform behavior
		_ = IsRunningInDockerDesktop(context.Background())
	})
}

func TestIsRunningInDockerDesktop_InContainer(t *testing.T) {
	t.Run("DOCKER_MCP_IN_CONTAINER=1 returns false", func(t *testing.T) {
		t.Setenv("DOCKER_MCP_IN_CONTAINER", "1")
		assert.False(t, IsRunningInDockerDesktop(context.Background()),
			"Should return false when running in container")
	})
}

func TestIsRunningInDockerDesktop_NoDockerDesktopContext(t *testing.T) {
	ctx := WithNoDockerDesktop(context.Background())
	assert.False(t, IsRunningInDockerDesktop(ctx),
		"Should return false when context has NoDockerDesktop set")
}
