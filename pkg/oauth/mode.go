package oauth

import (
	"context"
	"os"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// IsCEMode returns true if running in Docker CE mode (standalone OAuth flows).
// When false, uses Docker Desktop for OAuth orchestration.
//
// This uses the same logic as the feature flag system (features.IsRunningInDockerDesktop):
// - Container mode → CE mode (skip Desktop)
// - Windows/macOS → assume Docker Desktop (not CE mode)
// - Linux → check if Docker Desktop is running
//
// Set DOCKER_MCP_USE_CE=true to force CE mode.
func IsCEMode() bool {
	// Allow explicit override via environment variable
	if os.Getenv("DOCKER_MCP_USE_CE") == "true" {
		return true
	}

	// Use the same logic as feature flags
	// IsCEMode is the inverse of IsRunningInDockerDesktop
	return !desktop.IsRunningInDockerDesktop(context.Background())
}
