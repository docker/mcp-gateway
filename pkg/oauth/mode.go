package oauth

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// IsCEMode returns true if running in Docker CE mode (standalone OAuth flows).
// When false, uses Docker Desktop for OAuth orchestration.
//
// CE mode is automatically detected on Linux when Docker Desktop is not running.
// Set the environment variable DOCKER_MCP_USE_CE=true to force CE mode.
func IsCEMode() bool {
	// Allow explicit override via environment variable
	if os.Getenv("DOCKER_MCP_USE_CE") == "true" {
		return true
	}

	// When running inside the gateway container, skip Desktop detection
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		return true
	}

	// On Windows and macOS, Docker Desktop is always used
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return false
	}

	// On Linux, check if Docker Desktop is running
	// If not running, we're in CE mode
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := desktop.CheckDesktopIsRunning(ctx); err != nil {
		// Docker Desktop is not running, so we're in CE mode
		return true
	}

	return false
}
